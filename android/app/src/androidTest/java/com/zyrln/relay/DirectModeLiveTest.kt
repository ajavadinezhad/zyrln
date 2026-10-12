package com.zyrln.relay

import android.content.Context
import android.content.Intent
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.SystemClock
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import androidx.test.uiautomator.By
import androidx.test.uiautomator.UiDevice
import androidx.test.uiautomator.Until
import mobile.Mobile
import org.junit.Assert.*
import org.junit.Assume.assumeTrue
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File
import java.net.URL
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import javax.net.ssl.HttpsURLConnection

/** Opt-in public internet checks; uses no relay profile and normal Android TLS trust. */
@RunWith(AndroidJUnit4::class)
class DirectModeLiveTest {
    @Test
    fun directVpnCarriesVerifiedHttpsAndReconnects() {
        val args = InstrumentationRegistry.getArguments()
        assumeTrue("Explicit live direct test requested", args.getString("direct_live") == "true")
        val i = InstrumentationRegistry.getInstrumentation()
        val c = i.targetContext
        val prefs = c.getSharedPreferences("config", Context.MODE_PRIVATE)
        val saved = prefs.all
        val device = UiDevice.getInstance(i)
        val cm = c.getSystemService(ConnectivityManager::class.java)
        val trace = File(c.filesDir, "direct-live-trace.txt")
        trace.writeText("Direct live test API=${android.os.Build.VERSION.SDK_INT}\n")
        fun active() = cm.allNetworks.any { cm.getNetworkCapabilities(it)?.hasTransport(NetworkCapabilities.TRANSPORT_VPN) == true }
        fun waitFor(label: String, condition: () -> Boolean) {
            val end = SystemClock.elapsedRealtime() + 20000
            while (!condition() && SystemClock.elapsedRealtime() < end) SystemClock.sleep(100)
            assertTrue(label, condition())
        }
        val pool = Executors.newFixedThreadPool(3)
        prefs.edit().clear().putString("configs", "[]").putBoolean("direct_only", true)
            .putBoolean("direct_enabled", true).putBoolean("direct_opt_out", false).commit()
        SplitTunnelPrefs.save(prefs, SplitTunnelPrefs.MODE_OFF, emptySet())
        c.startActivity(Intent(c, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TASK))
        try {
            waitFor("Physical internet is validated before direct VPN") {
                cm.allNetworks.any { n -> cm.getNetworkCapabilities(n)?.let { it.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED) && !it.hasTransport(NetworkCapabilities.TRANSPORT_VPN) } == true }
            }
            val physical = cm.activeNetwork ?: error("Active physical network missing")
            assertFalse("Baseline uses active non-VPN network", cm.getNetworkCapabilities(physical)!!.hasTransport(NetworkCapabilities.TRANSPORT_VPN))
            for (url in listOf("https://www.wikipedia.org/", "https://www.gstatic.com/generate_204")) {
                val baseline = physical.openConnection(URL(url)) as HttpsURLConnection
                try {
                    baseline.connectTimeout = 15000; baseline.readTimeout = 20000
                    baseline.setRequestProperty("User-Agent", "Zyrln-Integration-Test/1.0")
                    val expected = if (url.contains("generate_204")) 204 else 200
                    assertEquals("HTTPS works outside the VPN before judging direct mode", expected, baseline.responseCode)
                    baseline.inputStream.use { it.readBytes() }
                    trace.appendText("Physical HTTPS baseline status=$expected\n")
                } finally { baseline.disconnect() }
            }
            repeat(2) { cycle ->
                val connect = device.wait(Until.findObject(By.res(c.packageName, "btnConnect")), 10000) ?: error("Connect control missing")
                connect.click()
                val permission = device.wait(Until.findObject(By.res("android", "button1")), 2000)
                if (permission?.text.equals("OK", true)) permission?.click()
                waitFor("Direct VPN started") { Mobile.isRunning() && active() }
                assertTrue("Direct enabled, with no relay profiles", Mobile.isDirectEnabled())
                assertEquals("[]", prefs.getString("configs", ""))
                val urls = listOf("https://www.wikipedia.org/", "https://www.gstatic.com/generate_204", "https://www.wikipedia.org/")
                val requests = urls.map { url ->
                    pool.submit<Boolean> {
                        val conn = URL(url).openConnection() as HttpsURLConnection
                        try {
                            conn.connectTimeout = 15000; conn.readTimeout = 20000
                            conn.setRequestProperty("User-Agent", "Zyrln-Integration-Test/1.0")
                            val expected = if (url.contains("generate_204")) 204 else 200
                            val status = conn.responseCode
                            val body = conn.inputStream.bufferedReader().use { it.readText() }
                            val ok = status == expected && (expected == 204 || body.contains("Wikipedia"))
                            synchronized(trace) { trace.appendText("cycle=${cycle + 1} HTTPS status=$status bytes=${body.length} passed=$ok\n") }
                            ok
                        } catch (failure: Throwable) {
                            synchronized(trace) { trace.appendText("HTTPS failure url=$url error=$failure\n") }
                            throw failure
                        } finally { conn.disconnect() }
                    }
                }
                requests.forEach { assertTrue("Verified HTTPS through direct VPN", it.get(45, TimeUnit.SECONDS)) }
                if (args.getString("direct_browser") == "true" && cycle == 1) {
                    val intent = Intent(Intent.ACTION_VIEW, android.net.Uri.parse("https://www.wikipedia.org/?zyrln-direct-test=" + SystemClock.elapsedRealtime())).setPackage("com.android.chrome").addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                    c.startActivity(intent)
                    device.wait(Until.findObject(By.res("com.android.chrome", "signin_fre_dismiss_button")), 5000)?.click()
                    device.wait(Until.findObject(By.text("No thanks")), 3000)?.click()
                    c.startActivity(intent)
                    val loaded = device.wait(Until.hasObject(By.textContains("The Free Encyclopedia")), 30000)
                    assertTrue("Chrome page through direct VPN", loaded)
                    device.waitForIdle()
                    SystemClock.sleep(2000)
                    val content = device.findObject(By.textContains("The Free Encyclopedia"))
                    assertNotNull("Chrome content remains present after painting", content)
                    assertTrue("Chrome content has visible screen bounds", content.visibleBounds.width() > 0 && content.visibleBounds.height() > 0)
                    device.takeScreenshot(File(c.filesDir, "direct-live-browser.png"))
                    device.dumpWindowHierarchy(File(c.filesDir, "direct-live-browser.xml"))
                    trace.appendText("Chrome direct page passed\n")
                    c.startActivity(Intent(c, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
                }
                device.wait(Until.findObject(By.res(c.packageName, "btnConnect")), 10000)!!.click()
                waitFor("Direct VPN removed") { !Mobile.isRunning() && !active() }
                trace.appendText("cycle=${cycle + 1} disconnected\n")
            }
        } catch (failure: Throwable) {
            trace.appendText("FAILURE $failure\n")
            trace.appendText("Native direct logs:\n" + Mobile.getAllLogs() + "\n")
            device.takeScreenshot(File(c.filesDir, "direct-live-failure.png"))
            throw failure
        } finally {
            pool.shutdownNow()
            c.startService(Intent(c, RelayVpnService::class.java).setAction(RelayVpnService.ACTION_STOP))
            waitFor("VPN stopped before restoring preferences") { !Mobile.isRunning() && !active() }
            val edit = prefs.edit().clear()
            saved.forEach { (name, value) ->
                when (value) {
                    is String -> edit.putString(name, value)
                    is Boolean -> edit.putBoolean(name, value)
                    is Int -> edit.putInt(name, value)
                    is Long -> edit.putLong(name, value)
                    is Float -> edit.putFloat(name, value)
                    is Set<*> -> edit.putStringSet(name, value.filterIsInstance<String>().toSet())
                }
            }
            edit.commit()
        }
    }
}
