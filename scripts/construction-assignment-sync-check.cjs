// Only the isolated TestSiteIntegration preview; creates labelled demo fixtures.
const {chromium}=require('playwright');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const os=require('node:os');
const base=process.env.UI_BASE_URL||'http://127.0.0.1:8099';
assert.equal(process.env.WORKSERVICE_DEMO_PREVIEW,'1');
assert(['localhost','127.0.0.1'].includes(new URL(base).hostname));
(async()=>{
 const browser=await chromium.launch({headless:true,channel:'msedge'});
 try{
  const context=await browser.newContext({viewport:{width:390,height:844},isMobile:true,hasTouch:true,serviceWorkers:'block',reducedMotion:'reduce'});
  const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto(base+'/login');await page.locator('#username').fill('admin');await page.locator('#password').fill('testpass');await page.locator('[type=submit]').click();await page.waitForURL('**/dashboard');
  await page.goto(base+'/object/object#object-documents');const csrf=await page.locator('#object-documents').getAttribute('data-csrf');
  const name='ДЕМО: автоматическое заполнение '+Date.now();
  let res=await context.request.post(base+'/objects/new',{form:{_csrf_token:csrf,name,address:'Минск, учебная площадка',status:'in_progress',responsible_user_id:'admin'}});assert.equal(res.status(),200);
  await page.goto(base+'/objects');const objectPath=await page.locator('a[href^="/object/"]').filter({hasText:name}).first().getAttribute('href'),id=objectPath.split('/')[2],endpoint=base+objectPath+'/documents';
  const assign=async(date,worker,notes,object=id)=>{
   const form=new URLSearchParams({_csrf_token:csrf,date,start_time:'08:00',end_time:'17:00',lunch_break_minutes:'60',notes,return_to:objectPath});form.append('worker_ids',worker);form.append('object_ids',object);
   const r=await context.request.post(base+'/schedule/new',{headers:{'Content-Type':'application/x-www-form-urlencoded'},data:form.toString()});assert.equal(r.status(),200);assert(new URL(r.url()).pathname===objectPath,'assignment was not saved');
  };
  await assign('2025-09-21','own','ДЕМО. Монтаж основной трассы');await assign('2025-09-22','other','ДЕМО. Установка решёток');await assign('2199-09-21','own','FUTURE_MUST_NOT_IMPORT');await assign('2025-09-23','own','OTHER_OBJECT_MUST_NOT_IMPORT','object');
  await page.route('**/documents',async route=>{if(route.request().method()!=='GET')return route.continue();const r=await route.fetch(),body=await r.json();body.weather={enabled:false,reason:'Disabled in assignment-only test'};await route.fulfill({response:r,json:body});});
  await page.goto(base+objectPath+'#object-documents');await page.locator('[data-docs=journal]').click();await page.locator('#docs-editor[open]').waitFor();
  assert((await page.locator('#docs-assignment-notice').innerText()).includes('2'));await page.locator('#docs-section-select').selectOption('4');assert.equal(await page.locator('.document-journal tbody tr').count(),2);
  await page.locator('.document-journal tbody tr').first().getByRole('button',{name:'Изменить запись',exact:true}).click();
  assert.equal(await page.locator('#doc-row-crew').inputValue(),'Монтажник Иванов Иван');
  for(const [key,value]of Object.entries({description:'РУЧНОЙ ТЕКСТ НЕ МЕНЯТЬ',weather:'Ручной замер: +10 °C',measures:'Нет особых условий',unit:'м',quantity:'12',control:'Ручной контроль — учебный пример',persons:'Ответственный, указанный вручную'}))await page.locator('#doc-row-'+key).fill(value);
  await page.locator('#docs-row-reviewed').check();await page.locator('#docs-row-form [type=submit]').click();
  const submit=async()=>{await page.locator('#docs-form [type=submit]').click();await page.locator('#docs-save-state').filter({hasText:/^Сохранено$/}).waitFor();};
  const collection=async()=>{const r=await context.request.get(endpoint);assert.equal(r.status(),200);return r.json();};
  await submit();let doc=(await collection()).documents.find(d=>d.content.kind==='journal');
  let saved=await(await context.request.get(endpoint+'/'+doc.id)).json();const original=saved.content.rows.find(r=>r.fields.description==='РУЧНОЙ ТЕКСТ НЕ МЕНЯТЬ');assert(original.reviewed);
  await assign('2025-09-21','idle','ДЕМО. Дополнительная работа в прежней смене');await assign('2025-09-23','other','ДЕМО. Новая смена');
  await page.getByRole('button',{name:'Обновить назначения',exact:true}).click();await page.locator('#docs-assignment-notice').filter({hasText:'Добавлено записей из назначений: 2.'}).waitFor();assert.equal(await page.locator('.document-journal tbody tr').count(),4);
  await page.getByRole('button',{name:'Обновить назначения',exact:true}).click();await page.locator('#docs-assignment-notice').filter({hasText:'Новых назначений'}).waitFor();assert.equal(await page.locator('.document-journal tbody tr').count(),4);await submit();
  saved=await(await context.request.get(endpoint+'/'+doc.id)).json();assert.deepEqual(saved.content.rows.find(r=>r.id===original.id),original);
  assert(saved.content.rows.some(r=>r.fields.description.includes('Дополнительная работа')&&r.fields.crew==='Без назначений'));
  assert(!JSON.stringify(saved.content.rows).includes('MUST_NOT_IMPORT'));const sources=saved.content.rows.flatMap(r=>r.sources);assert.equal(new Set(sources).size,sources.length);
  await page.locator('[data-docs=close]').click();await assign('2025-09-24','own','ДЕМО. Новое назначение после сохранения');
  const version=saved.version;await page.locator('.docs-document').filter({hasText:doc.content.number}).click();await page.locator('#docs-editor[open]').waitFor();await page.locator('#docs-section-select').selectOption('4');assert.equal(await page.locator('.document-journal tbody tr').count(),5);
  let persisted=await(await context.request.get(endpoint+'/'+doc.id)).json();assert.equal(persisted.version,version,'opening must not silently save a revision');assert.equal(persisted.content.rows.length,4);await submit();
  await page.locator('[data-docs=close]').click();saved=await(await context.request.get(endpoint+'/'+doc.id)).json();saved.content.archived=true;
  res=await context.request.post(endpoint,{headers:{'X-CSRF-Token':csrf},data:{id:doc.id,version:saved.version,content:saved.content,reason:'Архив — тест'}});assert.equal(res.status(),200);
  await assign('2025-09-25','own','ARCHIVED_MUST_NOT_IMPORT');await page.locator('[data-docs=refresh]').click();await page.locator('#docs-filter').selectOption('archived');await page.locator('.docs-document').filter({hasText:doc.content.number}).click();await page.locator('#docs-editor[open]').waitFor();await page.locator('#docs-section-select').selectOption('4');assert.equal(await page.locator('.document-journal tbody tr').count(),5);assert.equal(await page.getByRole('button',{name:'Обновить назначения',exact:true}).count(),0);
  assert.equal(await page.locator('#docs-save-state').innerText(),'Сохранено');
  const output=path.join(os.tmpdir(),'workservice-assignment-sync');fs.mkdirSync(output,{recursive:true});await page.screenshot({path:path.join(output,'journal-mobile.png')});
  await page.setViewportSize({width:1440,height:1000});assert(await page.locator('#docs-editor').evaluate(e=>e.scrollWidth<=e.clientWidth+1));await page.screenshot({path:path.join(output,'journal-desktop.png')});
  assert.deepEqual(errors,[]);console.log(JSON.stringify({passed:true,output,checks:'automatic new/existing journal drafts, refresh, per-object/future filters, same-shift increment without old crew, no duplicate sources, manual reviewed row preserved, no silent save, archive unchanged, mobile/desktop'}));
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
