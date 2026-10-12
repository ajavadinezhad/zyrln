package com.zyrln.relay

import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat

import android.content.Context
import android.content.Intent
import android.content.SharedPreferences
import android.content.pm.PackageManager
import android.os.Build
import android.graphics.Typeface
import android.graphics.drawable.Drawable
import android.os.Bundle
import android.text.Editable
import android.text.TextWatcher
import android.view.Gravity
import android.view.View
import android.widget.CheckBox
import android.widget.EditText
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.cardview.widget.CardView
import androidx.core.content.ContextCompat
import com.zyrln.relay.databinding.ActivitySplitTunnelBinding
import mobile.Mobile

class SplitTunnelAppsActivity : AppCompatActivity() {

    private lateinit var binding: ActivitySplitTunnelBinding
    private lateinit var prefs: SharedPreferences

    private var mode = SplitTunnelPrefs.MODE_OFF
    private val selected = linkedSetOf<String>()
    private var allApps = emptyList<AppEntry>()
    private var filterQuery = ""

    private data class AppEntry(
        val packageName: String,
        val label: String,
        val icon: Drawable,
    )

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        if (Mobile.isRunning()) {
            Toast.makeText(this, R.string.split_tunnel_running_hint, Toast.LENGTH_SHORT).show()
            finish()
            return
        }
        binding = ActivitySplitTunnelBinding.inflate(layoutInflater)
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
        mode = SplitTunnelPrefs.normalizeMode(SplitTunnelPrefs.mode(prefs), SplitTunnelPrefs.packages(prefs))
        selected.addAll(SplitTunnelPrefs.packages(prefs))

