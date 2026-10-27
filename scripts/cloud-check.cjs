// Run only against the isolated TestSiteIntegration preview, never production data.
const {chromium}=require('playwright');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const os=require('node:os');
const base=process.env.UI_BASE_URL||'http://127.0.0.1:8099';
const output=path.join(os.tmpdir(),'workservice-cloud');
async function login(context,name) {
  const page=await context.newPage();await page.goto(base+'/login');
  await page.locator('#username').fill(name);await page.locator('#password').fill('testpass');
  await Promise.all([page.waitForURL(/\/(dashboard|schedule)$/),page.locator('button[type=submit]').click()]);
  return page;
}
function testPDF() {
  const content='BT /F1 28 Tf 40 220 Td (WorkService PDF preview) Tj ET';
  const second='BT /F1 28 Tf 40 220 Td (Second page) Tj ET';
  const objects=['<< /Type /Catalog /Pages 2 0 R >>','<< /Type /Pages /Kids [3 0 R 6 0 R] /Count 2 >>','<< /Type /Page /Parent 2 0 R /MediaBox [0 0 450 300] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>','<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>',`<< /Length ${content.length} >>\nstream\n${content}\nendstream`,'<< /Type /Page /Parent 2 0 R /MediaBox [0 0 450 300] /Resources << /Font << /F1 4 0 R >> >> /Contents 7 0 R >>',`<< /Length ${second.length} >>\nstream\n${second}\nendstream`];
  let pdf='%PDF-1.4\n',offsets=[0];for(let i=0;i<objects.length;i++){offsets.push(Buffer.byteLength(pdf));pdf+=`${i+1} 0 obj\n${objects[i]}\nendobj\n`;}
  const start=Buffer.byteLength(pdf);pdf+=`xref\n0 ${objects.length+1}\n0000000000 65535 f \n${offsets.slice(1).map(n=>String(n).padStart(10,'0')+' 00000 n \n').join('')}trailer\n<< /Size ${objects.length+1} /Root 1 0 R >>\nstartxref\n${start}\n%%EOF`;
  return Buffer.from(pdf);
}
(async()=>{
  fs.mkdirSync(output,{recursive:true});
  const browser=await chromium.launch({headless:true,channel:process.env.UI_BROWSER||'msedge'});
  try {
    const context=await browser.newContext({viewport:{width:390,height:844},serviceWorkers:'block'});
    const page=await login(context,'admin');const errors=[],consoleErrors=[];
    page.on('pageerror',e=>errors.push(e.message));page.on('console',m=>{if(m.type()==='error')consoleErrors.push(m.text());});
    await page.goto(base+'/cloud');await page.locator('#cloud-status').waitFor({state:'attached'});
    const row=name=>page.locator('.cloud-row').filter({has:page.locator('.cloud-row-name',{hasText:new RegExp('^'+name.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')+'$')})});
    const csrf=await page.locator('.cloud-page').getAttribute('data-csrf');
    const menu=async(name,action)=>{await row(name).getByRole('button',{name:`Действия: ${name}`,exact:true}).click();await page.locator('#cloud-actions').getByRole('button',{name:action,exact:true}).click();};
    const submit=async()=>{await page.locator('#cloud-editor-submit').click();await page.waitForFunction(()=>!document.getElementById('cloud-editor').open);};
    const createFolder=async(name)=>{await page.locator('#cloud-new-folder').click();await page.locator('#cloud-name').fill(name);await submit();await row(name).waitFor();};
    await createFolder('Документы');await createFolder('Архив');
    await row('Документы').locator('.cloud-row-open').click();
    await createFolder('Вложенная');
    const image=await page.evaluate(()=>{const c=document.createElement('canvas');c.width=800;c.height=500;const ctx=c.getContext('2d');ctx.fillStyle='#d2e2e0';ctx.fillRect(0,0,800,500);ctx.fillStyle='#516d83';ctx.fillRect(60,60,680,380);ctx.fillStyle='#ffffff';ctx.font='36px sans-serif';ctx.fillText('WorkService',180,250);return c.toDataURL('image/png').split(',')[1];});
    const files=[{name:'Заметки.txt',mimeType:'text/plain',buffer:Buffer.from('Рабочие документы\nhello <safe> & original')},{name:'Фото.png',mimeType:'image/png',buffer:Buffer.from(image,'base64')},{name:'Документ.pdf',mimeType:'application/pdf',buffer:testPDF()},{name:'Страница.html',mimeType:'text/html',buffer:Buffer.from('<!doctype html><script>window.top.cloudXSS=true</script>')}];
    await page.locator('#cloud-files').setInputFiles(files);
    await page.waitForFunction(()=>document.querySelectorAll('.cloud-upload-item').length===4&&Array.from(document.querySelectorAll('.cloud-upload-item > span')).every(e=>e.textContent==='Готово'));
    await page.locator('#cloud-stop').click();
    await row('Заметки.txt').locator('.cloud-row-open').click();
    await page.locator('#cloud-preview-body pre').waitFor();assert((await page.locator('#cloud-preview-body pre').textContent()).includes('<safe>'));
    const downloadWait=page.waitForEvent('download');await page.locator('#cloud-preview-download').click();const downloaded=await downloadWait;await downloaded.saveAs(path.join(output,'original.txt'));
    assert.equal(fs.readFileSync(path.join(output,'original.txt'),'utf8'),files[0].buffer.toString());
    await page.keyboard.press('Escape');
    await menu('Заметки.txt','Переименовать');await page.locator('#cloud-name').fill('План работ.txt');await submit();
    await menu('План работ.txt','В избранное');await page.waitForFunction(()=>document.querySelector('.cloud-row.is-starred'));
    await page.locator('[data-view=starred]').click();assert.equal(await page.locator('.cloud-row').count(),1);
    await menu('План работ.txt','Переместить');await page.locator('#cloud-destination').selectOption({label:'Архив'});await submit();
    await page.locator('[data-view=files]').click();await row('Архив').locator('.cloud-row-open').click();await row('План работ.txt').waitFor();
    await page.locator('#cloud-search').fill('Фото');assert.equal(await page.locator('.cloud-row').count(),1);
    await row('Фото.png').locator('.cloud-row-open').click();await page.locator('#cloud-preview-body img').evaluate(img=>img.decode());
    assert(await page.locator('#cloud-preview-body img').evaluate(img=>{const c=document.createElement('canvas');c.width=2;c.height=1;const ctx=c.getContext('2d');ctx.drawImage(img,0,0,2,1);return ctx.getImageData(0,0,2,1).data[3]===255&&img.naturalWidth===800;}));
    await page.keyboard.press('Escape');await page.locator('#cloud-search').fill('');
    await page.locator('[data-view=files]').click();
    const zipWait=page.waitForEvent('download');await menu('Документы','Скачать');const zip=await zipWait;await zip.saveAs(path.join(output,'documents.zip'));assert.equal(fs.readFileSync(path.join(output,'documents.zip')).subarray(0,2).toString(),'PK');
    await row('Документы').locator('.cloud-row-open').click();
    await row('Страница.html').locator('.cloud-row-open').click();await page.getByText(/Для этого формата предпросмотр недоступен/).waitFor();assert.equal(await page.evaluate(()=>!!window.cloudXSS),false);await page.keyboard.press('Escape');
    await row('Документ.pdf').locator('.cloud-row-open').click();await page.locator('.cloud-pdf-text pre').filter({hasText:'WorkService PDF preview'}).waitFor({state:'attached'});
    assert(await page.locator('.cloud-pdf canvas').evaluate(c=>{const data=c.getContext('2d').getImageData(0,0,c.width,c.height).data;let dark=0;for(let i=0;i<data.length;i+=4)if(data[i]<100&&data[i+3]>0)dark++;return dark>50;}),'PDF is blank');
    await page.getByRole('button',{name:'Следующая страница',exact:true}).click();await page.locator('.cloud-pdf-text pre').filter({hasText:'Second page'}).waitFor({state:'attached'});
    await page.getByRole('combobox',{name:'Масштаб PDF'}).selectOption('1.5');
    await page.getByRole('button',{name:'Предыдущая страница',exact:true}).click();await page.locator('.cloud-pdf-text pre').filter({hasText:'WorkService PDF preview'}).waitFor({state:'attached'});
    await page.getByRole('combobox',{name:'Масштаб PDF'}).selectOption('1');
    await page.waitForTimeout(300);await page.screenshot({path:path.join(output,'pdf.png')});await page.keyboard.press('Escape');
    await page.route('**/cloud/upload?*',route=>route.abort());
    await page.locator('#cloud-files').setInputFiles({name:'Повтор.txt',mimeType:'text/plain',buffer:Buffer.from('retry me')});await page.locator('#cloud-retry').waitFor({state:'visible'});
    await page.unroute('**/cloud/upload?*');await page.locator('#cloud-retry').click();await row('Повтор.txt').waitFor();await page.waitForFunction(()=>!document.getElementById('cloud-upload').disabled);await page.locator('#cloud-stop').click();
    await menu('Повтор.txt','В корзину');await submit();await page.locator('[data-view=trash]').click();await row('Повтор.txt').waitFor();
    await menu('Повтор.txt','Восстановить');await submit();assert.equal(await page.locator('.cloud-row').count(),0);
    await page.locator('[data-view=files]').click();await row('Документы').locator('.cloud-row-open').click();await row('Повтор.txt').waitFor();
    await menu('Повтор.txt','Переименовать');const item=await context.request.get(base+'/cloud/items').then(r=>r.json()).then(s=>s.items.find(i=>i.name==='Повтор.txt'));
    const change=await context.request.post(base+'/cloud/action',{headers:{'X-CSRF-Token':csrf},data:{action:'rename',name:'Другой ввод.txt',items:[{id:item.id,version:item.version}]}});assert.equal(change.status(),200);
    await page.locator('#cloud-name').fill('Мой несохранённый ввод.txt');await page.locator('#cloud-editor-submit').click();await page.getByText('Данные изменились в другом окне. Обновите список и повторите действие',{exact:true}).waitFor();assert.equal(await page.locator('#cloud-name').inputValue(),'Мой несохранённый ввод.txt');await page.keyboard.press('Escape');await page.locator('#cloud-refresh').click();await row('Другой ввод.txt').waitFor();
    await row('Другой ввод.txt').locator('input[type=checkbox]').check();await page.locator('[data-bulk=trash]').click();await submit();await page.locator('[data-view=trash]').click();await row('Другой ввод.txt').locator('input[type=checkbox]').check();await page.locator('[data-bulk=purge]').click();await submit();assert.equal(await page.locator('.cloud-row').count(),0);
    await page.locator('[data-view=files]').click();await row('Документы').locator('.cloud-row-open').click();
    await page.locator('[data-page-toast] button[aria-label="Закрыть уведомление"]').click().catch(()=>{});
    for(const width of [320,390,768,1440])for(const theme of ['light','dark']) {
      await page.setViewportSize({width,height:width<600?844:1000});await page.evaluate(theme=>document.documentElement.dataset.theme=theme,theme);
      await page.waitForTimeout(300);
      assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`page overflow ${width}`);
      if(process.env.AXE_PATH){await page.addScriptTag({path:process.env.AXE_PATH});const issues=await page.evaluate(async()=>(await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>({target:n.target,summary:n.failureSummary}))})));assert.deepEqual(issues,[],JSON.stringify({width,theme,issues}));}
      await page.screenshot({path:path.join(output,`cloud-${width}-${theme}.png`),animations:'disabled'});
      await row('Фото.png').locator('.cloud-row-open').click();await page.locator('#cloud-preview-body img').evaluate(img=>img.decode());assert(await page.locator('#cloud-preview').evaluate(e=>e.scrollWidth<=e.clientWidth),'preview overflow');
      if(process.env.AXE_PATH){const issues=await page.evaluate(async()=>(await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(v=>({id:v.id,target:v.nodes.map(n=>n.target)})));assert.deepEqual(issues,[],JSON.stringify({width,theme,viewerIssues:issues}));}
      await page.screenshot({path:path.join(output,`preview-${width}-${theme}.png`),animations:'disabled'});await page.keyboard.press('Escape');
    }
    const second=await browser.newContext({serviceWorkers:'block'});const secondPage=await login(second,'admin2');await secondPage.goto(base+'/cloud');assert.equal(await secondPage.locator('.cloud-row').count(),0);
    const state=await context.request.get(base+'/cloud/items').then(r=>r.json());for(const file of state.items.filter(i=>i.kind==='file'))assert.equal((await second.request.get(base+'/cloud/file/'+file.id)).status(),404);
    const workerContext=await browser.newContext();const workerPage=await login(workerContext,'worker');assert.equal((await workerContext.request.get(base+'/cloud')).status(),403);
    await workerPage.goto(base+'/object/object');assert.equal(await workerPage.getByText('PRIVATE_OTHER_WORK',{exact:false}).count(),0);
    const offlineContext=await browser.newContext({serviceWorkers:'allow'});const offlinePage=await login(offlineContext,'admin');
    await offlinePage.goto(base+'/cloud');await offlinePage.evaluate(()=>navigator.serviceWorker.ready);
    await offlinePage.waitForFunction(()=>!!navigator.serviceWorker.controller);
    const cached=await offlinePage.evaluate(async()=>{const urls=[];for(const name of await caches.keys()){const cache=await caches.open(name);urls.push(...(await cache.keys()).map(r=>new URL(r.url).pathname));}return urls;});
    assert(cached.every(url=>url.startsWith('/static/')||url==='/manifest.webmanifest'||url==='/sw.js'),'private page in offline cache: '+cached.join(','));
    await offlineContext.setOffline(true);await offlinePage.goto(base+'/object/object');await offlinePage.getByRole('heading',{name:'Нет соединения'}).waitFor();assert(!await offlinePage.locator('body').innerText().then(t=>t.includes('PRIVATE_OTHER_WORK')));
    await offlineContext.close();
    assert.deepEqual(errors,[]);
    console.log(JSON.stringify({checks:'folders, original uploads, text/image/PDF, ZIP, rename/move/star/search, retries, conflicts, trash/restore/purge, owner isolation, 8 layouts/themes + axe',consoleErrors,output}));
  }finally{await browser.close();}
})().catch(error=>{console.error(error);process.exit(1);});
