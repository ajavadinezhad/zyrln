# Zyrln testing results

This is the single test-results document. The summary reflects the latest verified state on October 11, 2026; the detailed records below are historical snapshots. Earlier pending checks, service-stop statements and coverage percentages must be read in their original context. Use RELEASE_CHECKLIST.md for per-build steps and PROJECT_MEMORY.md for shared decisions.

## Recorded status — October 11, 2026

| Area | Verified result | Remaining limits |
| --- | --- | --- |
| Android compatibility and lint | Android 9/14 emulator checks; current APK/JVM/debug and release lint passed | Physical-device coverage is partial; Android 17 emulator runtime unverified |
| Quota and lifecycle | New edge tests passed on 9/14; Android 14 also passed existing traffic/socket/UI tests, six methods total | Quota warning is error-based; no remaining-quota API or progress counter |
| Phone | Current APK installed on S25 preserving profiles; visible unknown-quota notice verified | User reports network handover works; latest exhaustion report is not independently confirmed; requested large upload/download test remains paused and unrun |
| Live Android relays | Prior Cloudflare/VPS verified HTTPS, reconnect and Android 14 network recovery checks passed | Current-build live recovery was not repeated; initial Android 14 timeout remains unexplained; Cloudflare cannot tunnel to its own IP ranges |
| Desktop | Both exits passed browser rendering, bounded transfers, certificate lifecycle and isolated adapter recovery | Physical Windows trust/network checks and larger/incremental streaming remain unverified |
| Coverage | Earlier Go statement baseline was 67.2% | It is not a current-tree percentage or Android Kotlin coverage measurement |

At the end of the October 11 session, the dedicated VPS relay was running, the temporary phone transfer server and emulators were stopped, and phone upload/download tests had not run. These are historical observations, not today’s service status. No tests that clear profiles were run on the personal phone.

## Detailed verification records

