#!/usr/bin/env python3
"""Opt-in Google bridge test on a disposable emulator; keys never enter command arguments."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import time
import re

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--profile", required=True, type=Path, help="Private JSON containing url and key")
parser.add_argument("--avd", choices=["Zyrln_API_28", "Zyrln_API_34"], default="Zyrln_API_34")
parser.add_argument("--report", required=True, type=Path)
parser.add_argument("--reuse-build", action="store_true")
parser.add_argument("--cycles", type=int, default=2)
parser.add_argument("--rounds", type=int, default=1)
parser.add_argument("--pause-seconds", type=int, default=0)
parser.add_argument("--persistent-target", default="", help="Authorized test exit host:port exposing /healthz")
parser.add_argument("--expect-remote-close", action="store_true", help="Operator stops the dedicated exit after the trace readiness marker")
parser.add_argument("--network-recovery", action="store_true", help="Disable emulator Wi-Fi and data, then verify automatic relay recovery")
parser.add_argument("--browser-check", action="store_true", help="Open Chrome and confirm page rendering after the last traffic wave")
args = parser.parse_args()
if not 1 <= args.cycles <= 20 or not 1 <= args.rounds <= 60 or not 0 <= args.pause_seconds <= 60:
    parser.error("cycles: 1..20; rounds: 1..60; pause-seconds: 0..60")
if args.expect_remote_close and (not args.persistent_target or args.cycles != 1 or args.rounds != 1):
    parser.error("Controlled exit stop requires --persistent-target, --cycles 1 and --rounds 1")
if args.network_recovery and (args.rounds < 2 or args.expect_remote_close or args.persistent_target):
    parser.error("Network recovery requires at least two rounds and no persistent/remote-close mode")
profile = json.loads(args.profile.read_text())
if not profile.get("url", "").startswith("https://script.google.com/macros/s/") or not profile.get("key"):
    parser.error("Profile must contain an Apps Script deployment URL and a key")
root = Path(__file__).resolve().parent.parent
sdk = Path(os.environ.get("ANDROID_HOME", Path.home() / "Android/Sdk"))
adb = str(sdk / "platform-tools/adb")
serial = "emulator-5554"
package = "com.zyrln.relay.debug"
env = dict(os.environ, ANDROID_HOME=str(sdk), ZYRLN_AVD=args.avd)
def run(command, **kwargs):
    return subprocess.run(command, check=True, capture_output=True, **kwargs)
def device(*command, **kwargs):
    return run([adb, "-s", serial, *command], **kwargs)
if "emulator-" in run([adb, "devices"]).stdout.decode():
    parser.error("Stop existing emulators first; this procedure launches one disposable device")
if not args.reuse_build:
    with (root / "android/build/real-relay-build.log").open("wb") as log:
        subprocess.run(["./gradlew", "--no-daemon", "assembleDebug", "assembleDebugAndroidTest"],
                       cwd=root / "android", env=env, stdout=log, stderr=subprocess.STDOUT, check=True)
snapshot = None
network_snapshot = {}
captured = False
passed = False
logcat_process = None
logcat_file = None
started = time.monotonic()
args.report.parent.mkdir(parents=True, exist_ok=True)
try:
    run(["bash", "scripts/android-emulator.sh"], cwd=root, env=env)
    apk = next((root / "android/app/build/outputs/apk/androidTest/debug").glob("*.apk"))
    device("install", "-r", str(apk))
    device("shell", "am", "force-stop", package)
    saved = subprocess.run([adb, "-s", serial, "shell", "run-as", package, "cat", "shared_prefs/config.xml"], capture_output=True)
    snapshot = saved.stdout if saved.returncode == 0 else None
    captured = True
    if args.network_recovery:
        for radio, setting in (("wifi", "wifi_on"), ("data", "mobile_data")):
            network_snapshot[radio] = device("shell", "settings", "get", "global", setting).stdout.decode().strip() == "1"
    device("shell", "run-as", package, "rm", "-f", "files/real-relay-trace.txt", "files/real-relay-failure.png", "files/real-relay-failure.xml")
    device("logcat", "-c")
    logcat_file = args.report.with_suffix(".logcat.txt").open("wb")
    logcat_process = subprocess.Popen([adb, "-s", serial, "logcat", "-v", "threadtime"], stdout=logcat_file, stderr=subprocess.STDOUT)
    payload = json.dumps({"url": profile["url"], "key": profile["key"], "cycles": args.cycles, "rounds": args.rounds, "pause_ms": args.pause_seconds * 1000, "persistent_target": args.persistent_target, "expect_remote_close": args.expect_remote_close, "network_recovery": args.network_recovery, "browser_check": args.browser_check}).encode()
    device("shell", "run-as", package, "sh", "-c", "'mkdir -p files; cat > files/live-relay.json'", input=payload)
    result = device("shell", "am", "instrument", "-w", "-e", "class",
                    "com.zyrln.relay.RealRelayTest",
                    package + ".test/androidx.test.runner.AndroidJUnitRunner", timeout=max(300, args.cycles * (args.rounds * 100 + (args.rounds - 1) * args.pause_seconds + 30)))
    text = (result.stdout + result.stderr).decode(errors="replace")
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.report.write_text(text)
    passed = "OK (1 test)" in text
    print(text)
    if not passed:
        for extension in ("png", "xml"):
            capture = subprocess.run([adb, "-s", serial, "shell", "run-as", package, "cat",
                                      "files/real-relay-failure." + extension], capture_output=True)
            if capture.returncode == 0:
                args.report.with_suffix("." + extension).write_bytes(capture.stdout)
finally:
    if args.browser_check:
        for extension in ("png", "xml"):
            capture = subprocess.run([adb, "-s", serial, "shell", "run-as", package, "cat", "files/real-relay-browser." + extension], capture_output=True)
            if capture.returncode == 0:
                args.report.with_suffix(".browser." + extension).write_bytes(capture.stdout)
    for artifact in ("real-relay-trace.txt",):
        capture = subprocess.run([adb, "-s", serial, "shell", "run-as", package, "cat", "files/" + artifact], capture_output=True)
        if capture.returncode == 0:
            content = capture.stdout.decode(errors="replace").replace(profile["key"], "[redacted]").replace(profile["url"], "[deployment]")
            args.report.with_suffix(".trace.txt").write_text(content)
            completed_rounds = len(re.findall(r"cycle=\d+ round=\d+ passed", content))
            if passed and (completed_rounds != args.cycles * args.rounds or content.count("HTTP status=HTTP/1.1 200") != args.cycles * args.rounds * 3):
                passed = False
                print("Trace does not confirm the requested workload; skipped or stale tests cannot count as a pass")
            if passed and args.persistent_target and content.count("persistent TCP health passed") != args.cycles * args.rounds:
                passed = False
                print("Trace does not confirm the persistent TCP workload")
            if passed and args.expect_remote_close and "Remote close observed after" not in content:
                passed = False
                print("Trace does not confirm controlled remote socket closure")
            if passed and args.network_recovery and (content.count("NETWORK_RESTORED") != args.cycles or content.count("AUTOMATIC_RECOVERY passed") != args.cycles):
                passed = False
                print("Trace does not confirm actual network loss and recovery")
            if passed and args.browser_check and content.count("BROWSER_CHECK page passed") != args.cycles:
                passed = False
                print("Trace does not confirm browser page rendering")
        elif passed:
            passed = False
            print("Missing live trace; a skipped or stale test cannot count as a pass")
    if logcat_process is not None:
        logcat_process.terminate()
        logcat_process.wait(timeout=10)
        logcat_file.close()
        p = args.report.with_suffix(".logcat.txt")
        content = p.read_text(errors="replace").replace(profile["key"], "[redacted]").replace(profile["url"], "[deployment]")
        p.write_text(content)
    args.report.with_suffix(".timing.json").write_text(json.dumps({"host_seconds": round(time.monotonic() - started, 2), "passed": passed, "cycles": args.cycles, "rounds": args.rounds, "pause_seconds": args.pause_seconds}))
    for radio, enabled in network_snapshot.items():
        subprocess.run([adb, "-s", serial, "shell", "svc", radio, "enable" if enabled else "disable"], capture_output=True)
    subprocess.run([adb, "-s", serial, "shell", "am", "force-stop", package], capture_output=True)
    if captured:
        if snapshot is not None:
            device("shell", "run-as", package, "sh", "-c", "'cat > shared_prefs/config.xml'", input=snapshot)
        else:
            device("shell", "run-as", package, "rm", "-f", "shared_prefs/config.xml")
        device("shell", "run-as", package, "rm", "-f", "files/live-relay.json", "files/real-relay-trace.txt")
    subprocess.run([adb, "-s", serial, "emu", "kill"], capture_output=True)
sys.exit(0 if passed else 1)
