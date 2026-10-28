// Only the isolated TestSiteIntegration preview, with an explicitly trusted
// loopback proxy and WORKSERVICE_DAV_ORIGIN=https://127.0.0.1:8099.
const { chromium } = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const base = process.env.UI_BASE_URL || 'http://127.0.0.1:8099';
if (!/^http:\/\/127\.0\.0\.1:\d+$/.test(base)) throw new Error('Isolated local preview required');
const output = path.join(os.tmpdir(), 'workservice-dav-ui');
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const browser = await chromium.launch({ headless: true, channel: 'msedge' });
  try {
    const context = await browser.newContext({ serviceWorkers: 'block', reducedMotion: 'reduce', extraHTTPHeaders: { 'X-Forwarded-Proto': 'https' } });
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto(base + '/login');
    await page.locator('#username').fill('admin');
    await page.locator('#password').fill('testpass');
    await page.locator('[type=submit]').click();
    await page.waitForURL('**/dashboard');
    await page.goto(base + '/cloud');
    await page.getByRole('link', { name: 'Подключения к облаку' }).click();
    await page.waitForFunction(() => !document.getElementById('device-new').disabled);
    assert.equal(await page.locator('#device-windows').inputValue(), '\\\\127.0.0.1@SSL@8099\\DavWWWRoot\\dav');
    await page.getByRole('button', { name: 'Добавить устройство' }).click();
    await page.locator('#device-name').fill('Мой iPhone для рабочих документов');
    await page.locator('#device-confirm-password').fill('wrong');
    await page.getByRole('button', { name: 'Создать доступ' }).click();
    await page.getByRole('alert').filter({ hasText: 'Неверный текущий пароль' }).waitFor();
    await page.locator('#device-confirm-password').fill('testpass');
    await page.locator('#device-readonly').uncheck();
    await page.getByRole('button', { name: 'Создать доступ' }).click();
    await page.locator('#device-secret-dialog').waitFor({ state: 'visible' });
    const secret = await page.locator('#device-secret').inputValue();
    assert.equal(secret.length, 83);
    assert(!JSON.stringify(await page.evaluate(() => ({ ...localStorage, ...sessionStorage }))).includes(secret));
    await page.getByRole('button', { name: 'Готово', exact: true }).click();
    await page.waitForFunction(() => document.getElementById('device-secret').value === '');
    await page.locator('#device-list li').filter({ hasText: 'Мой iPhone для рабочих документов' }).first().waitFor();
    const auth = 'Basic ' + Buffer.from('admin:' + secret).toString('base64');
    let result = await context.request.put(base + '/dav/mobile.txt', { headers: { Authorization: auth }, data: 'Created from DAV' });
    assert.equal(result.status(), 201, await result.text());
    result = await context.request.get(base + '/cloud/items');
    assert((await result.json()).items.some(item => item.name === 'mobile.txt'));
    const settle = async () => {
      if (await page.evaluate(() => document.body.classList.contains('nav-open'))) await page.getByRole('button', { name: 'Закрыть меню', exact: true }).click();
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      await page.evaluate(async () => { await Promise.all(document.getAnimations().filter(a => Number.isFinite(a.effect.getTiming().iterations)).map(a => a.finished.catch(() => {}))); });
    };
    for (const width of [320, 390, 1440]) for (const colorScheme of ['light', 'dark']) {
      await page.setViewportSize({ width, height: 900 });
      await page.emulateMedia({ colorScheme });
      await settle();
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'overflow');
      await page.screenshot({ path: path.join(output, `connections-${width}-${colorScheme}.png`) });
      if (process.env.AXE_PATH) {
        await page.evaluate(fs.readFileSync(process.env.AXE_PATH, 'utf8'));
        const violations = await page.evaluate(async () => (await axe.run(document, { runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21aa'] } })).violations.map(v => ({ id: v.id, nodes: v.nodes.map(n => n.failureSummary) })));
        assert.deepEqual(violations, []);
      }
      await page.getByRole('button', { name: 'Добавить устройство' }).click();
      await page.screenshot({ path: path.join(output, `editor-${width}-${colorScheme}.png`) });
      assert(await page.locator('#device-editor').evaluate(d => d.getBoundingClientRect().width <= innerWidth), 'dialog overflow');
      await page.keyboard.press('Escape');
    }
    await page.locator('#device-list button').last().click();
    await page.locator('#device-revoke-confirm').click();
    await page.locator('#device-revoke-dialog').waitFor({ state: 'hidden' });
    result = await context.request.get(base + '/dav/mobile.txt', { headers: { Authorization: auth } });
    assert.equal(result.status(), 401);
    assert.deepEqual(errors, []);
    console.log(JSON.stringify({ passed: true, output, checks: 'device creation, confirmation, one-time secret, website/DAV interoperability, revocation, 320/390/1440 light/dark, accessibility' }));
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
