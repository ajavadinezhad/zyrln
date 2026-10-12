package com.zyrln.relay

import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat

import androidx.core.content.edit

import android.animation.AnimatorInflater
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.animation.ArgbEvaluator
import android.animation.ValueAnimator
import androidx.appcompat.app.AlertDialog
import android.content.BroadcastReceiver
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.SharedPreferences
import android.content.res.ColorStateList
import android.graphics.Typeface
import android.net.VpnService
import android.os.Bundle
import android.util.Log
import android.view.Gravity
import android.view.View
import android.view.animation.Animation
import android.view.animation.AnimationUtils
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import org.json.JSONArray
import org.json.JSONException
import org.json.JSONObject
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.appcompat.app.AppCompatDelegate
import androidx.cardview.widget.CardView
import androidx.core.content.ContextCompat
import androidx.core.os.LocaleListCompat
import com.zyrln.relay.databinding.ActivityMainBinding
import mobile.Mobile
import java.util.Locale

class MainActivity : AppCompatActivity() {

    companion object {
        // Survives activity recreation (theme/language change)
        private val logCache = StringBuilder()
        private const val MAX_LOG_LINES = 200
        private const val DEFAULT_DIRECT_ONLY = true
        private const val STATE_USER_REQUESTED_STOP = "user_requested_stop"
    }

    private lateinit var binding: ActivityMainBinding
    private lateinit var prefs: SharedPreferences
    @Volatile private var activeUrl: String? = null
    @Volatile private var activeKey: String? = null
    private var selectedUrl: String? = null
    private var selectedKey: String? = null
    private var directOnlySelected: Boolean = false
    private var userRequestedStop = false
    private var connectionPending = false
    private var relayNoticeResource: Int? = null

