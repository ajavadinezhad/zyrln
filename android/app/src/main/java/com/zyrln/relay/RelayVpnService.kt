package com.zyrln.relay

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.net.ProxyInfo
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.util.Log
import androidx.core.app.NotificationCompat
import mobile.Mobile

class RelayVpnService : VpnService() {

    private var vpnInterface: ParcelFileDescriptor? = null
    private var latestStartId = 0
    @Volatile private var startGeneration = 0
    @Volatile private var stopComplete = false
    private val stopLock = Any()
    private val mainHandler = android.os.Handler(android.os.Looper.getMainLooper())

    companion object {
        // The Go engine is process-global, so cleanup must remain ordered across service instances.
        private val lifecycle = java.util.concurrent.Executors.newSingleThreadExecutor()
        @Volatile var transitioning = false
            private set
        @Volatile var starting = false
            private set
        const val TAG = "RelayVpnService"
        const val ACTION_START = "com.zyrln.relay.START"
        const val ACTION_STOP = "com.zyrln.relay.STOP"
        const val ACTION_STARTED = "com.zyrln.relay.STARTED"
        const val ACTION_ERROR = "com.zyrln.relay.ERROR"
        const val ACTION_STOPPED = "com.zyrln.relay.STOPPED"
        const val EXTRA_URL = "url"
        const val EXTRA_KEY = "key"
        const val EXTRA_ERROR = "error"
        const val NOTIF_ID = 1
        // Channel importance is fixed at creation and can't be raised in place —
        // renaming the ID forces a fresh channel at IMPORTANCE_DEFAULT for
        // existing installs, which still have "zyrln_vpn" stuck at LOW.
        const val CHANNEL_ID = "zyrln_vpn_v2"
        private const val OLD_CHANNEL_ID = "zyrln_vpn"
        private const val PROXY_PORT = 8085
        private const val VPN_ADDRESS = "10.99.0.2"
        private const val VPN_PREFIX = 24
        private const val VPN_MTU = 1500
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        latestStartId = startId
        if (intent?.action == ACTION_STOP) {
            stopRelay()
            return START_NOT_STICKY
        }

        synchronized(stopLock) { stopComplete = false }

        val url = intent?.getStringExtra(EXTRA_URL) ?: return START_NOT_STICKY
        val key = intent.getStringExtra(EXTRA_KEY) ?: return START_NOT_STICKY

        startGeneration++

        startForeground(NOTIF_ID, buildNotification(url.isEmpty()))
        val gen = startGeneration
        starting = true
        transitioning = true
        lifecycle.execute {
            try {
                if (gen != startGeneration) return@execute
                if (Mobile.isRunning()) {
                    Mobile.setHostResolver(null)
                    Mobile.setSocketProtector(null)
                    Mobile.stop()
                    vpnInterface?.close()
                    vpnInterface = null
                }

                val vpnService = this
                Mobile.setSocketProtector(object : mobile.SocketProtector {
                    override fun protect(p0: Long): Boolean = vpnService.protect(p0.toInt())
                })
                val connectivity = getSystemService(ConnectivityManager::class.java)
                Mobile.setHostResolver(object : mobile.HostResolver {
                    override fun resolve(host: String): String {
                        val networks = connectivity.allNetworks.filter { network ->
                            connectivity.getNetworkCapabilities(network)?.let { caps ->
                                caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET) &&
                                    !caps.hasTransport(NetworkCapabilities.TRANSPORT_VPN)
                            } == true
                        }
                        for (network in networks) {
                            try {
                                val addresses = network.getAllByName(host)
                                if (addresses.isNotEmpty()) return addresses.joinToString("\n") { it.hostAddress.orEmpty() }
                            } catch (failure: java.net.UnknownHostException) {
                                Log.w(TAG, "Underlying DNS lookup failed for $host: ${failure.message}")
                            }
                        }
                        Log.w(TAG, "No underlying network resolved $host")
                        return ""
                    }
                })
                startRelay(url, key, gen)
            } catch (failure: Exception) {
                failStart(getString(R.string.error_relay_start_failed, failure.message.orEmpty()), gen)
            } finally {
                if (gen == startGeneration) { starting = false; transitioning = false }
            }
        }
        return START_STICKY
    }

    private fun startRelay(url: String, key: String, generation: Int) {
        if (generation != startGeneration) return

        val err = if (url.isEmpty()) {
            Mobile.startDirect("127.0.0.1:$PROXY_PORT")
        } else {
            Mobile.startTunnel(url, key, "127.0.0.1:$PROXY_PORT")
        }

        if (generation != startGeneration) return
        if (err.isNotEmpty()) {
            Log.e(TAG, "relay start failed: $err")
            failStart(getString(R.string.error_relay_start_failed, err), generation)
            return
        }
        Log.i(TAG, "relay started on 127.0.0.1:$PROXY_PORT (directOnly=${url.isEmpty()})")
        if (generation != startGeneration) {
            Mobile.setHostResolver(null)
            Mobile.setSocketProtector(null)
            Mobile.stop()
            return
        }

        try {
            // TUN routes all IPv4; Go forwarder dials tunnel/direct per flow. HTTP proxy kept for proxy-aware apps.
            val builder = Builder()
                .setSession(getString(R.string.vpn_session_name))
                .setMtu(VPN_MTU)
                .addAddress(VPN_ADDRESS, VPN_PREFIX)
                .addRoute("0.0.0.0", 0)
                .addDnsServer("8.8.8.8")
                .addDnsServer("1.1.1.1")
            // Android 9 uses the TUN forwarder; the optional proxy hint starts at Android 10.
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                builder.setHttpProxy(ProxyInfo.buildDirectProxy("127.0.0.1", PROXY_PORT))
            }
            applySplitTunnel(builder)
            vpnInterface = builder.establish()
            if (vpnInterface == null) {
                failStart(getString(R.string.error_vpn_permission), generation)
                return
            }
            if (generation != startGeneration) {
                Mobile.setHostResolver(null)
                Mobile.setSocketProtector(null)
                Mobile.stop()
                vpnInterface?.close()
                vpnInterface = null
                return
            }
            val tunErr = Mobile.attachTUN(vpnInterface!!.fd.toLong())
            if (tunErr.isNotEmpty()) {
                Log.e(TAG, "TUN forwarder failed: $tunErr")
                failStart(getString(R.string.error_relay_start_failed, tunErr), generation)
                return
            }
            Log.i(
                TAG,
                "TUN forwarder active fd=${vpnInterface!!.fd} routes=0.0.0.0/0 proxy=127.0.0.1:$PROXY_PORT"
            )
            if (generation != startGeneration) {
                Mobile.setHostResolver(null)
                Mobile.setSocketProtector(null)
                Mobile.stop()
                vpnInterface?.close()
                vpnInterface = null
                return
            }
            sendBroadcast(Intent(ACTION_STARTED).setPackage(packageName))
        } catch (e: Exception) {
            failStart(getString(R.string.error_relay_start_failed, e.message.orEmpty()), generation)
        }
    }

    private fun failStart(message: String, generation: Int) {
        if (generation != startGeneration) return
        Log.e(TAG, message)
        Mobile.setHostResolver(null)
        Mobile.setSocketProtector(null)
        Mobile.stop()
        vpnInterface?.close()
        vpnInterface = null

        mainHandler.post {
            if (generation != startGeneration) return@post
            stopForeground(STOP_FOREGROUND_REMOVE)
            sendBroadcast(Intent(ACTION_ERROR).setPackage(packageName).putExtra(EXTRA_ERROR, message))
            stopSelfResult(latestStartId)
        }
    }

    private fun stopRelay() {
        synchronized(stopLock) {
            if (stopComplete) return
            stopComplete = true
        }
        startGeneration++
        Log.i(TAG, "stopping relay")
        val generation = startGeneration
        starting = false
        transitioning = true
        lifecycle.execute {
            Mobile.setHostResolver(null)
            Mobile.setSocketProtector(null)
            Mobile.stop()
            synchronized(stopLock) {
                vpnInterface?.close()
                vpnInterface = null
            }
            mainHandler.post {
                if (generation != startGeneration) return@post
                transitioning = false
                stopForeground(STOP_FOREGROUND_REMOVE)
                sendBroadcast(Intent(ACTION_STOPPED).setPackage(packageName))
                stopSelfResult(latestStartId)
            }
        }
    }

    override fun onRevoke() {
        // Android may invoke revocation off the main thread.
        mainHandler.post { stopRelay() }
    }

    override fun onDestroy() {
        startGeneration++
        starting = false
        transitioning = true
        lifecycle.execute {
            Mobile.setHostResolver(null)
            Mobile.setSocketProtector(null)
            Mobile.stop()
            vpnInterface?.close()
            vpnInterface = null
        }
        transitioning = false

        super.onDestroy()
    }

    private fun applySplitTunnel(builder: Builder) {
        val prefs = getSharedPreferences("config", MODE_PRIVATE)
        SplitTunnelPrefs.resolvePackages(prefs, packageManager, persistPrune = true)
        val cfg = SplitTunnelPrefs.loadResolved(prefs, packageManager)
        if (!SplitTunnelPrefs.isActive(cfg)) {
            return
        }
        var applied = 0
        for (pkg in cfg.packages) {
            if (pkg == packageName) {
                continue
            }
            try {
                when (cfg.mode) {
                    SplitTunnelPrefs.MODE_BYPASS -> builder.addDisallowedApplication(pkg)
                    SplitTunnelPrefs.MODE_ONLY -> builder.addAllowedApplication(pkg)
                }
                applied++
            } catch (e: android.content.pm.PackageManager.NameNotFoundException) {
                Log.w(TAG, "split tunnel: unknown package $pkg")
            }
        }
        if (applied == 0) {
            Log.w(TAG, "split tunnel mode=${cfg.mode}: no valid packages, using full VPN")
        } else {
            Log.i(TAG, "split tunnel mode=${cfg.mode} apps=$applied")
        }
    }

    private fun buildNotification(directOnly: Boolean): Notification {
        createNotificationChannel()
        val openIntent = PendingIntent.getActivity(
            this, 0,
            Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        val stopIntent = PendingIntent.getService(
            this, 0,
            Intent(this, RelayVpnService::class.java).apply { action = ACTION_STOP },
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        val title = if (directOnly) {
            getString(R.string.direct_notification_title)
        } else {
            getString(R.string.vpn_notification_title)
        }
        val text = if (directOnly) {
            getString(R.string.direct_notification_text)
        } else {
            getString(R.string.vpn_notification_text)
        }
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle(title)
            .setContentText(text)
            .setSmallIcon(R.mipmap.ic_launcher)
            .setContentIntent(openIntent)
            .addAction(android.R.drawable.ic_media_pause, getString(R.string.btn_disconnect), stopIntent)
            .setOngoing(true)
            .build()
    }

    private fun createNotificationChannel() {
        val manager = getSystemService(NotificationManager::class.java)
        manager.deleteNotificationChannel(OLD_CHANNEL_ID)
        val channel = NotificationChannel(
            CHANNEL_ID,
            getString(R.string.channel_name_vpn),
            NotificationManager.IMPORTANCE_DEFAULT
        )
        manager.createNotificationChannel(channel)
    }
}
