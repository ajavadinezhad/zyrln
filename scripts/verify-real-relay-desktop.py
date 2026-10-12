#!/usr/bin/env python3
"""Opt-in live desktop HTTP/SOCKS checks against an isolated GUI instance.
Never prints the private profile key. Trusts only the supplied test CA.
"""
import argparse, concurrent.futures, json, subprocess, time, urllib.request
from pathlib import Path
p=argparse.ArgumentParser()
p.add_argument('--profile', type=Path, required=True)
p.add_argument('--ca', type=Path, required=True)
p.add_argument('--gui', default='http://127.0.0.1:18096')
p.add_argument('--http-proxy', default='http://127.0.0.1:18095')
p.add_argument('--socks-proxy', default='socks5h://127.0.0.1:11090')
p.add_argument('--cycles', type=int, default=3)
p.add_argument('--rounds', type=int, default=2)
p.add_argument('--pause', type=float, default=5)
p.add_argument('--report', type=Path, required=True)
a=p.parse_args()
if not 1 <= a.cycles <= 20 or not 1 <= a.rounds <= 60 or not 0 <= a.pause <= 60: p.error('invalid workload')
def api(path, data=None):
 req=urllib.request.Request(a.gui+path, data=None if data is None else json.dumps(data).encode(), headers={'Content-Type':'application/json'})
 with urllib.request.urlopen(req, timeout=10) as r:
  body=r.read()
  return json.loads(body) if body else None
cfg=json.loads(a.profile.read_text()); before=api('/api/config'); results=[]
def fetch(item):
 kind,url,expected=item
 proxy=a.http_proxy if kind=='http' else a.socks_proxy
 started=time.monotonic()
 r=subprocess.run(['curl','--silent','--show-error','--max-time','60','--cacert',str(a.ca),'--proxy',proxy,'--noproxy','','--user-agent','Zyrln-Integration-Test/1.0','--output','/dev/null','--write-out','%{http_code}',url],capture_output=True,text=True)
 return {'proxy':kind,'url':url,'status':r.stdout,'exit':r.returncode,'passed':r.returncode==0 and r.stdout==str(expected),'seconds':round(time.monotonic()-started,3),'error':r.stderr[:200]}
try:
 if api('/api/status')['running']: api('/api/stop', {})
 api('/api/config', {'fronted-appscript-url':cfg['url'],'auth-key':cfg['key'],'direct-enabled':'false'})
 api('/api/settings', {'directEnabled':False})
 for cycle in range(a.cycles):
  api('/api/start', {'directOnly':False})
  assert api('/api/status')['running']
  for wave in range(a.rounds):
   items=[(kind,url,status) for kind in ['http','socks'] for url,status in [('https://www.wikipedia.org/',200),('https://www.gstatic.com/generate_204',204)]]
   with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool: batch=list(pool.map(fetch,items))
   for r in batch: r.update(cycle=cycle+1, wave=wave+1)
   results.extend(batch)
   assert api('/api/status')['running'], 'GUI must stay responsive'
   assert not api('/api/settings')['directEnabled'], 'Direct bypass was enabled during relay validation'
   if wave+1 < a.rounds: time.sleep(a.pause)
  api('/api/stop', {})
  assert not api('/api/status')['running']
finally:
 try:
  if api('/api/status')['running']: api('/api/stop', {})
  api('/api/config', before)
  api('/api/settings', {'directEnabled':before.get('direct-enabled','false')=='true'})
 finally:
  a.report.write_text(json.dumps({'results':results,'passed':len(results)==a.cycles*a.rounds*4 and all(r['passed'] for r in results)},indent=2))
print('Requests:',len(results),'passed:',sum(r['passed'] for r in results))
raise SystemExit(0 if len(results)==a.cycles*a.rounds*4 and all(r['passed'] for r in results) else 1)
