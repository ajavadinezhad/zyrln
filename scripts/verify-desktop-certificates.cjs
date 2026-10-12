const fs=require('fs'),path=require('path'),cp=require('child_process');
const linux=process.platform==='linux';
const {firefox}=require(linux?'/home/aj/.config/zyrln-tests/browser-runtime/node_modules/playwright':'C:/Users/ahmad/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');
const base='http://127.0.0.1:18096',out=process.cwd(),results=[];
let context;
async function api(p,data) {const r=await fetch(base+p,data===undefined?{}:{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(data)});const text=await r.text();return {status:r.status,body:text?JSON.parse(text):null};}
const profiles=linux?'/home/aj/.config/zyrln-tests':'\\\\wsl.localhost\\Ubuntu\\home\\aj\\.config\\zyrln-tests';
function wslPath(p) {return linux?p:cp.execFileSync('wsl',['-d','Ubuntu','--','wslpath','-a',p],{encoding:'utf8'}).trim();}
function tool(name,args){return linux?cp.execFileSync(name,args):cp.execFileSync('wsl',['-d','Ubuntu','--',name,...args]);}
function record(name,passed,details={}) {results.push({name,passed,...details});console.log(name,passed?'PASS':'FAIL');if(!passed)throw new Error(name+' failed');}
async function probe(trustDir,expectSuccess,label) {
 context=trustDir?await firefox.launchPersistentContext(trustDir,{headless:linux,ignoreHTTPSErrors:false,proxy:{server:'http://127.0.0.1:18095'},firefoxUserPrefs:{'security.enterprise_roots.enabled':false,'browser.cache.disk.enable':false,'browser.cache.memory.enable':false,'network.http.use-cache':false}}):await (async()=>{const b=await firefox.launch({headless:linux});const c=await b.newContext({ignoreHTTPSErrors:false,proxy:{server:'http://127.0.0.1:18095'}});c._testBrowser=b;return c;})();
 const page=await context.newPage();let error='',status=0,text='';
 try {const r=await page.goto('https://www.wikipedia.org/?zyrln-cert='+Date.now(),{waitUntil:'load',timeout:60000});status=r.status();text=await page.locator('body').innerText();if(expectSuccess)await page.screenshot({path:path.join(out,label+'.png')});}catch(e){error=e.message;}
 await context.close();if(context._testBrowser)await context._testBrowser.close();context=null;
 record(label,expectSuccess?status===200&&text.includes('Wikipedia'):/SEC_ERROR_UNKNOWN(?:_ISSUER)?(?:\n|$)/.test(error),{status,error:error.slice(0,180)});
}
(async()=>{
 const saved=(await api('/api/config')).body;
 const trusted=fs.mkdtempSync(path.join(out,'desktop-cert-profile-'));
 const ca=path.join(out,'desktop-test-ca.pem'),newCa=path.join(out,'desktop-test-ca-rotated.pem');
 try {
  const missing=await fetch(base+'/api/download-ca');record('missing-certificate-download',missing.status===404);
  const profile=JSON.parse(fs.readFileSync(path.join(profiles,'cloudflare.json'),'utf8'));
  await api('/api/config',{'fronted-appscript-url':profile.url,'auth-key':profile.key,'direct-enabled':'false'});await api('/api/settings',{directEnabled:false});
  const missingStart=await fetch(base+'/api/start',{method:'POST',body:JSON.stringify({directOnly:false})});record('missing-certificate-start',missingStart.status===400);
  const b=await firefox.launch({headless:linux});const gui=await b.newPage({acceptDownloads:true});await gui.goto(base);
  const download=gui.waitForEvent('download');await gui.locator('#btnRegenCA').click();await (await download).saveAs(ca);await b.close();
  const serverCA=await fetch(base+'/api/download-ca');const bytes=Buffer.from(await serverCA.arrayBuffer());record('certificate-ui-download',bytes.equals(fs.readFileSync(ca))&&serverCA.headers.get('cache-control').includes('no-store')&&serverCA.headers.get('content-disposition').includes('zyrln-ca.pem'));
  tool('openssl',['x509','-in',wslPath(ca),'-noout','-checkend','0']);
  await api('/api/start',{directOnly:false});await probe(null,false,'untrusted-certificate-rejected');
  tool('certutil',['-N','-d','sql:'+wslPath(trusted),'--empty-password']);
  tool('certutil',['-A','-d','sql:'+wslPath(trusted),'-n','Zyrln disposable test CA','-t','C,,','-i',wslPath(ca)]);
  for(const backend of ['cloudflare','vps']) {
   await api('/api/stop',{});const p=JSON.parse(fs.readFileSync(path.join(profiles,backend+'.json'),'utf8'));
   await api('/api/config',{'fronted-appscript-url':p.url,'auth-key':p.key,'direct-enabled':'false'});await api('/api/start',{directOnly:false});
   await probe(trusted,true,backend+'-trusted-browser');record(backend+'-routing-guard',!(await api('/api/settings')).body.directEnabled&&(await api('/api/status')).body.running);
  }
  await api('/api/init-ca',{});record('regeneration-stops-active-proxy',!(await api('/api/status')).body.running);
  fs.writeFileSync(newCa,Buffer.from(await(await fetch(base+'/api/download-ca')).arrayBuffer()));record('regeneration-changes-certificate',!fs.readFileSync(ca).equals(fs.readFileSync(newCa)));
  await api('/api/start',{directOnly:false});await probe(trusted,false,'old-trust-rejects-regenerated-certificate');
  tool('certutil',['-D','-d','sql:'+wslPath(trusted),'-n','Zyrln disposable test CA']);
  tool('certutil',['-A','-d','sql:'+wslPath(trusted),'-n','Zyrln rotated disposable test CA','-t','C,,','-i',wslPath(newCa)]);
  await probe(trusted,true,'new-trust-restores-browser');
 } finally {
  if(context){await context.close();if(context._testBrowser)await context._testBrowser.close();}
  await api('/api/stop',{});await api('/api/config',saved);await api('/api/settings',{directEnabled:saved['direct-enabled']==='true'});
  fs.writeFileSync(path.join(out,'desktop-certificate-results.json'),JSON.stringify({results,passed:results.length===12&&results.every(r=>r.passed),ignoreHTTPSErrors:false,trustScope:'disposable Firefox profile',profile:trusted},null,2));
 }
})().catch(e=>{console.error(e.message);process.exitCode=1;});
