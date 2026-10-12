# Zyrln shared project memory

> Current status: consult TEST_RESULTS.md first. Chronological entries below preserve what was known at each stage; earlier “pending” or “stopped” statements may have been superseded. Current phone uploads/downloads remain unrun; user-reported quota exhaustion is not independently confirmed.

Updated October 11, 2026. This living document records the context established with the owner and should grow through our work together.

## Purpose and origin

The owner developed Zyrln because people in Iran, during war and high tension, reported having access to Google while much of the wider internet was unavailable. Google is the bridge: this is a core design constraint for technical decisions, explicitly reaffirmed by the owner in our conversation. Improvements must account for which Google services remain reachable in the affected environment, rather than assuming ordinary internet access.

The hardest early problem, according to the owner, was testing inside that environment. Progress depended on finding someone who could test there. Local tests alone therefore cannot establish whether the project works for its intended users. The owner says people used the app for a sustained period and asked us to read their public discussions before planning improvements.

Cloudflare documented Iranian traffic falling well below one percent of its previous level on February 28, 2026, consistent with aggressive filtering and selective access. That national observation supports the setting but does not establish Google reachability on a particular network. The Google access account above comes from the owner. [Cloudflare disruption report](https://blog.cloudflare.com/q1-2026-internet-disruption-summary/)

## Working relationship

The owner wants a substantial shared record built from conversations and evidence, not a tiny starter note. Preserve the reasons behind decisions and what we learn about actual users. Do not import previous Claude notes. Use other agents only when explicitly requested. Preserve existing uncommitted work.

For public feedback research, the owner specifically wants their external browser and Twitter/X. Read public posts and replies without posting or contacting people. The review covered the launch and major release discussions plus public search results; X search does not provide a guaranteed complete archive.

Two project-specific Claude storage folders were removed at the owner's request. Global Codex memory was not inspected or deleted. A Cursor testing rules file remains. Removing saved files does not erase the conversation context.

## Public development history

The public launch on May 4 described a Go rewrite inspired by mhr-cfw and an Android client. It introduced the Apps Script relay approach and desktop support. [Launch discussion](https://x.com/a_javadinezhad/status/2051370386818830451)

May releases addressed speed, connection preparation, certificates, and switching between connection modes. A May 11 post described testing direct Google access on Hamrah Aval; the May 15 prerelease added that approach. Version 1.6 on May 21 added direct access for domestic domains, reducing unnecessary Apps Script use. [Direct Google testing](https://x.com/a_javadinezhad/status/2053926785528520942), [domestic domain bypass](https://x.com/a_javadinezhad/status/2057448755134103555)

The May 28 version 2 prerelease introduced Android TCP tunneling through HTTP without installing a certificate. The announcement required matching script and server updates, initially required a VPS exit, and acknowledged Telegram and WhatsApp problems. These were historical release limitations, not automatically current bugs. [Android tunnel announcement](https://x.com/a_javadinezhad/status/2060035917113639175)

Later announcements addressed messaging apps, added a Cloudflare exit in June, introduced split tunneling, and added configuration sharing and missing preinstalled apps to the split list. The July 31 stable version 2 announcement emphasized full Android tunneling without certificates and preparedness for future shutdowns. [Split tunneling](https://x.com/a_javadinezhad/status/2066951288403698067), [configuration sharing](https://x.com/a_javadinezhad/status/2071308055766007897), [stable version 2](https://x.com/a_javadinezhad/status/2083229451001708623)

## What public users reported

There is concrete anecdotal evidence of practical use. One user reported more than 30 GB in one week. Others reported Mac access, YouTube access, and Instagram use. A community member published an independent setup tutorial. These are individual reports, not measured reliability or active-user statistics. [Weekly usage report](https://x.com/karim_dorzadeh/status/2053947326578724988), [Mac report](https://x.com/jan_farsoode/status/2053726475577085958), [YouTube report](https://x.com/Mim_Mikhanad/status/2055803821117329442), [community tutorial](https://x.com/jplamirhossein/status/2053053095504957661)

Replies to the May 28 release strengthen the evidence of use during disruption: one person said they stayed connected through most of the shutdown, another said they connected two or three other people, and another described continuing to connect with one click. [Shutdown use](https://x.com/rockstxr_dead/status/2060061919168328036), [helping others connect](https://x.com/lostguy_net/status/2060201444205043908), [continued use](https://x.com/Pejoonz/status/2060038097316741454)

Setup experiences varied sharply. Some users found it simple; one reported spending two hours without obtaining a working connection. Replies asked where to generate a key, requested Persian or video instructions and reachable downloads, and asked for a plain one-line explanation of the project. A Windows user described a connection that initially worked but later failed after reopening a profile. [Failed setup report](https://x.com/sobhan9335/status/2054364903465435513), [key generation question](https://x.com/Pejoonz/status/2053942741508202681), [profile reopening report](https://x.com/MH_Tirdad/status/2056855847691550908), [clear explanation request](https://x.com/king_mehrdad_/status/2060045760926773564)

Speed feedback was mixed. Early Android users described browser access while native apps did not work, and several requested whole-phone support or Cloudflare exits. Later releases directly addressed these needs. Current behavior must be checked before treating those early complaints as unresolved. One user associated an Instagram account suspension with their usage; the reply establishes only a temporal account, not causation.

## Current code understanding

Zyrln shares a Go engine across desktop and Android. Desktop uses a browser interface and relays HTTP requests with certificate-based HTTPS interception. Android uses Kotlin and a Go mobile binding to tunnel TCP through Apps Script to an exit relay. Desktop and Android therefore have different certificate requirements.

Routing includes direct Google access when enabled, domestic bypass, and relay fallback. Relay code batches requests, manages deployment failover, and handles tunnel sessions. Android's network processing handles IPv4 TCP and UDP DNS; it does not provide general UDP transport. Marketing descriptions such as full tunnel should be interpreted alongside those protocol limits.

Two code findings deserve validation before implementation: the Android binding discards the result of socket protection, and the tunnel server's advertised local HTTP proxy rejects ordinary non-CONNECT HTTP requests. Desktop event-stream handling also supplies keepalive comments instead of forwarding upstream events. These findings are separate from public connection complaints; no causal link has been demonstrated.

All existing Go tests passed during this collaboration. Android JVM and emulator tests now run as described below. Physical-device checks and tests inside the affected network environment have not run. Existing local Android edits include manual profile entry and notification improvements; they predate our changes. Android socket protection now stops a dial when protection fails and logs a clear error. Regression tests cover failure, success, and callback clearing. Android device validation remains pending.

## Direction for the next work

The owner wants us to learn enough first, then improve the project together. The evidence points toward clearer setup, dependable connection behavior, useful failure explanations, and compatibility with real applications. Before choosing fixes, compare historical feedback with the current version and preserve Google as the bridge constraint. Validate improvements with someone in the intended network environment whenever possible.

Record future decisions with their reason, evidence, validation, and remaining uncertainty. Do not turn proposed fixes into accepted priorities without the owner's agreement.

## Coverage baseline after the first fix

Aggregate Go statement coverage is 67.2%. Protection regression and race checks passed. Android TCP flow handling and DNS resolution are major uncovered paths. See TEST_RESULTS.md#go-coverage-baseline. Android device validation and rebuilding the generated AAR/APK remain pending.

## Android emulator and Android 9 support

The owner requested a virtual phone to avoid reinstalling each build on their own device, then requested checks of all features and a repeatable procedure for each build. Both VPS and Cloudflare backends must eventually be tested through Google Apps Script. Dedicated accounts and server selection remain pending; no cloud resources have been created.

The owner explicitly approved adding Ubuntu user aj to the kvm group after automatic approval rejected an earlier device ownership change. Ownership was not changed. Official Android emulator and system images were installed in the existing Ubuntu SDK. Only one virtual phone runs at a time; scripts use 1.5 GB guest RAM and build without a persistent Gradle daemon. Host memory includes graphics overhead and grows beyond guest RAM.

The Android minimum is now API 28 (Android 9), at the owner's request. The optional system proxy hint is used only on Android 10 and newer; Android 9 retains the TCP VPN forwarder. The generated Go AAR and debug APK have been rebuilt. Debug builds include native x86_64 code to avoid ARM translation crashes in the emulator; release architectures remain unchanged.

Android 9 emulator tests passed profile save/import/export, duplicate and invalid input handling, theme/language recreation, YouTube app discovery, split selection persistence, inactive profile removal, and actual local TCP traffic through two VPN connect/disconnect cycles. These do not establish real Apps Script or exit compatibility. The final updated suite passed on Android 14 as well. The combined per-build command passed end to end on Android 9. Fixture concurrency and asynchronous UI waits were corrected during validation. Lint reported four existing manifest/broadcast errors, with no NewApi errors.

The procedure is saved in RELEASE_CHECKLIST.md and EMULATOR_TESTING.md; scripts/verify-android-build.sh combines local checks. Pending matrix rows must not be marked as passed.

## Socket failure testing — October 11, 2026
Android 9 direct-VPN fault tests cover refused/reset/closed/truncated/stalled sockets and recovery. A blocked 8 MiB upload reproduced a 28,504 ms disconnect stall. The stricter SocketFailureTest intentionally fails pending a fix; the per-build instrumentation suite is therefore no longer passing. See TEST_RESULTS.md#android-socket-failure-checks. Real bridge faults remain untested.

## Socket freeze fix — October 11, 2026
The blocked-upload disconnect stall is fixed. TCP backend writes run in a per-flow writer without holding the flow lock or blocking the shared TUN reader. Buffering is bounded, writes have deadlines, shutdown interrupts I/O, and queued payload is delivered before FIN. Four new Go regression tests and targeted race checks passed. Android 9 full build verification passed; all three instrumentation tests passed on Android 9 and Android 14, including socket faults, concurrent stalled reads, blocked uploads, unaffected traffic, and disconnect/reconnect. The Android 14 UI test needed activity-idle synchronization; product UI was unchanged. Real Google bridge and exit failure checks remain pending. See TEST_RESULTS.md#android-socket-failure-checks for evidence and raw result filenames.

## Desktop browser verification — October 11, 2026
External Edge testing passed direct lifecycle, real HTTP/SOCKS5 HTTPS traffic (204), direct ping, profiles, multi-URL mobile export, theme/language persistence, and log controls. Invalid URL and missing CA errors remained recoverable. Concurrent config/settings writes lost custom listener addresses in 23/100 attempts; this is unresolved and can make displayed ports differ from actual listeners. Desktop backend race tests passed but do not cover file consistency. See TEST_RESULTS.md#desktop-web-interface-checks for evidence and remaining checks.

## Desktop configuration race fix — October 11, 2026
The race that lost custom listeners is fixed. The GUI serializes config/settings/profile transactions; config and profile files are replaced atomically from complete temporary files. Config updates preserve omitted keys. Failed persistence returns an error without altering runtime direct mode. Existing Unix permissions are retained. Five new regression tests, desktop race checks, and the full Go suite passed. Live simultaneous API saves lost 0/100 settings (before: 23/100). External Edge profile creation/deletion preserved custom listeners; real HTTPS returned 204 on HTTP 18085 and SOCKS5 11080. See TEST_RESULTS.md#desktop-web-interface-checks. These changes remain local; real bridge/exit and remaining web checks are still pending.

## Android lint fixes — October 11, 2026
All four lint errors are fixed without suppression: removed the unnecessary BIND_VPN_SERVICE permission request while retaining service binding protection; scoped started/stopped/error broadcasts to the app package. Debug and release lint pass with zero errors and 38 warnings each. Debug build and JVM tests passed; Android 9 and 14 VPN lifecycle tests passed (traffic, disconnect, reconnect). Debug lint is now part of the per-build verification script. Both emulators were stopped after the checks. See TEST_RESULTS.md#android-lint-fixes.

## Android warning review — October 11, 2026
All 38 warnings were triaged and 12 fixed without suppression: split-tunnel plural resources in English/Persian, duplicate screen backgrounds, app-search autofill, formatted UI text, hardcoded labels, and redundant notification-channel API guard. Debug/release lint now report zero errors and 26 warnings. Remaining: 8 dependency advisories, 9 unused resources, 8 launcher-icon findings and 1 technical JSON ellipsis example. Debug build/JVM tests passed; all three instrumentation tests passed on Android 9 and 14. Android 14 exposed a test wait issue after language-triggered activity recreation; corrected synchronization and the full rerun passed. Emulators stopped. Actual Google bridge/VPS/Cloudflare validation remains pending. See TEST_RESULTS.md#android-warning-review.

## Complete Android warning cleanup — October 11, 2026
Owner requested all remaining warnings fixed. All 38 original warnings plus newly exposed findings are fixed; debug/release lint report zero errors and zero warnings without new suppression. Dependencies, AGP/Gradle/Kotlin, JVM target and compile/target SDK updated (minimum Android 9 retained). Adaptive icons preserve original artwork, themed monochrome layer added, unused resources removed, JSON example clarified. Go native library rebuilt with 16 KB alignment and all build/CI paths retain the flag. Insets and Android 17 local-network permission handling added. Both APK builds/JVM tests passed; three tests passed on each of Android 9 and 14. Android 17 images confirm 16 KB pages, but runtime validation is blocked by OS graphics/task-snapshot crashes before tests begin. Three-button navigation stabilizes boot but not task-snapshot conversion. Do not claim Android 17 permission or 16 KB runtime tests passed. All emulators stopped; real Google bridge/VPS/Cloudflare and physical-device checks remain pending. See TEST_RESULTS.md#android-warning-review.

## Compact Android UI and import testing — October 11, 2026
Owner prefers smaller elements and an obvious imported-entry list. Compact artwork/spacing preserves touch targets; Import/manual actions now precede the list, saved count is shown, and new profiles are selected/revealed automatically. Inline manual-form errors preserve data; locale/theme recreation retains selection; last-profile removal restores direct guidance. Background relay prewarming avoids blocking UI. Final debug/release builds, JVM tests and lint passed (zero issues). Four instrumentation tests passed on each Android 9/14 emulator, followed by a final focused profile test on each. Eight disposable Apps Script-shaped imports include a comma-separated profile. Six VPN cycles/device carried 144 total local-fixture responses, with six sockets opened concurrently. Socket-fault recovery passed. Three manually entered profiles and six additional UI cycles passed on Android 14; compact EN/FA screenshots reviewed. Original emulator preferences restored; devices stopped. Real Google/VPS/Cloudflare and physical-phone checks remain pending. See TEST_RESULTS.md#android-ui-and-profile-checks and the repeatable RELEASE_CHECKLIST.md procedure.

## Real Cloudflare bridge and VPS preparation — October 11, 2026
Owner authorized testing the owner-approved Worker and Google bridge, then supplied the dedicated VPS SSH host (stored Windows SSH key works). Cloudflare Apps Script source matches repository apart from settings; bundled Worker methods/limits match, without claiming byte-identical builds. Authentication rejection, ping, HTTP relay, four parallel verified-TLS tunnel requests passed through Google → Worker. example.com:80 TCP was rejected by Cloudflare; supported HTTPS recovered. Android 9/14 each passed two VPN cycles with three concurrent verified-TLS responses per cycle and direct bypass off. Live tests found relay selection lost on disconnect; MainActivity now retains it for reconnect, verified by both device runs. Android 9 needed a locale-aware hint assertion; corrected. Release/JVM checks and debug/release lint pass with zero issues. Devices stopped/preferences restored. Private Cloudflare profile: ~/.config/zyrln-tests/cloudflare.json; do not copy keys into memory.
VPS has isolated transient zyrln-integration-test service, current exit binary and owner-only exit key under /opt/zyrln-integration-test. Health/unauthenticated-401 and four direct verified-TLS tunnels pass. Existing services, firewall and old bridge were not changed. New Apps Script project Zyrln VPS integration test is saved. Owner explicitly approved Anyone access/publication after automatic review required confirmation. Google unverified-app authorization is pending user action; do not claim Google → VPS passed yet. Separate staging keys: ~/.config/zyrln-tests/vps-staging.json (owner-only). See TEST_RESULTS.md#live-android-relay-integration and scripts/verify-real-relay-android.py for repeatable live checks.


## VPS Google bridge validation — October 11, 2026
Owner completed Google authorization and supplied the deployment URL. Auth rejection, ping, HTTP fetch and four simultaneous certificate-verified HTTPS tunnels passed through Google to the dedicated VPS. Android 14 initially timed out during a TLS handshake; unchanged diagnostic rerun passed both connection cycles and six HTTPS responses. Android 9 also passed both cycles and six responses. Initial timeout is unresolved, not a fixed bug. Evidence is in TEST_RESULTS.md#live-android-relay-integration and the workspace reports. Private profile remains outside the repository at ~/.config/zyrln-tests/vps.json. Emulators stopped, preferences restored, temporary zyrln-integration-test service stopped; deployment and isolated server files retained for reuse. No production code changed during this validation.


## Repeated VPS relay diagnosis and fixes — October 11, 2026
User requested repeated and longer-session tests and investigation of the earlier timeout. Ten initial Android 14 reconnect cycles passed, but the first longer-session attempt failed with three captured native errors resolving www.google.com: i/o timeout. Socket protection occurs after DNS resolution; bridge DNS now uses Android Network.getAllByName on a non-VPN network via a bounded cancellable Go callback. Main manifest declares ACCESS_NETWORK_STATE. Tests verify cancellation, IP bypass, preserved socket protection and unchanged TLS hostname checks.
A deterministic regression proved that a tunnel RX error left the local pipe open and its reader timed out. Bridge cleanup now closes the pipe before waiting for RX, including a blocked local writer. A controlled live Android 14 exit stop confirmed the established socket closed within 3,447 ms of the waiting marker; the app disconnected normally. Upload errors are now logged before context cancellation. Race detection exposed the session deployment-index data race; accesses are now atomic.
DNS-fixed Android 14 passed 10 reconnect cycles and 14 waves in a roughly 11-minute VPN session, with 42 verified HTTPS responses. Combined fixes passed 10 reconnect cycles on Android 9, then a roughly 11-minute session with 42 HTTPS responses plus 14 health responses over exactly one TCP connection separated by 45-second idle gaps. No native errors in the persistent session. Late app PSS was roughly 93–94 MiB over a three-minute sample, not a long-term leak proof.
The original earlier TLS timeout lacks sufficient logs to prove its exact triggering error. The confirmed DNS failure and pipe-cleanup mechanism are separate evidence; do not claim the original event is conclusively explained. Full Go tests, targeted race checks, Android debug/release builds, JVM tests and both lint reports (zero issues) passed. Updated opt-in runner preserves/redacts traces and logs and verifies requested workload counts, rejecting skipped/stale test success. Final results are in TEST_RESULTS.md#repeated-vps-relay-sessions. Changes remain uncommitted. Test resources use the approved private profile outside the repository; no keys in docs.

Final runner validation on Android 14 passed two additional reconnect cycles and six HTTPS responses. Both emulators stopped; injected profiles removed and original preferences restored. Dedicated VPS test service stopped and port 8787 confirmed closed; approved Google deployment and isolated server files retained for future reuse. Restart the service before real VPS tests.


## Network recovery and independent app check
Android 14: Cloudflare and VPS each passed one offline/restoration cycle with responsive UI, automatic HTTPS recovery without toggling VPN, and Chrome Wikipedia page content verified through the Google bridge with bypass disabled. See TEST_RESULTS.md#android-network-recovery. Android 9 outage recovery remains untested.


## Cloudflare destination limitation, October 11
Live Google-bridge tunnel-open probes: chatgpt.com and www.cloudflare.com were rejected by the Cloudflare runtime before TLS; www.wikipedia.org succeeded. Current DNS answers for ChatGPT are Cloudflare addresses. Official Cloudflare TCP socket documentation prohibits outbound sockets to Cloudflare IP ranges. This limits Android end-to-end TCP tunneling; replacing connect with fetch does not transparently preserve client TLS. A VPS exit is a candidate alternative, but ChatGPT page/login usability on that exit has not been verified. Evidence: cloudflare-destination-check.json in the shared workspace. Documentation: https://developers.cloudflare.com/workers/runtime-apis/tcp-sockets/


## Desktop relay reliability pass
October 11: 104 successful live HTTP/SOCKS requests across Cloudflare/VPS reconnect and roughly minute-long sessions; dedicated VPS outage returned 502 and restored traffic without reconnect. Fixed page reload enabling direct bypass and narrow window overflow. External Edge verified GUI; website traffic was from certificate-verifying test clients, not a proxied Edge page. See TEST_RESULTS.md#desktop-relays-and-transfers for evidence, limitations and repeatable checks.


## Desktop browser and bounded transfers follow-up
Owner explicitly approved Playwright in an isolated test browser with test-only temporary certificate handling. Four Wikipedia pages rendered through Cloudflare/VPS with text, images, styles and no observed failed resources; bypass off. Browser context allowed HTTPS certificate errors only for these tests, so certificate validation/installation is not proven. Both backends passed 1/4/8 MiB uploads/downloads via HTTP and SOCKS with exact SHA-256: 24 cases, 104 MiB total. Numeric-IP fixture failed through Cloudflare, DNS-hostname fixture passed; no production change required, upstream cause not established. Current bodies are buffered; incremental streaming and >8 MiB remain untested. See TEST_RESULTS.md#desktop-relays-and-transfers and the opt-in transfer/browser scripts. No ordinary browser or system trust settings modified.

Follow-up cleanup completed: isolated browsers, disposable GUI/proxy and both dedicated VPS services stopped. Approved Google deployments and private local profiles retained.


## Strict desktop certificate trust and isolated network loss
Owner requested the next desktop certificate/setup and network-loss checks. Twelve certificate lifecycle assertions passed using the real GUI download and Linux Firefox with HTTPS validation enabled and trust only in a disposable NSS profile. Missing CA blocks startup; unknown/stale trust rejects TLS; replacing regenerated trust restores browsing. Windows test Firefox had a side-by-side runtime failure, so normal Windows certificate import was not validated. Four actual tap-adapter down/up cycles in a dedicated namespace passed across Cloudflare/VPS without reconnecting proxy; GUI responsive, bypass off, HTTP+SOCKS and independent browser passed. Recovery measured 1.086/14.306 seconds for Cloudflare and 1.320/1.433 for VPS. Initial missing-default-route fixture and overly specific certificate/cached-page assertions were corrected; they do not establish app bugs. No production change necessary. Physical Wi-Fi/sleep/ISP/ Iran checks remain open. See TEST_RESULTS.md#desktop-certificates-and-adapter-recovery. Cleanup completed, including disposable trusted profiles and dedicated VPS service.


Desktop direct-mode recheck: eight live certificate-verifying HTTPS HTTP/SOCKS requests across two connect/disconnect cycles passed, plus direct ping and existing direct-mode regressions. No relay profile/key or MITM CA; temporary GUI/config cleaned up. Local desktop behavior verified, not Iran bypass effectiveness.


## Current mobile direct-mode verification
Android 9 and 14 current debug APK (matching app hash) each passed two direct-only VPN cycles and six verified HTTPS responses, no relay or custom CA. Chrome visibly rendered Wikipedia on Android 14 in the stable final run. Added opt-in DirectModeLiveTest and scripts/verify-direct-android.py with physical active-network HTTPS baselines, exact workload guards, APK identities and restored preferences. Initial Android 14 VPN HTTPS timeout remains unexplained; do not call it fixed. First cellular baseline DNS failure and later emulator System UI ANR are retained separately. Baselines warm connection/DNS state; unprimed cold starts, physical devices and Iran-filtering effectiveness remain untested. No production changes during this check. Both emulators stopped. See TEST_RESULTS.md#mobile-direct-mode.


## Quota notice and connection lifecycle edge cases
October 11: explicit Google quota exhaustion is distinguished from unknown HTML, transient rate limits and per-request limits. Notice above Connect/Disconnect shows remaining amount unavailable until an explicit quota error; no fake percentage or counter. Serialized VPN start/stop, stale callback guards and UI transition protection added. Two new emulator tests passed Android 9/14: duplicate imports, four profile-switch cycles, 200 forced button taps, queued start/stop, permission denial/recreation/backgrounding, actual second-VPN takeover/reconnect, selected-profile deletion and simulated notices. Android 14 existing traffic/socket/UI tests also passed (six methods total); runner count corrected from five, successful instrumentation retained. Latest app installed/opened on authorized S25 preserving profiles; actual unknown-quota notice verified, VPN currently disconnected. Dedicated VPS test relay restarted and remains running for phone testing. Physical handovers and current-build live network recovery not repeated; prior unresolved Android 14 timeout remains. See TEST_RESULTS.md#quota-notices-and-android-lifecycle.


## Documentation consolidation
Fourteen separate test reports were consolidated into TEST_RESULTS.md with section links. Historical failures and limits are retained; consult its current-status summary before relying on older snapshots. Keep new results in that document instead of creating another root report. Shared memory and the release checklist remain separate. User reported phone network switching works; big upload/download tests are paused, not passed. Latest quota-exhaustion report remains unconfirmed; dedicated VPS relay stays running and the temporary transfer fixture is stopped.


## Documentation and credential audit
The README changes only add the authorized Android 9 minimum-version note. Existing TODO edits predate this session and were preserved. Personal server address, account-specific Worker hostname, Google project identifier and user-specific profile paths were removed or made portable in project notes. Exact private relay endpoints/keys were not found in Markdown. However, the live Cloudflare test credential matches a value in an already tracked ConfigUtils unit test, including HEAD. It should not be treated as a secret and must be rotated before further Cloudflare test use. No credential or deployment was changed during this audit.


## Local-only credential cleanup
User explicitly limited the credential work to local changes. Replaced the ConfigUtils export-format fixture with an explicit dummy; no application behavior change, no commit/push and no remote deployment/credential change. The old fixture value was already introduced in committed history on June 26, 2026 and matches the active Cloudflare client key (not the VPS key). Local replacement cannot revoke that key remotely or remove history. Active reusable profile remains ~/.config/zyrln-tests/cloudflare.json. Fresh random candidate and replacement line are staged in cloudflare-rotation-pending.json and cloudflare-auth-key-replacement.txt in the same owner-only private directory; cloudflare-rotation-state.json records local-only status. Do not promote the staged key to active config until a matching remote deployment is explicitly authorized and verified. The browser publication handoff is closed. Large phone upload/download remains paused and unrun.
