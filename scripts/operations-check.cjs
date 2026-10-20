// Use only the isolated TestSiteIntegration preview, never production data.
const {chromium} = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const base = process.env.UI_BASE_URL || 'http://127.0.0.1:8099';
const output = path.join(os.tmpdir(),'workservice-operations');
(async()=>{
  fs.mkdirSync(output,{recursive:true});
  const browser=await chromium.launch({headless:true,channel:process.env.UI_BROWSER||'msedge'});
  try {
    const context=await browser.newContext({viewport:{width:390,height:844},serviceWorkers:'block'});
    const page=await context.newPage();
    const errors=[];
    page.on('pageerror',error=>errors.push(error.message));
    async function login(username){
      await page.goto(base+'/login');
      await page.locator('#username').fill(username);
      await page.locator('#password').fill('testpass');
      await Promise.all([page.waitForURL(username==='admin'?'**/dashboard':'**/schedule'),page.locator('button[type=submit]').click()]);
    }
    async function picker(scope=page){
      await scope.locator('.worker-picker-trigger').last().click();
      const dialog=page.locator('.worker-picker-dialog[open]');
      await dialog.locator('.worker-choice:not([disabled])').first().waitFor();
      return dialog;
    }
    await login('admin');
    await page.goto(base+'/schedule/new?date=2026-09-21');
    let dialog=await picker();
    let ids=await dialog.locator('.worker-choice').evaluateAll(nodes=>nodes.map(n=>n.dataset.workerId));
    assert.deepEqual(ids,['idle','other','own']);
    assert(await dialog.locator('[data-worker-id=own]').evaluate(e=>e.classList.contains('is-busy')));
    await page.screenshot({path:path.join(output,'workers-390-light.png')});
    await dialog.locator('[data-worker-id=own]').click();
    assert(await page.locator('.worker-picker-trigger').first().evaluate(e=>e.classList.contains('is-busy')));
    dialog=await picker();
    assert.equal(await dialog.locator('[data-worker-id=own]').count(),0,'duplicate worker offered');
    await dialog.getByRole('button',{name:'Закрыть',exact:true}).click();
    await page.locator('[data-remove-select]').last().click();
    await page.locator('#date').fill('2026-09-22');
    dialog=await picker();
    assert.equal(await dialog.locator('.worker-choice.is-busy').last().getAttribute('data-worker-id'),'other');
    await dialog.getByRole('button',{name:'Закрыть',exact:true}).click();

    // Dynamic scripts must work on every opening, including within the action modal.
    await page.goto(base+'/schedule?month=2026-09');
    for(let i=0;i<2;i++){
      await page.locator('[data-modal-url="/schedule/new"]:visible').first().click();
      const modal=page.locator('#app-action-modal');
      await modal.locator('.worker-picker-trigger').first().waitFor();
      await modal.locator('#date').fill('2026-09-22');
      dialog=await picker(modal);
      assert.equal(await dialog.locator('.worker-choice.is-busy').last().getAttribute('data-worker-id'),'other');
      await dialog.getByRole('button',{name:'Закрыть',exact:true}).click();
      await page.keyboard.press('Escape');
      await page.waitForFunction(()=>document.querySelector('#app-action-modal').getAttribute('aria-hidden')==='true');
    }
    await page.goto(base+'/schedule/edit/own-entry');
    dialog=await picker();
    assert.equal(await dialog.locator('.worker-choice.is-busy').count(),0,'editing counted current assignment');
    await dialog.getByRole('button',{name:'Закрыть',exact:true}).click();

    await page.goto(base+'/vehicles');
    const vehicle=await page.locator('a[href^="/vehicles/"]').first().getAttribute('href');
    await page.goto(base+vehicle);
    assert.equal(await page.locator('select[name=assigned_user_id]').count(),0);
    await page.getByRole('button',{name:'Снять привязку',exact:true}).click();
    await page.locator('select[name=assigned_user_id]').waitFor();
    assert.equal(await page.getByRole('button',{name:'Снять привязку',exact:true}).count(),0);
    await page.locator('select[name=assigned_user_id]').selectOption('other');
    await page.getByRole('button',{name:'Привязать',exact:true}).click();
    await page.getByRole('button',{name:'Снять привязку',exact:true}).waitFor();
    assert.equal(await page.locator('select[name=assigned_user_id]').count(),0);
    await page.getByRole('button',{name:'Снять привязку',exact:true}).click();
    await page.locator('select[name=assigned_user_id]').waitFor();
    await page.locator('select[name=assigned_user_id]').selectOption('user');
    await page.getByRole('button',{name:'Привязать',exact:true}).click();
    await page.getByRole('button',{name:'Снять привязку',exact:true}).waitFor();
    const month=await page.locator('#waybill_month').inputValue();
    const pdf=await context.request.get(base+vehicle+'/waybill?month='+month);
    assert.equal(pdf.status(),200);
    fs.writeFileSync(path.join(output,'waybill-ownership.pdf'),await pdf.body());

    await page.goto(base+'/object/object');
    await page.evaluate(async()=>{
      const section=document.querySelector('#object-inventory');
      const response=await fetch(section.dataset.endpoint,{method:'POST',headers:{'X-CSRF-Token':section.dataset.csrf},body:new URLSearchParams({name:'Лестница алюминиевая',unit:'pcs',quantity:'2'})});
      if(!response.ok)throw new Error('fixture inventory');
    });
    await page.goto(base+'/objects/edit/object');
    await page.locator('[name=responsible_user_id]').selectOption('admin');
    await page.locator('[name=status]').selectOption('paused');
    await page.locator('form[data-object-status-form] button[type=submit]').click();
    dialog=page.locator('dialog[open]');
    await dialog.waitFor();
    assert((await dialog.textContent()).includes('Лестница алюминиевая'));
    await page.screenshot({path:path.join(output,'object-warning-390-light.png')});
    await dialog.locator('[data-cancel]').click();
    assert(page.url().includes('/objects/edit/object'));
    await page.locator('form[data-object-status-form] button[type=submit]').click();
    await page.locator('dialog[open] [data-confirm]').click();
    await page.waitForURL('**/objects');
    // Reopen as a modal and exercise its nested native confirmation dialog.
    await page.locator('[data-modal-url="/objects/edit/object"]').click();
    const modal=page.locator('#app-action-modal');
    await modal.locator('[name=status]').selectOption('completed');
    await modal.locator('form[data-object-status-form] button[type=submit]').click();
    await page.locator('dialog[open] [data-confirm]').click();
    await page.waitForFunction(()=>document.querySelector('#app-action-modal').getAttribute('aria-hidden')==='true');
    await page.goto(base+'/object/object');
    assert((await page.locator('.profile-header-info').textContent()).includes('Окончен'));
    assert((await page.locator('#inventory-items').textContent()).includes('Лестница алюминиевая'));

    for(const width of [320,390,768,1440]){
      await page.setViewportSize({width,height:width<1024?844:1080});
      for(const theme of ['light','dark']){
        await page.goto(base+'/schedule/new?date=2026-09-22');
        await page.evaluate(theme=>document.documentElement.dataset.theme=theme,theme);
        dialog=await picker();
        assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'horizontal overflow');
        if(process.env.AXE_PATH){
          await page.addScriptTag({path:process.env.AXE_PATH});
          const issues=await page.evaluate(async()=>(await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.target)})));
          assert.equal(issues.length,0,JSON.stringify(issues));
        }
        await page.screenshot({path:path.join(output,`workers-${width}-${theme}.png`)});
        await dialog.getByRole('button',{name:'Закрыть',exact:true}).click();
        await page.goto(base+'/objects/edit/object');
        await page.evaluate(theme=>document.documentElement.dataset.theme=theme,theme);
        await page.locator('[name=status]').selectOption('paused');
        await page.locator('form[data-object-status-form] button[type=submit]').click();
        await page.locator('dialog[open]').waitFor();
        assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'warning overflow');
        await page.screenshot({path:path.join(output,`warning-${width}-${theme}.png`)});
        await page.locator('dialog[open] [data-cancel]').click();
      }
    }
    await page.goto(base+'/logout');
    await login('worker');
    await page.goto(base+'/schedule/new?date=2026-09-21');
    await page.locator('.worker-picker-trigger').first().waitFor();
    assert(await page.locator('.worker-picker-trigger').first().isDisabled());
    dialog=await picker();
    assert.equal(await dialog.locator('[data-worker-id=own]').count(),0);
    await dialog.getByRole('button',{name:'Закрыть',exact:true}).click();
    assert.equal(errors.length,0,errors.join('\n'));
    console.log('Operations checks passed. Screenshots and PDF: '+output);
  } catch(error) {
    for(const context of browser.contexts()) for(const page of context.pages()) await page.screenshot({path:path.join(output,'failure.png')}).catch(()=>{});
    throw error;
  } finally {await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
