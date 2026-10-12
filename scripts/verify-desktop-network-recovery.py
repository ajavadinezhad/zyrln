"""Run as root INSIDE only zyrln-desktop-test; toggles its dedicated tap0."""
import concurrent.futures,json,subprocess,tempfile,time,urllib.request
from pathlib import Path
out=Path('/mnt/c/Users/ahmad/.codex/.chatgpt-projects/g-p-6acbcc5da2908191badc0668f42dffb6')
ca=Path('/home/aj/.config/zyrln-tests/desktop-network/ca.pem')
results=[]
def api(path,data=None):
 req=urllib.request.Request('http://127.0.0.1:18096'+path,data=None if data is None else json.dumps(data).encode(),headers={'Content-Type':'application/json'})
 with urllib.request.urlopen(req,timeout=3) as r:
  b=r.read();return json.loads(b) if b else None
def probe(proxy='http',timeout=12):
 start=time.monotonic()
 with tempfile.NamedTemporaryFile() as f:
  r=subprocess.run(['curl','--silent','--show-error','--max-time',str(timeout),'--cacert',str(ca),'--proxy','http://127.0.0.1:18095' if proxy=='http' else 'socks5h://127.0.0.1:11090','--noproxy','','--user-agent','Zyrln-Integration-Test/1.0','--output',f.name,'--write-out','%{http_code}','https://www.wikipedia.org/'],capture_output=True,text=True)
  body=Path(f.name).read_bytes();return {'proxy':proxy,'status':r.stdout,'exit':r.returncode,'seconds':round(time.monotonic()-start,3),'bytes':len(body),'passed':r.returncode==0 and r.stdout=='200' and b'Wikipedia' in body,'error':r.stderr[:180]}
def adapter(up):
 subprocess.run(['ip','link','set','tap0','up' if up else 'down'],check=True)
 if up:subprocess.run(['ip','route','replace','default','via','10.0.2.2','dev','tap0'],check=True)
adapter(True)
before=api('/api/config')
try:
 for backend in ['cloudflare','vps']:
  if api('/api/status')['running']:api('/api/stop',{})
  cfg=json.loads(Path('/home/aj/.config/zyrln-tests',backend+'.json').read_text())
  api('/api/config',{'fronted-appscript-url':cfg['url'],'auth-key':cfg['key'],'direct-enabled':'false'});api('/api/settings',{'directEnabled':False});api('/api/start',{'directOnly':False})
  for cycle in [1,2]:
   with concurrent.futures.ThreadPoolExecutor(2) as pool:baseline=list(pool.map(probe,['http','socks']))
   assert all(r['passed'] for r in baseline),'baseline failed'
   adapter(False)
   try:
    flags=json.loads(subprocess.check_output(['ip','-json','link','show','tap0']))[0]['flags'];assert 'UP' not in flags
    no_internet=subprocess.run(['curl','--silent','--max-time','3','--noproxy','*','https://www.gstatic.com/generate_204'],capture_output=True).returncode!=0
    offline=probe(timeout=8)
    start=time.monotonic();status=api('/api/status');responsive=time.monotonic()-start
    assert no_internet and not offline['passed'] and (offline['exit']!=0 or offline['status'].startswith('5')),'offline not confirmed'
    assert status['running'] and not api('/api/settings')['directEnabled'] and responsive<2,'offline UI/routing failed'
   finally:adapter(True)
   restored=time.monotonic();attempts=[]
   usable=subprocess.run(['curl','--silent','--show-error','--max-time','10','--noproxy','*','--output','/dev/null','--write-out','%{http_code}','https://www.gstatic.com/generate_204'],capture_output=True,text=True)
   assert usable.returncode==0 and usable.stdout=='204','test network internet has not restored'
   while time.monotonic()-restored<60:
    with concurrent.futures.ThreadPoolExecutor(2) as pool:batch=list(pool.map(probe,['http','socks']))
    attempts.append(batch)
    if all(r['passed'] for r in batch):break
    time.sleep(2)
   passed=all(r['passed'] for r in attempts[-1]) and api('/api/status')['running'] and not api('/api/settings')['directEnabled']
   row={'backend':backend,'cycle':cycle,'baseline':baseline,'offline':offline,'adapterDown':True,'internetUnavailable':no_internet,'networkInternetRestored':True,'guiResponseSeconds':round(responsive,3),'recoverySeconds':round(time.monotonic()-restored,3),'attempts':attempts,'withoutReconnect':True,'passed':passed};results.append(row);print(backend,'cycle',cycle,'PASS' if passed else 'FAIL',flush=True);assert passed,'recovery failed'
  trust=json.loads((out/'desktop-certificate-results.json').read_text())['profile']
  browser=subprocess.run(['runuser','-u','aj','--','node',str(out/'desktop-recovery-browser.cjs'),trust,str(out/('desktop-'+backend+'-network-recovered.png'))],capture_output=True,text=True,timeout=75)
  results.append({'backend':backend,'browserAfterRecovery':True,'passed':browser.returncode==0,'details':browser.stdout.strip(),'error':browser.stderr[:200]});print(backend,'browser','PASS' if browser.returncode==0 else 'FAIL',flush=True);assert browser.returncode==0,'browser recovery failed'
finally:
 adapter(True)
 if api('/api/status')['running']:api('/api/stop',{})
 api('/api/config',before);api('/api/settings',{'directEnabled':before.get('direct-enabled')=='true'})
 (out/'desktop-network-recovery-results.json').write_text(json.dumps({'results':results,'passed':len(results)==6 and all(r['passed'] for r in results),'scope':'dedicated namespace tap0; physical host adapter unchanged'},indent=2))
