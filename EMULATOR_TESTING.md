# Android emulator testing

The development machine has Android 9, Android 14 and Android 17 Google APIs x86_64 virtual devices named Zyrln_API_28, Zyrln_API_34 and Zyrln_API_37_1 in the Ubuntu SDK at ~/Android/Sdk. The Ubuntu user belongs to kvm with the owner's explicit approval. The virtual device uses 1.5 GB guest RAM; host usage is higher because graphics and emulator overhead are additional.

Run from the repository in Ubuntu:

```sh
scripts/android-emulator.sh --build
```

The default device is Android 9. Set ZYRLN_AVD=Zyrln_API_34 for Android 14, after stopping the previous emulator. Only one emulator runs at a time.

This rebuilds the Go mobile library for all Android architectures, builds the debug APK, runs JVM unit tests and debug/release lint, installs the app, and opens it. Without --build it uses the last built APK. Debug builds include x86_64; release builds retain phone architectures. Logs are in /tmp/zyrln-emulator.log. Build processes use --no-daemon so they exit afterward.

The functional test needs the local fixture:

```sh
python3 scripts/emulator-http-fixture.py
```

In another terminal:

```sh
cd android
ANDROID_HOME="$HOME/Android/Sdk" ANDROID_SERIAL=emulator-5554 ./gradlew --no-daemon connectedDebugAndroidTest
```

VpnLifecycleTest runs on the disposable virtual device. It opens the app, accepts its VPN prompt, checks the Go proxy and Android VPN state, fetches a known TCP response through the VPN, disconnects, and repeats the cycle. It stops the VPN in cleanup. The fixture listens only on the host loopback address and does not require real relay credentials.

Reports are under android/app/build/reports/androidTests/connected/debug. This direct-mode local test does not establish that Apps Script, a real exit relay, Google direct access, or Iranian networks work. Those require separate integration checks with an actual test profile and network conditions.

Stop the emulator to reclaim its memory:

```sh
~/Android/Sdk/platform-tools/adb -s emulator-5554 emu kill
```

Run `scripts/verify-android-build.sh` to combine Go checks, rebuilding, JVM tests, and emulator tests.

## Updated build prerequisites
The Android build now requires JDK 17, SDK platform 37.0 and build-tools 36.0.0. The wrapper supplies Gradle 9.8.1; AGP 9.1.1 supplies Kotlin. Minimum Android remains 9. Both Makefile and emulator builds pass the native linker flag for 16 KB alignment.

Use ZYRLN_AVD=Zyrln_API_37_1 for the Android 17 image with 16 KB pages. Stop the preceding device first. Socket and VPN lifecycle tests handle the local-network permission prompt required on Android 17. The permission-specific smoke test can be run on a fresh install with:

```sh
cd android
./gradlew --no-daemon connectedDebugAndroidTest -Pandroid.testInstrumentationRunnerArguments.class=com.zyrln.relay.VpnLifecycleTest
```

A normal suite that already granted permission verifies traffic, not a fresh prompt; record those outcomes separately.

## Android 17 emulator limitation
The 16 KB image uses 4 GB test storage and 1.5 GB guest RAM. Android 36+ test devices are switched to three-button navigation to avoid an emulator graphics assertion during gesture-region sampling. This changes only the emulator. Android 17 still crashes system_server in task-snapshot conversion before tests start, so its suite and permission flow are not validated. See TEST_RESULTS.md#android-warning-review and android17-emulator-crash.log. Guest screenshots hit the same image bug; host emulator screenshots must be used when the image is repaired.
