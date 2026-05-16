package com.zyrln.relay

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.ProxyInfo
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.util.Log
import androidx.core.app.NotificationCompat
import mobile.Mobile
import java.io.File

class RelayVpnService : VpnService() {

    private var vpnInterface: ParcelFileDescriptor? = null

    companion object {
        const val TAG = "RelayVpnService"
        const val ACTION_START = "com.zyrln.relay.START"
        const val ACTION_STOP = "com.zyrln.relay.STOP"
        const val ACTION_ERROR = "com.zyrln.relay.ERROR"
        const val EXTRA_URL = "url"
        const val EXTRA_KEY = "key"
        const val EXTRA_ERROR = "error"
        const val NOTIF_ID = 1
        const val CHANNEL_ID = "zyrln_vpn"
        private const val PROXY_PORT = 8085

    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_STOP) {
            stopRelay()
            return START_NOT_STICKY
        }

        val url = intent?.getStringExtra(EXTRA_URL) ?: return START_NOT_STICKY
        val key = intent.getStringExtra(EXTRA_KEY) ?: return START_NOT_STICKY

        // Stop any existing session before starting a new one (handles rapid mode switching).
        if (Mobile.isRunning()) {
            Mobile.setSocketProtector(null)
            Mobile.stop()
            vpnInterface?.close()
            vpnInterface = null
        }

        startForeground(NOTIF_ID, buildNotification())
        val vpnService = this
        Mobile.setSocketProtector(object : mobile.SocketProtector {
            override fun protect(p0: Long): Boolean = vpnService.protect(p0.toInt())
        })
        startRelay(url, key)
        return START_STICKY
    }

    private fun startRelay(url: String, key: String) {
        val err = if (url.isEmpty()) {
            Mobile.startDirect("127.0.0.1:$PROXY_PORT")
        } else {
            val certDir = File(filesDir, "certs").also { it.mkdirs() }
            val certPath = File(certDir, "ca.pem").absolutePath
            val keyPath = File(certDir, "ca.key").absolutePath
            if (!File(certPath).exists() || !File(keyPath).exists()) {
                failStart(getString(R.string.error_ca_required))
                return
            }
            Mobile.start(url, key, "127.0.0.1:$PROXY_PORT", certPath, keyPath)
        }
        if (err.isNotEmpty()) {
            Log.e(TAG, "relay start failed: $err")
            failStart(getString(R.string.error_relay_start_failed, err))
            return
        }
        Log.i(TAG, "relay proxy started on 127.0.0.1:$PROXY_PORT")

        val builder = Builder()
            .setSession("Zyrln")
            .addAddress("10.99.0.2", 32)
            .setHttpProxy(ProxyInfo.buildDirectProxy("127.0.0.1", PROXY_PORT))

        try {
            vpnInterface = builder.establish()
            if (vpnInterface == null) {
                // establish() returns null when VPN permission is revoked mid-flow.
                failStart(getString(R.string.error_vpn_permission))
                return
            }
            Log.i(TAG, "VPN interface established")
            sendBroadcast(Intent("com.zyrln.relay.STARTED"))
        } catch (e: Exception) {
            Log.e(TAG, "VPN establish failed: ${e.message}")
            Mobile.setSocketProtector(null)
            Mobile.stop()
            vpnInterface?.close()
            vpnInterface = null
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelf()
        }
    }

    private fun failStart(message: String) {
        Log.e(TAG, message)
        Mobile.setSocketProtector(null)
        Mobile.stop()
        vpnInterface?.close()
        vpnInterface = null
        stopForeground(STOP_FOREGROUND_REMOVE)
        sendBroadcast(Intent(ACTION_ERROR).putExtra(EXTRA_ERROR, message))
        stopSelf()
    }

    private fun stopRelay() {
        Log.i(TAG, "stopping relay")
        Mobile.setSocketProtector(null)
        Mobile.stop()
        vpnInterface?.close()
        vpnInterface = null
        stopForeground(STOP_FOREGROUND_REMOVE)
        sendBroadcast(Intent("com.zyrln.relay.STOPPED"))
        stopSelf()
    }

    override fun onDestroy() {
        Mobile.setSocketProtector(null)
        Mobile.stop()
        vpnInterface?.close()
        vpnInterface = null
        sendBroadcast(Intent("com.zyrln.relay.STOPPED"))
        super.onDestroy()
    }

    private fun buildNotification(): Notification {
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

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle(getString(R.string.vpn_notification_title))
            .setContentText(getString(R.string.vpn_notification_text))
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setContentIntent(openIntent)
            .addAction(android.R.drawable.ic_media_pause, "Stop", stopIntent)
            .setOngoing(true)
            .build()
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                getString(R.string.channel_name),
                NotificationManager.IMPORTANCE_LOW
            )
            getSystemService(NotificationManager::class.java)
                .createNotificationChannel(channel)
        }
    }
}
