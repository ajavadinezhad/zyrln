# Zyrln build verification procedure

Run this procedure for each candidate build. Record the commit or working-tree state, app version, Android image/device, exit type, and results. A passing launch alone is not functional verification. Mark unrun checks as pending rather than passed.

Run `scripts/verify-android-build.sh` for the combined local checks.

## Automated checks

1. Run `go test ./...` and targeted race checks for changed concurrent code.
2. Rebuild the Go mobile AAR before the Android APK. `scripts/android-emulator.sh --build` does both and runs Kotlin JVM tests.
3. Start the loopback fixture with `python3 scripts/emulator-http-fixture.py`.
4. Run `ANDROID_HOME="$HOME/Android/Sdk" ANDROID_SERIAL=emulator-5554 ./gradlew --no-daemon connectedDebugAndroidTest` from android/.
5. Save Android test reports and Go coverage for the candidate. Coverage identifies gaps; it does not replace behavior checks.

## Feature matrix

| Feature | Required behavior | Current verification |
| --- | --- | --- |
| App launch | Native library loads and interface stays usable | Android 9 and Android 14 passed |
| Direct VPN | Establish, transfer TCP, stop, reconnect without leftover VPN | Android 9 and Android 14 passed |
| Socket protection | Rejection prevents outbound connection and records error | Go regression and race checks passed |
| Manual profiles | Save URL/key and retain after recreation | Android 9 and Android 14 passed |
| Clipboard profiles | Import/export round trip, duplicate and invalid input handling | Android 9 and Android 14 passed |
| Profile removal | Cancel, delete inactive, disconnect/delete active | Inactive removal passed on Android 9; active removal pending |
| Theme and language | Recreate activity and preserve configuration | Android 9 and Android 14 passed |
| Split discovery | Preinstalled apps are visible and searchable | Android 9 and Android 14 passed |
| Split routing | All apps, selected bypass, selected only; actual traffic follows selection | Pending |
| Direct Google | Probe/ping and actual Google HTTPS with relay enabled/disabled | Pending real-network check |
| Relay VPN through VPS | Actual app TCP/HTTPS through Google bridge to dedicated VPS | Passed Android 9/14 real traffic and repeated reconnect checks; see TEST_RESULTS.md#repeated-vps-relay-sessions |
| Relay VPN through Cloudflare | Actual supported destinations through Google bridge and Worker | Passed verified HTTPS/concurrent reconnect checks on Android 9/14; long sessions pending |
| Cloudflare destination limits | Unsupported destinations fail clearly; VPS comparison works | Pending test environment |
| Multiple scripts | URL list import, deployment failover, failure diagnostics | Go coverage exists; device integration pending |
| Domestic bypass | Domestic traffic avoids Apps Script and follows routing setting | Go coverage exists; network check pending |
| Diagnostics | Ping, uptime, logs, sharing, useful errors without credential exposure | Pending emulator and real relay |
| Lifecycle | Backgrounding, notification stop, screen lock, rapid reconnect | Pending |
| Network changes | Offline, recovery, Wi-Fi changes, exit downtime | Controlled VPS exit downtime and socket closure passed on Android 14; Wi-Fi/offline changes pending |
| Apps | Browser, YouTube, Telegram, WhatsApp, large transfers | Pending real relay |
| Desktop | HTTP/HTTPS/SOCKS, cert setup, profile sharing, streaming | Go suite passed; functional client checks pending |
| Resource use | Memory, CPU, battery implications, cleanup on disconnect | Baseline emulator host memory observed; app soak pending |
| Protocol limits | General UDP and IPv6 limitations accurately documented | Source reviewed; behavior checks pending |

## Static checks

Android debug and release lint now pass with zero errors and zero warnings each. Removed the unnecessary BIND_VPN_SERVICE uses-permission; the VPN service binding protection remains. The three internal status broadcasts now target the app package. Debug lint is included in scripts/android-emulator.sh --build and therefore the per-build verification procedure. No errors were suppressed.

