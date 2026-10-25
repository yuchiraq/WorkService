// Run against the isolated preview after quick-actions-check.cjs seeds owned vehicles.
const {chromium}=require('playwright');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const os=require('node:os');
const base=process.env.UI_BASE_URL||'http://127.0.0.1:8099';
const output=path.join(os.tmpdir(),'workservice-quick-record');
(async()=>{
 const browser=await chromium.launch({headless:true,channel:process.env.UI_BROWSER||'msedge'});
 fs.mkdirSync(output,{recursive:true});const errors=[];
 try{
  for(const user of ['worker','admin']){
   const context=await browser.newContext({viewport:{width:390,height:844},isMobile:true,hasTouch:true,serviceWorkers:'block',reducedMotion:'reduce'});
   const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
   await page.goto(base+'/login');await page.locator('#username').fill(user);await page.locator('#password').fill('testpass');
   await Promise.all([page.waitForURL(user==='admin'?'**/dashboard':'**/schedule'),page.locator('[type=submit]').click()]);
   await page.goto(base+'/objects');
   const open=async()=>{await page.locator('#quick-create-toggle').click();await page.getByRole('menuitem',{name:'Запись по авто'}).click();if(await page.locator('#quick-vehicle-picker[open]').count())await page.locator('.quick-vehicle-choice').first().click();await page.locator('[data-vehicle-quick-form]').waitFor();};
   await open();
   assert(await page.evaluate(()=>!document.activeElement.matches('input,textarea')),'mobile keyboard opened automatically');
   for(const [width,height] of [[320,700],[390,844],[844,390],[1440,1000]])for(const theme of ['light','dark']){
    await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme});
    const form=page.locator('[data-vehicle-quick-form]');
    assert(await form.evaluate(e=>e.scrollWidth<=e.clientWidth),'form overflows');
    for(const type of ['mileage','maintenance','fuel']){
     await form.locator('[name=type]').selectOption(type);
     assert.equal(await form.locator('[name=liters]').isVisible(),type==='fuel');
     assert.equal(await form.locator('[name=amount]').isEnabled(),type==='fuel');
    }
    await form.locator('[name=notes]').fill('Проверка сохранения данных');
    await form.locator('[type=submit]').scrollIntoViewIfNeeded();
    const rect=await form.locator('[type=submit]').boundingBox();assert(rect.y>=0&&rect.y+rect.height<=height,'submit not reachable');
    await page.screenshot({path:path.join(output,`${user}-${width}-${theme}.png`)});
    if(process.env.AXE_PATH){await page.addScriptTag({path:process.env.AXE_PATH});const v=await page.evaluate(async()=>(await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(x=>({id:x.id,nodes:x.nodes.map(n=>n.failureSummary)})));assert.deepEqual(v,[],JSON.stringify(v));}
   }
   await page.locator('[data-vehicle-quick-form] [name=liters]').fill('12.5');
   await page.route('**/vehicles/*/records',route=>route.fulfill({status:422,contentType:'application/json',body:JSON.stringify({error:'Проверка ошибки записи'})}));
   await page.locator('[data-vehicle-quick-form] [type=submit]').click();
   await page.getByRole('alert').filter({hasText:'Проверка ошибки записи'}).waitFor();
   assert.equal(await page.locator('[data-vehicle-quick-form] [name=liters]').inputValue(),'12.5');
   assert(!(await page.getByRole('alert').innerText()).includes('{"error"'),'raw JSON error');
   await page.keyboard.press('Escape');await context.close();
  }
  assert.deepEqual(errors,[]);console.log(JSON.stringify({passed:true,checks:'touch quick record for worker/admin, conditional fields, error preservation, 16 layouts, a11y',output}));
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
