package com.zyrln.relay

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
import org.junit.Test
import org.junit.runner.RunWith
import java.net.InetSocketAddress
import java.net.Socket
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit

@RunWith(AndroidJUnit4::class)
class VpnLifecycleTest {
    @Test
    fun directVpnCarriesTrafficAndReconnects() {
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        val context = instrumentation.targetContext
        val device = UiDevice.getInstance(instrumentation)
        val cm = context.getSystemService(ConnectivityManager::class.java)
        fun vpnActive(): Boolean = cm.allNetworks.any {
            cm.getNetworkCapabilities(it)?.hasTransport(NetworkCapabilities.TRANSPORT_VPN) == true
        }
        fun awaitCondition(description: String, condition: () -> Boolean) {
            val deadline = SystemClock.elapsedRealtime() + 15000
            while (!condition() && SystemClock.elapsedRealtime() < deadline) SystemClock.sleep(100)
            assertTrue(description, condition())
        }
        // Run on the disposable emulator with scripts/emulator-http-fixture.py on its host.
        context.startActivity(Intent(context, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TASK))
        val traffic = Executors.newFixedThreadPool(6)
        try {
            repeat(6) {
                val connect = device.wait(Until.findObject(By.res(context.packageName, "btnConnect")), 10000)
                assertNotNull("Connect control is visible", connect)
                connect.click()
                if (android.os.Build.VERSION.SDK_INT >= 37) {
                    val allow = device.wait(Until.findObject(By.res("com.android.permissioncontroller", "permission_allow_button")), 3000)
                    allow?.click()
                }
                val permission = device.wait(Until.findObject(By.res("android", "button1")), 2000)
                if (permission != null && permission.text.equals("OK", true)) permission.click()
                awaitCondition("Go proxy started") { Mobile.isRunning() }
                awaitCondition("Android VPN established") { vpnActive() }
                // Independent requests overlap on each reconnect, exercising flow cleanup.
                val openConnections = CountDownLatch(6)
                val requests = (1..12).map {
                    traffic.submit<Boolean> {
                        Socket().use { socket ->
                            socket.connect(InetSocketAddress("10.0.2.2", 8765), 10000)
                            openConnections.countDown()
                            assertTrue("Six TCP connections open together", openConnections.await(10, TimeUnit.SECONDS))
                            socket.soTimeout = 10000
                            socket.getOutputStream().write("GET / HTTP/1.0\r\nHost: 10.0.2.2\r\n\r\n".toByteArray())
                            socket.getInputStream().bufferedReader().readText().contains("zyrln-emulator-fixture")
                        }
                    }
                }
                requests.forEach { assertTrue("Concurrent TCP response traverses active VPN", it.get(15, TimeUnit.SECONDS)) }
                device.findObject(By.res(context.packageName, "btnConnect")).click()
                awaitCondition("Go proxy stopped") { !Mobile.isRunning() }
                awaitCondition("Android VPN removed") { !vpnActive() }
            }
        } finally {
            traffic.shutdownNow()
            context.startService(Intent(context, RelayVpnService::class.java).setAction(RelayVpnService.ACTION_STOP))
        }
    }
}