        binding.btnBack.setOnClickListener { finish() }
        binding.searchApps.addTextChangedListener(object : TextWatcher {
            override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) {}
            override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) {
                filterQuery = s?.toString()?.trim()?.lowercase().orEmpty()
                refreshAppList()
            }
            override fun afterTextChanged(s: Editable?) {}
        })

        allApps = loadLaunchableApps()
        selected.retainAll { pkg -> allApps.any { it.packageName == pkg } }
        refreshModeCards()
        refreshAppList()
        updateNote()
    }

    override fun onPause() {
        super.onPause()
        SplitTunnelPrefs.save(prefs, mode, selected)
    }

    override fun onResume() {
        super.onResume()
        if (Mobile.isRunning()) {
            Toast.makeText(this, R.string.split_tunnel_running_hint, Toast.LENGTH_SHORT).show()
            finish()
        }
    }

    private fun loadLaunchableApps(): List<AppEntry> {
        val pm = packageManager
        val launchIntent = Intent(Intent.ACTION_MAIN).addCategory(Intent.CATEGORY_LAUNCHER)
        val activities = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            pm.queryIntentActivities(
                launchIntent,
                PackageManager.ResolveInfoFlags.of(0),
            )
        } else {
            @Suppress("DEPRECATION")
            pm.queryIntentActivities(launchIntent, 0)
        }

        // One row per package — same apps the user sees in the launcher (YouTube, Gmail, …).
        val seen = LinkedHashMap<String, AppEntry>()
        for (ri in activities) {
            val pkg = ri.activityInfo.packageName
            if (pkg == packageName || pkg in seen) continue
            val label = ri.loadLabel(pm).toString().trim()
            if (label.isEmpty()) continue
            seen[pkg] = AppEntry(pkg, label, ri.loadIcon(pm))
        }
        return seen.values.sortedBy { it.label.lowercase() }
    }

    private fun refreshModeCards() {
        binding.modeList.removeAllViews()
        val dp = resources.displayMetrics.density
        val modes = listOf(
            SplitTunnelPrefs.MODE_OFF to getString(R.string.split_tunnel_mode_off),
            SplitTunnelPrefs.MODE_BYPASS to getString(R.string.split_tunnel_mode_bypass),
            SplitTunnelPrefs.MODE_ONLY to getString(R.string.split_tunnel_mode_only),
        )
        for ((value, label) in modes) {
            val selectedMode = mode == value
            val card = CardView(this).apply {
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT,
                ).apply { bottomMargin = (8 * dp).toInt() }
                radius = 14 * dp
                cardElevation = 0f
                setCardBackgroundColor(ContextCompat.getColor(this@SplitTunnelAppsActivity, R.color.card_bg))
            }
            val row = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                val p = (14 * dp).toInt()
                setPadding(p, p, p, p)
                background = ContextCompat.getDrawable(
                    this@SplitTunnelAppsActivity,
                    if (selectedMode) R.drawable.bg_card_selected else R.drawable.bg_card,
                )
            }
            val text = TextView(this).apply {
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
                this.text = label
                textSize = 15f
                setTextColor(ContextCompat.getColor(this@SplitTunnelAppsActivity, R.color.title))
                if (selectedMode) setTypeface(null, Typeface.BOLD)
            }
            row.addView(text)
            card.addView(row)
            row.setOnClickListener {
                mode = value
                refreshModeCards()
                refreshAppList()
                updateNote()
            }
            binding.modeList.addView(card)
        }
    }

    private fun refreshAppList() {
        binding.appList.removeAllViews()
        val dp = resources.displayMetrics.density
        val enabled = mode != SplitTunnelPrefs.MODE_OFF
        binding.searchApps.isEnabled = enabled
        binding.searchApps.alpha = if (enabled) 1f else 0.5f

        val apps = if (filterQuery.isEmpty()) {
            allApps
        } else {
            allApps.filter {
                it.label.lowercase().contains(filterQuery) ||
                    it.packageName.lowercase().contains(filterQuery)
            }
        }

        if (apps.isEmpty()) {
            val empty = TextView(this).apply {
                text = getString(R.string.split_tunnel_no_apps)
                textSize = 14f
                setTextColor(ContextCompat.getColor(this@SplitTunnelAppsActivity, R.color.text_dim))
                setPadding(0, (24 * dp).toInt(), 0, 0)
            }
            binding.appList.addView(empty)
            return
        }

        for (app in apps) {
            val checked = app.packageName in selected
            val card = CardView(this).apply {
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT,
                ).apply { bottomMargin = (8 * dp).toInt() }
                radius = 14 * dp
                cardElevation = 0f
                setCardBackgroundColor(ContextCompat.getColor(this@SplitTunnelAppsActivity, R.color.card_bg))
                alpha = if (enabled) 1f else 0.55f
            }
            val row = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                val p = (12 * dp).toInt()
                setPadding(p, p, p, p)
                background = ContextCompat.getDrawable(
                    this@SplitTunnelAppsActivity,
                    if (checked && enabled) R.drawable.bg_card_selected else R.drawable.bg_card,
                )
            }
            val icon = ImageView(this).apply {
                layoutParams = LinearLayout.LayoutParams((40 * dp).toInt(), (40 * dp).toInt())
                    .apply { marginEnd = (12 * dp).toInt() }
                setImageDrawable(app.icon)
            }
            val labelCol = LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
            }
            val name = TextView(this).apply {
                text = app.label
                textSize = 15f
                maxLines = 1
                ellipsize = android.text.TextUtils.TruncateAt.END
                setTextColor(ContextCompat.getColor(this@SplitTunnelAppsActivity, R.color.title))
                if (checked && enabled) setTypeface(null, Typeface.BOLD)
            }
            labelCol.addView(name)
            val check = CheckBox(this).apply {
                isChecked = checked
                isEnabled = enabled
                isClickable = false
                isFocusable = false
            }
            row.addView(icon)
            row.addView(labelCol)
            row.addView(check)
            card.addView(row)

            row.setOnClickListener {
                if (!enabled) return@setOnClickListener
                if (app.packageName in selected) selected.remove(app.packageName) else selected.add(app.packageName)
                refreshAppList()
                updateNote()
            }
            binding.appList.addView(card)
        }
    }

    private fun updateNote() {
        val active = mode != SplitTunnelPrefs.MODE_OFF && selected.isNotEmpty()
        binding.splitTunnelNote.visibility = if (active) View.VISIBLE else View.GONE
    }
}
