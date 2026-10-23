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
- v2.1-pre-1 was published October 15, 2026. Android and Go CI passed for its release commit. Check current CI before making new claims.
- Android 9/14 emulator tests and earlier live relay checks are recorded in TEST_RESULTS.md. Physical-device and restricted-network coverage is incomplete.
- Android uses a new permanent release signing key. Users upgrading from v2.0 must export configs and reinstall. Reuse the existing private key for future releases; never commit it or its passwords.
- Documentation separates app use from relay setup in both English and Persian. Keep the two languages aligned and avoid duplicating setup instructions.

## Shared memory
- Read PROJECT_MEMORY.md for the origin story, Google bridge constraint, release history, user feedback, and current technical findings.

- Run scripts/verify-android-build.sh for local Android build verification; use RELEASE_CHECKLIST.md for remaining real-service checks.
