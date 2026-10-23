# Build and contribute

For installation and connection, use the [English guide](../README.md) or [راهنمای فارسی](fa/guide.md). To deploy a relay, use [relay setup](relay-setup.md).

## Build

Desktop and VPS builds require Go 1.25. Android also requires JDK 17, the Android SDK platform 37, build-tools 36.0.0, and NDK 26.1.10909125. Install these before building. The repository includes its Gradle wrapper.

Run from the repository root:

```bash
make desktop          # local ./zyrln
make desktop-release  # Windows, Linux, and both macOS binaries in dist/
make vps-relay-bundle  # VPS install archive in dist/
```

For Android, install both mobile tools at the version in `go.mod`:

```bash
export GOTOOLCHAIN=go1.25.0
export ANDROID_HOME="$HOME/Android/Sdk"
export PATH="$HOME/go/bin:$PATH"
mobile_version="$(go list -m -f '{{.Version}}' golang.org/x/mobile)"
go install "golang.org/x/mobile/cmd/gomobile@${mobile_version}"
go install "golang.org/x/mobile/cmd/gobind@${mobile_version}"
mkdir -p android/app/libs
gomobile bind -ldflags="-extldflags=-Wl,-z,max-page-size=16384" -target android -androidapi 21 -o android/app/libs/mobile.aar zyrln/platforms/mobile
cd android
./gradlew --no-daemon assembleDebug assembleRelease testDebugUnitTest lintDebug lintRelease
```

Without a release signing configuration, Gradle signs the release build with a debug key. That is suitable for local testing, not an official release.

For a release, provide the existing private keystore through the ignored `android/keystore.properties` file (`storeFile`, `storePassword`, `keyAlias`, `keyPassword`). Keep the keystore and a backup outside Git. Then `make android` rebuilds the native library and produces the release APK in `dist/`. Do not generate a replacement signing key for an ordinary update.

`VERSION` sets release filenames and Android's displayed version; `VERSION_CODE` sets Android's increasing build number. Also update `appVersion` in `platforms/desktop/main.go` when changing the version.

## Test

```bash
go test ./...
```

Run race checks when changing shared state or connection handling. See [emulator tests](../EMULATOR_TESTING.md) and the [release checklist](../RELEASE_CHECKLIST.md) for device and real-service checks. Save dated results in [TEST_RESULTS.md](../TEST_RESULTS.md); record failed and skipped checks too.

## Find the code

| Location | Purpose |
|---|---|
| `platforms/desktop/` | Desktop program and browser interface |
| `platforms/mobile/` | Go functions called by Android |
| `android/app/src/main/` | Android interface and VPN service |
| `relay/route/` | Direct Google access and domestic bypass rules |
| `relay/appscript/` | Requests sent through Google Apps Script |
| `relay/mitm/` | Desktop proxy and local certificate handling |
| `relay/tunnel/` | Android TCP sessions through the bridge |
| `relay/tun/` | Android VPN packet handling |
| `relay/exit/` | VPS relay server |
| `relay/deploy/` | Apps Script and Cloudflare deployment files |
| `relay/core/` | Shared functions exported to the clients |

Routing checks domestic bypass first, direct Google access when enabled second, and the relay path last. Android's relay tunnel keeps destination HTTPS encrypted and needs no local certificate. The desktop HTTP relay decrypts HTTPS using the local certificate and passes requests to the bridge; it has different privacy properties.

## Change the relay messages

Keep the Go request/response formats and `relay/deploy/apps-script/Code.gs` compatible. The desktop HTTP relay and Android TCP tunnel use different messages. Update the matching tests, and test existing configs before declaring compatibility.

## Keep secrets private

Never commit real configs, app/exit keys, certificates, or signing passwords. Use [separate app and exit keys](relay-setup.md#1-prepare-two-separate-keys). Share error reports with those values removed.
