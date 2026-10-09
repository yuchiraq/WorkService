// Run only against the isolated TestSiteIntegration preview (WORKSERVICE_UI_PREVIEW=1).
const { chromium } = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const base = process.env.UI_BASE_URL || 'http://127.0.0.1:8100';
const output = path.join(os.tmpdir(), 'workservice-inventory');

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const browser = await chromium.launch({ headless: true, channel: process.env.UI_BROWSER || 'msedge' });
  try {
    const context = await browser.newContext({ viewport: { width: 390, height: 844 }, serviceWorkers: 'block' });
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto(base + '/login');
    await page.locator('#username').fill('worker');
    await page.locator('#password').fill('testpass');
    await Promise.all([page.waitForURL('**/schedule'), page.locator('button[type=submit]').click()]);
    await page.goto(base + '/object/object');
    await page.locator('#inventory-add').waitFor();
    assert(await page.locator('.side-nav a[href="/objects"]').count());
    const settle = () => page.waitForFunction(() => document.querySelector('#object-inventory').getAttribute('aria-busy') !== 'true');
    const rows = page.locator('.inventory-row');
    const api = async (suffix, values) => page.evaluate(async ({suffix, values}) => {
      const root = document.querySelector('#object-inventory');
      const response = await fetch(root.dataset.endpoint + suffix, {
        method: values ? 'POST' : 'GET', headers: { 'X-CSRF-Token': root.dataset.csrf },
        body: values ? new URLSearchParams(values) : undefined,
      });
      return {status: response.status, data: await response.json()};
    }, {suffix, values});
    // Leave the preview reusable between runs.
    for (const item of (await api('')).data.items) assert.equal((await api(`/${item.id}/delete`, {version:item.version})).status, 200);
    await page.reload();
    assert(await page.locator('#inventory-empty').isVisible());
    async function add(name, amount = '1', unit = 'pcs', note = '') {
      await page.locator('#inventory-add').click();
      await page.locator('#inventory-name').fill(name);
      await page.locator('#inventory-quantity').fill(amount);
      await page.locator('#inventory-unit').selectOption(unit);
      if (note) await page.locator('#inventory-note').fill(note);
      await page.locator('#inventory-submit').click();
      await page.locator('#inventory-editor').waitFor({state:'hidden'});
      await settle();
    }
    await add('Перфоратор Bosch', '2', 'pcs', 'Склад, полка 2');
    await add('Кабель силовой', '12,5', 'm');
    await add('Аптечка', '1', 'set');
    await add('Удлинитель промышленный с защитой от влаги для наружных работ', '3');
    await add('Кабель силовой', '0,125', 'm');
    assert.equal(await rows.count(), 4);
    assert.equal(await rows.first().locator('h3').textContent(), 'Аптечка');
    let cable = rows.filter({has:page.locator('h3', {hasText:/^Кабель силовой$/})});
    assert.equal(await cable.locator('input').inputValue(), '12,625');
    await cable.locator('[data-action=plus]').click();
    await settle();
    assert.equal(await cable.locator('input').inputValue(), '13,625');
    await cable.locator('[data-action=minus]').click();
    await settle();
    await cable.locator('input').fill('20,75');
    await cable.locator('input').press('Enter');
    await settle();
    assert.equal(await cable.locator('input').inputValue(), '20,75');
    await cable.locator('[data-action=edit]').click();
    await page.locator('#inventory-note').fill('Щитовая, второй этаж');
    await page.locator('#inventory-submit').click();
    await page.locator('#inventory-editor').waitFor({state:'hidden'});
    await settle();
    assert.equal(await cable.locator('.inventory-note').textContent(), 'Щитовая, второй этаж');
    await page.locator('#inventory-search').fill('щитовая');
    assert.equal(await rows.count(), 1);
    await page.locator('#inventory-search').fill('несуществующая позиция');
    assert(await page.locator('#inventory-empty').isVisible());
    await page.locator('#inventory-search').fill('');
    await page.reload();
    assert.equal(await rows.count(), 4);
    await page.locator('#inventory-add').click();
    const suggestions = await page.locator('#inventory-names option').evaluateAll(nodes => nodes.map(n => n.value));
    assert(suggestions.includes('Кабель силовой'));
    await page.locator('#inventory-name').fill('Кабель силовой');
    assert.equal(await page.locator('#inventory-unit').inputValue(), 'm');
    assert(await page.locator('#inventory-existing').isVisible());
    assert(await page.locator('#inventory-note').isDisabled());
    await page.locator('#inventory-editor').getByRole('button',{name:'Отмена',exact:true}).click();
    await page.waitForFunction(() => document.activeElement.id === 'inventory-add');

    // A stale absolute value must not overwrite a concurrent change.
    await cable.locator('input').fill('21');
    const item = (await api('')).data.items.find(i => i.name === 'Кабель силовой');
    assert.equal((await api(`/${item.id}/adjust`, {delta:'1000'})).status, 200);
    await cable.locator('[data-action=save]').click();
    await settle();
    await cable.locator('.inventory-conflict').waitFor();
    assert.equal(await cable.locator('input').inputValue(), '21');
    assert(await cable.locator('[data-action=save]').isDisabled());
    await cable.locator('[data-action=reset]').click();
    assert.equal(await cable.locator('input').inputValue(), '21,75');
    await page.locator('[data-page-toast] button').click();

    // Errors stay in the editor with all fields intact.
    await page.locator('#inventory-add').click();
    await page.locator('#inventory-name').fill('Ошибка количества');
    await page.locator('#inventory-quantity').fill('1000001');
    await page.locator('#inventory-submit').click();
    await page.locator('#inventory-editor [role=alert]').waitFor();
    assert.equal(await page.locator('#inventory-name').inputValue(), 'Ошибка количества');
    assert(page.url().endsWith('/object/object'));
    await page.keyboard.press('Escape');
    await page.locator('#inventory-add').click();
    assert.equal(await page.locator('#inventory-editor [data-page-toast]').count(), 0);
    await page.keyboard.press('Escape');

    // Drafts in other rows survive successful saves and refreshes.
    const drill = rows.filter({has:page.locator('h3',{hasText:/^Перфоратор Bosch$/})});
    await drill.locator('input').fill('7');
    await cable.locator('[data-action=plus]').click();
    await settle();
    assert.equal(await drill.locator('input').inputValue(),'7');
    await drill.locator('[data-action=save]').click();
    await settle();
    assert.equal(await drill.locator('input').inputValue(),'7');
    // A failed request must not discard the user's value or navigate away.
    await drill.locator('input').fill('9');
    await page.route('**/inventory/*/quantity', route => route.abort('failed'));
    await drill.locator('[data-action=save]').click();
    await page.getByText('Нет связи с сервером.',{exact:false}).waitFor();
    await settle();
    assert.equal(await drill.locator('input').inputValue(),'9');
    await page.unroute('**/inventory/*/quantity');
    await drill.locator('[data-action=reset]').click();
    await page.locator('[data-page-toast] button').click();
    await drill.locator('input').fill('0');
    await drill.locator('[data-action=save]').click();
    await settle();
    assert(await drill.locator('.inventory-zero').isVisible());
    assert(await drill.locator('[data-action=minus]').isDisabled());

    const layouts = [];
    for (const width of [320, 390, 768, 1024, 1440, 1920]) {
      await page.setViewportSize({width, height:width < 1024 ? 844 : 1080});
      for (const theme of ['light','dark']) {
        await page.evaluate(theme => document.documentElement.dataset.theme = theme, theme);
        await page.locator('#object-inventory').scrollIntoViewIfNeeded();
        const layout = await page.evaluate(() => ({width:innerWidth, scroll:document.documentElement.scrollWidth,
          fields:[...document.querySelectorAll('.inventory-row input')].map(e => ({width:e.getBoundingClientRect().width,height:e.getBoundingClientRect().height})),
          buttons:[...document.querySelectorAll('#object-inventory .inv-button:not([hidden])')].filter(e=>e.getClientRects().length).map(e=>({width:e.getBoundingClientRect().width,height:e.getBoundingClientRect().height}))}));
        assert(layout.scroll <= width, `overflow ${width} ${theme}: ${layout.scroll}`);
        assert(layout.buttons.every(b => b.width >= 44 && b.height >= 44), 'small tap target');
        layouts.push({theme,...layout});
        await page.screenshot({path:path.join(output,`${width}-${theme}.png`),fullPage:true});
        await cable.locator('input').fill('123456,789');
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'dirty row overflow');
        await cable.locator('[data-action=reset]').click();
        await page.locator('#inventory-add').click();
        await page.locator('#inventory-name').fill('Название для проверки формы');
        assert(await page.evaluate(() => document.querySelector('#inventory-editor').scrollWidth <= document.querySelector('#inventory-editor').clientWidth), 'dialog overflow');
        for(let i=0;i<12;i++) {
          await page.keyboard.press('Tab');
          assert(await page.locator('#inventory-editor').evaluate(e=>e.contains(document.activeElement)), 'dialog focus escaped');
        }
        await page.screenshot({path:path.join(output,`${width}-${theme}-dialog.png`)});
        if (process.env.AXE_PATH && [390,1440].includes(width)) {
          await page.addScriptTag({path:process.env.AXE_PATH});
          const result = await page.evaluate(async () => (await axe.run(document, {runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations);
          fs.writeFileSync(path.join(output,`axe-${width}-${theme}.json`), JSON.stringify(result,null,2));
          assert.equal(result.length,0, JSON.stringify(result.map(r=>({id:r.id,nodes:r.nodes.map(n=>n.target)}))));
        }
        await page.keyboard.press('Escape');
      }
    }

    await page.setViewportSize({width:390,height:844});
    await cable.locator('[data-action=delete]').click();
    await page.locator('#inventory-delete-confirm').click();
    await page.locator('#inventory-delete').waitFor({state:'hidden'});
    await settle();
    assert.equal(await rows.count(),3);
    await page.locator('#inventory-add').click();
    assert((await page.locator('#inventory-names option').evaluateAll(nodes=>nodes.map(n=>n.value))).includes('Кабель силовой'), 'deleted item lost from catalog');
    await page.keyboard.press('Escape');
    await page.locator('.inventory-history summary').click();
    assert((await page.locator('#inventory-history').textContent()).includes('Иванов Иван'));
    assert.equal(errors.length,0,errors.join('\n'));
    fs.writeFileSync(path.join(output,'report.json'),JSON.stringify({layouts,errors,passed:true},null,2));
    console.log(`Inventory UI checks passed. Screenshots: ${output}`);
  } finally { await browser.close(); }
})().catch(error => {console.error(error);process.exitCode=1;});
