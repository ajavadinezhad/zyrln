package com.zyrln.relay

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.SystemClock
import android.widget.TextView
import android.view.View
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import androidx.test.uiautomator.By
import androidx.test.uiautomator.UiDevice
import androidx.test.uiautomator.Until
import mobile.Mobile
import org.json.JSONArray
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith

/** Disposable emulator only: synthetic profiles test lifecycle, not live relay reachability. */
@RunWith(AndroidJUnit4::class)
class ConnectionEdgesTest {
    @Test fun importSwitchRapidTapAndQuotaNotice() {
        val i = InstrumentationRegistry.getInstrumentation()
        val c = i.targetContext
        val d = UiDevice.getInstance(i)
        assertTrue("Run only on a disposable emulator", android.os.Build.FINGERPRINT.contains("generic") || android.os.Build.MODEL.contains("sdk"))
        val prefs = c.getSharedPreferences("config", Context.MODE_PRIVATE)
        val saved = HashMap(prefs.all)
        val cm = c.getSystemService(ConnectivityManager::class.java)
        val monitor = i.addMonitor(MainActivity::class.java.name, null, false)
        val a = "https://127.0.0.1:1/profile-a"
        val b = "https://127.0.0.1:2/profile-b"
        val key = "disposable-edge-test"
        var activity: MainActivity? = null
        fun await(label: String, condition: () -> Boolean) {
            val deadline = SystemClock.elapsedRealtime() + 15000
            while (!condition() && SystemClock.elapsedRealtime() < deadline) SystemClock.sleep(50)
            assertTrue(label, condition())
        }
        fun button(id: String) = d.wait(Until.findObject(By.res(c.packageName, id)), 10000) ?: error("Missing $id")
        fun burst() = i.runOnMainSync {
            val control = activity!!.findViewById<View>(R.id.btnConnect)
            repeat(25) { control.performClick() }
        }
        fun vpn() = cm.allNetworks.any { cm.getNetworkCapabilities(it)?.hasTransport(NetworkCapabilities.TRANSPORT_VPN) == true }
        fun connected(url: String) {
            await("VPN established") { Mobile.isRunning() && vpn() && !RelayVpnService.transitioning }
            assertEquals("Expected selected backend", url, prefs.getString("url", null))
            await("Disconnect control enabled") { button("btnConnect").isEnabled }
            assertEquals(c.getString(R.string.btn_disconnect), button("btnConnect").text)
        }
        fun stopped() {
            await("VPN and native proxy removed") { !Mobile.isRunning() && !vpn() && !RelayVpnService.transitioning }
            await("Connect control enabled after cleanup") { button("btnConnect").isEnabled }
        }
        fun import(url: String) {
            i.runOnMainSync { c.getSystemService(ClipboardManager::class.java).setPrimaryClip(ClipData.newPlainText("edge test", ConfigUtils.toExportJson(url, key))) }
            button("btnImportConfig").click()
            i.waitForIdleSync()
        }
        fun notice(message: String) = i.runOnMainSync {
            val method = MainActivity::class.java.getDeclaredMethod("appendLogBatch", List::class.java)
            method.isAccessible = true
            method.invoke(activity, listOf("error" to message))
        }
        try {
            c.startService(Intent(c, RelayVpnService::class.java).setAction(RelayVpnService.ACTION_STOP))
            await("Initial proxy stopped") { !Mobile.isRunning() && !RelayVpnService.transitioning }
            prefs.edit().clear().putString("configs", "[]").putBoolean("direct_only", true).putBoolean("direct_enabled", false).putBoolean("direct_opt_out", true).commit()
            c.startActivity(Intent(c, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TASK))
            activity = i.waitForMonitorWithTimeout(monitor, 10000) as MainActivity
            button("btnImportConfig")
            import(a)
            import(a)
            assertEquals("Duplicate import unchanged", 1, JSONArray(prefs.getString("configs", "[]")).length())
            burst()
            d.wait(Until.findObject(By.res("android", "button1")), 2000)?.let { if (it.text.equals("OK", true)) it.click() }
            connected(a)
            notice("Apps Script returned HTML instead of JSON (quota/deploy issue)")
            assertEquals(c.getString(R.string.relay_unavailable_notice), button("relayNotice").text)
            notice("Apps Script quota exceeded")
            assertEquals(c.getString(R.string.relay_quota_notice), button("relayNotice").text)
            notice("Apps Script returned HTML instead of JSON")
            assertEquals("Quota remains visible after generic errors", c.getString(R.string.relay_quota_notice), button("relayNotice").text)
            d.takeScreenshot(java.io.File(c.getExternalFilesDir(null), "quota-notice.png"))
            burst()
            stopped()
            import(b)
            assertEquals("Second import visible", 2, JSONArray(prefs.getString("configs", "[]")).length())
            repeat(3) { cycle ->
                val target = if (cycle % 2 == 0) b else a
                if (cycle > 0) {
                    val label = ConfigUtils.configLabel(target)
                    val row = d.wait(Until.findObject(By.text(label)), 5000) ?: error("Missing profile row")
                    var card = row.parent
                    while (!card.isClickable) card = card.parent
                    card.click()
                }
                burst()
                connected(target)
                i.runOnMainSync { assertNotEquals("Old quota warning does not leak to next profile", c.getString(R.string.relay_quota_notice), activity!!.findViewById<TextView>(R.id.relayNotice).text.toString()) }
                burst()
                stopped()
            }
            // Stop immediately followed by a new start stresses the service queue, bypassing UI guards.
            repeat(8) {
                c.startService(Intent(c, RelayVpnService::class.java).setAction(RelayVpnService.ACTION_START).putExtra(RelayVpnService.EXTRA_URL, "").putExtra(RelayVpnService.EXTRA_KEY, ""))
                c.startService(Intent(c, RelayVpnService::class.java).setAction(RelayVpnService.ACTION_STOP))
            }
            c.startService(Intent(c, RelayVpnService::class.java).setAction(RelayVpnService.ACTION_START).putExtra(RelayVpnService.EXTRA_URL, "").putExtra(RelayVpnService.EXTRA_KEY, ""))
            await("Final start survives earlier stop callbacks") { Mobile.isRunning() && vpn() && !RelayVpnService.transitioning }
            SystemClock.sleep(500)
            assertTrue("Old stop cannot kill final start", Mobile.isRunning() && vpn())
            c.startService(Intent(c, RelayVpnService::class.java).setAction(RelayVpnService.ACTION_STOP))
            stopped()
            java.io.File(c.filesDir, "connection-edges-result.txt").writeText("PASS: duplicate import, four relay cycles, profile A/B/A/B, 200 UI taps, 8 start/stop pairs, final start, quota/HTML notices\n")
        } catch (failure: Throwable) {
            d.takeScreenshot(java.io.File(c.getExternalFilesDir(null), "connection-edges-failure.png"))
            java.io.File(c.filesDir, "connection-edges-result.txt").writeText("FAIL: " + failure.toString())
            throw failure
        } finally {
            c.startService(Intent(c, RelayVpnService::class.java).setAction(RelayVpnService.ACTION_STOP))
            await("Cleanup stopped") { !Mobile.isRunning() && !RelayVpnService.transitioning }
            val restore = prefs.edit().clear()
            saved.forEach { (name, value) -> when(value) {
                is String -> restore.putString(name, value)
                is Boolean -> restore.putBoolean(name, value)
                is Int -> restore.putInt(name, value)
                is Long -> restore.putLong(name, value)
                is Float -> restore.putFloat(name, value)
                is Set<*> -> restore.putStringSet(name, value.filterIsInstance<String>().toSet())
            } }
            restore.commit()
            i.removeMonitor(monitor)
        }
    }
}
