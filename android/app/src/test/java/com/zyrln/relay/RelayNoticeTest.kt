package com.zyrln.relay

import org.junit.Assert.*
import org.junit.Test

class RelayNoticeTest {
    @Test fun explicitQuotaIsDistinctFromUnknownHtml() {
        assertEquals(R.string.relay_quota_notice, RelayNotice.messageResource("tunnel rx: Apps Script quota exceeded"))
        assertEquals(R.string.relay_quota_notice, RelayNotice.messageResource("Service invoked too many times for one day: urlfetch"))
        assertEquals(R.string.relay_unavailable_notice, RelayNotice.messageResource("Apps Script returned HTML instead of JSON (quota/deploy issue)"))
        assertNull(RelayNotice.messageResource("destination quota exceeded"))
        assertNull(RelayNotice.messageResource("Service invoked too many times in a short time"))
        assertNull(RelayNotice.messageResource("connection refused"))
        assertNull(RelayNotice.messageResource("Limit exceeded: URLFetch URL Length."))
        assertEquals(R.string.relay_quota_notice, RelayNotice.messageResource("Exception: Service invoked too many times: UrlFetch."))
        assertNull(RelayNotice.messageResource("Service invoked too many times in a short time: UrlFetch."))
    }
}