## Test exits

Use dedicated Cloudflare, Apps Script, and VPS test resources identified by the owner. Store keys outside tracked files. Start with local fixtures before external deployment. Do not silently use production resources. Confirm no charges or account changes beyond the agreed scope.

For each exit record deployment version, authentication checks, successful traffic, failure behavior, request limits, and cleanup. Keep Google as the bridge in all end-to-end checks; a direct exit test alone does not validate that bridge.

## Environment limits

The emulator runs locally with hardware acceleration and 1.5 GB guest RAM. Host consumption includes extra emulator graphics overhead. Stop the emulator and build processes after testing when they are no longer needed. Local success cannot reproduce Iran's filtering; a tester in the affected environment remains necessary.

## Socket failure testing — October 11, 2026
Android 9 direct-VPN fault tests cover refused/reset/closed/truncated/stalled sockets and recovery. A blocked 8 MiB upload reproduced a 28,504 ms disconnect stall. Fixed with a bounded per-flow writer and interruptible shutdown. SocketFailureTest now passes on Android 9 and 14; the per-build checks pass again. See TEST_RESULTS.md#android-socket-failure-checks. Real bridge faults remain untested.

## Desktop browser verification — October 11, 2026
External Edge testing passed direct lifecycle, real HTTP/SOCKS5 HTTPS traffic (204), direct ping, profiles, multi-URL mobile export, theme/language persistence, and log controls. Invalid URL and missing CA errors remained recoverable. Concurrent config/settings writes lost custom listener addresses in 23/100 attempts; this is now fixed with serialized transactions, atomic file replacement, and merged config updates. The live stress check lost 0/100 settings after the fix, and both custom listener ports carried HTTPS successfully. Desktop backend race tests passed but do not cover file consistency. See TEST_RESULTS.md#desktop-web-interface-checks for evidence and remaining checks.

## Android warning review — October 11, 2026
Twelve of the 38 warnings were fixed without suppression. Debug/release lint now report zero errors and 26 warnings. Debug build and JVM tests passed; all three emulator tests passed on Android 9 and 14, including split-label quantities and Persian text. See TEST_RESULTS.md#android-warning-review for all decisions and evidence.

## Complete warning cleanup — October 11, 2026
All 38 original warnings and new findings from the upgraded lint engine are fixed without new suppressions. Debug/release APK builds and JVM tests passed. All three emulator tests passed on Android 9 and 14. Android 17 runtime checks are blocked by a system graphics/task-snapshot crash before instrumentation starts, and remain unverified. Build toolchain and target SDK are updated; minimum Android remains 9. Real-service and physical-device release checks remain pending. See TEST_RESULTS.md#android-warning-review for the full changes, versions and evidence.

## Compact UI and profile regression procedure
For each Android build, run the full connected instrumentation suite on Android 9 and 14 with the local HTTP fixture. Keep only one emulator running at a time.

1. Submit an empty manual profile, then URL only: errors must stay inline and preserve entered text. Save a valid disposable URL/key and verify count, selection and visibility.
2. Import eight distinct disposable Apps Script profiles, including one comma-separated two-endpoint URL. Verify duplicate and malformed imports do not add entries. Scroll and select profiles by their name; check export round-trip and cancel/confirm deletion.
3. Switch theme and English/Persian language. Verify selection, labels, scrolling and compact layout. Removing the last selected relay must restore Google-only guidance.
4. Repeat six Connect/Disconnect cycles and confirm VPN removal. Automated traffic checks must open six sockets together, carry responses, and recover from refusal/reset/close/truncation/stall/blocked upload while healthy traffic continues.
5. Manually enter three extra profiles and repeat six UI connection cycles. Save screenshots, build/device information and test reports; restore prior preferences and stop the emulator afterward.
6. Test deployed Google bridge with both VPS and Cloudflare exits separately before claiming real-relay validation.

Latest results: TEST_RESULTS.md#android-ui-and-profile-checks.