- [Go coverage baseline](#go-coverage-baseline)
- [Android compatibility and build history](#android-compatibility-and-build-history)
- [Android lint fixes](#android-lint-fixes)
- [Android warning review](#android-warning-review)
- [Android socket failure checks](#android-socket-failure-checks)
- [Android UI and profile checks](#android-ui-and-profile-checks)
- [Live Android relay integration](#live-android-relay-integration)
- [Repeated VPS relay sessions](#repeated-vps-relay-sessions)
- [Android network recovery](#android-network-recovery)
- [Mobile direct mode](#mobile-direct-mode)
- [Quota notices and Android lifecycle](#quota-notices-and-android-lifecycle)
- [Desktop web interface checks](#desktop-web-interface-checks)
- [Desktop relays and transfers](#desktop-relays-and-transfers)
- [Desktop certificates and adapter recovery](#desktop-certificates-and-adapter-recovery)

## Go coverage baseline

Historical verification record; later checks may supersede its pending items.

<details>
<summary>Evidence, failures and repeatable commands</summary>

Measured October 11, 2026 after the socket protection fix.

Aggregate Go statement coverage is **67.2%**, using shared instrumentation across all packages. All tested packages passed. An initial plain coverage run lacked the covdata tool for packages with no tests; the completed run included those packages through -coverpkg=./... while running packages with tests explicitly.

Package-local coverage from the initial run:

| Package | Statement coverage |
| --- | ---: |
| Desktop | 48.4% |
| Mobile Go binding | 72.5% |
| Apps Script relay | 73.3% |
| Connection helpers | 74.1% |
| Exit relay | 72.8% |
| HTTPS proxy | 65.6% |
| Protected dialer | 90.9% |
| Routing | 87.0% |
| Android packet engine | 37.7% |
| Tunnel relay | 72.0% |

Cross-package coverage for the changed path: mobile SetSocketProtector 88.9%, core forwarding wrapper 100%, netdial callback registration 100%, ProtectedDialer 87.5%. Tests verify that rejection aborts dialing, success allows it, clearing the callback allows ordinary dialing, and the mobile binding records rejection in its log. Race checks passed for netdial and mobile.

The largest meaningful gap is relay/tun/tcp.go: TCP flow connection, segment handling, pumping, and teardown functions had zero coverage. DNS resolution also had zero coverage. These paths deserve behavior tests before expanding the feature set. Coverage counts executed statements; it does not establish real network compatibility.

Android Kotlin has unit tests for configuration utilities and split preferences. Its build configuration does not enable a coverage report. Kotlin coverage and actual VpnService/device behavior were not measured in this run. The Android generated AAR and debug APK have now been rebuilt for this fix. JVM tests and emulator behavior tests run separately from this Go coverage measurement; they do not provide a Kotlin coverage percentage.

Raw aggregate profile: /tmp/zyrln-all-coverage.out (temporary).

</details>

## Android compatibility and build history

Historical verification record; later checks may supersede its pending items.

<details>
<summary>Evidence, failures and repeatable commands</summary>

Verified October 11, 2026 on the current working tree. Changes remain local and are not published.

Android minimum SDK is now 28 (Android 9). The optional system HTTP proxy hint runs only on Android 10 and newer. Android 9 uses the existing TCP VPN forwarder. Debug builds include native x86_64 code for emulator testing; release phone architectures are preserved.

| Check | Android 9 | Android 14 |
| --- | --- | --- |
| Launch and native library | Passed | Passed |
| Manual profile save | Passed | Passed |
| Export/import, duplicates, invalid input | Passed | Passed |
| Theme and language recreation | Passed | Passed |
| Preinstalled YouTube search | Passed | Passed |
| Split selection persistence | Passed | Passed |
| Inactive profile removal | Passed | Passed |
| VPN establish, TCP response, disconnect, reconnect | Passed | Passed |

Each device ran three instrumentation tests with zero failures in the final runs. Raw results are saved beside this report as android9-test-results.xml and android14-test-results.xml. The combined local build verification command also passed end to end on Android 9: full Go suite, targeted race checks, Go mobile AAR rebuild, debug APK build, JVM unit tests, and emulator tests. The generated APK declares minimum SDK 28.

The HTTP test fixture uses a concurrent server so idle connections do not block other requests. Tests wait for profile deletion to finish its animation. The split-app locator distinguishes the app label from the search field containing the same text. These fixture and test corrections do not change product behavior.

Android debug and release lint now pass with zero errors and zero warnings each. The four manifest/broadcast errors were fixed without suppression; see TEST_RESULTS.md#android-lint-fixes for the current checks.

Real Google Apps Script plus VPS and Cloudflare exits have not been deployed or verified. Split selection persistence is tested; actual per-app routing is still pending. The remaining feature checks are tracked in RELEASE_CHECKLIST.md. No physical Android 9 phone or Iranian network has been tested.

Run scripts/verify-android-build.sh from the Zyrln repository in Ubuntu. It defaults to the Android 9 emulator, builds and tests, then reinstalls and opens the app. The guide is EMULATOR_TESTING.md. Only one emulator runs at a time; Gradle build processes exit afterward.

Screenshot removed during the owner-approved cleanup (zyrln-android9.png).

Socket fault scenarios now pass on both Android 9 and Android 14 after the blocked-upload shutdown fix. See TEST_RESULTS.md#android-socket-failure-checks and the socket-failure-android9-fixed.xml / socket-failure-android14-fixed.xml reports for the latest three-test runs.

#### Android warning review — October 11, 2026
Twelve of the 38 warnings were fixed without suppression. Debug/release lint now report zero errors and 26 warnings. Debug build and JVM tests passed; all three emulator tests passed on Android 9 and 14, including split-label quantities and Persian text. See TEST_RESULTS.md#android-warning-review for all decisions and evidence.

#### Complete warning cleanup — October 11, 2026
All 38 original warnings and new findings from the upgraded lint engine are fixed without new suppressions. Debug/release APK builds and JVM tests passed. All three emulator tests passed on Android 9 and 14. Android 17 runtime checks are blocked by a system graphics/task-snapshot crash before instrumentation starts, and remain unverified. Build toolchain and target SDK are updated; minimum Android remains 9. Real-service and physical-device release checks remain pending. See TEST_RESULTS.md#android-warning-review for the full changes, versions and evidence.

#### Compact Android UI and import testing — October 11, 2026
Owner prefers smaller elements and an obvious imported-entry list. Compact artwork/spacing preserves touch targets; Import/manual actions now precede the list, saved count is shown, and new profiles are selected/revealed automatically. Inline manual-form errors preserve data; locale/theme recreation retains selection; last-profile removal restores direct guidance. Background relay prewarming avoids blocking UI. Final debug/release builds, JVM tests and lint passed (zero issues). Four instrumentation tests passed on each Android 9/14 emulator, followed by a final focused profile test on each. Eight disposable Apps Script-shaped imports include a comma-separated profile. Six VPN cycles/device carried 144 total local-fixture responses, with six sockets opened concurrently. Socket-fault recovery passed. Three manually entered profiles and six additional UI cycles passed on Android 14; compact EN/FA screenshots reviewed. Original emulator preferences restored; devices stopped. Real Google/VPS/Cloudflare and physical-phone checks remain pending. See TEST_RESULTS.md#android-ui-and-profile-checks and the repeatable RELEASE_CHECKLIST.md procedure.

#### Live Cloudflare-backed Google relay — October 11, 2026
Opt-in RealRelayTest passed on Android 9 and 14: two VPN cycles/device, three concurrent sockets/cycle, hostname/certificate-verified HTTPS 200, direct bypass off. Reconnect selection reset fixed; locale-aware post-disconnect assertion corrected. Release build/JVM tests and both lint variants pass (zero issues). VPS direct tests pass but Google → VPS awaits owner authorization. See TEST_RESULTS.md#live-android-relay-integration.


#### VPS Google bridge validation — October 11, 2026
Owner completed Google authorization and supplied the deployment URL. Auth rejection, ping, HTTP fetch and four simultaneous certificate-verified HTTPS tunnels passed through Google to the dedicated VPS. Android 14 initially timed out during a TLS handshake; unchanged diagnostic rerun passed both connection cycles and six HTTPS responses. Android 9 also passed both cycles and six responses. Initial timeout is unresolved, not a fixed bug. Evidence is in TEST_RESULTS.md#live-android-relay-integration and the workspace reports. Private profile remains outside the repository at ~/.config/zyrln-tests/vps.json. Emulators stopped, preferences restored, temporary zyrln-integration-test service stopped; deployment and isolated server files retained for reuse. No production code changed during this validation.


Repeated VPS Google-bridge tests and roughly eleven-minute sessions passed on Android 9 and 14. Android 9 held exactly one TCP health connection across fourteen rounds and idle gaps, alongside 42 certificate-verified HTTPS responses. Controlled exit-stop closure passed on Android 14. See TEST_RESULTS.md#repeated-vps-relay-sessions for the diagnosed DNS fault, reproduced local-pipe timeout, fixes, retained failed evidence and remaining limitations.

</details>

## Android lint fixes

Historical verification record; later checks may supersede its pending items.

<details>
<summary>Evidence, failures and repeatable commands</summary>

Verified October 11, 2026. All four previously reported Android lint errors are fixed without suppressions.

- Removed the BIND_VPN_SERVICE uses-permission request. The VPN service still declares android:permission="android.permission.BIND_VPN_SERVICE" to protect system binding.
- ACTION_STARTED, ACTION_STOPPED, and ACTION_ERROR broadcasts now target the app's own package, matching the non-exported runtime receivers. The error payload is preserved.
- scripts/android-emulator.sh --build now includes lintDebug, making it part of scripts/verify-android-build.sh.

| Verification | Result |
| --- | --- |
| lintDebug | Passed: 0 errors, 0 warnings |
| lintRelease | Passed: 0 errors, 0 warnings |
| assembleDebug | Passed |
| testDebugUnitTest | Passed |
| Android 9 VPN lifecycle | Passed: TCP traffic, disconnect, reconnect, two cycles |
| Android 14 VPN lifecycle | Passed: same checks, two cycles |
| Shell syntax and diff whitespace | Passed |

All 38 original warnings have now been reviewed; all are now fixed. See TEST_RESULTS.md#android-warning-review for the complete cleanup and Android 9/14 regression results. Emulator lifecycle testing checks normal start/stop behavior after the broadcast changes; it does not claim coverage of every service error branch or real Google bridge/exit behavior. Both emulators were stopped after verification to reclaim memory.

Raw reports are saved beside this document: android-lint-debug.xml, android-lint-release.xml, android-lint-lifecycle-api28.xml, android-lint-lifecycle-api34.xml.

Build verification command from the Android directory in Ubuntu:

```sh
ANDROID_HOME="$HOME/Android/Sdk" ./gradlew --no-daemon lintDebug lintRelease assembleDebug testDebugUnitTest
```

#### Complete warning cleanup — October 11, 2026
All 38 original warnings and new findings from the upgraded lint engine are fixed without new suppressions. Debug/release APK builds and JVM tests passed. All three emulator tests passed on Android 9 and 14. Android 17 runtime checks are blocked by a system graphics/task-snapshot crash before instrumentation starts, and remain unverified. Build toolchain and target SDK are updated; minimum Android remains 9. Real-service and physical-device release checks remain pending. See TEST_RESULTS.md#android-warning-review for the full changes, versions and evidence.

</details>

## Android warning review

Historical verification record; later checks may supersede its pending items.

<details>
<summary>Evidence, failures and repeatable commands</summary>

Updated October 11, 2026. All 38 original warnings are fixed. Both debug and release lint now report **zero errors and zero warnings**, with no new suppressions or baseline. New findings exposed by the newer lint engine were also addressed.

#### Original findings

| Finding | Original count | Fix |
| --- | ---: | --- |
| PluralsCandidate | 2 | English/Persian quantity resources for split-tunnel labels. |
| Overdraw | 2 | Removed duplicate root backgrounds; the theme supplies the background. |
| Autofill | 1 | Disabled autofill for installed-app search. |
| SetTextI18n | 4 | Formatted resources for version, uptime and direct-profile labels. |
| HardcodedText | 2 | Language badge and initial uptime use resources. |
| ObsoleteSdkInt | 1 | Removed the redundant notification-channel guard; minimum Android is 9. |
| GradleDependency | 8 | Updated all eight Android libraries and test dependencies. |
| UnusedResources | 9 | Removed unused colors, drawables, string and style. Reused launcher background/foreground resources in the adaptive icon. |
| Launcher icon shape, sizing, duplicates | 8 | Replaced ten duplicated density bitmaps with adaptive icons referencing one unchanged copy of the existing artwork. Launchers apply their own shape. |
| TypographyEllipsis | 1 | The JSON example now has explicit deployment URL and key placeholders instead of dots. |

#### Compatible build upgrade and additional findings

- Android Gradle Plugin 9.1.1, Gradle 9.8.1, built-in Kotlin, Java bytecode target 11 and compile/target SDK 37. Minimum SDK remains 28 (Android 9). Debug x86_64 and release phone architectures are preserved.
- Core 1.19.1, AppCompat 1.8.0, Material 1.14.0, ConstraintLayout 2.2.2, Lifecycle service 2.11.0, test runner 1.7.0, test JUnit extension 1.3.0, UI Automator 2.4.0 and JVM JSON 20260814.
- Migrated APK naming to the supported variant API and removed the obsolete Jetifier setting.
- Rebuilt Go mobile libraries with a 16 KB maximum page-size linker flag. Makefile, the emulator build script and GitHub CI retain this flag for future builds.
- Added monochrome icon artwork for themed launchers and removed the redundant v26 resource qualifier.
- Preference writes use KTX helpers, preserving their asynchronous apply behavior.
- Both activities account for system bars and display cutouts with the newer target.
- Android 17 direct connections request local-network permission. Denial displays a translated message and still proceeds to VPN setup; local access stays blocked while public internet can remain usable. Older Android versions skip this request.
- Emulator builds now run both debug and release lint; the release Makefile build also runs lint and JVM tests.

Official toolchain compatibility: https://developer.android.com/build/releases/agp-9-1-0-release-notes
Built-in Kotlin migration: https://developer.android.com/build/migrate-to-built-in-kotlin
Android 17 local-network behavior: https://developer.android.com/about/versions/17/behavior-changes-17
Dependency AAR metadata was checked directly from Google's Maven repository before upgrading.

#### Verification

- Debug and release lint: zero errors, zero warnings.
- Debug and release APKs: built successfully, including release shrinking.
- JVM unit tests: passed.
- Android 9: all three instrumentation tests passed (45.039 seconds; zero failures/errors).
- Android 14: all three instrumentation tests passed (56.540 seconds; zero failures/errors).
- Android 17 with 16 KB pages: runtime checks remain **unverified**. Both 37.0 and 37.1 images encountered `mapper.ranchu.so` / `hasReadColorBufferDma` failures. Three-button navigation permits boot, but system_server crashes while converting task snapshots before instrumentation starts. No Android 17 test was marked as passed. Native 16 KB alignment is verified by lint, not by this blocked runtime test.
- Both build variants and the Android 9/14 suites validate the dependency updates and native library. The circular adaptive launch icon was visually inspected on Android 14; its original artwork is preserved. Gesture navigation, the new local-network prompt/denial flow and physical ARM 16 KB devices still require runtime validation.
- Shell syntax and diff whitespace checks passed. All emulators were stopped after checks. The unused 37.0 test device/image were removed; the 37.1 device remains for follow-up diagnostics.

Raw evidence: android-all-warnings-lint-debug.xml, android-all-warnings-lint-release.xml, android-all-warnings-api28-0.xml, android-all-warnings-api34-0.xml and android17-emulator-crash.log.

The emulator-only navigation workaround is based on the matching report at https://github.com/hajisensai/Fushi/blob/main/docs/agent/integration-testing.md and was reproduced locally. It does not fully resolve task-snapshot failures; the app and physical-phone navigation settings are unchanged.

Existing unrelated edits were preserved. No APK was published. Actual Google Apps Script, VPS and Cloudflare backends remain unverified; the emulator traffic tests use a local HTTP fixture.

</details>

## Android socket failure checks

Historical verification record; later checks may supersede its pending items.

<details>
<summary>Evidence, failures and repeatable commands</summary>

Tested on Android 9 and Android 14 on October 11, 2026, through Zyrln's active direct VPN and native TCP forwarder. Faults are generated by a loopback-only host fixture. Google Apps Script and VPS/Cloudflare relay faults are not covered.

| Scenario | Observed result |
| --- | --- |
| Connection refused (unused host port) | Connection failed; subsequent TCP request succeeded |
| Peer TCP reset | Subsequent TCP request succeeded |
| Peer closes before response | Subsequent TCP request succeeded |
| Response shorter than advertised Content-Length | Incomplete response; subsequent TCP request succeeded |
| Peer accepts but sends no response | Client read timed out; subsequent TCP request succeeded |
| Four stalled reads while disconnecting | Disconnect and reconnect succeeded; fresh traffic succeeded |
| 8 MiB upload to peer that stops reading | **Passed after fix** on Android 9 and 14: unrelated traffic succeeds, disconnect takes less than 8 seconds, reconnect carries traffic |

Before the fix, Android 9 reproduced a 28,504 ms disconnect delay. That original failing result is retained in socket-failure-android9.xml. After the fix, both emulator suites ran three tests with zero failures. The socket scenario itself took 15.429 seconds on Android 9 including intentional client read timeouts; the disconnect assertion passed its 8-second limit.

The TCP forwarder now uses a separate writer per flow. Backend writes run without the flow mutex and outside the shared packet reader. Queued bytes are bounded to 64 KiB plus at most 64 KiB in the active write. Excess data is left unacknowledged for retransmission; bytes are acknowledged after successful backend delivery. Each write has the configured timeout. Shutdown closes the socket to interrupt outstanding I/O, and FIN waits for queued payload delivery.

Go regression tests cover blocked-write packet handling and shutdown, write timeout and flow cleanup, queue limits, and payload delivery before FIN. The full Go suite and race checks for relay/tun, relay/netdial, and platforms/mobile passed. The rebuilt native library passed the full Android 9 verification procedure and the Android 14 emulator suite.

An Android 14 profile test initially used a stale accessibility element during activity recreation. The test now waits for the screen to become idle before locating controls; the final full suite passed. Product UI behavior was not changed for that test correction.

scripts/verify-android-build.sh includes the socket scenarios and TCP concurrency regression checks. No crash was reproduced in these scenarios. Real Google Apps Script and VPS/Cloudflare failure scenarios remain untested.

Run from the repository in Ubuntu with the fixture running:

```sh
scripts/android-emulator.sh
cd android
ANDROID_HOME="$HOME/Android/Sdk" ANDROID_SERIAL=emulator-5554 ./gradlew --no-daemon connectedDebugAndroidTest -Pandroid.testInstrumentationRunnerArguments.class=com.zyrln.relay.SocketFailureTest
```

Raw passing results: socket-failure-android9-fixed.xml and socket-failure-android14-fixed.xml. The original failing result is retained separately.

</details>

## Android UI and profile checks

Historical verification record; later checks may supersede its pending items.

<details>
<summary>Evidence, failures and repeatable commands</summary>

Date: October 11, 2026. Changes are local and uncommitted.

#### Changes
Smaller header artwork, typography, spacing and profile actions make the screen more compact while preserving 48 dp touch targets. Connect/Disconnect is clearly labeled. Import and Enter manually sit above the scrolling saved-profile list. The heading shows the saved count; a newly saved/imported profile is selected and scrolled into view. English and Persian guidance distinguishes Google-only mode, relay selection and local VPN activation.

The manual form keeps incomplete entries open with inline errors. Profile selection survives language/theme changes, and deleting the selected last relay restores Google-only guidance. Relay prewarming runs off the UI thread. Exported configuration format remains compatible, including comma-separated Apps Script URLs.

#### Verification
- Debug and release APK builds, debug JVM tests: passed in the final build.
- Debug and release lint: zero issues. Patch whitespace check passed.
- Android 9 and Android 14: full four-test instrumentation suites passed on each device.
- After the final selection-persistence and removal-guidance changes, the focused profile regression passed again on both devices.
- Eight disposable Apps Script-shaped imports, including a two-endpoint profile: counts, automatic reveal, selection, duplicate rejection, malformed input, export and delete cancellation/confirmation passed.
- Six VPN cycles per device carried 72 successful TCP responses each through the VPN to the local fixture. A six-socket barrier verifies simultaneous open connections: 144 responses across both devices.
- Socket refusal, reset, early close, truncated response, stalled connections and blocked upload checks recovered while healthy requests remained usable. Disconnect/reconnect checks passed.
- Additional Android 14 checks using actual UI input saved three profiles and completed six Connect/Disconnect cycles. English/Persian populated-list screenshots and light/dark screen checks were reviewed. Emulator preferences were restored and devices stopped.

Evidence in the Codex task workspace: android-ui-api28-0.xml, android-ui-api34-0.xml, android-ui-api28-final-profile.xml, android-ui-api34-final-profile.xml, android-ui-manual-observations.txt, android-ui-manual-three-profiles.png, android-ui-manual-three-profiles-fa.png and android-ui-profile-dialog.png.

#### Limits
Disposable profile IDs and keys verify import/storage/UI behavior; they are not deployed Google relays. TCP tests use a local fixture through the actual VPN. Working Google bridge with VPS and Cloudflare exits, restricted-network conditions and physical phones still need separate validation. These checks establish no freeze/crash in the exercised scenarios, not exhaustive compatibility.

</details>

## Live Android relay integration

Historical verification record; later checks may supersede its pending items.

<details>
<summary>Evidence, failures and repeatable commands</summary>

Date: October 11, 2026. Local Android changes remain uncommitted.

#### Google bridge → Cloudflare
Owner explicitly authorized use of Worker the owner-approved test Worker in the personal Google/Cloudflare account. The test Worker has a `TUNNEL_HUB` Durable Object binding. Existing Apps Script project is Cloudflare; its published deployment is version 5 from June 5, 2026.

The Apps Script source matches current repository source after removing the four deployment constants and whitespace. The bundled Worker has the same methods and tunnel limits as current source; this is a feature comparison, not a byte-for-byte reproducible build proof. Its WORKER_HOST remains the template placeholder and no runtime exit-key variable is configured. These settings were observed, not changed. The normal browser GET preview produces a JSON parsing error; authenticated POST relay traffic works.

Verified:
- Invalid client key returns unauthorized; valid bridge ping succeeds.
- HTTP fetch through Google and Worker returns 200 for example.com.
- Four concurrent certificate-verified TCP/TLS sessions through Google and Worker return HTTP 200 from www.wikipedia.org (about 5.6–6.2 seconds to the tested response).
- Cloudflare rejects TCP opens to example.com:80 with its runtime proxy/service restriction error. Subsequent supported HTTPS sessions succeed. This result does not establish the exact platform policy for every destination.
- Android 14 and Android 9 each pass the opt-in RealRelayTest: two connect/disconnect cycles, three simultaneously opened sockets per cycle, TLS certificate/hostname verification and six HTTP 200 responses per device. Direct Google bypass is explicitly off and asserted off after VPN establishment, preventing a direct-only run from counting as relay validation.

The live test exposed an Android reconnect bug: stopping a relay cleared its selection and the next Connect started Google-only mode. MainActivity now retains the active relay selection before stopping, including service-stop broadcasts. The corrected live test passes both cycles on Android 9 and 14. An Android 9 post-disconnect UI assertion also needed English/Persian expected strings; the final rerun passed. These are distinct production and test changes.

Final debug compilation/instrumentation builds, release build, JVM tests and debug/release lint passed; both lint reports contain zero issues. Emulators were stopped and original preferences restored.

Evidence in the task workspace: cloudflare-bridge-initial-results.json, cloudflare-live-tcp-results.json (rejected destination), cloudflare-live-tls-results.json, cloudflare-real-vpn-api34.txt, cloudflare-real-vpn-api28.txt and cloudflare-mine-dashboard.png. Keys are omitted from reports. Reusable Cloudflare profile is stored owner-only outside the repo at ~/.config/zyrln-tests/cloudflare.json.

#### VPS preparation and direct exit check
Owner supplied the dedicated VPS SSH host. Stored Windows SSH identity works; Ubuntu identity does not. No existing Zyrln service or listener on 8787 was found. Existing Untitled Apps Script points to another IP, and was not changed.

Current repository exit binary was built for Linux amd64 and uploaded to /opt/zyrln-integration-test/relay. Isolated transient systemd unit zyrln-integration-test uses DynamicUser, NoNewPrivileges, ProtectSystem, ProtectHome and PrivateTmp. Separate client/exit keys are owner-only files; no key values appear here. The existing firewall is inactive and was not changed. Health check passes; unauthenticated relay POST returns 401. Four concurrent direct VPS TLS tunnels return certificate-verified HTTP 200 responses from Wikipedia. See vps-direct-live-tls-results.json. A direct exit check does not validate Google → VPS.

New Google project Zyrln VPS integration test contains the current Code.gs configured for this server with separate keys. Owner explicitly approved Anyone access and publication under the personal Google account after automatic review required confirmation. Owner completed authorization and supplied the published deployment URL. Invalid client key was rejected; authenticated ping passed; HTTP fetch returned 200. Four concurrent certificate-verified HTTPS tunnels through Google to the VPS returned HTTP 200, including a separate check of the exact destination IP resolved by Android.

The first Android 14 run failed with a TLS handshake read timeout. A diagnostic rerun passed without any code, timeout or deployment change: two connect/disconnect cycles and six verified HTTPS responses. Android 9 passed the same test. The initial timeout remains unexplained; successful reruns do not prove it is fixed. Logs from the first run were not captured before emulator cleanup. Evidence: vps-google-initial-results.json, vps-google-live-tls-results.json, vps-google-ip-tls-results.json, vps-real-vpn-api34.txt (initial failure), vps-real-vpn-api34-rerun.txt and vps-real-vpn-api28.txt. Android 9's instrumentation-reported duration was affected by emulator clock behavior; do not use it as a wall-clock benchmark.

Private reusable profile: ~/.config/zyrln-tests/vps.json. Both emulators were stopped and preferences restored. The temporary VPS service was stopped after testing; its isolated binary and owner-only configuration remain for future authorized test runs. The Google deployment remains published. Start the dedicated VPS service again before reuse.

#### Repeatable Android procedure
Use scripts/verify-real-relay-android.py with a private url/key JSON, one supported disposable emulator and an explicit report path. Example:

    python3 scripts/verify-real-relay-android.py --profile ~/.config/zyrln-tests/cloudflare.json --avd Zyrln_API_34 --report /tmp/cloudflare-api34.txt

Use --reuse-build for the next device after a successful build. Stop existing emulators before running. The script installs both APKs, copies only url/key through stdin to an app-private file, runs RealRelayTest, restores preference XML, removes the disposable profile and stops the emulator. Without an app-private live-relay.json the opt-in instrumentation test is skipped, so ordinary connected tests do not silently call production resources. Only authorized dedicated test resources should be supplied.

#### Remaining scope
Long sessions, network changes, quota/rate-limit recovery, physical devices, multiple real Apps Script deployment failover and testing under Iran's filtering remain separate checks. Current results do not establish every destination or Android version works.


Subsequent repeated and sustained VPS tests, captured failures and production fixes are documented in TEST_RESULTS.md#repeated-vps-relay-sessions. Android 9 and 14 each passed roughly eleven-minute sessions; the earlier TLS timeout trigger remains unconfirmed. Test service is stopped after these checks.

</details>

## Repeated VPS relay sessions

Historical verification record; later checks may supersede its pending items.

<details>
<summary>Evidence, failures and repeatable commands</summary>

Date: October 11, 2026. App version 2.0, base commit a74a06c with uncommitted working-tree changes. Final combined APK hashes are recorded in zyrln-reliability-build-info.json. Uses the authorized Google Apps Script bridge and dedicated VPS test service. Keys remain outside the repository. One emulator runs at a time.

#### What failed and what was fixed

The first attempt at a longer Android 14 session failed before the requested VPS tunnels opened. Captured native logs showed three identical errors: `relay POST failed: dial tcp: lookup www.google.com: i/o timeout`. The outbound socket protect hook runs after hostname resolution; it did not bind bridge DNS to a non-VPN network. The bridge now uses Android Network.getAllByName on an available physical network, with bounded, cancellable Go lookups. Standard Go DNS remains unchanged when no platform resolver is registered. IP literals bypass the callback. TLS continues verifying the original hostname; regression tests reject a wrong hostname. ACCESS_NETWORK_STATE is now declared in the main manifest, as required by this resolver.

A deterministic regression reproduced another failure: after a tunnel receive error, the bridge returned while its local pipe remained open. A pending reader reached its deadline (`read pipe: i/o timeout`) rather than receiving EOF. Cleanup now closes the pipe before waiting for the receive goroutine, which also unblocks a receive goroutine stuck writing locally. The regression passes after the change. A controlled live Android 14 outage confirmed the established socket closed within 3,447 ms of the test's waiting marker after the dedicated exit was stopped; this interval includes the operator's stop delay. The app disconnected normally afterward. The service was restored for subsequent checks.

Race detection also exposed concurrent reads/writes of the session's selected deployment index; that field is now atomic. Upload errors are logged before cancelling the bridge context, rather than being suppressed by that cancellation.

These establish a real DNS failure and a reproducible way that relay failures become silent socket timeouts. The original, earlier TLS handshake timeout was not logged sufficiently to prove its exact trigger. It remains unconfirmed; none of these results retroactively proves which error triggered it.

#### Completed checks

| Build / device | Workload | Result |
|---|---|---|
| Before changes / Android 14 | 10 reconnect cycles, 3 HTTPS requests each | Passed, 30 HTTP 200 responses |
| Before changes / Android 14 | First longer-session attempt | Failed: Google bridge DNS timeout; preserved trace |
| DNS fix / Android 14 | 10 reconnect cycles, 3 HTTPS requests each | Passed, 30 HTTP 200 responses |
| DNS fix / Android 14 | One VPN session, 14 waves, 3 HTTPS requests per wave, 45-second idle gaps | Passed, 42 HTTP 200 responses; host runner elapsed 633.43 seconds |
| Combined fixes / Android 14 | Verified HTTPS plus established TCP socket, then controlled dedicated-exit stop | Passed: 3 HTTPS responses, health response, local socket EOF rather than timeout |
| Combined fixes / Android 9 | 10 reconnect cycles, 3 HTTPS requests each, after exit restart | Passed, 30 HTTP 200 responses |
| Combined fixes / Android 9 | Persistent TCP session plus 14 concurrent HTTPS waves and 45-second idle gaps | Passed: 42 HTTP 200 responses and 14 health responses over exactly one TCP connection; host runner elapsed 671.08 seconds; no native errors |

The HTTPS requests verify certificate and hostname. Google direct bypass remains off and is asserted off after VPN startup. The first long-session test creates fresh HTTPS sockets while keeping the VPN connected; the Android 9 test additionally reuses exactly one TCP socket to the controlled exit's /healthz endpoint. That controlled TCP check uses HTTP, alongside the verified HTTPS requests; it is a socket-lifetime test, not a TLS security claim.

Final debug/release builds, JVM tests and debug/release lint passed. The full Go suite passed after the production changes. Race checks passed for relay/tunnel, relay/tun, relay/netdial and platforms/mobile. No physical-phone or Iran-filtering validation is implied.

#### Repeat the procedure

Private profile: ~/.config/zyrln-tests/vps.json. Restart the dedicated transient exit service before use. Run scripts/verify-real-relay-android.py with an explicit report path. Use --cycles 10 for reconnect repetition; --cycles 1 --rounds 14 --pause-seconds 45 for a longer VPN session. Add --persistent-target "$ZYRLN_TEST_EXIT_HOST:8787" to reuse one controlled health TCP connection across those gaps. --reuse-build is only appropriate after a successful current build.

--expect-remote-close is an operator-controlled fault check. Use one cycle and one round with the persistent target, wait for WAITING_FOR_CONTROLLED_EXIT_STOP in the app-private trace, then stop only zyrln-integration-test.service. Restore the service before other checks. This test is expected to fail if the operator never stops the exit; it must not count a read timeout as success.

The runner captures continuous logcat, per-request/native trace and host timing, including failures, redacts the client key and deployment URL, restores preferences, removes the injected private profile and stops the emulator. Reports use the test name as their prefix. Android instrumentation durations can differ from host time because of emulator clock behavior; use the host timing JSON for elapsed run time.


Late-session Android 9 memory samples, three minutes apart, measured total app PSS of 95,438 KiB and 95,952 KiB (about 93.2 and 93.7 MiB). This short sample does not establish absence of a long-term leak. See vps-persistent28-memory.json. Larger transfers, sustained heavy upload/download, network changes, physical devices and filtering in Iran remain separate checks.

Final workload-confirming runner check passed on Android 14 with the combined build: two reconnect cycles and six verified HTTPS responses. Both emulators are stopped; preferences restored and injected profiles removed. The dedicated VPS service is stopped and port 8787 verified closed. Isolated server files and owner-only reusable profiles remain for future tests.

</details>

## Android network recovery

Historical verification record; later checks may supersede its pending items.

<details>
<summary>Evidence, failures and repeatable commands</summary>

October 11, 2026. Android 14 emulator; one disconnect/restore cycle per backend, same VPN session. Both Cloudflare and VPS use the Google Apps Script bridge, with direct bypass disabled.

Both passed: three concurrent verified HTTPS requests before interruption, Wi-Fi and mobile data disabled, failed offline traffic and responsive app UI, network restored, automatic HTTPS recovery within one minute without toggling VPN, three further concurrent HTTPS requests, and Chrome loading a unique Wikipedia URL. Browser accessibility evidence includes the page title, language choices, article counts, and search field.

Initial failures distinguished network detection from usable connectivity. A recovery probe missing its user-agent received a confirmed HTTP 403; the probe was corrected. Chrome's first-run welcome screen was handled without account sign-in. No production change was required for this recovery check.

The Cloudflare screenshot was captured before its pixels finished drawing; the saved browser UI hierarchy verifies page content. This is one recovery cycle per backend on Android 14, not validation of every app or filtering conditions in Iran. Android 9 recovery and longer repeated outage sessions remain additional coverage.

#### Trace evidence

Cloudflare
10595ms NETWORK_LOSS begin
10847ms NETWORK_LOSS confirmed
12724ms NETWORK_LOSS UI responsive
12968ms NETWORK_RESTORED
15354ms Recovery attempt=1 status=null
19547ms Recovery attempt=2 status=HTTP/1.1 200 OK
19547ms AUTOMATIC_RECOVERY passed attempts=2
23805ms BROWSER_CHECK launched
38451ms BROWSER_CHECK page passed

VPS
9898ms NETWORK_LOSS begin
10143ms NETWORK_LOSS confirmed
12095ms NETWORK_LOSS UI responsive
12430ms NETWORK_RESTORED
14684ms Recovery attempt=1 status=null
18953ms Recovery attempt=2 status=HTTP/1.1 200 OK
18954ms AUTOMATIC_RECOVERY passed attempts=2
23448ms BROWSER_CHECK launched
33400ms BROWSER_CHECK page passed

</details>

## Mobile direct mode

Historical verification record; later checks may supersede its pending items.

<details>
<summary>Evidence, failures and repeatable commands</summary>

#### Verified result

Current debug app passed direct-only VPN connect/disconnect/reconnect and verified public HTTPS on Android 9 (API 28) and Android 14 (API 34). The final reports identify the same app APK SHA-256 on both devices. Each completed two cycles with three concurrent requests/cycle: two Wikipedia 200 responses with expected content and one gstatic 204. Final workloads total 12 verified VPN HTTPS responses, separate from physical-network baselines and earlier repeat runs. No relay profiles/keys or custom MITM certificate were supplied; normal Android TLS certificate/hostname checking stayed enabled. Chrome independently rendered Wikipedia through the Android 14 direct VPN; final screenshot was visually inspected (Wikipedia's donation overlay was rendered). Android 9 independent-browser rendering was not requested in this pass.

The test clears only disposable emulator preferences for direct-only mode, requires the Go engine and Android VPN to be active, checks direct mode remains enabled, then restores original preferences after stopping the VPN. No production code changed. Test APK changed to add the opt-in assertions and diagnostics; source app/native library were unchanged during these desktop/mobile follow-ups.

#### Failures and limits retained

- The first Android 14 run timed out reading an HTTPS response while the VPN was active. It lacked a physical-network baseline/native diagnostic trace sufficient to establish its exact cause. **Do not claim that timeout is fixed or definitively caused by emulator startup.**
- A diagnostic physical-network baseline selected the first network, the emulator's virtual cellular connection, and failed DNS before starting the VPN. It was replaced by a baseline on Android's active physical network. The passing guarded runs require physical Wikipedia/gstatic HTTPS before evaluating the VPN. Those baselines can warm DNS/connection state; unprimed cold-start reliability remains a separate check.
- An extra browser screenshot run was blocked before VPN startup by a captured **System UI isn't responding** emulator dialog over VPN permission. This is not a Zyrln crash. After restarting the sole emulator without a concurrent build, the guarded workload and visible browser capture passed. Initial browser evidence was captured before content painted; the test now requires settled content and non-empty screen bounds before capturing it. That earlier image is not full visual rendering evidence.
- This proves local emulator behavior for the selected destinations, not effectiveness under Iran filtering, every app/site, long-session stability, or physical-device validation. Android 14 proxy-aware HTTPS uses the VPN proxy hint; Android 9 uses the transparent forwarding path. Broader raw-socket HTTPS coverage on Android 14 remains separate.

#### Repeat per build

1. Build current debug app and Android test APK (`assembleDebug assembleDebugAndroidTest`), with a current native library when native sources change. Finish the build before test execution to avoid emulator contention.
2. Boot only one disposable Zyrln emulator at a time. Wait for boot and a responsive System UI; require validated physical internet. These runs used 1024 MiB guest RAM on Android 9 and 1536 MiB on Android 14. The stable Android 14 run used four virtual cores. Host emulator RSS was roughly 4 GiB, despite the smaller guest allocation; it was stopped after testing.
3. Run `scripts/verify-direct-android.py --report-dir OUTPUT` on Android 9. On Android 14 add `--browser`. It installs the current APKs and explicitly enables only `DirectModeLiveTest`; ordinary instrumentation runs skip this public-internet test.
4. Require the test runner success, six recorded HTTPS successes, two recorded disconnects and matching API. Browser-enabled runs also require the Chrome marker and inspected screenshot. A skipped test or stale trace cannot count as success; the runner deletes only its prior trace/browser files first. Reports include app and test APK hashes.
5. Keep initial failures, timing/trace evidence and screenshots separate; do not discard transient failures after a passing rerun. Confirm the VPN is stopped and preferences restored, then stop that emulator before testing another version.

#### Evidence

- android9-direct-final/results.json, trace.txt and instrumentation.txt.
- android14-direct-stable/results.json, trace.txt, instrumentation.txt, direct-live-browser.png and XML.
- android9-direct-live.txt / trace: initial six-response pass.
- android14-direct-live-initial: initial timeout; android14-direct-diagnostic: cellular-baseline DNS failure.
- android14-direct-active-network: first guarded HTTPS/Chrome-content pass; its early screenshot was blank before painting.
- android14-direct-final and android14-direct-ui-failure.png: extra run blocked by emulator System UI ANR.

App/test APK builds passed; runner Python syntax and git diff --check passed. Original preferences restored by the passing tests. Both emulators stopped after this pass. No system certificate trust or ordinary device settings changed.

</details>

## Quota notices and Android lifecycle

Historical verification record; later checks may supersede its pending items.

<details>
<summary>Evidence, failures and repeatable commands</summary>

Verified October 11, 2026. Changes remain uncommitted; existing unrelated work was preserved.

#### Behavior
The Google quota notice sits immediately above Connect/Disconnect. A selected relay normally shows “Google quota: remaining amount unavailable”. Confirmed Google daily-quota errors show a prominent red warning: “Google relay quota reached. Try a profile from another Google account, or try again later.” Unidentified HTML responses show Google relay unavailable instead of claiming quota exhaustion. Direct mode hides the relay notice. A new attempt/profile clears the previous warning; later generic HTML does not overwrite confirmed quota within the same attempt.

No documented URLFetchApp method exposes the remaining account-wide quota. No percentage, estimated request counter or progress bar was implemented. Other scripts/profiles under the same Google account can share the quota. Short-term rate limits and per-request limits such as URL length are not classified as daily exhaustion.

Official references: https://developers.google.com/apps-script/reference/url-fetch/url-fetch-app and https://developers.google.com/apps-script/guides/services/quotas.

#### Changes
Serialized native VPN startup/shutdown across service instances, guarded stale callbacks and pending stop requests, disabled repeated connection taps during transitions, restored connection controls after permission denial/startup failure, and retained selected relay context after external VPN revocation. Added quota/HTML classification tests in Go and Android, emulator lifecycle tests and a repeatable runner.

The second-VPN fixture lives only in the instrumentation APK. It is emulator-guarded and routes only the documentation subnet. Never install the instrumentation APK or run preference-clearing tests on a personal phone.

#### Results
| Check | Android 9 | Android 14 |
|---|---|---|
| Duplicate import; four relay cycles A/B/A/B | Pass | Pass |
| 200 forced button taps; eight raw start/stop pairs followed by final start | Pass | Pass |
| Simulated quota and unknown HTML notice precedence/reset | Pass | Pass |
| VPN permission denial and selected profile retention | Pass | Pass |
| Activity recreation while permission pending | Pass | Pass |
| Backgrounding during connection, foreground return | Pass | Pass |
| Actual second-app VPN takeover, cleanup and reconnect | Pass | Pass |
| Delete selected profile, start direct without stale endpoint | Pass | Pass |
| Existing traffic/reconnect, socket-failure and UI-feature suites on this build | Not repeated in this pass | All four test methods passed |

Android 9: two instrumentation tests passed. Android 14: six instrumentation tests passed, including two AppFeaturesTest methods. Android debug/test APK builds, JVM tests, debug/release lint and targeted Go tunnel/Apps Script tests passed. Native AAR rebuilt with the quota classification changes. Emulator profiles were restored; the final emulator was stopped.

Both emulator runs used identical app and test APKs. App SHA-256: 2bb876bb11decb2b59fc3083bef457d0be711075a015e0c48adfebc53d235243. Instrumentation APK SHA-256: daa833d0477d11d2dbdc38b6ce276675e9a21ea407b7a790f10e76d6edc53fd8.

Synthetic localhost profiles exercise selection and lifecycle, not actual relay reachability. Quota errors were injected into the notice test; no real account quota was deliberately exhausted. Android 14 socket tests exercise refused/reset/closed/truncated/stalled connections, concurrent healthy traffic, pending-flow disconnect, stalled uploads and recovery. These results do not prove absence of every crash or freeze.

#### Retained test-harness failures
Earlier failures came from expecting the notice to disappear despite a legitimate generic-HTML message; waiting for recreation before Android dismissed its permission dialog; attempting to revoke an established VPN by changing appops; and a standalone second-VPN fixture depending on runtime classes outside its APK. Final tests corrected those setups and performed real VPN takeover. The Android 14 runner initially expected five tests while instrumentation passed six. The original count-mismatch report is retained; count validation was corrected without rerunning or changing the successful test workload.

Raw Android 9 evidence: /tmp/zyrln-edges-java9. Android 14 evidence: /tmp/zyrln-edges-final14. These temporary paths are local evidence, not durable release storage.

#### Physical phone
Installed only the tested application APK with replace-in-place installation on the authorized Samsung S25, preserving profiles/settings. Opened successfully and verified the actual visible quota notice: “Google quota: remaining amount unavailable”, with Connect available. The VPN was disconnected by the update. No instrumentation/preferences-clearing tests ran on the phone. The dedicated VPS relay remains running for user phone testing.

#### Repeat for a build
1. Rebuild native AAR when Go changes; assemble app/test APKs, run JVM tests and debug/release lint. Finish builds before starting an emulator.
2. Boot one disposable Zyrln emulator, Android 9 then Android 14, using scripts/android-emulator.sh. Set ZYRLN_AVD=Zyrln_API_34 for Android 14.
3. Run python3 scripts/verify-connection-edges-android.py --report-dir <new-report-directory>. With the local HTTP fixture on port 8765, add --with-faults to run all six methods.
4. Require actual instrumentation success plus complete lifecycle traces; keep failures and APK identities. Inspect quota-notice.png as a simulated UI example.
5. Stop the emulator before switching versions. Test real Cloudflare/VPS profiles separately; these synthetic profile checks are not live-relay validation.

#### Remaining limits
Physical Wi-Fi/mobile-data handover, sleep/resume, force-stop/process death, and Iran filtering still need separate checks. Previous live Android network-loss/recovery tests remain recorded in TEST_RESULTS.md#android-network-recovery; they were not repeated on this new build in this pass. Existing unresolved initial Android 14 timeout remains unresolved. Cloudflare destination restrictions are unchanged. Quota visibility is error-based, not an exact remaining balance.

</details>

## Desktop web interface checks

Historical verification record; later checks may supersede its pending items.

<details>
<summary>Evidence, failures and repeatable commands</summary>

Tested October 11, 2026 in external Microsoft Edge against a disposable local GUI at http://localhost:18086. Configuration and CA paths were in /tmp/zyrln-web-test-rnauk8wb. The user's normal configuration and certificate paths were not used.

| Check | Result |
| --- | --- |
| Direct connect, disconnect, reconnect | Passed; controls disable while connected and return afterward |
| Real HTTPS through HTTP proxy | Passed: www.gstatic.com/generate_204 returned 204 |
| Real HTTPS through SOCKS5 proxy | Passed: same target returned 204 |
| Direct ping | Passed: displayed 18 ms |
| Profile creation, activation, deletion | Controls worked; settings persistence race found separately |
| Multiple relay URLs and mobile export | Passed; export modal included both disposable URLs and dummy key |
| Invalid URL connection attempt | Blocked with a useful error; invalid values can still be saved and enable some tools |
| Missing CA connection attempt | Failed clearly; UI remained usable |
| Theme and English/Persian switching | Passed; persisted after reload |
| Log filtering and clearing | Passed |
| Desktop backend race tests | go test -race ./platforms/desktop passed |
| Simultaneous config save and direct setting change | **Fixed: 0 lost settings in 100 live iterations**; previously 23/100 |

#### Confirmed configuration race

During the profile/settings flow the page continued to display HTTP 127.0.0.1:18085 and SOCKS5 127.0.0.1:11080, while the saved file had lost both fields. Starting the proxy used defaults 8085 and 1080; traffic to the displayed custom ports was refused. Traffic to the actual default ports succeeded. A plain settings save subsequently restored the custom addresses, so this is intermittent rather than a guaranteed failure on every save.

The disposable-instance stress test concurrently posted a complete configuration to /api/config and {"directEnabled":true} to /api/settings, then read /api/config after both requests finished. Listener values disappeared in 23 of 100 attempts. Both operations should preserve the submitted listener fields. The handlers currently perform unsynchronized configuration reads/writes, and saveConfig directly truncates the destination file. This is a likely explanation for the observed loss; a fix needs serialized read-modify-write operations and atomic file replacement, followed by the same stress and browser checks.

The fix serializes complete config/settings/profile read-modify-write transactions in the GUI handler and replaces config/profile files using a complete temporary file in the same directory. Config updates merge submitted keys so omitted settings survive. Existing Unix file permissions are preserved. Failed setting saves return HTTP 500 and leave runtime direct mode unchanged.

Five new regression tests cover partial config updates, 100 concurrent config/settings pairs, 20 concurrent profile creations, failed-save runtime behavior, and Unix permissions. The full Go suite and desktop race tests passed. The live API stress reproduced zero losses in 100 attempts. In external Edge, creating/deleting a profile preserved the displayed custom ports; actual HTTPS requests through HTTP 18085 and SOCKS5 11080 both returned 204. Disconnect returned the UI to its stopped state.

The original stress finding remains above as historical evidence. Passing Go memory race checks alone would not have detected the file consistency race.

#### Repeatable browser checks

1. Launch a disposable GUI with a temporary config and temporary CA paths using -gui, -gui-listen, -config, -ca-cert and -ca-key. Seed custom listener ports and verify those actual ports after every settings/profile change.
2. Exercise direct start/stop twice; fetch the 204 target through HTTP and SOCKS5; check direct ping.
3. Create a dummy profile, save two dummy Apps Script URLs and a disposable key, activate it and inspect mobile export. Never treat dummy URLs as deployed relays.
4. Check invalid URL and missing CA errors, profile removal, theme/language reload persistence, and log filtering/clearing.
5. Run simultaneous config/settings writes in that disposable instance and verify that custom listener fields survive. This check now passes.
6. Run go test -race ./platforms/desktop.

Still pending: real Apps Script diagnostics and both exit backends, desktop stalled-upload/download fault checks, browser certificate/download controls, and responsive viewport checks. CA generation/download endpoints are exercised by the existing Go suite, not by a browser download in this run. No system trust certificate or browser/system proxy setting was installed or changed.

The only captured console error came from a browser autofill extension, not the Zyrln script.

Screenshot removed during the owner-approved cleanup (web-app-test.jpg).

Screenshot removed during the owner-approved cleanup (web-app-fixed.jpg).

</details>

## Desktop relays and transfers

Historical verification record; later checks may supersede its pending items.

<details>
<summary>Evidence, failures and repeatable commands</summary>

#### Changes

- Browser startup now reflects saved relay settings and routing. Reloading the page no longer enables Direct or silently activates a different profile. Matching profiles are selected; unmatched saved relay settings appear as Current settings, with profile deletion disabled.
- Narrow layouts stack the configuration, tools and log panels, allow page scrolling, and wrap profile controls. Verified at width 390: page width 380, panels fit within x=24..356 and have separate vertical ranges. Normal viewport was restored.
- Added scripts/verify-real-relay-desktop.py: private profile input, isolated GUI, HTTP and SOCKS HTTPS, concurrent requests, reconnects, idle gaps, runtime direct-bypass guard, config restoration, and pass/fail JSON.
- Added two meaningful Go fault regressions: stalled relay response fails within its budget and subsequent traffic recovers; a stalled upload permits unrelated traffic and is closed by disconnect.

#### Live results

All destinations below were requested through the desktop HTTP or SOCKS proxy with the temporary CA trusted only by the test clients. Google Apps Script remains the bridge. Ordinary computer proxy settings and system certificate trust were not modified.

| Check | Result |
| --- | --- |
| Cloudflare: 3 reconnect cycles, 2 waves per cycle, 4 concurrent requests per wave | 24/24 passed |
| VPS: same reconnect workload | 24/24 passed |
| Cloudflare: one session, 6 waves, 10-second idle gaps | 24/24 passed |
| VPS: one session, 6 waves, 10-second idle gaps, runtime bypass guard | 24/24 passed |
| Final Cloudflare guarded check | 8/8 passed |
| Live VPS exit stopped while desktop remains connected | HTTP 502 in 0.822s; GUI status responsive, session running, bypass off |
| Same desktop session after exit restoration | HTTP 200 in 1.374s; no reconnect, bypass off |
| ChatGPT through Cloudflare desktop HTTP and SOCKS | HTTP 403 on both; not successful ChatGPT use |
| Active-session browser reload | Current settings retained; direct mode OFF; session stays running |
| Theme, English/Persian changes, mobile export from Current settings | Controls worked; export included the actual saved test profile |
| Narrow layout | No horizontal overflow or card overlap after fix |
| Full Go suite and desktop/MITM race suites | Passed |
| Fault regression repetition | 3 iterations each passed under race detector |

Successful live workloads above total 104 requests to Wikipedia (200) and gstatic (204). A first VPS extended run was invalidated because browser reload enabled direct bypass; it is excluded. Its replacement enforced bypass checks. Session lengths were roughly one minute, not hour-long stability proof. Controlled local test-fixture shutdown initially hung; cleanup was corrected and the regressions then passed.

#### Repeatable procedure

1. Use a disposable config directory, temporary CA and unused GUI/HTTP/SOCKS ports. Keep test profiles private and direct bypass off.
2. Start GUI and verify it shows the saved relay without changing routing. Connect, reload, and check the same routing/profile/listeners remain.
3. Run verify-real-relay-desktop.py against Cloudflare and VPS separately. Defaults cover three reconnects and two waves; extended sessions use one cycle, six rounds and ten-second pauses. Require every requested response and bypass guard to pass.
4. Stop only the dedicated test VPS service, require an HTTP failure and responsive GUI, restore that service, and verify a new request on the same local proxy session succeeds.
5. Run the desktop/MITM race tests and full Go suite. Review narrow and normal layouts, both languages and themes, profiles and export. Restore settings and stop temporary services.

#### Remaining coverage

The external Edge browser controlled the GUI. Actual website HTTPS traffic in this desktop pass came from certificate-verifying curl test clients, not an Edge page configured to use the proxy. The follow-up below completed selected browser page rendering and bounded large-payload integrity checks. Certificate installation/download flows, complete live diagnostics, physical Wi-Fi loss, incremental streaming, payload limits beyond 8 MiB, longer sessions, and Iran-specific filtering remain pending. This pass does not establish universal app/site compatibility.

Desktop uses HTTP relay/MITM, unlike Android's transparent TCP tunnel. ChatGPT's HTTP 403 therefore differs from the Android Cloudflare socket restriction; this result does not establish which anti-abuse check rejected the desktop request.


#### Follow-up: actual browser rendering and larger transfers

- A separate logged-out Edge process, launched with Playwright after explicit owner approval, used only the desktop HTTP proxy. Direct bypass remained off on both backends. No ordinary browser proxy or system certificate trust setting changed.
- The browser context temporarily allowed the disposable MITM certificate via `ignoreHTTPSErrors`. This is functional rendering evidence, **not browser certificate-validation or certificate-installation evidence**. Earlier curl HTTPS checks verified the supplied temporary CA.
- Wikipedia homepage and Computer article each rendered through Cloudflare and VPS: 4/4 page checks, expected readable text, loaded images and style sheets, no failed requests or HTTP error responses among observed page resources. Screenshots were inspected. This does not establish every site, login, lazy-loaded resource, or interactive feature works.
- A bounded generated-data fixture on the dedicated VPS served paced downloads and returned byte count plus SHA-256 for uploads. Each backend passed downloads and uploads at 1, 4 and 8 MiB through HTTP and SOCKS: **24/24 total**, 104 MiB of verified application payload combined. Cloudflare requests totaled 81.83 seconds, individual requests 2.512–16.476 seconds; VPS totaled 54.34 seconds, individual requests 1.926–10.426 seconds. These are test timings, not general throughput guarantees.
- Initial Cloudflare attempts to the numeric-IP fixture failed (GET 403, POST 502). Direct fixture and VPS transfers worked. Replacing the numeric address with a DNS hostname made all Cloudflare checks pass. This is an observed destination-addressing restriction in this deployed path; the exact upstream rejection mechanism is not established. Do not count the initial failures as payload corruption or claim a production-code fix.
- Transfers verified complete bodies and hashes; current relay buffering means this does **not** prove incremental streaming, transfers larger than 8 MiB, or bounded memory for arbitrarily large files.

##### Repeat these checks

1. Start the disposable desktop GUI and dedicated VPS relay, with private Cloudflare/VPS profiles outside the repository. Keep bypass off and use unused test ports.
2. Temporarily start the bounded `desktop-transfer-fixture.py` on the dedicated test VPS, port 8788, with a 128 MiB service memory limit. Its only data is generated bytes; no arbitrary files or forwarding. Use a DNS hostname for compatibility with the Cloudflare HTTP fetch path.
3. Run `scripts/verify-desktop-transfers.py --profile PRIVATE_PROFILE --target FIXTURE_URL --report REPORT.json` once per backend. Require all twelve cases per backend and exact hashes/byte counts. Default fixture address is specific to this authorized test VPS; override it elsewhere.
4. After explicit approval for the isolated browser and test-only certificate handling, run `scripts/verify-desktop-browser.cjs --allow-test-certificate` with Node and Playwright on Windows with Edge installed. Optional environment settings: ZYRLN_TEST_PROFILE_DIR, ZYRLN_TEST_GUI, ZYRLN_TEST_PROXY, ZYRLN_TEST_OUTPUT_DIR, PLAYWRIGHT_MODULE. The default profile directory references this WSL test setup. No keys in arguments or reports. Require four rendered pages, no observed failed resources, and bypass guards; inspect screenshots.
5. Both harnesses restore the disposable GUI configuration and stop their proxy session. Close the isolated browser, stop the temporary fixture and dedicated relay services, and stop the disposable GUI. Retain reports and screenshots as release evidence.

Evidence in the shared workspace: desktop-browser-results.json, four desktop backend/page screenshots, desktop-cloudflare-transfers.json, desktop-vps-transfers.json, and desktop-cloudflare-ip-transfers.json (failed initial address form retained).

Cleanup completed: isolated browsers closed; disposable GUI/proxy stopped; both dedicated VPS services stopped. Node/Python harness syntax checks and git diff --check passed. No production code changed during this follow-up.


#### Certificate and adapter-recovery follow-up
See TEST_RESULTS.md#desktop-certificates-and-adapter-recovery: 12 strict certificate lifecycle assertions, four isolated adapter loss/restoration cycles, and two browser recovery checks passed. Browser certificate trust was confined to a disposable Firefox profile; no HTTPS-error allowance. Physical Wi-Fi switching and the normal Windows certificate-import wizard remain untested. Temporary trust profiles, namespace/app/network helper and dedicated VPS service were cleaned up.


#### Direct mode follow-up
October 11: independent disposable direct-only config with no relay URL/key and no CA files passed eight live HTTPS checks (Wikipedia 200/content and gstatic 204, HTTP and SOCKS, two connect/disconnect cycles), using normal public-root TLS verification. Direct ping passed; direct settings remained enabled; no MITM CA generated. Existing direct config/settings/ping regressions passed. Disposable GUI stopped and its config removed. Evidence: desktop-direct-results.json in shared workspace. This verifies desktop direct mode on this network; Iran filtering/fragmentation effectiveness and a fresh Android direct-mode pass are not established by these checks.

</details>

## Desktop certificates and adapter recovery

Historical verification record; later checks may supersede its pending items.

<details>
<summary>Evidence, failures and repeatable commands</summary>

#### Result

12/12 certificate lifecycle assertions, 4/4 network loss/restoration cycles and 2/2 independent browser checks after recovery passed through the real Google bridge with Cloudflare and VPS exits. Direct bypass stayed disabled. No production changes were necessary.

#### Certificate lifecycle

The disposable GUI had its own config and CA paths. Actual Install Certificate button use in Firefox generated and downloaded the public PEM; its bytes matched the server download. Download headers included attachment filename and no-store. OpenSSL verified the downloaded certificate was not expired.

- Missing certificate: download returned 404; relay start returned 400.
- Without trust: Firefox rejected HTTPS with a certificate security error.
- Trust imported with NSS certutil **only into a disposable Firefox profile**: Wikipedia loaded with HTTP 200 through Cloudflare and VPS. Browser HTTPS validation remained enabled (`ignoreHTTPSErrors: false`); automatic system-root importing was disabled in that profile.
- Regenerating CA stopped the active proxy, changed the certificate and signing public key, and made the old trusted profile reject the new chain. Replacing old trust with the new CA restored a fresh HTTP 200 page.
- No Windows/system certificate-store changes or normal-browser profile changes. Temporary trusted profiles were deleted after the tests.

An initial VPS assertion encountered HTTP 304 from browser caching; it was excluded and rerun with cache disabled. A stale-trust assertion initially expected UNKNOWN_ISSUER only; Firefox returned a different certificate security rejection (SEC_ERROR_UNKNOWN). The negative test was corrected to accept those two certificate failures, while still rejecting unrelated errors, and the full 12-assertion run passed. Initial reports are retained. Windows Playwright Firefox could not launch due to side-by-side runtime configuration; Linux Firefox 151 was used instead. This does not validate the normal Windows certificate-import wizard or global certificate trust installation.

#### Real loss of the isolated adapter

The app ran in a dedicated Linux network namespace with a real virtual tap adapter. Slirp supplied user-mode internet access and forwarded only the three local test listeners. No host forwarding, firewall, routes, physical adapters or system proxy settings changed. Its adapter was brought down and up twice for each backend. While down, independent direct HTTPS failed, proxied traffic failed, and loopback GUI status remained responsive (~0.001 seconds) with the proxy still running. After adapter and route restoration, independent HTTPS first confirmed usable internet; then certificate-verifying HTTP and SOCKS clients loaded Wikipedia on the same proxy session without stop/start.

| Backend | Cycle | Measured recovery, including network-usability check |
| --- | --- | --- |
| Cloudflare | 1 | 1.086 seconds |
| Cloudflare | 2 | 14.306 seconds |
| VPS | 1 | 1.320 seconds |
| VPS | 2 | 1.433 seconds |

Cloudflare's second cycle needed a transient retry; recovery is not guaranteed instant. Each backend then passed a separate Firefox page load with its trusted disposable profile, HTTPS validation enabled, readable Wikipedia text and a decoded image. Screenshots inspected. Ordinary host internet also returned 204 during the run.

The initial network fixture brought its adapter up without restoring the default route, so usable internet was still absent. That failed run is preserved and excluded from app recovery conclusions. The corrected harness restores the namespace default route and explicitly verifies internet access before judging recovery.

**Limits:** This is an actual adapter interruption within the app's isolated virtual network, not physical Wi-Fi switching, sleep/resume, an ISP outage, or Iran filtering. No universal site/app compatibility claim. Normal Windows/Edge certificate installation remains a manual-platform check.

#### Repeatable procedure

1. Ensure only the disposable desktop instance uses GUI 18096 / HTTP 18095 / SOCKS 11090. Keep private profiles outside the repository. Start the dedicated VPS test relay.
2. Prepare a fresh disposable CA directory at the path in `scripts/desktop-network-setup.py`; preserve/move aside old generated test CA files before rerunning the missing-certificate checks. The namespace must not already exist.
3. Install slirp4netns and libnss3-tools in WSL if needed. The script uses root only to create/remove the isolated namespace and toggle its adapter; Zyrln itself runs as aj. Start that script as WSL root. It never changes the host routing/forwarding/firewall settings.
4. Use the Linux Playwright Firefox runtime in the private browser-runtime directory. Run `scripts/verify-desktop-certificates.cjs` as aj from the chosen report directory. It creates an isolated NSS profile, exercises the actual GUI button and twelve lifecycle assertions, and restores the test config. Require `passed: true` and all twelve results. Inspect the screenshots. It uses workspace-specific defaults; adjust private paths before use on another machine.
5. Run `scripts/verify-desktop-network-recovery.py` **inside the dedicated namespace as root**, from the same report environment. Its constants refer to the shared workspace and namespace CA; adjust before reuse elsewhere. Require four recovery cycles and two browser checks. It invokes `scripts/desktop-recovery-browser.cjs` (the workspace copy during this run), restores the adapter in a finally block, stops the proxy, and restores test config. Never run this adapter-toggling script in the ordinary host namespace.
6. Run `scripts/desktop-network-cleanup.py` as WSL root, stop the dedicated VPS relay, and delete only the disposable Firefox trust profiles. Preserve reports and screenshots. Cleanup completed for this run.

#### Evidence and validation

- desktop-certificate-results.json: final twelve passing assertions, strict browser HTTPS verification.
- desktop-network-recovery-results.json: four recovery timings, offline proof, responsive GUI, bypass guards and two browser checks.
- Initial certificate and network reports retained separately, including fixture/assertion failures.
- cloudflare-trusted-browser.png, vps-trusted-browser.png, new-trust-restores-browser.png; desktop-cloudflare-network-recovered.png and desktop-vps-network-recovered.png.
- Python and Node syntax checks, and git diff --check passed. Temporary app/browser/helper processes and the namespace stopped; VPS test relay stopped. No production code changed.

</details>



## Credential audit follow-up
The active Cloudflare client key matched a ConfigUtils unit-test value introduced in an older export-feature commit on June 26, 2026. The value is present in committed history and the cached remote branch. This session has not committed or pushed changes. Local test data is now an explicit non-authentication dummy; the existing configuration tests are being checked. Removing the value locally does not revoke the key accepted by the deployed bridge or erase history.

At the user's request, cleanup is local-only. A fresh random replacement is staged in owner-only files outside the repository; the active private profile, deployed Google bridge and Worker are unchanged. Future work can use the private configuration files; never copy their contents into memory, documentation or tests. Actual rotation would require updating the bridge and client profile, then confirming old-key rejection; it has not happened. VPS credential did not match the scanned repository text. No phone large-transfer workload has run.
