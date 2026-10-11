// Run only against the isolated TestSiteIntegration preview.
const {chromium}=require('playwright');
const {PNG}=require('pngjs');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const os=require('node:os');
const path=require('node:path');
const base=process.env.UI_BASE_URL||'http://127.0.0.1:8100';
const output=path.join(os.tmpdir(),'workservice-motion');

(async()=>{
  fs.mkdirSync(output,{recursive:true});
  const browser=await chromium.launch({headless:true,channel:process.env.UI_BROWSER||'msedge'});
  try {
    const context=await browser.newContext({viewport:{width:390,height:844},serviceWorkers:'block',colorScheme:'light',reducedMotion:'no-preference'});
    const page=await context.newPage();const errors=[];
    page.on('pageerror',error=>errors.push(error.message));
    await page.goto(base+'/login');
    assert.equal(await page.evaluate(()=>getComputedStyle(document.body,'::before').animationName),'ambient-gradient');
    const firstPosition=await page.evaluate(()=>getComputedStyle(document.body,'::before').backgroundPosition);
    await page.waitForTimeout(350);
    assert.notEqual(await page.evaluate(()=>getComputedStyle(document.body,'::before').backgroundPosition),firstPosition,'background is static');
    await page.locator('#username').fill('admin');await page.locator('#password').fill('testpass');
    await Promise.all([page.waitForURL('**/dashboard'),page.locator('button[type=submit]').click()]);
    await page.goto(base+'/profile/menu');
    await page.getByLabel('Тёмная',{exact:true}).check();
    assert.equal(await page.locator('html').getAttribute('data-theme'),'dark');
    await page.goto(base+'/cloud');assert.equal(await page.locator('html').getAttribute('data-theme'),'dark','theme not retained');
    const indicator=page.locator('.cloud-tabs > .ui-tab-indicator');
    const initial=await indicator.evaluate(e=>e.style.transform);
    await page.locator('[data-view=starred]').click();
    await page.waitForFunction(initial=>document.querySelector('.cloud-tabs > .ui-tab-indicator').style.transform!==initial,initial);
    await page.waitForTimeout(350);
    assert(await page.evaluate(()=>{const a=document.querySelector('.cloud-tabs [aria-current]').getBoundingClientRect(),b=document.querySelector('.cloud-tabs .ui-tab-indicator').getBoundingClientRect();return Math.abs(a.x-b.x)<1&&Math.abs(a.width-b.width)<1;}),'tab indicator misaligned');
    await page.locator('[data-view=files]').click();
    const button=page.locator('#cloud-new-folder'),bounds=await button.boundingBox();
    await page.mouse.move(bounds.x+8,bounds.y+8);await page.mouse.down();
    assert.equal(await button.locator('.ui-ripple').count(),1,'pointer ripple missing');
    assert.equal(await button.getAttribute('aria-label'),null);
    await page.mouse.up();await page.keyboard.press('Escape');await page.waitForTimeout(650);assert.equal(await page.locator('.ui-ripple').count(),0,'ripple not cleaned up');
    await button.focus();await page.keyboard.down('Enter');assert.equal(await button.locator('.ui-ripple').count(),1,'keyboard ripple missing');await page.keyboard.up('Enter');await page.keyboard.press('Escape');
    for(const width of [320,390,768,1440])for(const theme of ['light','dark']) {
      await page.setViewportSize({width,height:width<600?844:1000});
      await page.evaluate(theme=>{localStorage.setItem('workservice-theme',theme);},theme);
      await page.goto(base+'/cloud');await page.evaluate(()=>document.fonts.ready);
      await page.waitForTimeout(300);
      assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'horizontal overflow');
      const imageAt=async time=>{
        await page.evaluate(time=>{const animation=document.getAnimations().find(a=>a.animationName==='ambient-gradient');animation.pause();animation.currentTime=time;},time);
        await page.waitForTimeout(40);return page.screenshot();
      };
      const before=PNG.sync.read(await imageAt(0)),afterBuffer=await imageAt(24000),after=PNG.sync.read(afterBuffer);
      let difference=0;for(const [x,y] of [[2,2],[width-2,100],[2,600]]){const i=(y*before.width+x)*4;for(let channel=0;channel<3;channel++)difference+=Math.abs(before.data[i+channel]-after.data[i+channel]);}
      assert(difference>10,`gradient pixels unchanged ${width} ${theme}`);
      assert(await page.evaluate(()=>{const s=getComputedStyle(document.body,'::before');return s.position==='fixed'&&s.top==='0px'&&Math.abs(parseFloat(s.width)-innerWidth)<1&&s.pointerEvents==='none';}),'background layer geometry');
      if(process.env.AXE_PATH){await page.addScriptTag({path:process.env.AXE_PATH});const issues=await page.evaluate(async()=>(await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>({target:n.target,summary:n.failureSummary}))})));assert.deepEqual(issues,[],JSON.stringify({width,theme,issues}));}
      fs.writeFileSync(path.join(output,`cloud-${width}-${theme}.png`),afterBuffer);
      await page.goto(base+'/profile/menu');await page.screenshot({path:path.join(output,`appearance-${width}-${theme}.png`),animations:'disabled'});
      if(process.env.AXE_PATH){await page.addScriptTag({path:process.env.AXE_PATH});const issues=await page.evaluate(async()=>(await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>({target:n.target,summary:n.failureSummary}))})));assert.deepEqual(issues,[],JSON.stringify({width,theme,appearanceIssues:issues}));}
    }
    await page.getByLabel('Анимация интерфейса',{exact:true}).uncheck();
    assert.equal(await page.evaluate(()=>getComputedStyle(document.body,'::before').animationName),'none');
    await page.goto(base+'/cloud');assert.equal(await page.locator('html').getAttribute('data-motion'),'off');
    await page.locator('#cloud-new-folder').click();assert.equal(await page.locator('.ui-ripple').count(),0);await page.keyboard.press('Escape');
    await page.goto(base+'/profile/menu');await page.getByLabel('Анимация интерфейса',{exact:true}).check();
    await page.emulateMedia({reducedMotion:'reduce'});assert.equal(await page.locator('html').getAttribute('data-motion'),'off');assert(await page.locator('#appearance-motion').isDisabled());
    assert.equal(await page.evaluate(()=>getComputedStyle(document.body,'::before').animationName),'none');
    await page.emulateMedia({reducedMotion:'no-preference'});await page.getByLabel('Системная',{exact:true}).check();await page.emulateMedia({colorScheme:'dark'});
    assert.equal(await page.locator('html').getAttribute('data-theme'),null);assert.equal(await page.evaluate(()=>getComputedStyle(document.documentElement).colorScheme),'dark');
    await page.emulateMedia({colorScheme:'light'});assert.equal(await page.evaluate(()=>getComputedStyle(document.documentElement).colorScheme),'light');
    const noStorage=await browser.newContext({serviceWorkers:'block'});await noStorage.addInitScript(()=>{Storage.prototype.getItem=function(){throw new Error('storage unavailable');};Storage.prototype.setItem=function(){throw new Error('storage unavailable');};});
    const fallback=await noStorage.newPage();fallback.on('pageerror',e=>errors.push(e.message));await fallback.goto(base+'/login');assert.equal(await fallback.locator('html').getAttribute('data-motion'),'on');await noStorage.close();
    assert.deepEqual(errors,[]);console.log(JSON.stringify({checks:'moving pixels, full viewport, two palettes, fluid tabs, pointer/keyboard ripple, cleanup, preferences, reduced motion, unavailable storage, 8 layouts and axe',output}));
  } finally {await browser.close();}
})().catch(error=>{console.error(error);process.exit(1);});
