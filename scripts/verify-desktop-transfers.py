#!/usr/bin/env python3
"""Opt-in bounded synthetic-data transfer checks through an isolated desktop GUI."""
import argparse,json,urllib.request,subprocess,tempfile,hashlib,time
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--profile',type=Path,required=True);p.add_argument('--report',type=Path,required=True);p.add_argument('--target',default='http://62.238.49.147.sslip.io:8788');a=p.parse_args()
def api(path,data=None):
 req=urllib.request.Request('http://127.0.0.1:18096'+path,data=None if data is None else json.dumps(data).encode(),headers={'Content-Type':'application/json'})
 with urllib.request.urlopen(req,timeout=10) as r:
  b=r.read();return json.loads(b) if b else None
before=api('/api/config');cfg=json.loads(a.profile.read_text());results=[]
def curl(kind,url,dest,upload=None):
 proxy='http://127.0.0.1:18095' if kind=='http' else 'socks5h://127.0.0.1:11090'
 args=['curl','--silent','--show-error','--max-time','120','--proxy',proxy,'--noproxy','','--output',str(dest),'--write-out','%{http_code}',url]
 if upload:args+=['--data-binary','@'+str(upload),'-H','Content-Type: application/octet-stream']
 start=time.monotonic();r=subprocess.run(args,capture_output=True,text=True)
 return {'status':r.stdout,'exit':r.returncode,'seconds':round(time.monotonic()-start,3),'error':r.stderr[:200]}
try:
 if api('/api/status')['running']:api('/api/stop',{})
 api('/api/config',{'fronted-appscript-url':cfg['url'],'auth-key':cfg['key'],'direct-enabled':'false'});api('/api/settings',{'directEnabled':False});api('/api/start',{'directOnly':False})
 with tempfile.TemporaryDirectory(prefix='zyrln-transfer-') as tmp:
  d=Path(tmp)
  for size in [1024*1024,4*1024*1024,8*1024*1024]:
   data=bytes(range(256))*(size//256);expected=hashlib.sha256(data).hexdigest();source=d/'upload.bin';source.write_bytes(data)
   for kind in ['http','socks']:
    dest=d/'download.bin';r=curl(kind,a.target+'/download/'+str(size),dest);received=dest.read_bytes() if dest.exists() else b''
    r.update(direction='download',proxy=kind,bytes=len(received),expected_bytes=size,hash_match=hashlib.sha256(received).hexdigest()==expected);r['passed']=r['status']=='200' and r['exit']==0 and r['hash_match'];results.append(r)
    dest=d/'upload.json';r=curl(kind,a.target+'/upload',dest,source)
    try:got=json.loads(dest.read_text())
    except Exception:got={}
    r.update(direction='upload',proxy=kind,bytes=got.get('bytes'),expected_bytes=size,hash_match=got.get('sha256')==expected);r['passed']=r['status']=='200' and r['exit']==0 and r['hash_match'] and r['bytes']==size;results.append(r)
    assert api('/api/status')['running'];assert not api('/api/settings')['directEnabled']
finally:
 try:
  if api('/api/status')['running']:api('/api/stop',{})
  api('/api/config',before);api('/api/settings',{'directEnabled':before.get('direct-enabled')=='true'})
 finally:a.report.write_text(json.dumps({'results':results,'passed':len(results)==12 and all(x['passed'] for x in results)},indent=2))
print('Transfer checks:',len(results),'passed:',sum(x['passed'] for x in results))
raise SystemExit(0 if len(results)==12 and all(x['passed'] for x in results) else 1)
