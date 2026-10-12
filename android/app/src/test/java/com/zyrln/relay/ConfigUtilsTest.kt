package com.zyrln.relay

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class ConfigUtilsTest {
    @Test
    fun parseImportText_acceptsPlainJson() {
        val config = ConfigUtils.parseImportText(
            """{"url":"https://script.google.com/macros/s/ABC/exec","key":"secret"}"""
        )

        assertEquals("https://script.google.com/macros/s/ABC/exec", config.url)
        assertEquals("secret", config.key)
    }

    @Test
    fun parseImportText_extractsJsonFromSurroundingText() {
        val config = ConfigUtils.parseImportText(
            """
            Copy this into Zyrln:
            {"url":"https://script.google.com/macros/s/ABC/exec","key":"secret"}
            done
            """.trimIndent()
        )

        assertEquals("https://script.google.com/macros/s/ABC/exec", config.url)
        assertEquals("secret", config.key)
    }

    @Test
    fun parseImportText_acceptsUppercaseKeysAndRemovesUrlWhitespace() {
        val config = ConfigUtils.parseImportText(
            """{"URL":"https://script.google.com/macros/s/ABC\n /exec","KEY":"  secret  "}"""
        )

        assertEquals("https://script.google.com/macros/s/ABC/exec", config.url)
        assertEquals("secret", config.key)
    }

    @Test
    fun parseImportText_rejectsMissingFields() {
        assertThrows(IllegalArgumentException::class.java) {
            ConfigUtils.parseImportText("""{"url":"https://script.google.com/macros/s/ABC/exec"}""")
        }
    }

    @Test
    fun toExportJson_roundTripsWithParseImportText() {
        val url = "https://script.google.com/macros/s/ABC/exec"
        val key = "secret"
        val exported = ConfigUtils.toExportJson(url, key)
        val parsed = ConfigUtils.parseImportText(exported)
        assertEquals(url, parsed.url)
        assertEquals(key, parsed.key)
    }

    @Test
    fun toExportJson_matchesDesktopFormat() {
        val url = "https://script.google.com/macros/s/AKfycb/exec"
        val key = "fixture-only-export-key-not-for-authentication"
        val exported = ConfigUtils.toExportJson(url, key)
        assertEquals("""{"url":"$url","key":"$key"}""", exported)
        assertEquals(-1, exported.indexOf("\\/"))
    }

    @Test
    fun configLabel_usesStableWordLabelForAppsScriptIds() {
        val one = ConfigUtils.configLabel("https://script.google.com/macros/s/ABCDEFGHIJK/exec")
        val two = ConfigUtils.configLabel("https://script.google.com/macros/s/ABCDEFGHIJK/exec")

        assertEquals(one, two)
        assertNotEquals("script.google.com", one)
    }

    @Test
    fun configLabel_fallsBackToHostnameForNonAppsScriptUrl() {
        assertEquals("example.com", ConfigUtils.configLabel("https://www.example.com/path"))
    }

    @Test
    fun configLabel_usesFirstUrlFromCommaSeparatedList() {
        assertEquals(
            ConfigUtils.configLabel("https://script.google.com/macros/s/ABCDEF/exec"),
            ConfigUtils.configLabel("https://script.google.com/macros/s/ABCDEF/exec,https://example.com")
        )
    }
}
