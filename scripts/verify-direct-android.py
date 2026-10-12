#!/usr/bin/env python3
"""Opt-in current-build direct HTTPS checks on an already booted disposable emulator."""
import argparse,hashlib,json,os,subprocess
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--serial',default='emulator-5554');p.add_argument('--browser',action='store_true');p.add_argument('--report-dir',type=Path,required=True);a=p.parse_args()
repo=Path(__file__).resolve().parents[1]
adb=Path(os.environ.get('ANDROID_HOME','/home/aj/Android/Sdk'))/'platform-tools/adb'
def run(*args,timeout=180,binary=False):
 return subprocess.run([str(adb),'-s',a.serial,*args],capture_output=True,text=not binary,timeout=timeout)
avd=run('emu','avd','name').stdout.splitlines()[0]
if avd not in ['Zyrln_API_28','Zyrln_API_34']:p.error('Use only a disposable Zyrln Android 9 or 14 emulator')
api=run('shell','getprop','ro.build.version.sdk').stdout.strip();a.report_dir.mkdir(parents=True,exist_ok=True)
version=(repo/'VERSION').read_text().strip();app=repo/'android/app/build/outputs/apk/debug'/f'zyrln-{version}-debug.apk';test=repo/'android/app/build/outputs/apk/androidTest/debug/app-debug-androidTest.apk'
for apk in [app,test]:
 r=run('install','-r',str(apk));assert r.returncode==0 and 'Success' in r.stdout,'APK install failed'
run('shell','run-as','com.zyrln.relay.debug','rm','-f','files/direct-live-trace.txt','files/direct-live-browser.png','files/direct-live-browser.xml')
args=['shell','am','instrument','-w','-r','-e','class','com.zyrln.relay.DirectModeLiveTest','-e','direct_live','true']
if a.browser:args+=['-e','direct_browser','true']
args+=['com.zyrln.relay.debug.test/androidx.test.runner.AndroidJUnitRunner']
r=run(*args,timeout=240)
(a.report_dir/'instrumentation.txt').write_text(r.stdout+r.stderr)
trace=run('shell','run-as','com.zyrln.relay.debug','cat','files/direct-live-trace.txt').stdout
(a.report_dir/'trace.txt').write_text(trace)
passed=r.returncode==0 and 'OK (1 test)' in r.stdout and trace.startswith('Direct live test API='+api) and trace.count('passed=true')==6 and trace.count('disconnected')==2 and (not a.browser or 'Chrome direct page passed' in trace)
if a.browser:
 for name in ['direct-live-browser.png','direct-live-browser.xml']:
  image=run('exec-out','run-as','com.zyrln.relay.debug','cat','files/'+name,binary=True)
  if image.returncode==0:(a.report_dir/name).write_bytes(image.stdout)
report={'avd':avd,'api':api,'version':version,'appSha256':hashlib.sha256(app.read_bytes()).hexdigest(),'testSha256':hashlib.sha256(test.read_bytes()).hexdigest(),'httpsResponses':trace.count('passed=true'),'cycles':trace.count('disconnected'),'browserRequested':a.browser,'browserPassed':'Chrome direct page passed' in trace,'passed':passed}
(a.report_dir/'results.json').write_text(json.dumps(report,indent=2))
print('Mobile direct API',api,':','PASS' if passed else 'FAIL','HTTPS',report['httpsResponses'],'cycles',report['cycles'],'browser',report['browserPassed'])
if not passed:print(r.stdout[-1200:])
raise SystemExit(0 if passed else 1)
