# Zyrln shared context

## Working agreement
- Work directly with the user; use other agents only when explicitly requested.
- Use current source and user-approved decisions; do not import previous Claude notes.
- Preserve existing uncommitted work unless the user requests changes to it.
- Keep project instructions concise; maintain substantial shared context in PROJECT_MEMORY.md. Distinguish verified facts from proposals.

## Verified facts
- Shared Go engine, desktop browser interface, Android Kotlin VPN client.
- Desktop relays HTTP requests; Android tunnels TCP through Apps Script and an exit relay.
- Existing local Android changes include manual profile entry and a clearer notification.

## Current state
- The last full Go suite passed on October 11, 2026; subsequent quota-classification changes passed targeted tunnel/Apps Script tests. Rerun the full suite for a release.
- Android 9/14 passed current connection/quota lifecycle tests. Earlier real Cloudflare/VPS checks passed; current-build live recovery and Iran filtering remain unverified. S25 has the updated APK and its notice was inspected; user-reported handover is separate from automated proof. See TEST_RESULTS.md for current status and historical evidence.
- Android socket protection now stops a dial when protection fails and logs a clear error. Regression tests cover failure, success, and callback clearing. Emulator behavior passed; physical socket-failure coverage remains pending.
- Expand this context with decisions agreed with the user.

## Shared memory
- Read PROJECT_MEMORY.md for the origin story, Google bridge constraint, release history, user feedback, and current technical findings.

- Run scripts/verify-android-build.sh for local Android build verification; use RELEASE_CHECKLIST.md for remaining real-service checks.
