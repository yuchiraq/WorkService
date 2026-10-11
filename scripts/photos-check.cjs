// Run only against the isolated TestSiteIntegration preview.
const {chromium} = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const base = process.env.UI_BASE_URL || 'http://127.0.0.1:8100';
const output = path.join(os.tmpdir(), 'workservice-photos');
(async () => {
  fs.mkdirSync(output, {recursive:true});
  const browser = await chromium.launch({headless:true,channel:process.env.UI_BROWSER || 'msedge'});
  try {
    const context = await browser.newContext({viewport:{width:390,height:844},serviceWorkers:'block'});
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror',error=>errors.push(error.message));
    await page.goto(base+'/login');
    await page.locator('#username').fill('worker');
    await page.locator('#password').fill('testpass');
    await Promise.all([page.waitForURL('**/schedule'),page.locator('button[type=submit]').click()]);
    await page.goto(base+'/object/object#object-photos');
    const root = page.locator('#object-photos');
    const files = [];
    for (let i=0;i<3;i++) {
      const data = await page.evaluate((i)=>{
        const c=document.createElement('canvas'); c.width=1000; c.height=750;
        const ctx=c.getContext('2d');
        ctx.fillStyle=['#c3d8e5','#bac9b7','#bcbfc4'][i]; ctx.fillRect(0,0,1000,750);
        ctx.fillStyle='#858a8c'; ctx.fillRect(0,570,1000,180);
        ctx.fillStyle=['#f2f4f6','#e5e8e8','#dcdfdf'][i]; ctx.fillRect(120,160,730,420);
        ctx.fillStyle='#3e555d';
        for(let x=0;x<5;x++) for(let y=0;y<3;y++) ctx.fillRect(155+x*135,200+y*115,92,65);
        return c.toDataURL('image/jpeg').split(',')[1];
      },i);
      files.push({name:`object-${i}.jpg`,mimeType:'image/jpeg',buffer:Buffer.from(data,'base64')});
    }
    await page.locator('#photos-files').setInputFiles(files);
    await page.waitForFunction(()=>document.querySelectorAll('.photo-tile').length===3 && document.getElementById('photos-upload').hidden);
    await page.locator('#photos-files').setInputFiles(files[0]);
    await page.waitForFunction(()=>document.getElementById('photos-status').textContent.includes('Уже есть: 1'));
    assert.equal(await page.locator('.photo-tile').count(),3,'duplicate stored');
    await page.locator('.photo-tile').first().click();
    const viewer=page.locator('#photo-viewer');
    await viewer.locator('#photo-image').evaluate(img=>img.decode());
    await page.locator('#photo-caption').fill('Монтаж окон, второй этаж');
    await viewer.getByRole('button',{name:'Сохранить подпись'}).click();
    await page.waitForFunction(()=>document.querySelector('.photo-tile span').textContent==='Монтаж окон, второй этаж');
    const photoID=await page.locator('.photo-tile').first().getAttribute('data-id');
    const csrf=await root.getAttribute('data-csrf');
    const concurrent=await context.request.post(`${base}/object/object/photos/${photoID}/caption`,{headers:{'X-CSRF-Token':csrf},form:{version:'2',caption:'Изменено в другом окне'}});
    assert.equal(concurrent.status(),200);
    await page.locator('#photo-caption').fill('Несохранённый ввод');
    await viewer.getByRole('button',{name:'Сохранить подпись'}).click();
    await page.locator('#photo-reload').waitFor({state:'visible'});
    assert.equal(await page.locator('#photo-caption').inputValue(),'Несохранённый ввод');
    await page.locator('#photo-reload').click();
    await page.waitForFunction(()=>document.getElementById('photo-caption').value==='Изменено в другом окне');
    await page.locator('#photo-caption').fill('Монтаж окон, второй этаж');
    await viewer.getByRole('button',{name:'Сохранить подпись'}).click();
    await page.waitForFunction(()=>document.querySelector('.photo-tile span').textContent==='Монтаж окон, второй этаж');
    const downloadPromise=page.waitForEvent('download');
    await page.locator('#photo-download').click();
    const download=await downloadPromise;
    await download.saveAs(path.join(output,'download.jpg'));
    assert(fs.statSync(path.join(output,'download.jpg')).size>100);
    await viewer.getByRole('button',{name:'Закрыть',exact:true}).click();
    for (const width of [320,390,768,1440]) for (const theme of ['light','dark']) {
      await page.setViewportSize({width,height:width<600?844:1000});
      await page.evaluate(theme=>document.documentElement.dataset.theme=theme,theme);
      await root.scrollIntoViewIfNeeded();
      assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`page overflow ${width}`);
      assert(await page.locator('.photo-tile img').first().evaluate(img=>Math.abs(img.clientWidth/img.clientHeight-4/3)<0.03),'thumbnail aspect ratio');
      await page.screenshot({path:path.join(output,`photos-${width}-${theme}.png`),animations:'disabled'});
      await page.locator('.photo-tile').first().click();
      await viewer.locator('#photo-image').evaluate(img=>img.decode());
      assert(await viewer.evaluate(e=>e.scrollWidth<=e.clientWidth),'viewer overflow');
      if(process.env.AXE_PATH) {
        await page.addScriptTag({path:process.env.AXE_PATH});
        const issues=await page.evaluate(async()=>(await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>({target:n.target,summary:n.failureSummary}))})));
        assert.deepEqual(issues,[],JSON.stringify({width,theme,issues}));
      }
      await page.screenshot({path:path.join(output,`viewer-${width}-${theme}.png`),animations:'disabled'});
      await page.keyboard.press('ArrowRight');
      assert.equal(await page.locator('#photo-position').textContent(),'2 / 3');
      await page.keyboard.press('Escape');
    }
    await page.locator('#photos-files').setInputFiles({name:'invalid.jpg',mimeType:'image/jpeg',buffer:Buffer.from('not a photo')});
    await page.locator('#photos-errors').waitFor({state:'visible'});
    assert.equal(await page.locator('.photo-tile').count(),3,'bad image changed list');
    assert.equal(await page.locator('#photos-retry').isVisible(),true);
    await page.route('**/object/object/photos',route=>route.request().method()==='POST'?route.abort():route.continue());
    await page.locator('#photos-files').setInputFiles(files[1]);
    await page.waitForFunction(()=>document.getElementById('photos-errors').textContent.includes('Нет соединения'));
    assert.equal(await page.locator('.photo-tile').count(),3);
    await page.unroute('**/object/object/photos');
    await page.locator('#photos-retry').click();
    await page.waitForFunction(()=>document.getElementById('photos-status').textContent.includes('Уже есть: 1'));
    await page.locator('.photo-tile').first().click();
    await page.locator('#photo-delete').click();
    await page.locator('#photo-delete-cancel').click();
    assert.equal(await page.locator('.photo-tile').count(),3);
    await page.locator('#photo-delete').click();
    await page.locator('#photo-delete-confirm').click();
    await page.waitForFunction(()=>!document.getElementById('photo-viewer').open && document.querySelectorAll('.photo-tile').length===2);
    assert.deepEqual(errors,[]);
    console.log(JSON.stringify({checks:'batch, dedupe, caption, download, errors, retry, deletion, 8 layouts/themes with accessibility',output}));
  } finally { await browser.close(); }
})().catch(error=>{console.error(error);process.exit(1);});