    private val localNetworkPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        if (!granted) Toast.makeText(this, R.string.local_network_denied, Toast.LENGTH_LONG).show()
        prepareVpnConnection()
    }

    private fun requestConnectionPermissions() {
        connectionPending = true
        relayNoticeResource = null
        binding.relayNotice.visibility = View.GONE
        refreshList(running = false)
        if (android.os.Build.VERSION.SDK_INT >= 37 &&
            prefs.getBoolean("direct_enabled", true) &&
            ContextCompat.checkSelfPermission(this, android.Manifest.permission.ACCESS_LOCAL_NETWORK) != android.content.pm.PackageManager.PERMISSION_GRANTED) {
            localNetworkPermissionLauncher.launch(android.Manifest.permission.ACCESS_LOCAL_NETWORK)
        } else {
            prepareVpnConnection()
        }
    }

    private fun prepareVpnConnection() {
        val vpnIntent = VpnService.prepare(this)
        if (vpnIntent != null) vpnPermissionLauncher.launch(vpnIntent) else launchRelayService()
    }

    private val vpnPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == RESULT_OK) {
            launchRelayService()
        } else {
            connectionPending = false
            retainActiveProfileSelection()
            activeUrl = null
            activeKey = null
            Toast.makeText(this, R.string.error_vpn_permission, Toast.LENGTH_SHORT).show()
            updateUI(running = false)
        }
    }

    private val startedReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            if (userRequestedStop) return
            connectionPending = false
            userRequestedStop = false
            startUptimeTicker()
            startLogPolling()
            updateUI(running = true)
        }
    }

    private val logPollHandler = Handler(Looper.getMainLooper())
    private val logPollTick = object : Runnable {
        override fun run() {
            val raw = Mobile.pollLogs()
            if (raw.isNotEmpty()) {
                val entries = ArrayList<Pair<String, String>>(8)
                for (line in raw.split('\n')) {
                    val tab = line.indexOf('\t')
                    if (tab < 0) continue
                    val level = line.substring(0, tab)
                    if (level == "debug") continue
                    entries.add(level to line.substring(tab + 1))
                }
                if (entries.isNotEmpty()) appendLogBatch(entries)
            }
            logPollHandler.postDelayed(this, 500)
        }
    }

    private fun startLogPolling() {
        logPollHandler.removeCallbacks(logPollTick)
        logPollHandler.post(logPollTick)
    }

    private fun stopLogPolling() {
        logPollHandler.removeCallbacks(logPollTick)
    }

    private val stopReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            connectionPending = false
            userRequestedStop = false
            retainActiveProfileSelection()
            activeUrl = null
            activeKey = null
            stopLogPolling()
            stopUptimeTicker()
            updateUI(running = false)
        }
    }

    private val errorReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            connectionPending = false
            userRequestedStop = false
            retainActiveProfileSelection()
            activeUrl = null
            activeKey = null
            stopLogPolling()
            stopUptimeTicker()
            updateUI(running = false)
            val message = intent?.getStringExtra(RelayVpnService.EXTRA_ERROR)
                ?: getString(R.string.error_relay_start_generic)
            showRelayNotice(message)
            Toast.makeText(this@MainActivity, message, Toast.LENGTH_LONG).show()
        }
    }

    override fun onSaveInstanceState(outState: Bundle) {
        super.onSaveInstanceState(outState)
        outState.putBoolean(STATE_USER_REQUESTED_STOP, userRequestedStop)
        outState.putBoolean("connection_pending", connectionPending)
        relayNoticeResource?.let { outState.putInt("relay_notice", it) }
        outState.putString("selected_url", selectedUrl)
        outState.putString("selected_key", selectedKey)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        userRequestedStop = savedInstanceState?.getBoolean(STATE_USER_REQUESTED_STOP, false) ?: false
        connectionPending = savedInstanceState?.getBoolean("connection_pending", false) ?: false
        relayNoticeResource = savedInstanceState?.getInt("relay_notice", 0)?.takeIf { it != 0 }
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)
        // Android 15+ enforces edge-to-edge. Earlier versions already fit system bars.
        if (android.os.Build.VERSION.SDK_INT >= 35) {
            val initialPadding = intArrayOf(binding.root.paddingLeft, binding.root.paddingTop, binding.root.paddingRight, binding.root.paddingBottom)
            ViewCompat.setOnApplyWindowInsetsListener(binding.root) { view, insets ->
                val bars = insets.getInsets(WindowInsetsCompat.Type.systemBars() or WindowInsetsCompat.Type.displayCutout())
                view.setPadding(initialPadding[0] + bars.left, initialPadding[1] + bars.top, initialPadding[2] + bars.right, initialPadding[3] + bars.bottom)
                WindowInsetsCompat.CONSUMED
            }
            ViewCompat.requestApplyInsets(binding.root)
        }


        prefs = getSharedPreferences("config", Context.MODE_PRIVATE)
        Mobile.setCacheDir(filesDir.absolutePath)
        binding.versionTag.text = getString(R.string.version_label, BuildConfig.VERSION_NAME)

        binding.btnImportConfig.setOnClickListener { importConfig() }
        binding.btnManualEntry.setOnClickListener { manualEntry() }
        binding.btnSplitTunnel.setOnClickListener { openSplitTunnelSettings() }
        binding.btnLanguage.setOnClickListener { toggleLanguage() }
        binding.btnTheme.setOnClickListener { toggleTheme() }
        binding.btnConnect.setOnClickListener { onConnectClicked() }
        binding.btnDirect.setOnClickListener { toggleDirectMode() }
        binding.btnShareLog.setOnClickListener { shareLog() }
        binding.btnPing.setOnClickListener {
            if (directOnlySelected || (Mobile.isRunning() && activeUrl == null)) {
                binding.pingResult.visibility = View.VISIBLE
                binding.pingResult.text = "…"
                Thread {
                    val result = Mobile.pingDirect()
                    runOnUiThread { binding.pingResult.text = result; showRelayNotice(result) }
                }.start()
                return@setOnClickListener
            }
            val configs = loadConfigs()
            if (configs.isEmpty()) {
                Toast.makeText(this, R.string.empty_configs, Toast.LENGTH_SHORT).show()
                return@setOnClickListener
            }
            val (pingUrl, pingKey) = when {
                selectedUrl != null -> Pair(selectedUrl!!, selectedKey ?: "")
                activeUrl != null -> Pair(activeUrl!!, activeKey ?: "")
                else -> configs[0]
            }
            if (selectedUrl == null && activeUrl == null) {
                selectedUrl = configs[0].first
                selectedKey = configs[0].second
                refreshList()
            }
            binding.pingResult.visibility = View.VISIBLE
            binding.pingResult.text = "…"
            Thread {
                val result = Mobile.ping(pingUrl, pingKey)
                runOnUiThread { binding.pingResult.text = result; showRelayNotice(result) }
            }.start()
        }

        // Press scale animation on primary buttons
        val pressAnim = AnimatorInflater.loadStateListAnimator(this, R.animator.btn_press)
        binding.btnConnect.stateListAnimator = pressAnim
        binding.btnImportConfig.stateListAnimator =
            AnimatorInflater.loadStateListAnimator(this, R.animator.btn_press)

        syncModeFromPrefs()
        if (savedInstanceState != null && !vpnAppearsRunning()) {
            selectedUrl = savedInstanceState.getString("selected_url")
            selectedKey = savedInstanceState.getString("selected_key")
        }
        applyDirectEnabledFromPrefs()
        val initialRunning = vpnAppearsRunning()
        updateUI(running = initialRunning)
        updateSplitTunnelBtn()
        updateLanguageButton()
        updateThemeButton()

        if (!initialRunning && !prefs.getBoolean("direct_only", false)) {
            loadConfigs().firstOrNull()?.let { (u, k) ->
                Thread { Mobile.warmupTunnel(u, k) }.start()
            }
        }

        // Restore log cache after recreation (theme/language change)
        if (logCache.isNotEmpty() && initialRunning) {
            binding.logOutput.text = logCache.toString()
            binding.logScroll.post { binding.logScroll.fullScroll(View.FOCUS_DOWN) }
        }

        // Entrance animation — only animate views that are actually visible
        val panelViews = if (initialRunning)
            listOf<View?>(binding.statusCard, binding.logCard)
        else
            listOf<View?>(binding.statusCard, binding.bottomActions)
        panelViews.forEachIndexed { i, v ->
            v ?: return@forEachIndexed
            val anim = AnimationUtils.loadAnimation(this, R.anim.card_enter)
            anim.startOffset = i * 60L
            v.startAnimation(anim)
        }

        val savedTheme = prefs.getInt("theme_mode", AppCompatDelegate.MODE_NIGHT_FOLLOW_SYSTEM)
        if (AppCompatDelegate.getDefaultNightMode() != savedTheme) {
            AppCompatDelegate.setDefaultNightMode(savedTheme)
        }
    }

    override fun onResume() {
        super.onResume()
        androidx.core.content.ContextCompat.registerReceiver(this, startedReceiver, IntentFilter(RelayVpnService.ACTION_STARTED), androidx.core.content.ContextCompat.RECEIVER_NOT_EXPORTED)
        androidx.core.content.ContextCompat.registerReceiver(this, stopReceiver, IntentFilter(RelayVpnService.ACTION_STOPPED), androidx.core.content.ContextCompat.RECEIVER_NOT_EXPORTED)
        androidx.core.content.ContextCompat.registerReceiver(this, errorReceiver, IntentFilter(RelayVpnService.ACTION_ERROR), androidx.core.content.ContextCompat.RECEIVER_NOT_EXPORTED)
        if (RelayVpnService.starting) connectionPending = true
        else if (Mobile.isRunning() || VpnService.prepare(this) == null) connectionPending = false
        if (!RelayVpnService.transitioning && !Mobile.isRunning()) userRequestedStop = false
        syncModeFromPrefs()
        applyDirectEnabledFromPrefs()
        val running = vpnAppearsRunning()
        if (running) resumeUptimeTicker() else stopUptimeTicker()
        if (running) startLogPolling() else stopLogPolling()
        updateUI(running = running)
        updateSplitTunnelBtn()
    }

    override fun onPause() {
        super.onPause()
        unregisterReceiver(startedReceiver)
        unregisterReceiver(stopReceiver)
        unregisterReceiver(errorReceiver)
        stopLogPolling()
        uptimeHandler.removeCallbacks(uptimeTick)
    }

    override fun onDestroy() {
        colorAnimator?.cancel()
        stopLogPolling()
        uptimeHandler.removeCallbacks(uptimeTick)
        super.onDestroy()
    }

    private fun onConnectClicked() {
        if (connectionPending || userRequestedStop) return
        if (vpnAppearsRunning()) {
            playMotion(binding.btnConnect, R.anim.motion_soft)
            stopVpn()
            return
        }
        if (userRequestedStop && Mobile.isRunning()) return
        when {
            directOnlySelected -> connectDirect()
            selectedUrl != null -> connectConfig(selectedUrl!!, selectedKey ?: "")
            else -> { /* no-op */ }
        }
    }

    private var connectBtnCurrentColor: Int = 0
    private var colorAnimator: ValueAnimator? = null
    private var uptimeStart: Long = 0L
    private val uptimeHandler = Handler(Looper.getMainLooper())
    private val uptimeTick = object : Runnable {
        override fun run() {
            val elapsed = SystemClock.elapsedRealtime() - uptimeStart
            val h = elapsed / 3_600_000
            val m = (elapsed % 3_600_000) / 60_000
            val s = (elapsed % 60_000) / 1_000
            binding.uptimeValue.text = getString(R.string.uptime_format, h, m, s)
            uptimeHandler.postDelayed(this, 1_000)
        }
    }

    private fun startUptimeTicker() {
        uptimeStart = SystemClock.elapsedRealtime()
        prefs.edit { putLong("uptime_start_wall", System.currentTimeMillis()) }
        uptimeHandler.removeCallbacks(uptimeTick)
        uptimeHandler.post(uptimeTick)
    }

    private fun resumeUptimeTicker() {
        val wallStart = prefs.getLong("uptime_start_wall", 0L)
        if (wallStart == 0L) {
            startUptimeTicker()
            return
        }
        val elapsedSinceStart = System.currentTimeMillis() - wallStart
        uptimeStart = SystemClock.elapsedRealtime() - elapsedSinceStart
        uptimeHandler.removeCallbacks(uptimeTick)
        uptimeHandler.post(uptimeTick)
    }

    private fun stopUptimeTicker() {
        uptimeHandler.removeCallbacks(uptimeTick)
        prefs.edit { remove("uptime_start_wall") }
        binding.uptimeValue.setText(R.string.uptime_zero)
    }

    private fun updateUI(running: Boolean) {
        runOnUiThread {
            binding.statusText.setText(if (running) R.string.status_running else R.string.status_stopped)
            binding.btnConnect.setIconResource(if (running) R.drawable.ic_pause else R.drawable.ic_connect)
            binding.btnConnect.setText(if (running) R.string.btn_disconnect else R.string.connect_action)
            binding.btnConnect.backgroundTintList = ContextCompat.getColorStateList(this,
                if (running) R.color.accent_error else R.color.accent_success)

            val targetColor = ContextCompat.getColor(
                this, android.R.color.white
            )
            if (connectBtnCurrentColor == 0) {
                connectBtnCurrentColor = targetColor
                binding.btnConnect.iconTint = ColorStateList.valueOf(targetColor)
            } else if (connectBtnCurrentColor != targetColor) {
                colorAnimator?.cancel()
                colorAnimator = ValueAnimator.ofObject(ArgbEvaluator(), connectBtnCurrentColor, targetColor).apply {
                    duration = 220
                    addUpdateListener { animator ->
                        val color = animator.animatedValue as Int
                        binding.btnConnect.iconTint = ColorStateList.valueOf(color)
                    }
                    start()
                }
                connectBtnCurrentColor = targetColor
            }

            binding.btnImportConfig.isEnabled = !running
            binding.btnImportConfig.alpha = if (running) 0.5f else 1f
            binding.logCard.visibility = if (running) View.VISIBLE else View.GONE
            binding.bottomActions.visibility = if (running) View.GONE else View.VISIBLE
            binding.profileActions.visibility = if (running) View.GONE else View.VISIBLE
            if (!running) { binding.logOutput.text = ""; logCache.clear() }
            refreshList(running)
            relayNoticeResource?.let {
                binding.relayNotice.setText(it)
                binding.relayNotice.visibility = View.VISIBLE
            }
            updateDirectBtn()
            updateSplitTunnelBtn()
        }
    }

    private fun vpnAppearsRunning(): Boolean = Mobile.isRunning() && !userRequestedStop

    private fun openSplitTunnelSettings() {
        if (connectionPending || userRequestedStop || vpnAppearsRunning()) {
            Toast.makeText(this, R.string.split_tunnel_running_hint, Toast.LENGTH_SHORT).show()
            return
        }
        startActivity(Intent(this, SplitTunnelAppsActivity::class.java))
    }

    private fun updateSplitTunnelBtn() {
        val cfg = SplitTunnelPrefs.loadResolved(prefs, packageManager)
        val count = cfg.packages.size
        val summary = when (cfg.mode) {
            SplitTunnelPrefs.MODE_BYPASS -> resources.getQuantityString(R.plurals.split_tunnel_summary_bypass, count, count)
            SplitTunnelPrefs.MODE_ONLY -> resources.getQuantityString(R.plurals.split_tunnel_summary_only, count, count)
            else -> getString(R.string.split_tunnel_summary_off)
        }
        binding.btnSplitTunnel.text = if (SplitTunnelPrefs.isActive(cfg)) {
            getString(R.string.split_tunnel_button_summary, getString(R.string.btn_split_tunnel), summary)
        } else {
            getString(R.string.btn_split_tunnel)
        }
        val running = vpnAppearsRunning()
        binding.btnSplitTunnel.isEnabled = !running && !connectionPending && !userRequestedStop
        binding.btnSplitTunnel.alpha = if (binding.btnSplitTunnel.isEnabled) 1f else 0.5f
    }

    private fun toggleLanguage() {
        val currentLocale = AppCompatDelegate.getApplicationLocales()[0]?.language ?: Locale.getDefault().language
        val newLocale = if (currentLocale == "fa") "en" else "fa"
        AppCompatDelegate.setApplicationLocales(LocaleListCompat.forLanguageTags(newLocale))
    }

    private fun toggleTheme() {
        val isNight = (resources.configuration.uiMode and android.content.res.Configuration.UI_MODE_NIGHT_MASK) ==
                android.content.res.Configuration.UI_MODE_NIGHT_YES
        val nextMode = if (isNight) AppCompatDelegate.MODE_NIGHT_NO else AppCompatDelegate.MODE_NIGHT_YES
        AppCompatDelegate.setDefaultNightMode(nextMode)
        prefs.edit { putInt("theme_mode", nextMode) }
        updateThemeButton()
    }

    private fun updateLanguageButton() {
        val currentLocale = AppCompatDelegate.getApplicationLocales()[0]?.language ?: Locale.getDefault().language
        binding.btnLanguage.setText(if (currentLocale == "fa") R.string.language_en else R.string.language_fa)
    }

    private fun updateThemeButton() {
        val isNight = (resources.configuration.uiMode and android.content.res.Configuration.UI_MODE_NIGHT_MASK) ==
                android.content.res.Configuration.UI_MODE_NIGHT_YES
        binding.btnTheme.setImageResource(if (isNight) R.drawable.ic_sun else R.drawable.ic_moon)
        binding.btnTheme.imageTintList = ContextCompat.getColorStateList(this, R.color.icon)
    }

    private fun refreshList(running: Boolean = vpnAppearsRunning()) {
        val configs = loadConfigs()
        binding.configList.removeAllViews()
        binding.profileHeading.text = getString(R.string.saved_profiles_heading, configs.size)
        binding.profileHeading.visibility = if (running) View.GONE else View.VISIBLE

        // Connect is always enabled — direct mode works without any config
        binding.btnConnect.visibility = View.VISIBLE
        binding.btnConnect.isEnabled = !connectionPending && !userRequestedStop
        binding.btnConnect.alpha = if (binding.btnConnect.isEnabled) 1f else 0.5f
        if (connectionPending || userRequestedStop) {
            val label = if (userRequestedStop) R.string.status_disconnecting else R.string.status_connecting
            binding.btnConnect.setText(label)
            binding.statusText.setText(label)
        }
        binding.configScroll.isEnabled = !connectionPending && !userRequestedStop
        binding.btnImportConfig.isEnabled = !running && !connectionPending && !userRequestedStop
        binding.btnManualEntry.isEnabled = !running && !connectionPending && !userRequestedStop

        if (running) {
            binding.configScroll.visibility = View.GONE
            binding.emptyState.visibility = View.GONE
            return
        }

        binding.configScroll.visibility = View.VISIBLE
        binding.emptyState.visibility = View.GONE

        val dp = resources.displayMetrics.density

        // Auto-select direct card if nothing else is selected
        if (!directOnlySelected && selectedUrl == null && activeUrl == null) {
            directOnlySelected = true
        }

        // Direct (Google only) — no relay config required
        run {
            val isSelected = directOnlySelected
            val card = CardView(this).apply {
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { bottomMargin = (10 * dp).toInt() }
                radius = 14 * dp
                cardElevation = 0f
                setCardBackgroundColor(ContextCompat.getColor(this@MainActivity, R.color.card_bg))
            }
            val row = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                layoutDirection = View.LAYOUT_DIRECTION_LTR
                minimumHeight = (48 * dp).toInt()
                setPadding((12 * dp).toInt(), (8 * dp).toInt(), (8 * dp).toInt(), (8 * dp).toInt())
                background = ContextCompat.getDrawable(this@MainActivity,
                    if (isSelected) R.drawable.bg_card_selected else R.drawable.bg_card)
            }
            val label = TextView(this).apply {
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
                text = getString(R.string.direct_profile_label, getString(R.string.direct_only_label))
                textSize = 15f
                setTextColor(ContextCompat.getColor(this@MainActivity, R.color.title))
                if (isSelected) setTypeface(null, android.graphics.Typeface.BOLD)
            }
            row.addView(label)
            card.addView(row)
            row.setOnClickListener {
                if (connectionPending || userRequestedStop) return@setOnClickListener
                directOnlySelected = true
                selectedUrl = null
                selectedKey = null
                refreshList(running = false)
                updateDirectBtn()
            }
            binding.configList.addView(card)
        }

        val iconButtonSize = (48 * dp).toInt()
        val iconButtonPadding = (12 * dp).toInt()
        val hostnames = configs.map { (url, _) -> configLabel(url) }

        configs.forEach { (url, key) ->
            val isActive = url == activeUrl && key == activeKey
            val isSelected = !isActive && url == selectedUrl && key == selectedKey

            val card = CardView(this).apply {
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { bottomMargin = (10 * dp).toInt() }
                radius = 14 * dp
                cardElevation = 0f
                setCardBackgroundColor(ContextCompat.getColor(this@MainActivity, R.color.card_bg))
            }

            val rowBg = when {
                isActive -> R.drawable.bg_card_active
                isSelected -> R.drawable.bg_card_selected
                else -> R.drawable.bg_card
            }
            val row = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                layoutDirection = View.LAYOUT_DIRECTION_LTR
                minimumHeight = (48 * dp).toInt()
                setPadding((12 * dp).toInt(), (8 * dp).toInt(), (8 * dp).toInt(), (8 * dp).toInt())
                background = ContextCompat.getDrawable(this@MainActivity, rowBg)
            }

            val baseLabel = configLabel(url)
            val displayLabel = if (hostnames.count { it == baseLabel } > 1)
                "$baseLabel …${key.takeLast(4)}" else baseLabel

            val label = TextView(this).apply {
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
                text = displayLabel
                textSize = 15f
                maxLines = 1
                ellipsize = android.text.TextUtils.TruncateAt.END
                textAlignment = View.TEXT_ALIGNMENT_VIEW_START
                textDirection = View.TEXT_DIRECTION_LTR
                setTextColor(ContextCompat.getColor(this@MainActivity, R.color.title))
                if (isActive || isSelected) setTypeface(null, Typeface.BOLD)
            }

            val urlList = url.split(",").map { it.trim() }.filter { it.isNotEmpty() }
            val infoBtn = android.widget.ImageButton(this).apply {
                visibility = if (urlList.size > 1) View.VISIBLE else View.GONE
                contentDescription = getString(R.string.profile_details)
                setImageDrawable(ContextCompat.getDrawable(this@MainActivity, R.drawable.ic_info))
                background = null
                imageTintList = ContextCompat.getColorStateList(this@MainActivity, R.color.text_dim)
                scaleType = android.widget.ImageView.ScaleType.CENTER
                setPadding(iconButtonPadding, iconButtonPadding, iconButtonPadding, iconButtonPadding)
                layoutParams = LinearLayout.LayoutParams(iconButtonSize, iconButtonSize)
                    .apply { marginEnd = (4 * dp).toInt() }
            }
            infoBtn.setOnClickListener {
                val lines = urlList.mapIndexed { i, u ->
                    val id = u.substringAfter("/macros/s/", "").substringBefore("/")
                    val short = if (id.length >= 6) "…${id.takeLast(10)}" else u.substringAfter("://").substringBefore("/")
                    "${i + 1}. $short"
                }.joinToString("\n")
                AlertDialog.Builder(this@MainActivity, R.style.Dialog_Zyrln)
                    .setTitle(R.string.profile_details)
                    .setMessage(lines)
                    .setPositiveButton(R.string.btn_ok, null)
                    .show()
            }

            val exportBtn = android.widget.ImageButton(this).apply {
                layoutParams = LinearLayout.LayoutParams(iconButtonSize, iconButtonSize)
                    .apply { marginStart = (4 * dp).toInt() }
                setImageDrawable(ContextCompat.getDrawable(this@MainActivity, R.drawable.ic_share))
                background = null
                imageTintList = ContextCompat.getColorStateList(this@MainActivity, R.color.text_dim)
                scaleType = android.widget.ImageView.ScaleType.CENTER
                setPadding(iconButtonPadding, iconButtonPadding, iconButtonPadding, iconButtonPadding)
                contentDescription = getString(R.string.btn_export_config)
            }
            exportBtn.setOnClickListener {
                copyConfigToClipboard(url, key)
                playMotion(exportBtn, R.anim.motion_confirm)
            }

            val deleteBtn = android.widget.ImageButton(this).apply {
                layoutParams = LinearLayout.LayoutParams(iconButtonSize, iconButtonSize)
                    .apply { marginStart = (6 * dp).toInt() }
                contentDescription = getString(R.string.btn_remove)
                setImageDrawable(ContextCompat.getDrawable(this@MainActivity, R.drawable.ic_delete))
                background = null
                imageTintList = ContextCompat.getColorStateList(this@MainActivity, R.color.text_dim)
                scaleType = android.widget.ImageView.ScaleType.CENTER
                setPadding(iconButtonPadding, iconButtonPadding, iconButtonPadding, iconButtonPadding)
            }

            row.addView(label)
            row.addView(infoBtn)
            row.addView(exportBtn)
            row.addView(deleteBtn)
            card.addView(row)

            card.setOnClickListener {
                if (connectionPending || userRequestedStop) return@setOnClickListener
                if (isActive && running) {
                    stopVpn()
                } else if (!running) {
                    // Select this row; deselect if already selected
                    if (isSelected) {
                        selectedUrl = null
                        selectedKey = null
                    } else {
                        selectedUrl = url
                        selectedKey = key
                        relayNoticeResource = null
                        directOnlySelected = false
                        prefs.edit { putBoolean("direct_only", false) }
                        Thread { Mobile.warmupTunnel(url, key) }.start()

        }
                    refreshList(running = false)
                    updateDirectBtn()
                }
            }

            deleteBtn.isEnabled = !connectionPending && !userRequestedStop
            deleteBtn.setOnClickListener {
                if (connectionPending || userRequestedStop) return@setOnClickListener
                AlertDialog.Builder(this, R.style.Dialog_Zyrln_Destructive)
                    .setTitle(R.string.dialog_remove_title)
                    .setMessage(if (isActive && running)
                        getString(R.string.dialog_remove_active, displayLabel)
                    else
                        getString(R.string.dialog_remove_inactive, displayLabel))
                    .setPositiveButton(R.string.btn_remove) { _, _ ->
                        val deleteAnim = AnimationUtils.loadAnimation(this, R.anim.motion_delete)
                        deleteAnim.setAnimationListener(object : Animation.AnimationListener {
                            override fun onAnimationStart(a: Animation?) {}
                            override fun onAnimationRepeat(a: Animation?) {}
                            override fun onAnimationEnd(a: Animation?) {
                                if (isActive && running) stopVpn()
                                deleteConfig(url, key)
                                if (selectedUrl == url && selectedKey == key) {
                                    selectedUrl = null
                                    selectedKey = null
                                }
                                refreshList(running = vpnAppearsRunning())
                            }
                        })
                        card.startAnimation(deleteAnim)
                    }
                    .setNegativeButton(R.string.btn_cancel, null)
                    .show()
            }

            binding.configList.addView(card)

            // Staggered entrance: each card slides up with a small delay
            val enterAnim = AnimationUtils.loadAnimation(this, R.anim.card_enter)
            val configIndex = configs.indexOf(Pair(url, key))
            enterAnim.startOffset = (configIndex * 40L).coerceAtMost(120L)
            card.startAnimation(enterAnim)
        }
        updateDirectBtn()
    }

    private fun connectConfig(url: String, key: String) {
        Thread { Mobile.warmupTunnel(url, key) }.start()
        directOnlySelected = false
        prefs.edit {
            putBoolean("direct_only", false)
            putString("url", url)
            putString("key", key)
        }
        activeUrl = url
        activeKey = key
        selectedUrl = null
        selectedKey = null
        refreshList(running = false)
        requestConnectionPermissions()
    }

    private fun connectDirect() {
        Mobile.setDirectEnabled(true)
        directOnlySelected = true
        activeUrl = null
        activeKey = null
        prefs.edit {
            putString("url", "")
            putString("key", "")
            putBoolean("direct_only", true)
            putBoolean("direct_enabled", true)
        }
        updateDirectBtn()
        requestConnectionPermissions()
    }

    private fun appendLogBatch(entries: List<Pair<String, String>>) {
        if (entries.isEmpty()) return
        runOnUiThread {
            val scroll = binding.logScroll
            val atBottom = !scroll.canScrollVertically(1)
            val timeFmt = java.text.SimpleDateFormat("HH:mm:ss", java.util.Locale.US)
            for ((level, msg) in entries) {
                if (level == "error") showRelayNotice(msg)
                val time = timeFmt.format(java.util.Date())
                logCache.append('[').append(time).append("] ").append(msg).append('\n')
            }
            trimLogCache()
            binding.logOutput.text = logCache.toString()
            if (atBottom) scroll.post { scroll.fullScroll(View.FOCUS_DOWN) }
        }
    }

    private fun showRelayNotice(message: String) {
        RelayNotice.messageResource(message)?.let { notice ->
            // Keep confirmed exhaustion visible after later generic bridge failures.
            if (relayNoticeResource != R.string.relay_quota_notice) relayNoticeResource = notice
            binding.relayNotice.setText(relayNoticeResource!!)
            binding.relayNotice.setTextColor(ContextCompat.getColor(this, R.color.accent_error))
            binding.relayNotice.visibility = View.VISIBLE
        }
    }

    private fun trimLogCache() {
        var newlines = 0
        for (i in logCache.indices) if (logCache[i] == '\n') newlines++
        if (newlines <= MAX_LOG_LINES) return
        var toRemove = newlines - MAX_LOG_LINES
        var pos = 0
        while (toRemove > 0 && pos < logCache.length) {
            if (logCache[pos] == '\n') toRemove--
            pos++
        }
        logCache.delete(0, pos)
    }

    private fun applyDirectEnabledFromPrefs() {
        val hasRelay = !prefs.getString("url", null).isNullOrEmpty() &&
            !prefs.getBoolean("direct_only", DEFAULT_DIRECT_ONLY)
        if (hasRelay && !prefs.getBoolean("direct_opt_out", false)) {
            // Builds before v1.6 defaulted relay profiles to direct OFF; turn Google bypass on.
            if (!prefs.getBoolean("direct_enabled", true)) {
                prefs.edit { putBoolean("direct_enabled", true) }

        }
        }
        Mobile.setDirectEnabled(prefs.getBoolean("direct_enabled", true))
    }

    private fun launchRelayService() {
        connectionPending = true
        userRequestedStop = false
        applyDirectEnabledFromPrefs()
        val url = prefs.getString("url", "") ?: ""
        val key = prefs.getString("key", "") ?: ""
        playMotion(binding.btnConnect, R.anim.motion_confirm)
        startUptimeTicker()
        startLogPolling()
        updateUI(running = true)
        ContextCompat.startForegroundService(this, Intent(this, RelayVpnService::class.java).apply {
            action = RelayVpnService.ACTION_START
            putExtra(RelayVpnService.EXTRA_URL, url)
            putExtra(RelayVpnService.EXTRA_KEY, key)
        })
    }

    private fun retainActiveProfileSelection() {
        activeUrl?.let { url ->
            selectedUrl = url
            selectedKey = activeKey
            directOnlySelected = false
        }
    }

    private fun stopVpn() {
        if (userRequestedStop) return
        connectionPending = false
        retainActiveProfileSelection()
        userRequestedStop = true
        startService(Intent(this, RelayVpnService::class.java).apply { action = RelayVpnService.ACTION_STOP })
        activeUrl = null
        activeKey = null
        stopLogPolling()
        stopUptimeTicker()
        updateUI(running = false)
    }

    private fun importConfig() {
        val clipboard = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        val rawText = clipboard.primaryClip?.getItemAt(0)?.getText()?.toString()?.trim()
        if (rawText.isNullOrEmpty()) {
            Toast.makeText(this, R.string.msg_clipboard_empty, Toast.LENGTH_SHORT).show()
            return
        }
        try {
            val (url, key) = ConfigUtils.parseImportText(rawText)
            if (saveConfig(url, key)) {
                revealSavedConfig(url, key)
                Thread { Mobile.warmupTunnel(url, key) }.start()
                playMotion(binding.btnImportConfig, R.anim.motion_confirm)
                Toast.makeText(this, R.string.msg_config_saved_connect, Toast.LENGTH_SHORT).show()
            } else {
                Toast.makeText(this, R.string.msg_already_exists, Toast.LENGTH_SHORT).show()
            }
        } catch (e: Exception) {
            Log.e("MainActivity", "Import failed: ${e.message}")
            Toast.makeText(this, getString(R.string.msg_invalid_config), Toast.LENGTH_LONG).show()
        }
    }

    private fun manualEntry() {
        val dp = resources.displayMetrics.density
        val sidePadding = (24 * dp).toInt()

        val urlInput = android.widget.EditText(this).apply {
            hint = getString(R.string.hint_manual_url)
            inputType = android.text.InputType.TYPE_CLASS_TEXT or android.text.InputType.TYPE_TEXT_VARIATION_URI
            maxLines = 1
            setTextColor(ContextCompat.getColor(this@MainActivity, R.color.title))
            setHintTextColor(ContextCompat.getColor(this@MainActivity, R.color.text_dim))
        }
        val keyInput = android.widget.EditText(this).apply {
            hint = getString(R.string.hint_manual_key)
            maxLines = 1
            setTextColor(ContextCompat.getColor(this@MainActivity, R.color.title))
            setHintTextColor(ContextCompat.getColor(this@MainActivity, R.color.text_dim))
        }
        urlInput.hint = null
        keyInput.hint = null
        urlInput.background = null
        keyInput.background = null
        val urlLayout = com.google.android.material.textfield.TextInputLayout(this).apply {
            hint = getString(R.string.hint_manual_url)
            isErrorEnabled = true
            addView(urlInput)
        }
        val keyLayout = com.google.android.material.textfield.TextInputLayout(this).apply {
            hint = getString(R.string.hint_manual_key)
            isErrorEnabled = true
            addView(keyInput)
        }
        val container = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(sidePadding, (8 * dp).toInt(), sidePadding, 0)
            addView(TextView(this@MainActivity).apply {
                setText(R.string.manual_setup_hint)
                setTextColor(ContextCompat.getColor(this@MainActivity, R.color.subtitle))
                textSize = 14f
                setPadding(0, 0, 0, (12 * dp).toInt())
            })
            addView(urlLayout)
            addView(keyLayout, LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT, LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply { topMargin = (12 * dp).toInt() })
        }

        val dialog = AlertDialog.Builder(this, R.style.Dialog_Zyrln)
            .setTitle(R.string.dialog_manual_entry_title)
            .setView(container)
            .setPositiveButton(R.string.save_profile, null)
            .setNegativeButton(R.string.btn_cancel, null)
            .create()
        dialog.setOnShowListener {
            dialog.getButton(AlertDialog.BUTTON_POSITIVE).setOnClickListener {
                val url = urlInput.text.toString().trim()
                val key = keyInput.text.toString().trim()
                urlLayout.error = if (url.isEmpty()) getString(R.string.required_url) else null
                keyLayout.error = if (key.isEmpty()) getString(R.string.required_key) else null
                if (url.isEmpty() || key.isEmpty()) {
                    if (url.isEmpty()) urlInput.requestFocus() else keyInput.requestFocus()
                    return@setOnClickListener
                }
                if (saveConfig(url, key)) {
                    revealSavedConfig(url, key)
                    Thread { Mobile.warmupTunnel(url, key) }.start()
                    Toast.makeText(this, R.string.msg_config_saved_connect, Toast.LENGTH_SHORT).show()
                } else {
                    Toast.makeText(this, R.string.msg_already_exists, Toast.LENGTH_SHORT).show()
                }
                dialog.window?.let { androidx.core.view.WindowCompat.getInsetsController(it, urlInput).hide(WindowInsetsCompat.Type.ime()) }
                dialog.dismiss()
            }
        }
        dialog.show()
    }

    private fun revealSavedConfig(url: String, key: String) {
        selectedUrl = url
        selectedKey = key
        relayNoticeResource = null
        directOnlySelected = false
        prefs.edit { putBoolean("direct_only", false) }
        refreshList()
        updateDirectBtn()
        // Bring the newly added entry into view even when many profiles are saved.
        binding.configScroll.post {
            val last = binding.configList.getChildAt(binding.configList.childCount - 1)
            if (last != null) binding.configScroll.scrollTo(0, last.top)
        }
    }

    private fun copyConfigToClipboard(url: String, key: String) {
        if (url.isBlank() || key.isBlank()) return
        val json = ConfigUtils.toExportJson(url, key)
        val clipboard = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        val clip = android.content.ClipData.newPlainText("Zyrln config", json)
        if (android.os.Build.VERSION.SDK_INT >= 33) {
            clip.description.extras = android.os.PersistableBundle().apply {
                putBoolean(android.content.ClipDescription.EXTRA_IS_SENSITIVE, true)
            }
        }
        clipboard.setPrimaryClip(clip)
        Toast.makeText(this, R.string.msg_export_copied, Toast.LENGTH_SHORT).show()
    }

    private fun loadConfigs(): List<Pair<String, String>> {
        val raw = prefs.getString("configs", "[]") ?: "[]"
        return try {
            val arr = JSONArray(raw)
            (0 until arr.length()).map { i ->
                val o = arr.getJSONObject(i)
                Pair(o.getString("url"), o.getString("key"))
            }
        } catch (e: JSONException) { emptyList() }
    }

    private fun saveConfig(url: String, key: String): Boolean {
        val existing = loadConfigs()
        if (existing.any { it.first == url && it.second == key }) return false
        val arr = JSONArray()
        existing.forEach { (u, k) -> arr.put(JSONObject().put("url", u).put("key", k)) }
        arr.put(JSONObject().put("url", url).put("key", key))
        prefs.edit { putString("configs", arr.toString()) }
        return true
    }

    private fun deleteConfig(url: String, key: String) {
        val arr = JSONArray()
        loadConfigs().filter { it.first != url || it.second != key }
            .forEach { (u, k) -> arr.put(JSONObject().put("url", u).put("key", k)) }
        prefs.edit { putString("configs", arr.toString()) }
    }

    private fun playMotion(v: View?, animRes: Int) {
        v ?: return
        val anim = AnimationUtils.loadAnimation(this, animRes)
        v.startAnimation(anim)
    }

    private fun configLabel(url: String) = ConfigUtils.configLabel(url)

    private fun shareLog() {
        // GetAllLogs includes debug-level entries (race probe failures etc.) that
        // are filtered from the display — important for diagnosing connectivity issues.
        val raw = Mobile.getAllLogs().trim()
        if (raw.isEmpty()) {
            Toast.makeText(this, R.string.log_empty, Toast.LENGTH_SHORT).show()
            return
        }
        val lines = raw.split('\n').joinToString("\n") { line ->
            val tab = line.indexOf('\t')
            if (tab >= 0) "[${line.substring(0, tab)}] ${line.substring(tab + 1)}" else line
        }
        val header = buildString {
            appendLine("Zyrln Android v${BuildConfig.VERSION_NAME} (${BuildConfig.VERSION_CODE})")
            appendLine("Device: ${android.os.Build.MANUFACTURER} ${android.os.Build.MODEL}")
            appendLine("Android: ${android.os.Build.VERSION.RELEASE} (SDK ${android.os.Build.VERSION.SDK_INT})")
            appendLine("Time: ${java.text.SimpleDateFormat("yyyy-MM-dd HH:mm:ss z", java.util.Locale.US).format(java.util.Date())}")
            appendLine("---")
        }
        val intent = Intent(Intent.ACTION_SEND).apply {
            type = "text/plain"
            putExtra(Intent.EXTRA_SUBJECT, getString(R.string.log_share_subject))
            putExtra(Intent.EXTRA_TEXT, header + lines)
        }
        startActivity(Intent.createChooser(intent, getString(R.string.log_share_subject)))
    }

    private fun syncModeFromPrefs() {
        val savedUrl = prefs.getString("url", null)?.takeIf { it.isNotEmpty() }
        if (vpnAppearsRunning() || connectionPending) {
            activeUrl = savedUrl
            activeKey = if (savedUrl != null) prefs.getString("key", null) else null
        } else {
            activeUrl = null
            activeKey = null
            if (selectedUrl == null && savedUrl != null &&
                loadConfigs().any { it.first == savedUrl && it.second == prefs.getString("key", null) }) {
                selectedUrl = savedUrl
                selectedKey = prefs.getString("key", null)
            }
        }
        directOnlySelected = savedUrl == null && prefs.getBoolean("direct_only", DEFAULT_DIRECT_ONLY)
    }

    private fun toggleDirectMode() {
        // ⚡ toggles Google direct bypass for relay/tunnel profiles — not the "Direct only" card.
        if (connectionPending || userRequestedStop || vpnAppearsRunning() || directOnlySelected) return
        if (selectedUrl == null && activeUrl == null) return
        val next = !Mobile.isDirectEnabled()
        Mobile.setDirectEnabled(next)
        prefs.edit {
            putBoolean("direct_enabled", next)
            putBoolean("direct_opt_out", !next)
        }
        updateDirectBtn()
    }

    private fun updateDirectBtn() {
        val hasRelay = selectedUrl != null || activeUrl != null
        binding.relayNotice.visibility = if (hasRelay && !directOnlySelected) View.VISIBLE else View.GONE
        binding.relayNotice.setText(relayNoticeResource ?: R.string.relay_quota_unknown)
        binding.relayNotice.setTextColor(ContextCompat.getColor(this,
            if (relayNoticeResource == null) R.color.subtitle else R.color.accent_error))
        // Show ⚡ when a relay profile is selected, or direct-only card (status only — disabled).
        val relayContext = selectedUrl != null || activeUrl != null
        val showDirectToggle = directOnlySelected || relayContext
        binding.btnDirect.visibility = if (showDirectToggle) View.VISIBLE else View.GONE
        binding.modeHint.setText(when {
            directOnlySelected && vpnAppearsRunning() -> R.string.mode_direct_connected
            directOnlySelected -> R.string.mode_direct_hint
            vpnAppearsRunning() -> R.string.mode_relay_connected
            else -> R.string.mode_relay_hint
        })
        if (!showDirectToggle) return
        val on = Mobile.isDirectEnabled() || directOnlySelected
        val color = ContextCompat.getColor(
            this,
            if (on) R.color.accent_success else R.color.text_dim
        )
        binding.btnDirect.imageTintList = ColorStateList.valueOf(color)
        binding.btnDirect.contentDescription = getString(if (on) R.string.google_direct_on else R.string.google_direct_off)
        binding.btnDirect.tooltipText = binding.btnDirect.contentDescription
        binding.btnDirect.alpha = 1f
        binding.btnDirect.isEnabled = !connectionPending && !userRequestedStop && !directOnlySelected && !vpnAppearsRunning()
    }
}
