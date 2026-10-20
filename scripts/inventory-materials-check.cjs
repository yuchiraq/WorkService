// Only run against the isolated TestSiteIntegration preview.
const {chromium} = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const base = process.env.UI_BASE_URL || 'http://127.0.0.1:8099';
const output = path.join(os.tmpdir(), 'workservice-materials');
(async () => {
  fs.mkdirSync(output, {recursive:true});
  const browser = await chromium.launch({headless:true,channel:process.env.UI_BROWSER || 'msedge'});
  const errors = [];
  try {
    async function login(options) {
      const context = await browser.newContext({serviceWorkers:'block',...options});
      const page = await context.newPage();
      page.on('pageerror', e => errors.push(e.message));
      await page.goto(base+'/login');
      await page.locator('#username').fill('admin');
      await page.locator('#password').fill('testpass');
      await Promise.all([page.waitForURL('**/dashboard'),page.locator('button[type=submit]').click()]);
      return {page,context};
    }
    const {page,context} = await login({viewport:{width:390,height:844},isMobile:true,hasTouch:true});
    await page.goto(base+'/object/object');
    const root = page.locator('#object-inventory');
    const csrf = await root.getAttribute('data-csrf');
    let response = await context.request.get(base+'/object/object/inventory');
    for (const item of (await response.json()).items) {
      response = await context.request.post(base+`/object/object/inventory/${item.id}/delete`,{headers:{'X-CSRF-Token':csrf},form:{version:String(item.version)}});
      assert.equal(response.status(),200);
    }
    await page.reload();
    const settle = () => page.waitForFunction(()=>document.querySelector('#object-inventory').getAttribute('aria-busy') !== 'true');
    for (const [name,material] of [['Нерж 160 отвод 90','Нержавейка'],['Нерж 250 труба 1 м','Нержавейка'],['ПВХ 250 1,5 м','ПВХ'],['ПВХ 160 тройник','ПВХ'],['Крепление монтажное','Алюминий']]) {
      await page.locator('#inventory-search').fill(name);
      await page.locator('#inventory-search-create').click();
      assert.equal(await page.locator('#inventory-name').inputValue(),name);
      if (material === 'Алюминий') await page.locator('#inventory-material').fill(material);
      else assert.equal(await page.locator('#inventory-material').inputValue(),material);
      await page.locator('#inventory-quantity').fill('3');
      await page.locator('#inventory-submit').click();
      await page.locator('#inventory-editor').waitFor({state:'hidden'});
      await settle();
      assert(await page.locator('#inventory-search-create').isHidden());
      assert.equal(await page.locator('.inventory-row').count(),1);
    }
    await page.locator('#inventory-search').fill('');
    assert.equal(await page.locator('.inventory-group').count(),3);
    const stainless = page.locator('.inventory-group[data-material="Нержавейка"]');
    assert.equal(await stainless.locator('.inventory-row').count(),2);
    await stainless.locator('summary.inventory-group-heading').click();
    assert.equal(await stainless.getAttribute('open'),null);
    const pvc = page.locator('.inventory-group[data-material="ПВХ"]');
    await pvc.locator('[data-action=plus]').first().click();
    await settle();
    assert.equal(await stainless.getAttribute('open'),null, 'collapsed state lost after a save');
    await page.locator('#inventory-search').fill('Нержавейка');
    assert(await stainless.locator('.inventory-row').first().isVisible());
    await page.locator('#inventory-search').fill('');
    assert.equal(await stainless.getAttribute('open'),null, 'search lost collapsed state');
    await page.locator('#inventory-collapse').click();
    assert.equal(await page.locator('.inventory-group[open]').count(),0);
    await page.locator('#inventory-collapse').click();
    assert.equal(await page.locator('.inventory-group[open]').count(),3);

    const custom = page.locator('.inventory-row').filter({hasText:'Крепление монтажное'});
    await custom.locator('[data-action=menu]').click();
    await custom.locator('[data-action=edit]').click();
    await page.locator('#inventory-material').fill('ПВХ');
    await page.locator('#inventory-submit').click();
    await page.locator('#inventory-editor').waitFor({state:'hidden'});
    await settle();
    assert.equal(await page.locator('.inventory-group').count(),2);
    assert.equal(await pvc.locator('.inventory-row').count(),3);
    await page.reload();
    assert.equal(await pvc.locator('.inventory-row').count(),3);
    await page.locator('#inventory-add').click();
    await page.locator('#inventory-name').fill('Крепление монтажное');
    assert.equal(await page.locator('#inventory-material').inputValue(),'ПВХ');
    assert(await page.locator('#inventory-material').isDisabled(), 'adding quantity must preserve material');
    await page.keyboard.press('Escape');

    for (const width of [320,390,768,1440]) for (const theme of ['light','dark']) {
      await page.setViewportSize({width,height:width<600?844:1000});
      await page.evaluate(theme => document.documentElement.dataset.theme=theme,theme);
      await root.scrollIntoViewIfNeeded();
      assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`overflow ${width}/${theme}`);
      const rows = await page.locator('.inventory-row').evaluateAll(rows=>rows.map(row=>({height:row.getBoundingClientRect().height,controls:row.querySelector('.inventory-stepper').getBoundingClientRect().width})));
      assert(rows.every(row=>row.height<=100),`rows too tall ${JSON.stringify(rows)}`);
      await page.screenshot({path:path.join(output,`${width}-${theme}.png`)});
      await pvc.locator('[data-action=menu]').first().click();
      assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'menu overflow');
      if(process.env.AXE_PATH){
        await page.addScriptTag({path:process.env.AXE_PATH});
        const issues = await page.evaluate(async()=>(await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>({target:n.target,summary:n.failureSummary}))})));
        assert.deepEqual(issues,[],JSON.stringify(issues));
      }
      await page.keyboard.press('Escape');
    }
    // On touch screens the worker picker must not focus a text field, even after refresh.
    await page.setViewportSize({width:390,height:844});
    for (const modal of [false,true]) {
      await page.goto(base+(modal?'/objects':'/schedule/new?date=2026-09-21'));
      if (modal) await page.locator('.floating-create-btn:visible').click();
      const scope = modal?page.locator('#app-action-modal'):page;
      await scope.locator('.worker-picker-trigger').last().click();
      await page.locator('.worker-picker-dialog[open] .worker-choice:not([disabled])').first().waitFor();
      assert(await page.evaluate(()=>!document.activeElement.matches('input,textarea')), 'touch picker focused input');
      await page.locator('.worker-picker-search').last().click();
      assert(await page.locator('.worker-picker-search').last().evaluate(e=>e===document.activeElement),'manual search focus broken');
      await page.keyboard.press('Escape');
    }
    const {page:desktop} = await login({viewport:{width:1440,height:1000}});
    await desktop.goto(base+'/schedule/new');
    await desktop.locator('.worker-picker-trigger').last().click();
    await desktop.locator('.worker-picker-dialog[open] .worker-choice:not([disabled])').first().waitFor();
    assert(await desktop.locator('.worker-picker-search').evaluate(e=>e===document.activeElement),'desktop search no longer focused');
    assert.deepEqual(errors,[]);
    console.log(JSON.stringify({passed:true,checks:'material sections, create from search, inference, regrouping, catalog, collapse and filter state, 8 layouts, a11y, touch and desktop picker focus',output}));
  } finally {await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
