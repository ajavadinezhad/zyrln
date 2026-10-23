# Android emulator tests

These instructions are for development. For app installation, use the [user guide](README.md).

## Prepare the tools

Install JDK 17, Go 1.25, and the Android SDK. The project uses Android platform 37, build-tools 36.0.0, and NDK 26.1.10909125. The Gradle wrapper supplies the Gradle version. Install the Go mobile tools as described in [build instructions](docs/contributing.md#build).

Set the SDK location:

```bash
export ANDROID_HOME="$HOME/Android/Sdk"
export PATH="$HOME/go/bin:$ANDROID_HOME/platform-tools:$PATH"
```

The helper expects a hardware-accelerated x86_64 virtual device named `Zyrln_API_28` (Android 9) or `Zyrln_API_34` (Android 14). Create these with Android Studio's Device Manager or the SDK tools if they do not exist. These are local names, not devices supplied by the repository. On Linux, your user needs access to `/dev/kvm`.

## Build and open the app

From the repository root:

```bash
scripts/android-emulator.sh --build
```

This starts Android 9, rebuilds the Go library and debug APK, runs unit tests and lint, then installs and opens the app. Without `--build`, it installs the existing debug APK.

Only run one emulator at a time. To switch to Android 14, stop the previous one, then run:

```bash
ZYRLN_AVD=Zyrln_API_34 scripts/android-emulator.sh
```

The script uses port 5554 and 1.5 GB of guest RAM. Actual host memory use is higher. Emulator logs go to `/tmp/zyrln-emulator.log`.

## Run the local tests

The tests can change or clear profiles. Use a disposable emulator, not a personal phone.

Start the local test server in one terminal:

```bash
python3 scripts/emulator-http-fixture.py
```

In another terminal, from the repository root:

```bash
cd android
ANDROID_HOME="$HOME/Android/Sdk" ANDROID_SERIAL=emulator-5554 ./gradlew --no-daemon connectedDebugAndroidTest
```

Reports are in `android/app/build/reports/androidTests/connected/debug`. Check for skipped tests; live relay checks require a private profile and are not part of a normal local pass.

For the combined Go, build, and emulator checks, run:

```bash
scripts/verify-android-build.sh
```

## Stop the emulator

```bash
"$ANDROID_HOME/platform-tools/adb" -s emulator-5554 emu kill
```

Stop the local test server too when finished.

## What these tests establish

Local tests cover app controls, VPN lifecycle, local traffic, and simulated connection failures. They do not prove Google reachability, VPS/Cloudflare availability, or operation under filtering. Follow the [release checklist](RELEASE_CHECKLIST.md) for live checks.

Android 17 was not validated: the previously tested emulator image crashed before app tests could run. Treat that historical result as an environment problem, not an Android 17 compatibility claim. See [the recorded warning review](TEST_RESULTS.md#android-warning-review).
