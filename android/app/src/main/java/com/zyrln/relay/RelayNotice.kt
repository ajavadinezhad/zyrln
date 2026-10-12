package com.zyrln.relay

import java.util.Locale

/** Only explicit daily/quota failures indicate exhaustion; generic HTML does not. */
object RelayNotice {
    fun messageResource(message: String): Int? {
        val m = message.lowercase(Locale.ROOT)
        val bridge = m.contains("apps script") || m.contains("appsscript")
        if (m.contains("service invoked too many times for one day") ||
            m.contains("service invoked too many times: urlfetch") ||
            m.contains("service using too much computer time for one day") ||
            m.contains("exception: bandwidth quota exceeded") ||
            (bridge && (m.contains("quota exceeded") || m.contains("daily quota")))) {
            return R.string.relay_quota_notice
        }
        if (bridge && m.contains("html instead of json")) return R.string.relay_unavailable_notice
        return null
    }
}
