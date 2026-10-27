// Real upstream smoke test and labelled demo data, only for the isolated preview.
const {chromium}=require('playwright');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const os=require('node:os');
const base=process.env.UI_BASE_URL||'http://127.0.0.1:8099';
assert.equal(process.env.WORKSERVICE_DEMO_PREVIEW,'1','Explicit isolated-preview confirmation required');
assert(['127.0.0.1','localhost'].includes(new URL(base).hostname),'Loopback preview only');
const output=path.join(os.tmpdir(),'workservice-live-weather');
(async()=>{
 const browser=await chromium.launch({headless:true,channel:'msedge'});
 try{
  fs.mkdirSync(output,{recursive:true});
  const context=await browser.newContext({viewport:{width:1440,height:1100},serviceWorkers:'block',reducedMotion:'reduce'});
  const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto(base+'/login');await page.locator('#username').fill('admin');await page.locator('#password').fill('testpass');await page.locator('[type=submit]').click();await page.waitForURL('**/dashboard');
  await page.goto(base+'/object/object#object-documents');
  const csrf=await page.locator('#object-documents').getAttribute('data-csrf');
  const name='ДЕМО: вентиляция административного здания';
  await page.goto(base+'/objects');
  let link=page.locator('a[href^="/object/"]').filter({hasText:name}).first();
  if(!await link.count()){
   const res=await context.request.post(base+'/objects/new',{form:{_csrf_token:csrf,name,address:'Минск, ул. Демонстрационная, 15',status:'in_progress',responsible_user_id:'admin'}});assert.equal(res.status(),200,await res.text());
   await page.reload();link=page.locator('a[href^="/object/"]').filter({hasText:name}).first();
  }
  const objectPath=await link.getAttribute('href'),id=objectPath.split('/')[2];assert(id&&id!=='object');
  await page.goto(base+objectPath+'#object-documents');const endpoint=base+objectPath+'/documents';
  const get=async suffix=>{const res=await context.request.get(endpoint+(suffix||''));assert.equal(res.status(),200,await res.text());return res.json();};
  const save=async data=>{const res=await context.request.post(endpoint,{headers:{'X-CSRF-Token':csrf},data});assert.equal(res.status(),200,await res.text());return res.json();};
  let state=await get();assert.equal(state.weather.enabled,true,'default weather mode is disabled');
  const {cities}=await get('/cities?q='+encodeURIComponent('Минск'));const city=cities.find(c=>c.name==='Минск');assert(city,'real city lookup failed');
  const existing=state.documents.find(d=>d.content.kind==='requisites');
  const fields={object_name:name,address:state.object.address,contractor:'ООО «Демо-Монтаж»',contractor_rep:'Администратор',customer:'ООО «Демо-Заказчик», директор Орлов О. О. (пример)',designer:'ООО «Демо-Проект», ГИП Сидоров С. С. (пример)',technical_rep:'Инженер Петров П. П. (демонстрационный пример)',quality_rep:'Прораб Петров Петр',responsible:'Администратор',project:'ДЕМО-ОВ-2025, листы 1–8. Учебный пример, не рабочая документация.',permit:'Демонстрационные данные — разрешение не выдавалось',weather_city:city.label,weather_latitude:String(city.latitude),weather_longitude:String(city.longitude)};
  await save({id:existing?.id||'',version:existing?.version||0,content:{kind:'requisites',number:'',fields,rows:[],archived:false},reason:'Демонстрационные реквизиты'});
  let source=await get('/sources?from=2025-09-21&to=2025-09-23&section=4');
  const descriptions={'2025-09-21':'Разметка трасс и монтаж подвесов воздуховодов, оси А–Б/1–4, этаж 2.','2025-09-22':'Монтаж прямых участков и фасонных деталей воздуховодов.','2025-09-23':'Установка решёток, проверка креплений и подготовка к испытаниям.'};
  for(const [date,notes]of Object.entries(descriptions))if(!source.rows.some(r=>r.fields.date===date)){
   const form=new URLSearchParams({_csrf_token:csrf,date,start_time:'08:00',end_time:'17:00',lunch_break_minutes:'60',notes:'ДЕМО. '+notes,return_to:objectPath});form.append('worker_ids','own');form.append('worker_ids','other');form.append('object_ids',id);
   const res=await context.request.post(base+'/schedule/new',{headers:{'Content-Type':'application/x-www-form-urlencoded'},data:form.toString()});assert.equal(res.status(),200,await res.text());
  }
  source=await get('/sources?from=2025-09-21&to=2025-09-23&section=4');assert.equal(source.rows.length,3);assert(source.rows.every(r=>r.fields.crew.includes('Иванов Иван')&&r.fields.crew.includes('Петров Петр')));
  for(const row of source.rows)Object.assign(row.fields,{measures:'Учебный пример. Фактические условия проверяет ответственный.',unit:'м²',quantity:'12',control:'ДЕМО: пример записи о проверке креплений; выполнение контроля не подтверждено.'});
  state=await get();const previous=state.documents.find(d=>d.content.number==='ДЕМО-ЖР-01');
  const journalFields={...Object.fromEntries(state.schema.journal.filter(f=>fields[f.key]).map(f=>[f.key,fields[f.key]])),issuer:'Директор ООО «Демо-Монтаж» (пример)',issue_date:'2025-09-20',planned_start:'2025-09-21',planned_end:'2025-09-30',indicators:'Учебный пример: административное здание, вентиляция второго этажа.'};
  await save({id:previous?.id||'',version:previous?.version||0,content:{kind:'journal',number:'ДЕМО-ЖР-01',fields:journalFields,rows:source.rows,archived:false},reason:'Демонстрационный журнал с назначениями'});
  await page.reload();await page.locator('.docs-document').filter({hasText:'ДЕМО-ЖР-01'}).click();await page.locator('#docs-editor[open]').waitFor();assert.equal(await page.locator('#docs-section-select').inputValue(),'0');
  await page.screenshot({path:path.join(output,'title-desktop.png')});await page.setViewportSize({width:390,height:844});await page.screenshot({path:path.join(output,'title-mobile.png')});
  await page.locator('#docs-section-select').selectOption('4');await page.getByRole('button',{name:'Заполнить погоду',exact:true}).click();
  await page.waitForFunction(()=>[...document.querySelectorAll('.document-journal [id$="-weather"]')].length===3&&[...document.querySelectorAll('.document-journal [id$="-weather"]')].every(e=>e.value.includes('°C')),{},{timeout:30000});
  await page.locator('#docs-reason').fill('Автоматическая погода Open-Meteo за три дня');await page.locator('#docs-form [type=submit]').click();await page.locator('#docs-save-state').filter({hasText:/^Сохранено$/}).waitFor();
  const latest=(await get()).documents.find(d=>d.content.number==='ДЕМО-ЖР-01');const saved=await get('/'+latest.id);assert(saved.content.rows.every(r=>r.fields.weather_date===r.fields.date&&r.fields.weather_source.includes('Open-Meteo')));
  await page.setViewportSize({width:1440,height:1100});await page.screenshot({path:path.join(output,'weather-desktop.png')});
  const pdf=await context.request.get(endpoint+'/'+latest.id+'/pdf');assert.equal(pdf.status(),200,await pdf.text());fs.writeFileSync(path.join(output,'demo-journal.pdf'),await pdf.body());
  assert.deepEqual(errors,[]);console.log(JSON.stringify({passed:true,objectURL:base+objectPath+'#object-documents',output,city:city.label,weather:saved.content.rows.map(r=>({date:r.fields.date,text:r.fields.weather})),checks:'real free API, three assignment crews, live batch weather, persisted source/dates, separate cover, PDF'}));
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
