// Run only against the isolated integration preview, never production storage.
const {chromium}=require('playwright');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const os=require('node:os');
const path=require('node:path');
const {randomUUID}=require('node:crypto');
const base=process.env.UI_BASE_URL||'http://127.0.0.1:8099';
const output=path.join(os.tmpdir(),'workservice-cloud-sharing');
(async()=>{
  fs.mkdirSync(output,{recursive:true});
  const browser=await chromium.launch({headless:true,channel:process.env.UI_BROWSER||'msedge'});
  const errors=[];
  try{
    const context=await browser.newContext({viewport:{width:390,height:844},isMobile:true,hasTouch:true,serviceWorkers:'block',reducedMotion:'reduce'});
    const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
    await page.goto(base+'/login');await page.locator('#username').fill('admin');await page.locator('#password').fill('testpass');
    await Promise.all([page.waitForURL('**/dashboard'),page.locator('[type=submit]').click()]);await page.goto(base+'/cloud');
    const csrf=await page.locator('main').getAttribute('data-csrf');
    const folderName='Документы для просмотра '+Date.now();
    let response=await context.request.post(base+'/cloud/action',{headers:{'X-CSRF-Token':csrf},data:{action:'folder',name:folderName}});
    assert(response.ok());const folder=(await response.json()).items.find(i=>i.name===folderName);
    const upload=async(parent,name,buffer)=>{
      const response=await context.request.post(base+'/cloud/upload?parent='+parent,{headers:{'X-CSRF-Token':csrf,'X-Upload-ID':randomUUID()},multipart:{file:{name,mimeType:'application/octet-stream',buffer}}});
      assert(response.ok(),await response.text());return (await response.json()).item;
    };
    const file=await upload(folder.id,'Инструкция.txt',Buffer.from('Документ для просмотра'));
    await upload(folder.id,'Фото объекта.png',fs.readFileSync(path.join(__dirname,'../web/static/img/logo-192.png')));
    const privateFile=await upload('','Личный документ.txt',Buffer.from('PRIVATE_DO_NOT_SHARE'));
    await page.reload();
    const folderRow=page.locator('.cloud-row').filter({hasText:folderName});
    await folderRow.getByRole('button',{name:'Действия: '+folderName,exact:true}).click();
    await page.locator('#cloud-actions').getByRole('button',{name:'Доступ по ссылке',exact:true}).click();
    await page.locator('#cloud-share').waitFor();
    await page.locator('#cloud-share-expiry').selectOption('7');
    await page.locator('#cloud-share-create').click();
    await page.locator('#cloud-share-link').waitFor();
    const url=await page.locator('#cloud-share-link').inputValue();assert(url.includes('/s/'));
    assert((await page.locator('#cloud-share-status').innerText()).includes('до'));
    const anonContext=await browser.newContext({serviceWorkers:'block',reducedMotion:'reduce'});
    const anon=await anonContext.newPage();anon.on('pageerror',e=>errors.push(e.message));
    await anon.goto(url);assert.equal(await anon.locator('.cloud-public-list li').count(),2);
    assert(!(await anon.locator('body').innerText()).includes(privateFile.name));
    response=await anonContext.request.get(url+'/file/'+privateFile.id);assert.equal(response.status(),404);
    response=await anonContext.request.get(url+'/file/'+file.id);assert.equal(await response.text(),'Документ для просмотра');
    const axe=async(p)=>{if(!process.env.AXE_PATH)return;await p.evaluate(fs.readFileSync(process.env.AXE_PATH,'utf8'));const v=await p.evaluate(async()=>(await window.axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(x=>({id:x.id,nodes:x.nodes.map(n=>n.failureSummary)})));assert.deepEqual(v,[],JSON.stringify(v));};
    for(const width of [320,390,768,1440])for(const theme of ['light','dark']){
      await page.setViewportSize({width,height:844});await anon.setViewportSize({width,height:844});
      await page.emulateMedia({colorScheme:theme});await anon.emulateMedia({colorScheme:theme});
      assert(await page.locator('#cloud-share').evaluate(e=>e.scrollWidth<=e.clientWidth),'share dialog overflow');
      assert(await anon.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'public page overflow');
      await axe(page);await axe(anon);
      await page.screenshot({path:path.join(output,`share-${width}-${theme}.png`)});
      await anon.screenshot({path:path.join(output,`public-${width}-${theme}.png`)});
    }
    await anon.locator('.cloud-public-open').filter({hasText:'Фото объекта.png'}).click();await anon.locator('.cloud-public-file img').waitFor();
    assert(await anon.locator('img').evaluate(e=>e.complete&&e.naturalWidth>0));
    await page.locator('#cloud-share-revoke').click();await page.locator('#cloud-share-link').waitFor({state:'hidden'});
    response=await anon.goto(url);assert.equal(response.status(),404);
    response=await anonContext.request.get(url+'/file/'+file.id);assert.equal(response.status(),404);
    await page.locator('#cloud-share-create').click();await page.locator('#cloud-share-link').waitFor();
    const renewed=await page.locator('#cloud-share-link').inputValue();assert.notEqual(renewed,url);
    await page.keyboard.press('Escape');assert(await folderRow.locator('.cloud-shared-badge').isVisible());
    await folderRow.locator('.cloud-row-open').click();
    const row=page.locator('.cloud-row').filter({hasText:file.name});
    await row.getByRole('button',{name:'Действия: '+file.name,exact:true}).click();
    await page.locator('#cloud-actions').getByRole('button',{name:'Доступ по ссылке',exact:true}).click();
    await page.locator('#cloud-share-create').click();await page.locator('#cloud-share-link').waitFor();
    const fileURL=await page.locator('#cloud-share-link').inputValue();await anon.goto(fileURL);
    assert.equal(await anon.locator('h1').innerText(),file.name);
    assert(await anon.getByRole('link',{name:'Скачать файл'}).isVisible());
    assert.deepEqual(errors,[]);console.log(JSON.stringify({passed:true,checks:'file/folder sharing, anonymous downloads, private-file isolation, expiry UI, revocation, token rotation, 8 layouts, accessibility',output}));
  }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
