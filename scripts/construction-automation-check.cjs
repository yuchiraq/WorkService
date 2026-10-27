// Only against the isolated TestSiteIntegration preview. Weather is mocked in
// the browser; backend provider parsing has its own no-network tests.
const {chromium}=require('playwright');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const os=require('node:os');
const base=process.env.UI_BASE_URL||'http://127.0.0.1:8099';
const output=path.join(os.tmpdir(),'workservice-document-automation');
(async()=>{
 const browser=await chromium.launch({headless:true,channel:process.env.UI_BROWSER||'msedge'});fs.mkdirSync(output,{recursive:true});
 const errors=[];
 try{
  const context=await browser.newContext({viewport:{width:390,height:844},isMobile:true,hasTouch:true,serviceWorkers:'block',reducedMotion:'reduce'});
  const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
  await page.goto(base+'/login');await page.locator('#username').fill('admin');await page.locator('#password').fill('testpass');await page.locator('[type=submit]').click();await page.waitForURL('**/dashboard');
  await page.goto(base+'/object/object#object-documents');
  const csrf=await page.locator('#object-documents').getAttribute('data-csrf'),endpoint=base+'/object/object/documents';
  const get=async()=>{const res=await context.request.get(endpoint);assert.equal(res.status(),200,await res.text());return res.json();};
  const save=async data=>{const res=await context.request.post(endpoint,{headers:{'X-CSRF-Token':csrf},data});assert.equal(res.status(),200,await res.text());return res.json();};
  let state=await get();assert.equal(state.object.id,'object');
  const update=await context.request.post(base+'/objects/edit/object',{form:{_csrf_token:csrf,name:state.object.name,address:state.object.address,status:'in_progress',responsible_user_id:'admin'}});assert.equal(update.status(),200);
  const req=state.documents.find(d=>d.content.kind==='requisites');
  await save({id:req?.id||'',version:req?.version||0,content:{kind:'requisites',number:'',archived:false,rows:[],fields:{...(req?.content.fields||{}),quality_rep:'Прораб Петров Петр',contractor_rep:'Администратор',technical_rep:'Внешний инженер Согласованный С. С.',weather_city:'Минск',weather_latitude:'53.9',weather_longitude:'27.5667'}},reason:'Настройка автоматизации для теста'});
  let cityRequests=0,weatherRequests=0;
  // Automatic bulk import has its own end-to-end suite; keep this check focused
  // on editing a single row and its date/people/weather guards.
  await page.route('**/object/object/documents/assignments',route=>route.fulfill({json:{rows:[]}}));
  await page.route('**/object/object/documents',async route=>{
   if(route.request().method()!=='GET')return route.continue();const res=await route.fetch(),body=await res.json();body.weather={enabled:true,reason:'',provider:'Open-Meteo'};await route.fulfill({response:res,json:body});
  });
  await page.route('**/object/object/documents/cities?*',route=>{cityRequests++;return route.fulfill({json:{cities:[{id:1,name:'Минск',label:'Минск, Минск',latitude:53.9,longitude:27.5667}]}});});
  await page.route('**/object/object/documents/weather?*',async route=>{
   weatherRequests++;const q=new URL(route.request().url()).searchParams,from=q.get('from'),to=q.get('to');
   await new Promise(r=>setTimeout(r,from==='2026-09-21'?250:40));
   const days={};for(let d=new Date(from+'T12:00:00Z');d<=new Date(to+'T12:00:00Z');d.setUTCDate(d.getUTCDate()+1)){const date=d.toISOString().slice(0,10);days[date]={date,text:`+4…+12 °C; облачно (${date})`,source:`Open-Meteo · Минск · ${date} · модельные данные, тест`,kind:'reanalysis'};}
   await route.fulfill({json:{days}});
  });
  await page.reload();await page.locator('[data-docs=requisites]').click();
  assert.equal(await page.locator('#doc-field-responsible').inputValue(),'Администратор');
  await page.locator('#doc-field-quality_rep').fill('Пет');await page.getByRole('option').filter({hasText:'Прораб Петров Петр'}).first().tap();
  assert.equal(await page.locator('#doc-field-quality_rep').inputValue(),'Прораб Петров Петр');
  await page.locator('#doc-field-weather_city').fill('Мин');await page.getByRole('option').filter({hasText:'Минск, Минск'}).tap();assert(cityRequests>0);
  await page.locator('#docs-reason').fill('Выбор города и контролирующего');await page.locator('#docs-form [type=submit]').click();await page.locator('#docs-save-state').filter({hasText:/^Сохранено$/}).waitFor();await page.locator('[data-docs=close]').click();
  state=await get();assert.equal(state.automation.defaults.responsible,'Администратор');assert.equal(state.automation.defaults.weather_city,'Минск, Минск');
  await page.locator('[data-docs=act]').click();assert.equal(await page.locator('#doc-field-contractor_rep').inputValue(),'Администратор');assert.equal(await page.locator('#doc-field-contractor_signer').inputValue(),'Администратор');assert.equal(await page.locator('#doc-field-technical_rep').inputValue(),'Внешний инженер Согласованный С. С.');assert.equal(await page.locator('#doc-field-contractor_signed_at-date').inputValue(),'');
  await page.locator('[data-docs=close]').click();await page.locator('#docs-confirm-yes').click();
  await page.locator('[data-docs=journal]').click();await page.locator('#docs-section-select').selectOption('4');await page.getByRole('button',{name:'Добавить запись',exact:true}).click();
  await page.locator('#doc-row-date').fill('2026-09-21');
  await page.waitForFunction(()=>document.querySelector('#doc-row-crew').value.includes('Иванов Иван'));
  await page.waitForFunction(()=>document.querySelector('#doc-row-weather').value.includes('2026-09-21'));
  assert((await page.locator('#doc-row-persons').inputValue()).includes('Администратор'));assert((await page.locator('#doc-row-persons').inputValue()).includes('Прораб Петров Петр'));assert.equal(await page.locator('#doc-row-control').inputValue(),'');assert.equal(await page.locator('#doc-row-quantity').inputValue(),'');
  await page.locator('#doc-row-date').fill('2026-09-22');await page.waitForFunction(()=>document.querySelector('#doc-row-crew').value.includes('Петров Петр'));assert(!(await page.locator('#doc-row-crew').inputValue()).includes('Иванов Иван'));
  await page.locator('#doc-row-crew').fill('Бригада уточнена вручную');await page.locator('#doc-row-date').fill('2026-09-21');await page.waitForFunction(()=>document.querySelector('#doc-row-weather').value.includes('2026-09-21'));assert.equal(await page.locator('#doc-row-crew').inputValue(),'Бригада уточнена вручную');
  await page.locator('#doc-row-description').fill('Повторяемая запись: монтаж воздуховодов в осях А-Б');
  await page.locator('#doc-row-control').fill('Проверка плотности соединений по проекту');
  await page.locator('#docs-row-form [type=submit]').click();await page.locator('#docs-form [type=submit]').click();await page.locator('#docs-save-state').filter({hasText:/^Сохранено$/}).waitFor();
  let latest=(await get()).documents.find(d=>d.content.kind==='journal');const detail=await context.request.get(endpoint+'/'+latest.id);const journal=await detail.json();assert.equal(journal.content.rows[0].fields.weather_date,'2026-09-21');assert(journal.content.rows[0].fields.weather_source.includes('Open-Meteo'));
  await page.locator('[data-docs=close]').click();await page.reload();
  await page.locator('[data-docs=journal]').click();await page.locator('#docs-section-select').selectOption('4');await page.getByRole('button',{name:'Добавить запись',exact:true}).click();await page.locator('#doc-row-date').fill('2026-09-21');await page.waitForFunction(()=>document.querySelector('#doc-row-weather').value.includes('2026-09-21'));
  await page.locator('#doc-row-description').fill('Повторяемая');await page.getByRole('option').filter({hasText:'Повторяемая запись: монтаж воздуховодов'}).click();assert.equal(await page.locator('#doc-row-description').inputValue(),'Повторяемая запись: монтаж воздуховодов в осях А-Б');
  await page.locator('#doc-row-control').fill('Проверка плот');await page.getByRole('option').filter({hasText:'Проверка плотности соединений по проекту'}).click();
  // Old date responses must not overwrite weather for a newly selected day.
  await page.locator('#doc-row-weather').fill('');await page.locator('#doc-row-date').fill('2026-09-20');await page.locator('#doc-row-date').fill('2026-09-22');await page.waitForFunction(()=>document.querySelector('#doc-row-weather').value.includes('2026-09-22'));
  await page.locator('#doc-row-weather').fill('Ручной замер: +10 °C');await page.locator('#doc-row-date').fill('2026-09-23');
  await page.locator('#doc-row-persons').fill('Пет');await page.getByRole('option').filter({hasText:'Прораб Петров Петр'}).first().waitFor();
  for(const [width,height]of [[320,740],[390,844],[1440,1000]])for(const theme of ['light','dark']){
   await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});await page.evaluate(async()=>{await Promise.all(document.getAnimations().filter(a=>Number.isFinite(a.effect.getTiming().iterations)).map(a=>a.finished.catch(()=>{})));});
   await page.locator('#doc-row-persons').focus();await page.getByRole('option').filter({hasText:'Прораб Петров Петр'}).first().waitFor();
   const box=await page.locator('#document-suggestion-list').boundingBox();assert(box.x>=0&&box.x+box.width<=width+1,'popup overflow');await page.screenshot({path:path.join(output,`people-${width}-${theme}.png`)});
   if(process.env.AXE_PATH){await page.addScriptTag({path:process.env.AXE_PATH});const violations=await page.evaluate(async()=>(await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.failureSummary)})));assert.deepEqual(violations,[],JSON.stringify(violations));}
  }
  await page.keyboard.press('ArrowDown');await page.keyboard.press('Enter');assert.equal(await page.locator('#doc-row-persons').inputValue(),'Прораб Петров Петр');assert.equal(await page.locator('#doc-row-weather').inputValue(),'Ручной замер: +10 °C');
  await page.locator('#docs-row-form [type=submit]').click();await page.locator('#docs-form [type=submit]').click();await page.locator('#docs-save-state').filter({hasText:/^Сохранено$/}).waitFor();
  latest=(await get()).documents.find(d=>d.content.kind==='journal');const res=await context.request.get(endpoint+'/'+latest.id);const last=await res.json();assert(!last.content.rows[0].fields.weather_source,'manual weather kept external provenance');
  assert(weatherRequests>0);assert.deepEqual(errors,[]);console.log(JSON.stringify({passed:true,output,checks:'responsible/crew/control defaults, signing names without signing facts, people/city typeahead, persisted text reuse, weather/date guards, no manual overwrite, keyboard, six layouts and a11y'}));
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
