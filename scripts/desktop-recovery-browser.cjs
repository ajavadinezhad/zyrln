const fs=require('fs'),path=require('path');
const {firefox}=require('/home/aj/.config/zyrln-tests/browser-runtime/node_modules/playwright');
(async()=>{
 const context=await firefox.launchPersistentContext(process.argv[2],{headless:true,ignoreHTTPSErrors:false,proxy:{server:'http://127.0.0.1:18095'},firefoxUserPrefs:{'security.enterprise_roots.enabled':false,'browser.cache.disk.enable':false,'browser.cache.memory.enable':false,'network.http.use-cache':false}});
 let passed=false;
 try {const page=await context.newPage();const r=await page.goto('https://www.wikipedia.org/',{waitUntil:'load',timeout:60000});const d=await page.evaluate(()=>({text:document.body.innerText,images:[...document.images].filter(i=>i.naturalWidth>0).length}));passed=r.status()===200&&d.text.includes('Wikipedia')&&d.images>0;await page.screenshot({path:process.argv[3]});console.log(JSON.stringify({passed,status:r.status(),images:d.images,ignoreHTTPSErrors:false}));}
 finally{await context.close();}
 process.exitCode=passed?0:1;
})().catch(e=>{console.error(e.message);process.exitCode=1;});
