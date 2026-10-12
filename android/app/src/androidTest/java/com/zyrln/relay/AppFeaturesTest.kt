package com.zyrln.relay

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import androidx.test.uiautomator.Direction
import androidx.test.uiautomator.By
import androidx.test.uiautomator.UiDevice
import androidx.test.uiautomator.Until
import org.json.JSONArray
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class AppFeaturesTest {
    @Test
    fun profilesAndInterfaceControls() {
        val i = InstrumentationRegistry.getInstrumentation()
        val c = i.targetContext
        val prefs = c.getSharedPreferences("config", Context.MODE_PRIVATE)
        val saved = prefs.getString("configs", "[]")
        val savedSplit = SplitTunnelPrefs.load(prefs)
        val d = UiDevice.getInstance(i)
        val clip = c.getSystemService(ClipboardManager::class.java)
        val url = "https://127.0.0.1:1/emulator-test"
        val key = "disposable-test-key"
        fun button(id: String): androidx.test.uiautomator.UiObject2 {
            // Theme/language changes recreate the activity; wait before taking a new handle.
            i.waitForIdleSync()
            d.waitForIdle()
            return d.wait(Until.findObject(By.res(c.packageName, id)), 10000)
                ?: error("Control missing: $id")
        }
        fun configs() = JSONArray(prefs.getString("configs", "[]"))
        prefs.edit().putString("configs", "[]").commit()
        SplitTunnelPrefs.save(prefs, SplitTunnelPrefs.MODE_OFF, emptySet())
        c.startActivity(Intent(c, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TASK))
        try {
            button("btnManualEntry").click()
            assertTrue(d.wait(Until.hasObject(By.clazz("android.widget.EditText")), 5000))
            val fields = d.findObjects(By.clazz("android.widget.EditText"))
            assertEquals(2, fields.size)
            d.findObject(By.res("android", "button1")).click()
            assertTrue("Missing fields keep setup open", d.hasObject(By.clazz("android.widget.EditText")))
            assertEquals("Incomplete setup creates no profile", 0, configs().length())
            d.waitForIdle()
            fields[0].text = url
            d.findObject(By.res("android", "button1")).click()
            assertTrue("Missing key preserves URL and dialog", d.hasObject(By.text(url)))
            assertEquals(0, configs().length())
            d.waitForIdle()
            fields[1].text = key
            d.waitForIdle()
            d.findObject(By.res("android", "button1")).click()
            i.waitForIdleSync()
            assertEquals("Complete form saves a profile", 1, configs().length())
            assertTrue(d.wait(Until.hasObject(By.text("127.0.0.1:1")), 5000))
            d.findObject(By.desc("Export to Clipboard")).click()
            var exported = ""
            i.runOnMainSync { exported = clip.primaryClip!!.getItemAt(0).text.toString() }
            assertEquals(RelayConfig(url, key), ConfigUtils.parseImportText(exported))
            button("btnImportConfig").click()
            assertEquals("Duplicate import must not add another profile", 1, configs().length())
            i.runOnMainSync { clip.setPrimaryClip(ClipData.newPlainText("test", "invalid config")) }
            button("btnImportConfig").click()
            assertEquals("Invalid import must not alter profiles", 1, configs().length())
            button("btnTheme").click()
            button("btnManualEntry")
            assertEquals("Selected relay survives theme recreation", c.getString(R.string.mode_relay_hint), button("modeHint").text)
            button("btnLanguage").click()
            button("btnManualEntry")
            button("btnLanguage").click()
            button("btnManualEntry")
            assertEquals("Selected relay survives language recreation", c.getString(R.string.mode_relay_hint), button("modeHint").text)
            button("btnSplitTunnel").click()
            d.wait(Until.findObject(By.text("Bypass selected apps")), 5000).parent.click()
            val search = button("searchApps")
            search.text = "YouTube"
            assertTrue("Preinstalled YouTube is discoverable", d.wait(Until.hasObject(By.text("YouTube").clazz("android.widget.TextView")), 5000))
            // Accessibility can omit layout containers; choose the actual clickable ancestor.
            var row = d.findObject(By.text("YouTube").clazz("android.widget.TextView"))
            while (!row.isClickable) row = row.parent
            row.click()
            assertTrue(d.wait(Until.hasObject(By.clazz("android.widget.CheckBox").checked(true)), 5000))
            button("btnBack").click()
            button("btnManualEntry")
            assertTrue("Split selection persists", SplitTunnelPrefs.packages(prefs).contains("com.google.android.youtube"))
            assertTrue("One selected app uses singular label", button("btnSplitTunnel").text.endsWith("Bypass 1 app"))
            button("btnSplitTunnel").click()
            button("searchApps").text = "Settings"
            assertTrue(d.wait(Until.hasObject(By.text("Settings").clazz("android.widget.TextView")), 5000))
            d.waitForIdle()
            var settingsRow = d.findObject(By.text("Settings").clazz("android.widget.TextView"))
            while (!settingsRow.isClickable) settingsRow = settingsRow.parent
            settingsRow.click()
            assertTrue(d.wait(Until.hasObject(By.clazz("android.widget.CheckBox").checked(true)), 5000))
            button("btnBack").click()
            assertTrue("Two selected apps use plural label", button("btnSplitTunnel").text.endsWith("Bypass 2 apps"))
            button("btnLanguage").click()
            assertTrue("Persian split summary remains translated", button("btnSplitTunnel").text.contains("عبور"))
            button("btnLanguage").click()

            assertEquals("Profiles survive activity recreation", 1, configs().length())
            button("btnManualEntry") // Wait for language-triggered activity recreation.
            val exportControl = d.wait(Until.findObject(By.desc("Export to Clipboard")), 5000)
                ?: error("Profile export control missing after recreation")
            exportControl.parent.children.last().click()
            d.wait(Until.findObject(By.res("android", "button1")), 5000).click()
            // Removal is committed after the card's exit animation, not when its dialog closes.
            val deadline = android.os.SystemClock.elapsedRealtime() + 5000
            while (configs().length() != 0 && android.os.SystemClock.elapsedRealtime() < deadline) {
                android.os.SystemClock.sleep(100)
            }
            assertEquals("Inactive profile removed", 0, configs().length())
            assertEquals("Removing the selected relay restores Google-only guidance", c.getString(R.string.mode_direct_hint), button("modeHint").text)
        } catch (failure: Throwable) {
            val directory = c.getExternalFilesDir(null)!!
            d.takeScreenshot(java.io.File(directory, "profile-failure.png"))
            d.dumpWindowHierarchy(java.io.File(directory, "profile-failure.xml"))
            throw failure
        } finally {
            prefs.edit().putString("configs", saved).commit()
            SplitTunnelPrefs.save(prefs, savedSplit.mode, savedSplit.packages)
        }
    }
    @Test
    fun multipleImportsSelectionAndRemoval() {
        val i = InstrumentationRegistry.getInstrumentation()
        val c = i.targetContext
        val d = UiDevice.getInstance(i)
        val prefs = c.getSharedPreferences("config", Context.MODE_PRIVATE)
        val saved = prefs.getString("configs", "[]")
        val clip = c.getSystemService(ClipboardManager::class.java)
        fun button(id: String) = d.wait(Until.findObject(By.res(c.packageName, id)), 10000)
            ?: error("Missing control: $id")
        fun count() = JSONArray(prefs.getString("configs", "[]")).length()
        fun clipboard(text: String) { i.runOnMainSync { clip.setPrimaryClip(ClipData.newPlainText("test profiles", text)) } }
        fun urlFor(port: Int): String {
            val primary = "https://script.google.com/macros/s/AKfycb_DisposableEmulatorProfile_000$port/exec"
            return if (port == 8) "$primary,https://script.google.com/macros/s/AKfycb_DisposableEmulatorBackup_0008/exec" else primary
        }
        fun profile(port: Int): androidx.test.uiautomator.UiObject2 {
            val selector = By.text(ConfigUtils.configLabel(urlFor(port)))
            val scroll = button("configScroll")
            scroll.scrollUntil(Direction.UP, Until.hasObject(By.text(c.getString(R.string.direct_profile_label, c.getString(R.string.direct_only_label)))))
            scroll.scrollUntil(Direction.DOWN, Until.hasObject(selector))
            d.waitForIdle()
            return d.wait(Until.findObject(selector), 5000) ?: error("Profile $port not discoverable")
        }
        prefs.edit().putString("configs", "[]").commit()
        c.startActivity(Intent(c, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TASK))
        try {
            for (port in 1..8) {
                clipboard(ConfigUtils.toExportJson(urlFor(port), "disposable-key-$port"))
                button("btnImportConfig").click()
                i.waitForIdleSync()
                assertEquals("Each valid import adds one profile", port, count())
            }
            clipboard(ConfigUtils.toExportJson(urlFor(8), "disposable-key-8"))
            button("btnImportConfig").click()
            assertEquals("Duplicate does not add a ninth entry", 8, count())
            clipboard("{bad json}")
            button("btnImportConfig").click()
            assertEquals("Malformed import preserves all entries", 8, count())
            android.os.SystemClock.sleep(6500) // Let the system clipboard preview leave the profile list.
            for (port in listOf(1, 8, 3, 7)) {
                profile(port).click() // Tap the name; the card center can contain the details button.
                assertTrue(d.wait(Until.hasObject(By.text(c.getString(R.string.mode_relay_hint))), 5000))
                assertEquals(c.getString(R.string.connect_action), button("btnConnect").text)
            }
            val last = profile(8)
            last.parent.children.last().click()
            d.wait(Until.findObject(By.res("android", "button2")), 5000).click()
            assertEquals("Cancel retains profile", 8, count())
            profile(8).parent.children.last().click()
            d.wait(Until.findObject(By.res("android", "button1")), 5000).click()
            val deadline = android.os.SystemClock.elapsedRealtime() + 5000
            while (count() != 7 && android.os.SystemClock.elapsedRealtime() < deadline) android.os.SystemClock.sleep(100)
            assertEquals("Removal changes only one profile", 7, count())
            assertTrue(JSONArray(prefs.getString("configs", "[]")).toString().contains("DisposableEmulatorProfile_0001"))
        } catch (failure: Throwable) {
            val directory = c.getExternalFilesDir(null)!!
            d.takeScreenshot(java.io.File(directory, "profile-failure.png"))
            d.dumpWindowHierarchy(java.io.File(directory, "profile-failure.xml"))
            throw failure
        } finally {
            prefs.edit().putString("configs", saved).commit()
        }
    }

}
