package com.zyrln.relay

import android.content.res.Configuration
import java.util.Locale
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
import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Assume.assumeTrue
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File
import java.net.InetAddress
import java.net.InetSocketAddress
import java.net.Socket
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import javax.net.ssl.SSLSocket
import javax.net.ssl.SSLSocketFactory

/** Opt-in live test: put a dedicated url/key JSON in app-private files/live-relay.json. */
@RunWith(AndroidJUnit4::class)
class RealRelayTest {
    @Test
    fun relayVpnCarriesVerifiedHttpsAndReconnects() {
        val i = InstrumentationRegistry.getInstrumentation()
        val c = i.targetContext
        val file = File(c.filesDir, "live-relay.json")
        assumeTrue("A dedicated real relay profile must be provided", file.exists())
        val profile = JSONObject(file.readText())
        val url = profile.getString("url")
        val key = profile.getString("key")
        val cycles = profile.optInt("cycles", 2).also { require(it in 1..20) }
        val rounds = profile.optInt("rounds", 1).also { require(it in 1..60) }
        val networkRecovery = profile.optBoolean("network_recovery")
        val pauseMs = profile.optLong("pause_ms", 0).also { require(it in 0..60000) }
        val trace = File(c.filesDir, "real-relay-trace.txt")
        trace.writeText("")
        val started = SystemClock.elapsedRealtime()
        fun record(message: String) = synchronized(trace) {
            trace.appendText("${SystemClock.elapsedRealtime() - started}ms $message\n")
        }
        require(url.startsWith("https://script.google.com/macros/s/"))
        val prefs = c.getSharedPreferences("config", Context.MODE_PRIVATE)
        val saved = prefs.all
        val device = UiDevice.getInstance(i)
        val cm = c.getSystemService(ConnectivityManager::class.java)
        val destination = InetAddress.getByName("www.wikipedia.org")
        val relayHints = listOf("en", "fa").map { language ->
            val config = Configuration(c.resources.configuration).apply { setLocale(Locale(language)) }
            c.createConfigurationContext(config).getString(R.string.mode_relay_hint)
        }.toSet()
        fun active() = cm.allNetworks.any { cm.getNetworkCapabilities(it)?.hasTransport(NetworkCapabilities.TRANSPORT_VPN) == true }
        fun waitFor(label: String, condition: () -> Boolean) {
            val end = SystemClock.elapsedRealtime() + 20000
            while (!condition() && SystemClock.elapsedRealtime() < end) SystemClock.sleep(100)
            assertTrue(label, condition())
        }
        fun physicalNetwork(validated: Boolean = false) = cm.allNetworks.any {
            cm.getNetworkCapabilities(it)?.let { caps ->
                caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET) &&
                    (!validated || caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED)) &&
                    !caps.hasTransport(NetworkCapabilities.TRANSPORT_VPN)
            } == true
        }
        val wifiWasEnabled = device.executeShellCommand("settings get global wifi_on").trim() == "1"
        val dataWasEnabled = device.executeShellCommand("settings get global mobile_data").trim() == "1"
        fun restoreNetwork() {
            device.executeShellCommand("svc wifi " + if (wifiWasEnabled) "enable" else "disable")
            device.executeShellCommand("svc data " + if (dataWasEnabled) "enable" else "disable")
        }
        fun connectControl() = device.wait(Until.findObject(By.res(c.packageName, "btnConnect")), 10000) ?: error("Connect control missing")
        val pool = Executors.newFixedThreadPool(3)
        var persistentSocket: Socket? = null
        var persistentReader: java.io.BufferedReader? = null
        prefs.edit().putString("configs", JSONArray().put(profile).toString())
            .putBoolean("direct_only", false).putBoolean("direct_enabled", false)
            .putBoolean("direct_opt_out", true).putString("split_mode", "off").commit()
        SplitTunnelPrefs.save(prefs, SplitTunnelPrefs.MODE_OFF, emptySet())
        c.startActivity(Intent(c, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TASK))
        try {
            device.wait(Until.findObject(By.text(ConfigUtils.configLabel(url))), 10000)!!.click()
            if (Mobile.isDirectEnabled()) {
                device.wait(Until.findObject(By.res(c.packageName, "btnDirect")), 10000)!!.click()
            }
            assertFalse("Google direct toggle is off before connecting", Mobile.isDirectEnabled())
            repeat(cycles) { cycle ->
                println("Live relay cycle ${cycle + 1}")
                connectControl().click()
                val permission = device.wait(Until.findObject(By.res("android", "button1")), 2000)
                if (permission?.text.equals("OK", true)) permission?.click()
                waitFor("Relay VPN established") { Mobile.isRunning() && active() }
                assertFalse("Direct bypass is disabled for live relay traffic", Mobile.isDirectEnabled())
                val persistentTarget = profile.optString("persistent_target")
                if (persistentTarget.isNotEmpty()) {
                    val host = persistentTarget.substringBeforeLast(":")
                    val port = persistentTarget.substringAfterLast(":").toInt()
                    persistentSocket = Socket().apply {
                        connect(InetSocketAddress(host, port), 30000)
                        soTimeout = 45000
                    }
                    persistentReader = persistentSocket!!.getInputStream().bufferedReader(Charsets.US_ASCII)
                    record("persistent TCP connected target=$persistentTarget")
                }
                repeat(rounds) { round ->
                record("cycle=${cycle + 1} round=${round + 1} begin")
                val opened = CountDownLatch(3)
                val requests = (1..3).map { request ->
                    pool.submit<Boolean> {
                        Socket().use { raw ->
                            record("cycle=${cycle + 1} round=${round + 1} request=$request TCP begin")
                            raw.connect(InetSocketAddress(destination, 443), 30000)
                            record("request=$request TCP connected")
                            raw.soTimeout = 45000
                            opened.countDown()
                            assertTrue("Three connections open simultaneously", opened.await(35, TimeUnit.SECONDS))
                            val tls = (SSLSocketFactory.getDefault() as SSLSocketFactory).createSocket(raw, "www.wikipedia.org", 443, true) as SSLSocket
                            tls.use {
                                val params = tls.sslParameters
                                params.endpointIdentificationAlgorithm = "HTTPS"
                                tls.sslParameters = params
                                record("request=$request TLS begin")
                                tls.startHandshake()
                                record("request=$request TLS established")
                                tls.outputStream.write("GET / HTTP/1.1\r\nHost: www.wikipedia.org\r\nUser-Agent: Zyrln-Integration-Test/1.0\r\nConnection: close\r\n\r\n".toByteArray())
                                val status = tls.inputStream.bufferedReader().readLine()
                                record("request=$request HTTP status=$status")
                                status?.startsWith("HTTP/1.1 200") == true
                            }
                        }
                    }
                }
                requests.forEach { assertTrue("Certificate-verified HTTPS response through Google relay", it.get(90, TimeUnit.SECONDS)) }
                if (persistentSocket != null) {
                    persistentSocket!!.getOutputStream().write("GET /healthz HTTP/1.1\r\nHost: $persistentTarget\r\nConnection: keep-alive\r\n\r\n".toByteArray())
                    val reader = persistentReader!!
                    val status = reader.readLine()
                    assertTrue("Persistent health HTTP response", status?.startsWith("HTTP/1.1 200") == true)
                    var length = -1
                    while (true) {
                        val header = reader.readLine() ?: error("Persistent connection closed in headers")
                        if (header.isEmpty()) break
                        if (header.startsWith("Content-Length:", true)) length = header.substringAfter(":").trim().toInt()
                    }
                    require(length in 1..4096)
                    val body = CharArray(length)
                    var offset = 0
                    while (offset < length) {
                        val n = reader.read(body, offset, length - offset)
                        require(n > 0) { "Persistent connection closed in body" }
                        offset += n
                    }
                    assertEquals("ok", String(body).trim())
                    record("persistent TCP health passed round=${round + 1}")
                    if (profile.optBoolean("expect_remote_close")) {
                        record("WAITING_FOR_CONTROLLED_EXIT_STOP")
                        val closeStarted = SystemClock.elapsedRealtime()
                        try {
                            assertEquals("Remote failure closes local reader", -1, reader.read())
                        } catch (failure: java.net.SocketTimeoutException) {
                            throw AssertionError("Remote failure left local socket waiting until timeout", failure)
                        } catch (expected: java.io.IOException) {
                            record("Remote failure reported: ${expected.javaClass.simpleName}")
                        }
                        record("Remote close observed after ${SystemClock.elapsedRealtime() - closeStarted}ms")
                    }
                }
                if (profile.optBoolean("browser_check") && round + 1 == rounds) {
                    val browserUrl = "https://www.wikipedia.org/?zyrln-test=" + java.util.UUID.randomUUID().toString()
                    c.startActivity(Intent(Intent.ACTION_VIEW, android.net.Uri.parse(browserUrl)).setPackage("com.android.chrome").addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
                    record("BROWSER_CHECK launched")
                    device.takeScreenshot(File(c.filesDir, "real-relay-browser.png"))
                    device.dumpWindowHierarchy(File(c.filesDir, "real-relay-browser.xml"))
                    // Complete Chrome's observed emulator welcome screen without signing in.
                    device.wait(Until.findObject(By.res("com.android.chrome", "signin_fre_dismiss_button")), 5000)?.click()
                    device.wait(Until.findObject(By.text("No thanks")), 3000)?.click()
                    c.startActivity(Intent(Intent.ACTION_VIEW, android.net.Uri.parse(browserUrl)).setPackage("com.android.chrome").addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
                    val loaded = device.wait(Until.hasObject(By.textContains("The Free Encyclopedia")), 30000)
                    device.takeScreenshot(File(c.filesDir, "real-relay-browser.png"))
                    device.dumpWindowHierarchy(File(c.filesDir, "real-relay-browser.xml"))
                    assertTrue("Chrome renders the Wikipedia page through the VPN", loaded)
                    record("BROWSER_CHECK page passed")
                    c.startActivity(Intent(c, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
                }
                record("cycle=${cycle + 1} round=${round + 1} passed")
                if (networkRecovery && round == 0) {
                    record("NETWORK_LOSS begin")
                    device.executeShellCommand("svc wifi disable")
                    device.executeShellCommand("svc data disable")
                    waitFor("Physical network is actually unavailable") { !physicalNetwork() }
                    record("NETWORK_LOSS confirmed")
                    try {
                        Socket().use { socket ->
                            socket.connect(InetSocketAddress(destination, 443), 8000)
                            socket.soTimeout = 8000
                            socket.outputStream.write(byteArrayOf(1))
                            val received = socket.inputStream.read()
                            assertEquals("Offline stream cannot receive remote data", -1, received)
                        }
                    } catch (expected: java.io.IOException) {
                        record("Offline socket failed: ${expected.javaClass.simpleName}")
                    }
                    device.pressHome()
                    c.startActivity(Intent(c, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
                    assertTrue("Connect control stays responsive offline", connectControl().isEnabled)
                    assertTrue("Service stays running during network loss", Mobile.isRunning())
                    record("NETWORK_LOSS UI responsive")
                    restoreNetwork()
                    waitFor("Physical network restored and validated") { physicalNetwork(validated = true) }
                    record("NETWORK_RESTORED")
                    val recoveryDeadline = SystemClock.elapsedRealtime() + 60000
                    var recovered = false
                    var recoveryAttempt = 0
                    while (!recovered && SystemClock.elapsedRealtime() < recoveryDeadline) {
                        recoveryAttempt++
                        try {
                            Socket().use { raw ->
                                raw.connect(InetSocketAddress(destination, 443), 8000)
                                raw.soTimeout = 8000
                                val tls = (SSLSocketFactory.getDefault() as SSLSocketFactory).createSocket(raw, "www.wikipedia.org", 443, true) as SSLSocket
                                tls.use {
                                    val params = tls.sslParameters
                                    params.endpointIdentificationAlgorithm = "HTTPS"
                                    tls.sslParameters = params
                                    tls.soTimeout = 8000
                                    tls.startHandshake()
                                    tls.outputStream.write("GET / HTTP/1.1\r\nHost: www.wikipedia.org\r\nUser-Agent: Zyrln-Integration-Test/1.0\r\nConnection: close\r\n\r\n".toByteArray())
                                    val status = tls.inputStream.bufferedReader().readLine()
                                    record("Recovery attempt=$recoveryAttempt status=$status")
                                    recovered = status?.startsWith("HTTP/1.1 200") == true
                                }
                            }
                        } catch (expected: java.io.IOException) {
                            record("Recovery attempt=$recoveryAttempt failed=${expected.javaClass.simpleName}")
                        }
                        if (!recovered) SystemClock.sleep(1000)
                    }
                    assertTrue("Real HTTPS recovers within one minute without VPN toggle", recovered && SystemClock.elapsedRealtime() <= recoveryDeadline)
                    record("AUTOMATIC_RECOVERY passed attempts=$recoveryAttempt")
                    assertFalse("Recovery does not enable direct bypass", Mobile.isDirectEnabled())
                    // The next wave must recover automatically, without toggling the VPN.
                }
                if (round + 1 < rounds) SystemClock.sleep(pauseMs)
                }
                persistentSocket?.close()
                persistentSocket = null
                connectControl().click()
                waitFor("Relay VPN removed") { !Mobile.isRunning() && !active() }
                waitFor("Relay stays selected after disconnect") {
                    device.findObject(By.res(c.packageName, "modeHint"))?.text in relayHints
                }
            }
        } catch (failure: Throwable) {
            record("failure=$failure cause=${failure.cause}")
            println("Post-disconnect hint: " + device.findObject(By.res(c.packageName, "modeHint"))?.text)
            device.takeScreenshot(File(c.filesDir, "real-relay-failure.png"))
            device.dumpWindowHierarchy(File(c.filesDir, "real-relay-failure.xml"))
            throw failure
        } finally {
            trace.appendText("NATIVE LOGS\n" + Mobile.getAllLogs().replace(key, "[redacted]").replace(url, "[deployment]"))
            if (networkRecovery) restoreNetwork()
            persistentSocket?.close()
            pool.shutdownNow()
            c.startService(Intent(c, RelayVpnService::class.java).setAction(RelayVpnService.ACTION_STOP))
            waitFor("Service stopped before restoring preferences") { !Mobile.isRunning() && !active() }
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
            file.delete()
        }
    }
}
