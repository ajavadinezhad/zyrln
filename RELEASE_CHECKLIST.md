# Release checklist

Use this for each candidate build. Record the commit, version, device, backend, and result in [TEST_RESULTS.md](TEST_RESULTS.md). Use **passed**, **failed**, or **not run**. Results from an older build do not automatically apply to a new one.

## 1. Build and check

1. Run `go test ./...`. Run race tests for changed connection or shared-state code.
2. Rebuild the Go Android library before building the APK. See [build instructions](docs/contributing.md#build).
3. Run Android unit tests and debug/release lint. Use [emulator instructions](EMULATOR_TESTING.md) for device tests on Android 9 and 14, one at a time.
4. Check that the app displays the version in `VERSION` and that Android uses `VERSION_CODE`.
5. Check release APK signing. Use the existing release key so users can update. If it must change, explain that users need to export configs, uninstall, and reinstall.

## 2. Check the app

| Check | Expected result |
|---|---|
| Import and save | Manual entry and complete JSON imports work; malformed or duplicate entries show useful errors. |
| Profile list | Several profiles stay visible, selectable, and saved after closing/reopening. |
| Export and delete | Export imports correctly; deleting the selected profile does not leave a stale connection. |
| Repeated taps | Rapid Connect/Disconnect taps and switching profiles do not freeze the app or leave an old VPN running. |
| Permissions | Denying permission, another VPN taking over, and returning from the background leave the controls usable. |
| Split tunnel | Check actual traffic with all apps, selected apps only, and selected apps excluded. Disconnect before changing settings. |
| Appearance | English/Persian text, light/dark themes, and small windows/screens remain readable. |
| Errors | Quota exhaustion, unknown HTML responses, wrong keys, and unreachable exits show different useful messages. |

Run tests that clear profiles or install a second VPN only on a disposable emulator. Do not run them on the owner's phone.

## 3. Check real connections

Use private test profiles and dedicated test exits. Both backends must go through Google Apps Script. Disable direct bypass when checking relay traffic.

For **Cloudflare and VPS separately**:

1. Connect, load a website in a separate browser/app, disconnect, and reconnect.
2. Run several requests together and leave the connection idle before trying again.
3. Upload and download a known test file; compare its size and content. Record the tested size.
4. Interrupt only the test connection, restore it, and check that browsing resumes without reconnecting Zyrln.
5. Confirm that one stalled or failed request does not block other traffic or Disconnect.
6. Check destination failures explicitly. A page loaded through one backend does not establish compatibility with every site or app.

On desktop, also check the browser proxy, certificate import, certificate replacement, page refresh while connected, and small-window layout. Use a separate browser profile for certificate tests.

On a real Android phone, check Wi-Fi/mobile-data handover, screen lock, background use, and a longer session. Testing from an ordinary network does not establish that the app works under Iran's filtering.

## 4. Useful test scripts

Run from the repository root. Use the scripts' `--help` for required private-profile and report arguments.

| Script | Purpose |
|---|---|
| `scripts/verify-android-build.sh` | Go checks, Android build, unit tests, lint, and emulator tests |
| `scripts/verify-connection-edges-android.py` | Profile switching, rapid taps, permission and quota-message tests |
| `scripts/verify-direct-android.py` | Live direct-mode traffic and reconnects |
| `scripts/verify-real-relay-android.py` | Live Android relay traffic, repeated sessions, and recovery |
| `scripts/verify-real-relay-desktop.py` | Live desktop relay requests and reconnects |
| `scripts/verify-desktop-browser.cjs` | Browser page loading through the desktop proxy |
| `scripts/verify-desktop-transfers.py` | Upload/download content checks |
| `scripts/verify-desktop-certificates.cjs` | Certificate trust and replacement |
| `scripts/verify-desktop-network-recovery.py` | Recovery after an isolated test-network interruption |

The detailed records in [TEST_RESULTS.md](TEST_RESULTS.md) describe earlier runs and their limits. Do not treat old service addresses, temporary files, or stopped/running statements as current setup instructions.

## 5. Package and publish

- Build the Android release APK, Windows/Linux desktop binaries, both macOS desktop binaries, and the VPS bundle.
- Verify APK version and signing, check the VPS archive, and calculate file checksums.
- Publish the six standard files. Keep debug APKs, test captures, configs, certificates, and signing files out of release assets.
- State user-visible changes, deployment requirements, known limitations, and any reinstall requirement in the release notes.
- Mark preview releases as prereleases.
- Stop emulators and temporary test services when finished. Keep any service still needed by the test team available and record who uses it.
