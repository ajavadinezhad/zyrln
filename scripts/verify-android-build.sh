#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export ANDROID_HOME="${ANDROID_HOME:-$HOME/Android/Sdk}"
export GOTOOLCHAIN=go1.25.0
export ANDROID_SERIAL=emulator-5554
fixture_pid=""
cleanup() { if [[ -n "$fixture_pid" ]]; then kill "$fixture_pid" 2>/dev/null || true; fi; }
trap cleanup EXIT
if ! python3 -c 'import urllib.request; assert urllib.request.urlopen("http://127.0.0.1:8765", timeout=1).read() == b"zyrln-emulator-fixture"' 2>/dev/null; then
    python3 scripts/emulator-http-fixture.py > /tmp/zyrln-fixture.log 2>&1 &
    fixture_pid=$!
fi
go test ./...
go test -race ./relay/tun ./relay/netdial ./platforms/mobile
scripts/android-emulator.sh --build
(cd android && ./gradlew --no-daemon connectedDebugAndroidTest)
# Gradle removes test installations; restore the app for interactive checks.
scripts/android-emulator.sh
echo "Local Android checks passed. Complete the real-service rows in RELEASE_CHECKLIST.md before release."
