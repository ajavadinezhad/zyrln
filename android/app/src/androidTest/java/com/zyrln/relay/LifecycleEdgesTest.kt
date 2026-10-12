package com.zyrln.relay

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.SystemClock
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

@RunWith(AndroidJUnit4::class)
class LifecycleEdgesTest {
    @Test fun permissionRecreationBackgroundDeletionAndSystemRevocation() {
        val i = InstrumentationRegistry.getInstrumentation()
        val c = i.targetContext
        assertTrue("Disposable emulator only", android.os.Build.FINGERPRINT.contains("generic") || android.os.Build.MODEL.contains("sdk"))
        val d = UiDevice.getInstance(i)
        val prefs = c.getSharedPreferences("config", Context.MODE_PRIVATE)
        val saved = HashMap(prefs.all)
        val monitor = i.addMonitor(MainActivity::class.java.name, null, false)
        val cm = c.getSystemService(ConnectivityManager::class.java)
        val url = "https://127.0.0.1:3/lifecycle-test"
        val key = "disposable-lifecycle-key"
        var activity: MainActivity? = null
        val trace = java.io.File(c.filesDir, "lifecycle-edges-result.txt")
        trace.writeText("")
        fun record(s: String) = trace.appendText(s + "\n")
        fun await(label: String, condition: () -> Boolean) {
            val end = SystemClock.elapsedRealtime() + 15000
            while (!condition() && SystemClock.elapsedRealtime() < end) SystemClock.sleep(50)
            assertTrue(label, condition())
        }
        fun button(id: String) = d.wait(Until.findObject(By.res(c.packageName, id)), 10000) ?: error("Missing $id")
        fun vpn() = cm.allNetworks.any { cm.getNetworkCapabilities(it)?.hasTransport(NetworkCapabilities.TRANSPORT_VPN) == true }
        fun stopped() {
            await("Proxy and VPN stopped") { !Mobile.isRunning() && !vpn() && !RelayVpnService.transitioning }
            await("Connect available") { button("btnConnect").isEnabled }
            assertEquals(c.getString(R.string.connect_action), button("btnConnect").text)
        }
        fun connected() {
            await("Proxy and VPN connected") { Mobile.isRunning() && vpn() && !RelayVpnService.transitioning }
            await("Disconnect available") { button("btnConnect").isEnabled }
            assertEquals(c.getString(R.string.btn_disconnect), button("btnConnect").text)
            assertEquals("Expected profile retained", url, prefs.getString("url", null))
        }
        fun approve() {
            val ok = d.wait(Until.findObject(By.res("android", "button1")), 5000) ?: error("VPN approval missing")
            assertTrue("VPN confirmation button", ok.text.equals("OK", true))
            ok.click()
        }
        fun revoke() = d.executeShellCommand("appops set ${c.packageName} ACTIVATE_VPN ignore")
        fun stopFixture() = i.context.startActivity(Intent(i.context, LifecycleVpnFixtureActivity::class.java)
            .putExtra("stop_fixture", true).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
        try {
            c.startService(Intent(c, RelayVpnService::class.java).setAction(RelayVpnService.ACTION_STOP))
            await("Initial cleanup") { !Mobile.isRunning() && !RelayVpnService.transitioning }
            revoke()
            prefs.edit().clear().putString("configs", "[]").putBoolean("direct_only", true).putBoolean("direct_enabled", false).putBoolean("direct_opt_out", true).commit()
            c.startActivity(Intent(c, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TASK))
            activity = i.waitForMonitorWithTimeout(monitor, 10000) as MainActivity
            button("btnImportConfig")
            i.runOnMainSync { c.getSystemService(ClipboardManager::class.java).setPrimaryClip(ClipData.newPlainText("test", ConfigUtils.toExportJson(url, key))) }
            button("btnImportConfig").click()
            button("btnConnect").click()
            val cancel = d.wait(Until.findObject(By.res("android", "button2")), 5000) ?: error("VPN cancel missing")
            cancel.click()
            stopped()
            assertEquals("Denied permission keeps relay selected", c.getString(R.string.mode_relay_hint), button("modeHint").text)
            record("PASS permission denial: no VPN, Connect restored, selected relay retained")

            button("btnConnect").click()
            assertNotNull("Permission pending", d.wait(Until.findObject(By.res("android", "button1")), 5000))
            val previous = activity
            i.runOnMainSync { activity!!.recreate() }
            // Android can defer recreating a paused activity until its permission dialog closes.
            approve()
            await("Activity recreation completes after permission result") {
                i.runOnMainSync {
                    activity = androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry.getInstance()
                        .getActivitiesInStage(androidx.test.runner.lifecycle.Stage.RESUMED)
                        .filterIsInstance<MainActivity>().firstOrNull()
                }
                activity != null && activity !== previous
            }
            connected()
            assertEquals("Relay context survives recreation", c.getString(R.string.mode_relay_connected), button("modeHint").text)
            assertFalse("Import unavailable while connected", d.hasObject(By.res(c.packageName, "btnImportConfig")))
            record("PASS recreation requested while awaiting VPN permission, connected relay context")
            button("btnConnect").click()
            stopped()

            i.runOnMainSync { activity!!.findViewById<View>(R.id.btnConnect).performClick() }
            d.pressHome()
            await("Start survives backgrounding") { Mobile.isRunning() && vpn() && !RelayVpnService.transitioning }
            c.startActivity(Intent(c, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            connected()
            record("PASS background during connection and foreground return")
            // A second installed test app becoming the VPN really invokes Android's onRevoke.
            i.context.startActivity(Intent(i.context, LifecycleVpnFixtureActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            d.wait(Until.findObject(By.res("android", "button1")), 5000)?.let { if (it.text.equals("OK", true)) it.click() }
            await("Android revoked Zyrln after second VPN takeover") { !Mobile.isRunning() && !RelayVpnService.transitioning }
            stopFixture()
            c.startActivity(Intent(c, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            stopped()
            assertEquals("Revocation retains selected relay", c.getString(R.string.mode_relay_hint), button("modeHint").text)
            record("PASS second VPN takeover: system revocation, cleanup, UI stopped, selected relay retained")
            button("btnConnect").click()
            d.wait(Until.findObject(By.res("android", "button1")), 2000)?.let { if (it.text.equals("OK", true)) it.click() }
            connected()
            record("PASS reconnect after revocation")
            button("btnConnect").click()
            stopped()
            val remove = d.wait(Until.findObject(By.desc(c.getString(R.string.btn_remove))), 5000) ?: error("Remove missing")
            remove.click()
            d.wait(Until.findObject(By.res("android", "button1")), 5000).click()
            await("Profile removed after animation") { JSONArray(prefs.getString("configs", "[]")).length() == 0 }
            assertEquals("Deleted profile is no longer selected", c.getString(R.string.mode_direct_hint), button("modeHint").text)
            button("btnConnect").click()
            await("Direct fallback starts without deleted profile") { Mobile.isRunning() && vpn() && !RelayVpnService.transitioning }
            assertEquals("Deleted relay not reused", "", prefs.getString("url", null))
            record("PASS deletion of selected profile, direct fallback avoids stale endpoint")
            button("btnConnect").click()
            stopped()
            record("PASS lifecycle edge suite complete")
        } catch (failure: Throwable) {
            record("FAIL " + failure.toString())
            d.takeScreenshot(java.io.File(c.getExternalFilesDir(null), "lifecycle-edges-failure.png"))
            d.dumpWindowHierarchy(java.io.File(c.filesDir, "lifecycle-edges-failure.xml"))
            throw failure
        } finally {
            stopFixture()
            c.startService(Intent(c, RelayVpnService::class.java).setAction(RelayVpnService.ACTION_STOP))
            await("Cleanup stopped") { !Mobile.isRunning() && !RelayVpnService.transitioning }
            d.executeShellCommand("appops set ${c.packageName} ACTIVATE_VPN allow")
            val restore = prefs.edit().clear()
            saved.forEach { (name,value) -> when(value) {
                is String -> restore.putString(name,value)
                is Boolean -> restore.putBoolean(name,value)
                is Int -> restore.putInt(name,value)
                is Long -> restore.putLong(name,value)
                is Float -> restore.putFloat(name,value)
                is Set<*> -> restore.putStringSet(name,value.filterIsInstance<String>().toSet())
            } }
            restore.commit()
            i.removeMonitor(monitor)
        }
    }
}
