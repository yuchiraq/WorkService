// Run only against the isolated TestSiteIntegration preview, never production data.
const {chromium} = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const base = process.env.UI_BASE_URL || 'http://127.0.0.1:8100';
const output = path.join(os.tmpdir(), 'workservice-quick-actions');

(async () => {
  fs.mkdirSync(output, {recursive:true});
  const browser = await chromium.launch({headless:true, channel:process.env.UI_BROWSER || 'msedge'});
  const errors = [];
  try {
    async function session(username, options = {}) {
      const context = await browser.newContext({viewport:{width:390,height:844}, serviceWorkers:'block', ...options});
      const page = await context.newPage();
      page.on('pageerror', error => errors.push(error.message));
      await page.goto(base + '/login');
      await page.locator('#username').fill(username);
      await page.locator('#password').fill('testpass');
      await Promise.all([page.waitForURL(username === 'admin' ? '**/dashboard' : '**/schedule'), page.locator('button[type=submit]').click()]);
      return {context, page};
    }
    async function axe(page) {
      if (!process.env.AXE_PATH) return;
      await page.addScriptTag({path:process.env.AXE_PATH});
      const issues = await page.evaluate(async () => (await axe.run(document, {runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(v => ({id:v.id,nodes:v.nodes.map(n=>({target:n.target,summary:n.failureSummary}))})));
      assert.deepEqual(issues, [], JSON.stringify(issues));
    }
    async function aligned(page, selector) {
      await page.waitForTimeout(420);
      assert(await page.locator(selector).evaluate(group => {
        const a = group.querySelector('[aria-selected=true],[aria-current],.active').getBoundingClientRect();
        const b = group.querySelector('.ui-tab-indicator').getBoundingClientRect();
        return Math.abs(a.x-b.x)<1 && Math.abs(a.y-b.y)<1 && Math.abs(a.width-b.width)<1;
      }), 'indicator is misaligned');
    }
    const {context:adminContext,page:admin} = await session('admin');
    const {context:workerContext,page:worker} = await session('worker');
    assert.equal(await admin.locator('#quick-create-toggle').count(), 0, 'admin must not get other people\'s cars');
    const trigger = worker.locator('#quick-create-toggle');
    await trigger.click();
    assert.equal(await worker.getByRole('menuitem').count(), 2);
    await worker.keyboard.press('End');
    assert.equal(await worker.locator(':focus').innerText(), 'Запись по авто');
    await worker.keyboard.press('Home');
    assert.equal(await worker.locator(':focus').innerText(), 'Назначение');
    await worker.keyboard.press('Escape');
    assert(await trigger.evaluate(e=>e === document.activeElement));
    await trigger.click();
    await worker.getByRole('menuitem',{name:'Назначение',exact:true}).click();
    await worker.locator('#app-action-modal #date').waitFor();
    await worker.keyboard.press('Escape');
    assert(await trigger.evaluate(e=>e === document.activeElement), 'modal focus must return to visible +');
    await trigger.click();
    const singleCar = await worker.getByRole('menuitem',{name:'Запись по авто',exact:true}).getAttribute('href');
    await worker.getByRole('menuitem',{name:'Запись по авто',exact:true}).click();
    await worker.waitForURL('**/vehicles/*#vehicle-record-form');
    assert.equal(await worker.locator('#record_type').inputValue(), 'fuel');
    assert(await worker.locator('#vehicle-record-form').evaluate(e=>e === document.activeElement));
    assert(await worker.locator('#record_type').isInViewport?.() ?? await worker.locator('#record_type').isVisible());

    // Object panels and deep links preserve the current document and form state.
    await worker.goto(base + '/object/object');
    const tabs = worker.locator('[data-object-tabs]');
    await worker.evaluate(()=>{window.sameDocument='object';});
    assert(await worker.locator('#object-inventory').isVisible());
    assert(await worker.locator('#object-photos').isHidden());
    await worker.locator('#inventory-search').fill('unchanged');
    await worker.getByRole('tab',{name:'Фото',exact:true}).click();
    assert(await worker.locator('#object-inventory').isHidden());
    assert(await worker.locator('#object-photos').isVisible());
    await aligned(worker, '[data-object-tabs]');
    await worker.getByRole('tab',{name:'Назначения',exact:true}).click();
    assert(await worker.locator('#object-assignments').isVisible());
    assert(!(await worker.content()).includes('PRIVATE_OTHER_WORK'), 'private assignments leaked');
    await worker.goBack();
    assert(await worker.locator('#object-photos').isVisible());
    await worker.getByRole('tab',{name:'Инвентарь',exact:true}).click();
    assert.equal(await worker.locator('#inventory-search').inputValue(), 'unchanged');
    assert.equal(await worker.evaluate(()=>window.sameDocument), 'object');
    await worker.getByRole('tab',{name:'Инвентарь',exact:true}).focus();
    await worker.keyboard.press('ArrowRight');
    assert.equal(await worker.getByRole('tab',{name:'Инвентарь',exact:true}).getAttribute('aria-selected'), 'true');
    await worker.keyboard.press('Space');
    assert(await worker.locator('#object-photos').isVisible());
    await worker.reload();
    assert(await worker.locator('#object-photos').isVisible());
    await worker.goto(base + '/object/object#object-assignments');
    assert(await worker.locator('#object-assignments').isVisible());

    // List tabs must move in-place, preserve filters and handle rapid clicks and errors.
    await admin.goto(base + '/workers?q=Иванов');
    await admin.evaluate(()=>{window.sameDocument='workers';});
    await admin.locator('[data-fluid-navigation] a').nth(1).click();
    await admin.waitForURL('**/workers?*tab=fired');
    assert.equal(await admin.locator('#q').inputValue(), 'Иванов');
    assert.equal(await admin.locator('[name=tab]').inputValue(), 'fired');
    await aligned(admin, '[data-fluid-navigation]');
    await admin.goBack();
    await admin.waitForFunction(()=>document.querySelector('[name=tab]').value === 'active');
    assert.equal(await admin.evaluate(()=>window.sameDocument), 'workers');
    await admin.locator('[data-search-toggle]').click();
    assert.equal(await admin.locator('[data-search-toggle]').getAttribute('aria-expanded'), 'false');
    await admin.goto(base + '/objects');
    await admin.evaluate(()=>{window.sameDocument='objects';});
    const listTabs = admin.locator('[data-fluid-navigation] a');
    await listTabs.nth(1).click();
    await admin.waitForURL('**/objects?tab=completed');
    assert.equal(await admin.locator('.object-card').count(), 0);
    await listTabs.nth(0).click();
    await admin.waitForURL('**/objects?tab=active');
    assert(await admin.locator('.object-card').count() > 0);
    assert.equal(await admin.evaluate(()=>window.sameDocument), 'objects');
    await admin.route('**/objects?tab=completed', route=>route.abort());
    await listTabs.nth(1).click();
    await admin.locator('[data-page-toast]').waitFor();
    assert.equal(await listTabs.nth(0).getAttribute('aria-current'), 'page');
    assert(await admin.locator('.object-card').count() > 0);
    await admin.unroute('**/objects?tab=completed');
    await admin.route('**/objects?tab=completed', async route=>{await new Promise(r=>setTimeout(r,300));await route.continue().catch(()=>{});});
    await listTabs.nth(1).click();
    await listTabs.nth(0).click();
    await admin.waitForTimeout(600);
    assert.equal(await listTabs.nth(0).getAttribute('aria-current'), 'page');
    assert(await admin.locator('.object-card').count() > 0);
    await admin.unroute('**/objects?tab=completed');

    // Seed additional cars only in the isolated preview.
    await admin.goto(base + '/vehicles');
    const csrf = await admin.locator('[name=_csrf_token]').first().inputValue();
    for (const [name, number, owner] of [['Audi Q7','DIAL-2','user'],['Служебный автомобиль администратора','DIAL-3','admin']]) {
      const response = await adminContext.request.post(base+'/vehicles/new', {form:{name,registration_number:number,assigned_user_id:owner,_csrf_token:csrf}});
      assert(response.ok());
    }
    await worker.goto(base + '/object/object');
    await trigger.click();
    await worker.getByRole('menuitem',{name:'Запись по авто',exact:true}).click();
    const picker = worker.locator('#quick-vehicle-picker');
    assert(await picker.isVisible());
    assert.equal(await picker.locator('.quick-vehicle-choice').count(), 2);
    assert((await picker.locator('.quick-vehicle-choice').first().innerText()).includes('Audi'));
    assert(!(await picker.innerText()).includes('DIAL-3'));
    await worker.keyboard.press('Escape');
    assert(await trigger.evaluate(e=>e === document.activeElement));
    await trigger.click();
    await worker.keyboard.press('Tab');
    assert.equal(await trigger.getAttribute('aria-expanded'), 'false');
    await trigger.click();
    await worker.locator('.profile-header').click();
    assert.equal(await trigger.getAttribute('aria-expanded'), 'false');
    await trigger.click();
    await worker.getByRole('menuitem',{name:'Запись по авто',exact:true}).click();
    await picker.locator('.quick-vehicle-choice').first().click();
    await worker.waitForURL('**/vehicles/*#vehicle-record-form');
    assert.equal(await worker.locator('h1').innerText(), 'Audi Q7');
    assert.equal(await worker.locator('#record_type').inputValue(), 'fuel');
    await admin.goto(base + '/cloud');
    await admin.locator('#quick-create-toggle').click();
    assert.equal(await admin.getByRole('menuitem').count(), 3);
    const fileChooser = admin.waitForEvent('filechooser');
    await admin.getByRole('menuitem',{name:'Загрузить файлы',exact:true}).click();
    await fileChooser;
    assert.equal(await admin.locator('#quick-create-toggle').getAttribute('aria-expanded'), 'false');

    for (const width of [320,390,768,1440]) for (const theme of ['light','dark']) {
      await worker.setViewportSize({width,height:width < 600 ? 844 : 1000});
      await worker.emulateMedia({colorScheme:theme});
      await worker.goto(base + '/object/object');
      for (const name of ['Инвентарь','Фото','Назначения']) {
        await worker.getByRole('tab',{name,exact:true}).click();
        await aligned(worker,'[data-object-tabs]');
        assert(await worker.evaluate(()=>document.documentElement.scrollWidth <= innerWidth), `overflow ${width} ${theme}`);
        await axe(worker);
      }
      await trigger.click();
      await axe(worker);
      await worker.screenshot({path:path.join(output,`dial-${width}-${theme}.png`),animations:'disabled'});
      await worker.getByRole('menuitem',{name:'Запись по авто',exact:true}).click();
      await axe(worker);
      assert(await picker.evaluate(e=>e.scrollWidth <= e.clientWidth));
      await worker.screenshot({path:path.join(output,`cars-${width}-${theme}.png`),animations:'disabled'});
      await worker.keyboard.press('Escape');
    }
    await worker.emulateMedia({reducedMotion:'reduce'});
    await trigger.click();
    assert.equal(await worker.getByRole('menuitem').first().evaluate(e=>getComputedStyle(e).animationName), 'none');
    await worker.keyboard.press('Escape');
    await worker.getByRole('tab',{name:'Фото',exact:true}).click();
    assert.equal(await tabs.locator('.ui-tab-indicator').evaluate(e=>getComputedStyle(e).transitionDuration), '0s');
    const noJS = await browser.newContext({javaScriptEnabled:false,storageState:await workerContext.storageState()});
    const basic = await noJS.newPage();
    await basic.goto(base + '/object/object');
    assert(await basic.locator('[data-quick-fallback]').isVisible());
    assert(await basic.locator('#quick-create-toggle').isHidden());
    assert(await basic.locator('#object-inventory').isVisible());
    assert(await basic.locator('#object-photos').isVisible());
    await noJS.close();
    assert.deepEqual(errors, []);
    console.log(JSON.stringify({checks:'no/one/multiple vehicles, ownership, cloud upload, focus, keyboard, deep links, panel state, list filters/history/errors/races, 8 layouts, axe, reduced motion, no JS',singleCar,output}));
  } finally {await browser.close();}
})().catch(error=>{console.error(error);process.exit(1);});
