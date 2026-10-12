// Opt-in isolated browser check. Requires explicit approval for test-only TLS validation.
const fs = require('fs');
const path = require('path');
if(!process.argv.includes('--allow-test-certificate')) throw new Error('Explicit test-only certificate approval is required. Use --allow-test-certificate only after approval.');
let playwright;
try { playwright=require('playwright'); }
catch { playwright=require(process.env.PLAYWRIGHT_MODULE || 'C:/Users/ahmad/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright'); }
const {chromium}=playwright;
const base = process.env.ZYRLN_TEST_GUI || 'http://127.0.0.1:18096';
const profiles=process.env.ZYRLN_TEST_PROFILE_DIR || '\\\\wsl.localhost\\Ubuntu\\home\\aj\\.config\\zyrln-tests';
const output=process.env.ZYRLN_TEST_OUTPUT_DIR || process.cwd();
async function api(p, data) {
  const r = await fetch(base+p, data === undefined ? {} : {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(data)});
  if(!r.ok) throw new Error('GUI request failed: '+r.status);
  const body=await r.text();
  return body ? JSON.parse(body) : null;
}
(async () => {
  const saved = await api('/api/config');
  const results=[];
  let browser;
  try {
    for(const backend of ['cloudflare','vps']) {
      if((await api('/api/status')).running) await api('/api/stop',{});
      const profile=JSON.parse(fs.readFileSync(path.join(profiles,backend+'.json'),'utf8'));
      await api('/api/config',{'fronted-appscript-url':profile.url,'auth-key':profile.key,'direct-enabled':'false'});
      await api('/api/settings',{directEnabled:false});
      await api('/api/start',{directOnly:false});
      browser=await chromium.launch({channel:'msedge',headless:false,proxy:{server:process.env.ZYRLN_TEST_PROXY || 'http://127.0.0.1:18095'},args:['--proxy-bypass-list=<-loopback>']});
      const context=await browser.newContext({ignoreHTTPSErrors:true,viewport:{width:1200,height:800}});
      const page=await context.newPage();
      for(const [label,url,expected] of [
        ['wikipedia-home','https://www.wikipedia.org/','Wikipedia'],
        ['wikipedia-article','https://en.wikipedia.org/wiki/Computer','Computer']
      ]) {
        const failures=[]; const responses=[];
        const failed=r=>failures.push({type:r.resourceType(),error:r.failure()?.errorText});
        const responded=r=>responses.push({status:r.status(),type:r.request().resourceType()});
        page.on('requestfailed',failed);page.on('response',responded);
        const start=Date.now();let row={backend,label};
        try {
          const response=await page.goto(url,{waitUntil:'load',timeout:90000});
          await page.waitForTimeout(3000);
          const rendered=await page.evaluate(() => ({title:document.title,text:document.body.innerText.slice(0,1000),images:[...document.images].filter(i=>i.complete&&i.naturalWidth>0).length,styles:[...document.styleSheets].length,width:document.body.getBoundingClientRect().width}));
          row={...row,status:response.status(),seconds:(Date.now()-start)/1000,title:rendered.title,images:rendered.images,styles:rendered.styles,resourceResponses:responses,failures,passed:response.status()===200&&rendered.text.includes(expected)&&rendered.images>0&&rendered.styles>0&&rendered.width>0&&failures.length===0&&responses.every(r=>r.status<400)};
          await page.screenshot({path:path.join(output,'desktop-'+backend+'-'+label+'.png'),fullPage:false});
        } catch(e) { row={...row,passed:false,error:e.message,failures}; }
        page.off('requestfailed',failed);page.off('response',responded);
        row.bypassOff=!(await api('/api/settings')).directEnabled;
        row.passed=row.passed&&row.bypassOff&&(await api('/api/status')).running;
        results.push(row);
        console.log(backend,label,row.passed?'PASS':'FAIL');
      }
      await browser.close();browser=null;
    }
  } finally {
    if(browser) await browser.close();
    if((await api('/api/status')).running) await api('/api/stop',{});
    await api('/api/config',saved);
    await api('/api/settings',{directEnabled:saved['direct-enabled']==='true'});
    fs.writeFileSync(path.join(output,'desktop-browser-results.json'),JSON.stringify({results,passed:results.length===4&&results.every(r=>r.passed),testOnlyTlsValidation:true},null,2));
  }
  process.exitCode=results.length===4&&results.every(r=>r.passed)?0:1;
})().catch(e=>{console.error(e.message);process.exitCode=1;});
