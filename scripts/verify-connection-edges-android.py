#!/usr/bin/env python3
"""Run connection edge cases only on an already booted disposable Zyrln emulator."""
import argparse, hashlib, json, os, subprocess
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--serial',default='emulator-5554');p.add_argument('--report-dir',type=Path,required=True);p.add_argument('--with-faults',action='store_true');a=p.parse_args()
repo=Path(__file__).resolve().parents[1];adb=Path(os.environ.get('ANDROID_HOME','/home/aj/Android/Sdk'))/'platform-tools/adb'
def run(*args,timeout=300,binary=False):
 return subprocess.run([str(adb),'-s',a.serial,*args],capture_output=True,text=not binary,timeout=timeout)
avd=run('emu','avd','name').stdout.splitlines()[0]
if avd not in ['Zyrln_API_28','Zyrln_API_34']:p.error('This test modifies preferences; use only a disposable Zyrln emulator')
a.report_dir.mkdir(parents=True,exist_ok=True);version=(repo/'VERSION').read_text().strip();app=repo/'android/app/build/outputs/apk/debug'/f'zyrln-{version}-debug.apk';test=repo/'android/app/build/outputs/apk/androidTest/debug/app-debug-androidTest.apk'
for apk in [app,test]:
 r=run('install','-r',str(apk));assert r.returncode==0 and 'Success' in r.stdout, 'APK install failed'
run('shell','run-as','com.zyrln.relay.debug','rm','-f','files/connection-edges-result.txt','files/lifecycle-edges-result.txt')
run('shell','run-as','com.zyrln.relay.debug','rm','-f',*[f'/sdcard/Android/data/com.zyrln.relay.debug/files/{name}' for name in ['quota-notice.png','connection-edges-failure.png','lifecycle-edges-failure.png']])
classes='com.zyrln.relay.ConnectionEdgesTest,com.zyrln.relay.LifecycleEdgesTest'
if a.with_faults:classes+=',com.zyrln.relay.VpnLifecycleTest,com.zyrln.relay.SocketFailureTest,com.zyrln.relay.AppFeaturesTest'
r=run('shell','am','instrument','-w','-r','-e','class',classes,'com.zyrln.relay.debug.test/androidx.test.runner.AndroidJUnitRunner')
(a.report_dir/'instrumentation.txt').write_text(r.stdout+r.stderr)
trace=run('shell','run-as','com.zyrln.relay.debug','cat','files/connection-edges-result.txt').stdout
(a.report_dir/'edge-result.txt').write_text(trace)
lifecycle=run('shell','run-as','com.zyrln.relay.debug','cat','files/lifecycle-edges-result.txt').stdout
(a.report_dir/'lifecycle-result.txt').write_text(lifecycle)
count=6 if a.with_faults else 2
passed=r.returncode==0 and f'OK ({count} test' in r.stdout and trace.startswith('PASS:') and 'PASS lifecycle edge suite complete' in lifecycle
report={'avd':avd,'api':run('shell','getprop','ro.build.version.sdk').stdout.strip(),'passed':passed,'classes':classes.split(','),'appSha256':hashlib.sha256(app.read_bytes()).hexdigest(),'testSha256':hashlib.sha256(test.read_bytes()).hexdigest(),'edgeResult':trace.strip(),'lifecycleResult':lifecycle.strip()}
(a.report_dir/'results.json').write_text(json.dumps(report,indent=2));print(avd, 'PASS' if passed else 'FAIL',trace.strip())
for name in ['quota-notice.png','connection-edges-failure.png','lifecycle-edges-failure.png']:
 image=run('exec-out','run-as','com.zyrln.relay.debug','cat','/sdcard/Android/data/com.zyrln.relay.debug/files/'+name,binary=True)
 if image.returncode==0:(a.report_dir/name).write_bytes(image.stdout)
if not passed:
 print(r.stdout[-3500:])
 image=run('exec-out','run-as','com.zyrln.relay.debug','cat','/sdcard/Android/data/com.zyrln.relay.debug/files/connection-edges-failure.png',binary=True)
 if image.returncode==0:(a.report_dir/'failure.png').write_bytes(image.stdout)
raise SystemExit(0 if passed else 1)