## Real-service procedure
Use scripts/verify-real-relay-android.py with an owner-approved private profile stored outside the repository. It runs two real relay VPN cycles with three concurrent certificate-verified HTTPS requests/cycle and verifies relay selection survives disconnect. Direct bypass must be off. Run on Android 9 and 14 one at a time; never treat the normal skipped opt-in test as a real-service pass. See TEST_RESULTS.md#live-android-relay-integration for the actual deployed versions and limits.


VPS real-service check (October 11, 2026): Google bridge HTTP and concurrent verified HTTPS passed. Android 9 and 14 passed two VPN connection cycles with three simultaneous verified HTTPS requests per cycle. Initial Android 14 handshake timeout is unresolved and retained in the results; repeated and roughly eleven-minute VPS sessions subsequently passed (see TEST_RESULTS.md#repeated-vps-relay-sessions); the original timeout trigger remains unconfirmed. Temporary VPS service is stopped and must be restarted before repeating tests.


## Repeated relay diagnostics
Use scripts/verify-real-relay-android.py --cycles 10 for reconnect repetition. Use --cycles 1 --rounds 14 --pause-seconds 45 for a sustained VPN session. Add --persistent-target with an authorized test exit host:port serving /healthz to hold one TCP connection across idle gaps. The --expect-remote-close option is an operator-controlled dedicated-exit outage check; follow TEST_RESULTS.md#repeated-vps-relay-sessions. Save trace, logcat, host timing and instrumentation output for successes and failures. A skipped or stale instrumentation test cannot count as a live-service pass.


### Desktop relay regression procedure
Use TEST_RESULTS.md#desktop-relays-and-transfers and scripts/verify-real-relay-desktop.py for private-profile live Cloudflare/VPS checks. Require runtime direct-bypass guards. Test page reload while connected, narrow layouts and controlled dedicated exit outage. Selected proxied browser pages and 1/4/8 MiB upload/download integrity passed; repeat the opt-in browser and transfer procedures in TEST_RESULTS.md#desktop-relays-and-transfers. Certificate flows, incremental streaming, >8 MiB limits and physical-network outage remain pending.


### Desktop certificate and adapter recovery
Repeat TEST_RESULTS.md#desktop-certificates-and-adapter-recovery with private profiles: fresh disposable CA lifecycle (12 assertions, real GUI download, no HTTPS-error allowance), dedicated namespace adapter loss/restoration twice per backend (4 cycles), HTTP/SOCKS recovery and separate browser content (2 checks), no reconnect and bypass off. Never toggle the ordinary host adapter using the namespace harness. Clean up temporary trust profiles/processes/namespace and stop the dedicated VPS service. Normal Windows certificate-import wizard, physical Wi-Fi switching and sleep/resume remain platform checks.


### Mobile direct mode
Use TEST_RESULTS.md#mobile-direct-mode and scripts/verify-direct-android.py on Android 9 and 14 separately. Require six verified HTTPS responses and two VPN cycles per OS, no relay/custom CA, active physical-network baselines and restored preferences. Add --browser on Android 14, require visible page content and inspect screenshot. Stop one emulator before starting another. Keep transient failures; the initial Android 14 timeout remains unresolved, and baseline-primed passes do not prove unprimed cold-start or Iran bypass reliability.


### Quota and connection edge regressions
Follow TEST_RESULTS.md#quota-notices-and-android-lifecycle. Run scripts/verify-connection-edges-android.py on disposable Android 9/14 only; --with-faults requires the local fixture and expects six test methods. Require simulated explicit quota vs unknown HTML, notice reset/precedence, rapid taps/profile switching, permission denial/recreation/backgrounding, real second-app VPN takeover and deleted-profile fallback. Never run the preference-changing suite or install its second-VPN fixture on a personal phone. Quota remaining is unavailable; exact balance/progress is not established. Latest app installed on S25; physical handovers and current-build live relay recovery remain separate.
