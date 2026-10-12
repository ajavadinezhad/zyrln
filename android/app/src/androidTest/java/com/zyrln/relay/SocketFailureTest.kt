package com.zyrln.relay

import android.content.Intent
import android.os.SystemClock
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import androidx.test.uiautomator.By
import androidx.test.uiautomator.UiDevice
import androidx.test.uiautomator.Until
import mobile.Mobile
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import java.net.InetSocketAddress
import java.net.Socket
import java.net.SocketTimeoutException
import java.io.IOException
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit

@RunWith(AndroidJUnit4::class)
class SocketFailureTest {
    @Test fun socketFaultsDoNotFreezeVpnAndReconnectWorks() {
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        val context = instrumentation.targetContext
        val device = UiDevice.getInstance(instrumentation)
        fun awaitState(running: Boolean) {
            val deadline = SystemClock.elapsedRealtime() + 15000
            while (Mobile.isRunning() != running && SystemClock.elapsedRealtime() < deadline) SystemClock.sleep(100)
            assertEquals("VPN running state", running, Mobile.isRunning())
        }
        fun toggle() {
            val button = device.wait(Until.findObject(By.res(context.packageName, "btnConnect")), 5000)
            assertNotNull("UI remains responsive", button)
            button.click()
        }
        fun start() {
            toggle()
            if (android.os.Build.VERSION.SDK_INT >= 37) {
                device.wait(Until.findObject(By.res("com.android.permissioncontroller", "permission_allow_button")), 3000)?.click()
            }
            device.wait(Until.findObject(By.res("android", "button1")), 1000)?.let {
                if (it.text.equals("OK", true)) it.click()
            }
            awaitState(true)
        }
        fun request(path: String): String = Socket().use { socket ->
            socket.connect(InetSocketAddress("10.0.2.2", 8765), 5000)
            socket.soTimeout = 3000
            socket.getOutputStream().write("GET $path HTTP/1.0\r\nHost: fixture\r\n\r\n".toByteArray())
            socket.getInputStream().bufferedReader().readText()
        }
        fun healthy() = assertTrue("New traffic succeeds after fault", request("/").contains("zyrln-emulator-fixture"))
        context.startActivity(Intent(context, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TASK))
        val executor = Executors.newFixedThreadPool(8)
        try {
            start()
            // Refused host port must fail without breaking subsequent connections.
            try {
                Socket().use { it.connect(InetSocketAddress("10.0.2.2", 8766), 3000) }
                fail("Unused fixture port unexpectedly accepted connection")
            } catch (expected: IOException) { }
            healthy()
            for (path in listOf("/reset", "/close", "/truncate")) {
                val response = try { request(path) } catch (expected: IOException) { "" }
                if (path == "/truncate") assertFalse("Truncated response remains incomplete", response.length >= 1000)
                else assertFalse("Fault endpoint cannot return success marker", response.contains("zyrln-emulator-fixture"))
                healthy()
            }
            try { request("/stall"); fail("Stalled socket should time out") }
            catch (expected: SocketTimeoutException) { }
            healthy()
            // Several stalled flows must not block the UI or VPN shutdown.
            val pending = (1..4).map { executor.submit<Unit> {
                try { request("/stall") } catch (expected: IOException) { }
            } }
            SystemClock.sleep(500)
            val parallelHealthy = (1..4).map { executor.submit<Unit> { healthy() } }
            parallelHealthy.forEach { it.get(8, TimeUnit.SECONDS) }
            toggle()
            awaitState(false)
            pending.forEach { it.get(8, TimeUnit.SECONDS) }
            start()
            healthy()
            // Backpressure: the host accepts a request but stops reading its upload.
            val upload = Socket()
            try {
                upload.connect(InetSocketAddress("10.0.2.2", 8765), 5000)
                upload.getOutputStream().write("GET /stall HTTP/1.0\r\nHost: fixture\r\n\r\n".toByteArray())
                val writer = executor.submit<Unit> {
                    try {
                        val chunk = ByteArray(65536)
                        repeat(128) { upload.getOutputStream().write(chunk) }
                    } catch (expected: IOException) { }
                }
                SystemClock.sleep(1500)
                healthy() // Unrelated connections must work while this upload is blocked.
                val stopStarted = SystemClock.elapsedRealtime()
                toggle()
                awaitState(false)
                assertTrue("Disconnect blocked by upload for ${SystemClock.elapsedRealtime() - stopStarted} ms", SystemClock.elapsedRealtime() - stopStarted < 8000)
                upload.close()
                writer.get(8, TimeUnit.SECONDS)
            } finally { upload.close() }
            start()
            healthy()
            toggle()
            awaitState(false)
        } finally {
            executor.shutdownNow()
            context.startService(Intent(context, RelayVpnService::class.java).setAction(RelayVpnService.ACTION_STOP))
        }
    }
}
