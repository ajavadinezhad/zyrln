#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export ANDROID_HOME="${ANDROID_HOME:-$HOME/Android/Sdk}"
export PATH="$HOME/go/bin:$ANDROID_HOME/platform-tools:$PATH"
export GOTOOLCHAIN=go1.25.0
AVD="${ZYRLN_AVD:-Zyrln_API_28}"
SERIAL=emulator-5554
if ! adb devices | grep -q "^$SERIAL"; then
    "$ANDROID_HOME/emulator/emulator" -accel-check
    nohup "$ANDROID_HOME/emulator/emulator" -avd "$AVD" -port 5554 -memory 1536 -gpu software -no-audio -no-boot-anim > /tmp/zyrln-emulator.log 2>&1 &
fi
adb -s "$SERIAL" wait-for-device
running_avd="$(adb -s "$SERIAL" emu avd name | head -n 1 | tr -d '\r')"
[[ "$running_avd" == "$AVD" ]] || { echo "A different emulator is running ($running_avd). Stop it before selecting $AVD."; exit 1; }
navigation_ready=true
for attempt in {1..90}; do
    api="$(adb -s "$SERIAL" shell getprop ro.build.version.sdk | tr -d '\r')"
    navigation_ready=true
    if [[ "$api" =~ ^[0-9]+$ ]] && (( api >= 36 )); then
        # Current images crash SurfaceFlinger's gesture-region sampling. This is
        # an emulator-only workaround; it does not alter the app or real phones.
        navigation_ready=false
        if adb -s "$SERIAL" shell cmd overlay enable --user 0 com.android.internal.systemui.navbar.threebutton >/dev/null 2>&1 &&
           adb -s "$SERIAL" shell cmd overlay disable --user 0 com.android.internal.systemui.navbar.gestural >/dev/null 2>&1; then
            navigation_ready=true
        fi
    fi
    [[ "$(adb -s "$SERIAL" shell getprop sys.boot_completed | tr -d '\r')" == 1 && "$navigation_ready" == true ]] && break
    sleep 2
done
[[ "$(adb -s "$SERIAL" shell getprop sys.boot_completed | tr -d '\r')" == 1 && "$navigation_ready" == true ]] || { echo "Emulator did not finish booting; check /tmp/zyrln-emulator.log"; exit 1; }
if [[ "${1:-}" == --build ]]; then
    cd "$ROOT"
    gomobile bind -ldflags="-extldflags=-Wl,-z,max-page-size=16384" -target android -androidapi 21 -o android/app/libs/mobile.aar zyrln/platforms/mobile
    (cd android && ./gradlew --no-daemon assembleDebug testDebugUnitTest lintDebug lintRelease)
fi
APK="$ROOT/android/app/build/outputs/apk/debug/zyrln-$(cat "$ROOT/VERSION")-debug.apk"
[[ -f "$APK" ]] || { echo "Build first with: scripts/android-emulator.sh --build"; exit 1; }
adb -s "$SERIAL" install -r "$APK"
adb -s "$SERIAL" shell am start -n com.zyrln.relay.debug/com.zyrln.relay.MainActivity
