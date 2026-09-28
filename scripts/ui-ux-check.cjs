// Run against the isolated TestSiteIntegration preview, never production data.
const { chromium } = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const base = process.env.UI_BASE_URL || 'http://127.0.0.1:8100';
const output = process.env.UI_OUTPUT_DIR || path.join(os.tmpdir(), 'workservice-ux');

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const browser = await chromium.launch({ headless: true, channel: process.env.UI_BROWSER || 'msedge' });
  try {
    const context = await browser.newContext({ viewport: { width: 390, height: 844 }, serviceWorkers: 'block' });
    const page = await context.newPage();
    const errors = [];
    const issues = [];
    const accessibility = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto(base + '/login');
    await page.locator('#username').fill('admin');
    await page.locator('#password').fill('testpass');
    await Promise.all([page.waitForURL('**/dashboard'), page.locator('button[type=submit]').click()]);
    await page.goto(base + '/vehicles');
    const vehicle = await page.locator('a[href^="/vehicles/"]').first().getAttribute('href');
    const routes = ['/dashboard', '/workers', '/worker/own', '/workers/new', '/workers/edit/own', '/objects', '/object/object', '/objects/new', '/schedule?month=2026-09', '/schedule/new', '/schedule/edit/absence', '/timesheets?month=2026-09', '/profile', '/profile/menu', '/users', '/users/new', '/users/edit/user', '/vehicles', vehicle, '/gallery', '/settings', '/improvements'];
    for (const width of [320, 390, 768, 1024, 1440, 1920, 2560]) {
      await page.setViewportSize({ width, height: width < 1024 ? 844 : 1080 });
      for (const theme of ['light', 'dark']) {
        await page.emulateMedia({ colorScheme: theme });
        for (const route of routes) {
          const response = await page.goto(base + route);
          await page.evaluate(() => document.fonts.ready);
          const layout = await page.evaluate(() => {
            const main = document.querySelector('body > .main-content');
            const rect = main.getBoundingClientRect();
            const header = document.querySelector('.top-nav').getBoundingClientRect();
            return {
              width: innerWidth, scroll: document.documentElement.scrollWidth,
              mainRight: rect.right, mainTop: rect.top, headerBottom: header.bottom,
              bodyTop: document.body.getBoundingClientRect().top,
              background: getComputedStyle(document.documentElement).backgroundImage,
              mainMaxWidth: getComputedStyle(main).maxWidth,
              unnamed: [...document.querySelectorAll('input:not([type=hidden]),select,textarea')].filter(field => !field.labels?.length && !field.getAttribute('aria-label') && !field.getAttribute('aria-labelledby')).map(field => field.name)
            };
          });
          if (response.status() !== 200 || layout.scroll > width || layout.bodyTop !== 0 || layout.background === 'none' || layout.mainTop < layout.headerBottom || (width >= 1024 && layout.mainRight < width - 32) || layout.unnamed.length) issues.push({ route, width, theme, ...layout });
          if (width === 1440 && process.env.AXE_PATH) {
            await page.addScriptTag({ path: process.env.AXE_PATH });
            const results = await page.evaluate(async () => (await axe.run(document, { runOnly: {type:'tag',values:['wcag2a','wcag2aa','wcag21aa']} })).violations.map(v => ({id:v.id,impact:v.impact,nodes:v.nodes.map(n=>({target:n.target,summary:n.failureSummary}))})));
            if (results.length) accessibility.push({route, theme, results});
          }
          if ([390, 1920].includes(width) && ['/dashboard','/profile','/worker/own','/timesheets?month=2026-09'].includes(route)) await page.screenshot({path:path.join(output,`${width}-${theme}-${route.replaceAll(/[^\w]/g,'_')}.png`),fullPage:true});
        }
      }
    }

    fs.writeFileSync(path.join(output,'layout-report.json'),JSON.stringify({issues,accessibility,errors},null,2));
    await page.setViewportSize({width:390,height:844});
    await page.goto(base + '/workers');
    await page.locator('[data-mobile-nav-toggle]').click();
    assert(await page.locator('.side-nav').evaluate(e => e.contains(document.activeElement)));
    for (let i=0;i<18;i++) {
      await page.keyboard.press('Tab');
      assert(await page.locator('.side-nav').evaluate(e => e.contains(document.activeElement)), 'Navigation focus escaped');
    }
    await page.keyboard.press('Escape');
    assert(await page.locator('[data-mobile-nav-toggle]').evaluate(e => e === document.activeElement));
    assert(await page.locator('.side-nav').evaluate(e => e.inert), 'Hidden navigation remains focusable');

    await page.locator('[data-modal-url="/workers/new"]:visible').first().click();
    const modal = page.locator('#app-action-modal');
    await modal.locator('#name').waitFor();
    for(let i=0;i<12;i++) {
      await page.keyboard.press('Tab');
      assert(await modal.evaluate(e => e.contains(document.activeElement)), 'Modal focus escaped');
    }
    await modal.locator('button[type=submit]').click();
    assert.equal(await modal.locator('#name').getAttribute('aria-invalid'),'true');
    assert(await modal.locator('.field-feedback').first().isVisible());
    await modal.locator('#name').fill('UI regression fixture');
    await modal.locator('#position').fill('Test');
    const buttonBefore = await modal.locator('button[type=submit]').boundingBox();
    let submitted = 0;
    await page.route('**/workers/new', async route => {
      if (route.request().method() !== 'POST') return route.continue();
      submitted++;
      await new Promise(resolve => setTimeout(resolve, 250));
      await route.fulfill({status:400,contentType:'text/plain; charset=utf-8',body:'Проверьте данные работника.'});
    });
    await modal.locator('button[type=submit]').click();
    assert(await modal.locator('form').evaluate(e => e.getAttribute('aria-busy')==='true'));
    const buttonPending = await modal.locator('button[type=submit]').boundingBox();
    assert.equal(buttonPending.width,buttonBefore.width,'Submit button changed width');
    await modal.locator('form').evaluate(form=>form.requestSubmit());
    await modal.locator('[role=alert]').waitFor();
    assert.equal(submitted,1);
    assert.equal(await modal.locator('#name').inputValue(),'UI regression fixture');
    assert.equal(await modal.locator('button[type=submit]').isEnabled(),true);
    await page.screenshot({path:path.join(output,'modal-error.png')});
    await page.keyboard.press('Escape');
    assert.equal(await modal.getAttribute('aria-hidden'),'true');
    assert(await page.locator('body > .main-content').evaluate(e=>!e.inert));
    await page.locator('[data-modal-url="/workers/new"]:visible').first().click();
    await modal.locator('#name').waitFor();
    assert.equal(await modal.locator('[data-page-toast]').count(),0,'Stale error on reopened form');
    await page.keyboard.press('Escape');

    await page.goto(base + '/workers/new');
    await page.locator('#name').fill('Preserved after server error');
    await page.locator('#position').fill('Test');
    await page.locator('button[type=submit]').click();
    await page.locator('[role=alert]').waitFor();
    assert.equal(await page.locator('#name').inputValue(),'Preserved after server error');
    await page.unroute('**/workers/new');
    await page.route('**/workers/new',route=>route.request().method()==='POST' ? route.abort('failed') : route.continue());
    await page.locator('button[type=submit]').click();
    await page.getByText('Нет ответа от сервера.',{exact:false}).waitFor();
    assert.equal(await page.locator('#name').inputValue(),'Preserved after server error');
    await page.unroute('**/workers/new');
    const htmlError = await page.evaluate(()=>WorkServiceUI.responseError(new Response('<div class="form-error">Ошибка периода</div>',{headers:{'Content-Type':'text/html'}}),'<div class="form-error">Ошибка периода</div>'));
    assert.equal(htmlError,'Ошибка периода','HTML validation returned with HTTP 200 was lost');

    await page.goto(base+'/workers/edit/own');
    await page.getByRole('button',{name:'Уволить',exact:true}).click();
    assert(await page.locator('dialog').evaluate(e=>e.open));
    await page.keyboard.press('Escape');
    assert.equal(await page.locator('dialog').evaluate(e=>e.open),false);
    await page.waitForFunction(()=>document.activeElement?.textContent.trim()==='Уволить');

    await page.goto(base + '/profile');
    await page.evaluate(()=>document.querySelector('.profile-identity h1').textContent='Очень длинное имя пользователя для проверки адаптивной верстки');
    await page.addStyleTag({content:'html { font-size: 200%; }'});
    assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'200% scaling overflow');
    assert(await page.locator('.settings-form .form-group-edit').first().evaluate(group=>group.querySelector('input').getBoundingClientRect().top >= group.querySelector('label').getBoundingClientRect().bottom),'Large text must stack profile fields');
    await page.screenshot({path:path.join(output,'profile-zoom.png'),fullPage:true});
    const report = {pages:routes.length*14, issues, accessibility, errors, interactions:'passed'};
    fs.writeFileSync(path.join(output,'report.json'),JSON.stringify(report,null,2));
    console.log(JSON.stringify(report,null,2));
    assert.equal(issues.length,0,'Layout/label issues');
    assert.equal(accessibility.length,0,'Accessibility issues');
    assert.equal(errors.length,0,'Browser errors');
  } finally { await browser.close(); }
})().catch(error => {console.error(error);process.exitCode=1;});
