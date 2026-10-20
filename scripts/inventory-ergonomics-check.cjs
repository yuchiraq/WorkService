// Destructive fixtures: run only against TestSiteIntegration with WORKSERVICE_UI_PREVIEW=1.
const {chromium} = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const base = process.env.UI_BASE_URL || 'http://127.0.0.1:8099';
const output = path.join(os.tmpdir(), 'workservice-inventory-ergonomics');
(async () => {
  fs.mkdirSync(output, {recursive:true});
  const browser = await chromium.launch({headless:true,channel:process.env.UI_BROWSER || 'msedge'});
  const errors = [];
  try {
    const context = await browser.newContext({viewport:{width:390,height:844},isMobile:true,hasTouch:true,serviceWorkers:'block',reducedMotion:'reduce'});
    const page = await context.newPage();
    page.on('pageerror', error => errors.push(error.message));
    await page.goto(base+'/login');
    await page.locator('#username').fill('admin');
    await page.locator('#password').fill('testpass');
    await Promise.all([page.waitForURL('**/dashboard'),page.locator('button[type=submit]').click()]);
    await page.goto(base+'/object/object');
    const csrf = await page.locator('#object-inventory').getAttribute('data-csrf');
    const api = async (suffix='',form) => {
      const url = base+'/object/object/inventory'+suffix;
      const response = form ? await context.request.post(url,{headers:{'X-CSRF-Token':csrf},form}) : await context.request.get(url);
      assert.equal(response.status(),200,await response.text());
      return response.json();
    };
    for (const item of (await api()).items) await api(`/${item.id}/delete`,{version:String(item.version)});
    for (const [name,material,quantity,note] of [
      ['ПВХ 250 труба 1,5 м','ПВХ','999999.999','Склад, второй этаж'],
      ['ПВХ 160 тройник','ПВХ','0',''],
      ['Нерж 160 отвод 90','Нержавейка','12',''],
      ['Крепление для вентиляционных труб с виброизоляцией и усиленной монтажной пластиной','Оцинковка','20','Длинное примечание для проверки читаемости и доступности полного текста в редакторе'],
      ['Без материала','','1','']
    ]) await api('',{name,material,quantity,note,unit:'pcs'});
    await page.reload();
    const rows = page.locator('.inventory-row');
    const search = page.locator('#inventory-search');
    const material = page.locator('#inventory-filter-material');
    const stock = page.locator('#inventory-filter-stock');
    const settle = () => page.waitForFunction(()=>document.querySelector('#object-inventory').getAttribute('aria-busy') !== 'true');
    await search.fill('250 пвх');
    assert.equal(await rows.count(),1,'search should accept unordered words');
    await page.locator('#inventory-search-clear').click();
    assert.equal(await rows.count(),5);
    assert(await search.evaluate(el=>el===document.activeElement));
    await material.selectOption('material:ПВХ');
    assert.equal(await rows.count(),2);
    await stock.selectOption('empty');
    assert.equal(await rows.count(),1);
    assert.equal(await page.locator('#inventory-count').textContent(),'1 из 5');
    await rows.locator('[data-action=plus]').click();
    await settle();
    assert.equal(await rows.count(),0,'stock filter must update after mutation');
    assert(await page.locator('#inventory-empty').isVisible());
    await stock.selectOption('available');
    assert.equal(await rows.count(),2);
    const draft = rows.filter({hasText:'ПВХ 160 тройник'}).locator('input');
    await draft.fill('2,5');
    await material.selectOption('material:Нержавейка');
    await material.selectOption('material:ПВХ');
    assert.equal(await draft.inputValue(),'2,5','filter discarded draft');
    await draft.locator('..').locator('..').locator('[data-action=reset]').click();
    assert(await page.evaluate(()=>!document.activeElement.matches('input,textarea')),'reset opened mobile keyboard');
    await page.locator('#inventory-add').click();
    assert.equal(await page.locator('#inventory-material').inputValue(),'ПВХ');
    assert(await page.evaluate(()=>!document.activeElement.matches('input,textarea')),'editor opened mobile keyboard');
    await page.keyboard.press('Escape');
    await page.locator('#inventory-filter-reset').click();
    assert.equal(await rows.count(),5);
    assert.equal(await material.inputValue(),'');
    assert.equal(await stock.inputValue(),'');
    await material.selectOption('material:');
    assert.equal(await rows.count(),1,'ungrouped filter');
    await page.locator('#inventory-filter-reset').click();

    for (const width of [320,390,600,768,1440]) for (const theme of ['light','dark']) {
      await page.setViewportSize({width,height:width<700?844:1000});
      await page.evaluate(theme=>document.documentElement.dataset.theme=theme,theme);
      await page.locator('#object-inventory').evaluate(el=>window.scrollTo(0,scrollY+el.getBoundingClientRect().top-110));
      assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`page overflow ${width}/${theme}`);
      const targets = await page.locator('.inventory-row input,.inventory-row button:visible,.inventory-row summary,.inventory-group-heading,#inventory-search,#inventory-filter-material,#inventory-filter-stock').evaluateAll(elements=>elements.map(el=>({name:el.getAttribute('aria-label')||el.className,w:el.getBoundingClientRect().width,h:el.getBoundingClientRect().height})));
      assert(targets.every(t=>t.w>=48 && t.h>=48),JSON.stringify(targets.filter(t=>t.w<48 || t.h<48)));
      const geometry = await rows.evaluateAll(elements=>elements.map(el=>{
        const info=el.querySelector('.inventory-info').getBoundingClientRect(),controls=el.querySelector('.inventory-stepper').getBoundingClientRect();
        return info.right<=controls.left || info.bottom<=controls.top;
      }));
      assert(geometry.every(Boolean),'name overlaps quantity');
      const maxAmount = rows.filter({hasText:'ПВХ 250 труба'}).locator('input');
      assert(await maxAmount.evaluate(el=>el.scrollWidth<=el.clientWidth),'max quantity clipped');
      await page.screenshot({path:path.join(output,`${width}-${theme}.png`)});
      if(process.env.AXE_PATH){
        await page.addScriptTag({path:process.env.AXE_PATH});
        const violations=await page.evaluate(async()=>(await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>({target:n.target,summary:n.failureSummary}))})));
        assert.deepEqual(violations,[],JSON.stringify(violations));
      }
    }
    await page.setViewportSize({width:390,height:844});
    const lastMenu=rows.last().locator('summary');
    await lastMenu.evaluate(el=>window.scrollTo(0,scrollY+el.getBoundingClientRect().bottom-innerHeight+20));
    await lastMenu.click();
    await page.waitForFunction(()=>document.querySelector('.inventory-row-actions[open]')?.classList.contains('opens-up'));
    const menuBounds=await rows.last().locator('.inventory-row-menu').boundingBox();
    assert(menuBounds.y>=0 && menuBounds.y+menuBounds.height<=844,'menu outside viewport');
    await page.keyboard.press('Escape');
    await page.locator('.inventory-tools').evaluate(el=>window.scrollTo(0,scrollY+el.getBoundingClientRect().top+100));
    const sticky=await page.locator('.inventory-tools').evaluate(el=>({top:el.getBoundingClientRect().top,expected:parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--content-top'))}));
    assert(Math.abs(sticky.top-sticky.expected)<2,JSON.stringify(sticky));
    await page.locator('#inventory-add').click();
    for(const [width,height] of [[390,844],[320,480],[844,390]]){
      await page.setViewportSize({width,height});
      const footer=await page.locator('#inventory-submit').boundingBox();
      assert(footer.y>=0 && footer.y+footer.height<=height,'save button outside viewport');
      await page.locator('#inventory-note').fill('Проверка формы');
      const fields=await page.locator('.inventory-dialog-fields').boundingBox();
      const button=await page.locator('#inventory-submit').boundingBox();
      assert(fields.y+fields.height<=button.y,'form fields overlap save');
      await page.screenshot({path:path.join(output,`editor-${width}.png`)});
    }
    await page.keyboard.press('Escape');
    assert.deepEqual(errors,[]);
    console.log(JSON.stringify({passed:true,checks:'filters, unordered search, draft retention, material prefill, mobile focus, 48px targets, 10 layouts, max quantity, a11y, sticky search, menu collision, compact editor',output}));
  } finally {await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
