package com.zyrln.relay

import androidx.core.content.edit

import android.content.SharedPreferences
import android.content.pm.PackageManager
import org.json.JSONArray

/** Per-app split tunnel settings stored in the main config SharedPreferences. */
object SplitTunnelPrefs {
    const val MODE_OFF = 0
    const val MODE_BYPASS = 1
    const val MODE_ONLY = 2

    private const val KEY_MODE = "split_tunnel_mode"
    private const val KEY_PACKAGES = "split_tunnel_packages"

    data class Config(val mode: Int, val packages: Set<String>)

    fun load(prefs: SharedPreferences): Config = Config(mode(prefs), packages(prefs))

    fun mode(prefs: SharedPreferences): Int = prefs.getInt(KEY_MODE, MODE_OFF)

    fun setMode(prefs: SharedPreferences, mode: Int) {
        prefs.edit { putInt(KEY_MODE, mode) }
    }

    fun packages(prefs: SharedPreferences): Set<String> {
        val raw = prefs.getString(KEY_PACKAGES, "[]") ?: "[]"
        return try {
            val arr = JSONArray(raw)
            buildSet {
                for (i in 0 until arr.length()) {
                    val pkg = arr.optString(i, "").trim()
                    if (pkg.isNotEmpty()) add(pkg)
                }
            }
        } catch (_: Exception) {
            emptySet()
        }
    }

    fun setPackages(prefs: SharedPreferences, packages: Set<String>) {
        val arr = JSONArray()
        packages.sorted().forEach { arr.put(it) }
        prefs.edit { putString(KEY_PACKAGES, arr.toString()) }
    }

    /** Drops uninstalled packages; optionally writes the pruned set back to prefs. */
    fun resolvePackages(
        prefs: SharedPreferences,
        pm: PackageManager,
        persistPrune: Boolean = false,
    ): Set<String> {
        val stored = packages(prefs)
        if (stored.isEmpty()) return emptySet()
        val resolved = stored.filter { pkg ->
            try {
                pm.getApplicationInfo(pkg, 0)
                true
            } catch (_: PackageManager.NameNotFoundException) {
                false
            }
        }.toSet()
        if (persistPrune && (resolved != stored || (resolved.isEmpty() && mode(prefs) != MODE_OFF))) {
            setPackages(prefs, resolved)
            val normalized = normalizeMode(mode(prefs), resolved)
            if (normalized != mode(prefs)) {
                setMode(prefs, normalized)
            }
        }
        return resolved
    }

    fun loadResolved(prefs: SharedPreferences, pm: PackageManager): Config {
        val cfg = load(prefs)
        return Config(cfg.mode, resolvePackages(prefs, pm))
    }

    fun isActive(cfg: Config): Boolean {
        return cfg.mode != MODE_OFF && cfg.packages.isNotEmpty()
    }

    fun normalizeMode(mode: Int, packages: Set<String>): Int {
        return if (mode != MODE_OFF && packages.isEmpty()) MODE_OFF else mode
    }

    fun save(prefs: SharedPreferences, mode: Int, packages: Set<String>) {
        val normalizedMode = normalizeMode(mode, packages)
        setMode(prefs, normalizedMode)
        setPackages(prefs, packages)
    }

    fun summary(prefs: SharedPreferences, offLabel: String, bypassLabel: String, onlyLabel: String): String {
        val cfg = load(prefs)
        if (!isActive(cfg)) return offLabel
        val n = cfg.packages.size
        return when (cfg.mode) {
            MODE_BYPASS -> String.format(bypassLabel, n)
            MODE_ONLY -> String.format(onlyLabel, n)
            else -> offLabel
        }
    }

    fun summaryResolved(
        prefs: SharedPreferences,
        pm: PackageManager,
        offLabel: String,
        bypassLabel: String,
        onlyLabel: String,
    ): String {
        val cfg = loadResolved(prefs, pm)
        if (!isActive(cfg)) return offLabel
        val n = cfg.packages.size
        return when (cfg.mode) {
            MODE_BYPASS -> String.format(bypassLabel, n)
            MODE_ONLY -> String.format(onlyLabel, n)
            else -> offLabel
        }
    }
}
