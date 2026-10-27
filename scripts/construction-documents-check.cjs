// Run only against the isolated TestSiteIntegration preview, never production.
const {chromium} = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const base = process.env.UI_BASE_URL || 'http://127.0.0.1:8099';
const output = path.join(os.tmpdir(), 'workservice-construction-documents');
(async () => {
  fs.mkdirSync(output,{recursive:true});
  const browser = await chromium.launch({headless:true,channel:process.env.UI_BROWSER || 'msedge'});
  const errors=[];
  try {
    const context = await browser.newContext({viewport:{width:390,height:844},isMobile:true,hasTouch:true,serviceWorkers:'block',reducedMotion:'reduce'});
    const page=await context.newPage(); page.on('pageerror',e=>errors.push(e.message));
    await page.goto(base+'/login');await page.locator('#username').fill('admin');await page.locator('#password').fill('testpass');
    await Promise.all([page.waitForURL('**/dashboard'),page.locator('[type=submit]').click()]);
    await page.goto(base+'/object/object#object-documents');
    await page.locator('#object-documents:visible').waitFor();
    const csrf=await page.locator('#object-documents').getAttribute('data-csrf');
    const endpoint=base+'/object/object/documents';
    const get=async()=>{const r=await context.request.get(endpoint);assert.equal(r.status(),200);return r.json();};
    const save=async body=>{const r=await context.request.post(endpoint,{headers:{'X-CSRF-Token':csrf},data:body});assert.equal(r.status(),200,await r.text());return r.json();};
    let state=await get();
    const existing=state.documents.find(d=>d.content.kind==='requisites');
    const fields={object_name:'Жилой комплекс на улице Центральной',address:'г. Минск, ул. Центральная, 15',contractor:'ООО «МонтажСтрой»',contractor_rep:'Производитель работ Иванов И. И.',technical_rep:'Инженер технического надзора Петров П. П.',author_rep:'Инженер проекта Сидоров С. С.',customer:'ООО «Заказчик», директор Орлов О. О.',designer:'ООО «Проект», ГИП Сидоров С. С.',project:'ООО «Проект», ВК-26, листы 12-14 от 01.09.2026',permit:'Уведомление № 14 от 01.09.2026',responsible:'Производитель работ Иванов И. И.'};
    await save({id:existing?.id||'',version:existing?.version||0,content:{kind:'requisites',number:'',fields,rows:[],archived:false},reason:'Тестовые реквизиты'});
    await page.locator('[data-docs=refresh]').click();
    await page.locator('[data-docs=act]').click();
    await page.locator('#docs-editor[open]').waitFor();
    assert(await page.evaluate(()=>!document.activeElement.matches('input,textarea')),'mobile autofocus');
    assert.equal(await page.locator('#doc-field-project').inputValue(),fields.project);
    await page.locator('#doc-field-work_name').fill('Прокладка трубопроводов водоснабжения');
    await page.locator('#doc-field-works').fill('Монтаж трубопроводов холодного водоснабжения в осях А-Б/1-4, отметка +3,200, второй этаж.');
    await page.locator('#doc-field-materials').fill('Труба ПВХ 25 мм, паспорт партии № 257 от 10.09.2026; сертификат соответствия BY/112 02.01. Тестовые реквизиты.');
    await page.locator('#doc-field-start_date').fill('2026-10-20');await page.locator('#doc-field-end_date').fill('2026-10-21');
    await page.locator('#doc-field-violations').selectOption('none');
    assert(await page.locator('#doc-field-deviations').isHidden());
    await page.locator('#doc-field-contractor_signer').fill('Иванов И. И.');
    await page.locator('#doc-field-contractor_signed_at-date').fill(state.today);
    await page.locator('#doc-field-contractor_signed_at-time').fill('10:30');
    await page.locator('[data-docs=preview]').click();
    assert.equal(await page.locator('.document-paper .document-input').count(),0,'preview has editing controls');
    assert((await page.locator('.document-paper').innerText()).includes('Прокладка трубопроводов'));
    await page.screenshot({path:path.join(output,'act-paper-mobile.png')});
    await page.locator('[data-docs=preview]').click();
    assert.equal(await page.locator('#doc-field-contractor_signed_at-date').inputValue(),state.today);
    assert.equal(await page.locator('#doc-field-contractor_signed_at-time').inputValue(),'10:30');
    await page.setViewportSize({width:1440,height:1000});
    await page.locator('#docs-editor-body').evaluate(e=>{e.scrollTop=0;});
    await page.screenshot({path:path.join(output,'act-paper-desktop.png')});
    await page.setViewportSize({width:390,height:844});
    const submit=async()=>{await page.locator('#docs-form > .docs-footer [type=submit]').click();await page.locator('#docs-save-state').filter({hasText:/^Сохранено$/}).waitFor();};
    await submit();
    let act=(await get()).documents.find(d=>d.content.kind==='act');
    assert(act && act.version===1);
    const download=page.waitForEvent('download');await page.locator('[data-docs=pdf]').click();await (await download).saveAs(path.join(output,'act.pdf'));
    await page.locator('#doc-field-works').fill('Уточнение осей А-Б/1-5.');await page.locator('#docs-reason').fill('Уточнение расположения по чертежу');await submit();
    await page.locator('[data-docs=history]').click();assert.equal(await page.locator('.docs-revision').count(),2);await page.locator('[data-docs=close-history]').click();
    await page.locator('[data-docs=close]').click();
    await page.locator('[data-docs=journal]').click();await page.locator('#docs-editor[open]').waitFor();
    assert.equal(await page.locator('#docs-section-select').inputValue(),'0');
    assert(await page.locator('.document-title-paper').isVisible(),'separate journal title missing');
    for(const [width,height] of [[320,740],[390,844],[1440,1000]]){
      await page.setViewportSize({width,height});assert(await page.locator('#docs-editor').evaluate(e=>e.scrollWidth<=e.clientWidth+1));
      await page.screenshot({path:path.join(output,`title-${width}.png`)});
      if(process.env.AXE_PATH){await page.addScriptTag({path:process.env.AXE_PATH});const v=await page.evaluate(async()=>(await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.failureSummary)})));assert.deepEqual(v,[],JSON.stringify(v));}
    }
    await page.setViewportSize({width:390,height:844});await page.getByRole('button',{name:'Общие сведения',exact:true}).click();
    await page.locator('#doc-field-issuer').fill('Директор ООО «МонтажСтрой» Иванов И. И.');
    await page.locator('#docs-section-select').selectOption('4');await page.getByRole('button',{name:'Из назначений',exact:true}).click();
    await page.locator('#docs-source-form [name=from]').fill('2026-09-01');await page.locator('#docs-source-form [name=to]').fill('2026-09-30');
    await page.locator('#docs-source-form [type=submit]').click();await page.locator('.docs-source-row').first().waitFor();
    assert.equal(await page.locator('.docs-source-row').count(),2);
    assert.equal(await page.locator('.docs-source-row input:disabled').count(),2,'automatic assignments must already be imported');
    await page.locator('[data-docs=close-import]').click();assert.equal(await page.locator('.document-journal tbody > tr').count(),2);
    await page.getByRole('button',{name:'Из назначений',exact:true}).click();await page.locator('.docs-source-row').first().waitFor();assert.equal(await page.locator('.docs-source-row input:disabled').count(),2);await page.locator('[data-docs=close-import]').click();
    await page.locator('.document-journal tbody > tr').first().getByRole('button',{name:'Изменить запись',exact:true}).click();
    for(const [key,value] of Object.entries({description:'Монтаж трубопроводов в осях А-Б/1-4, отметка +3,200.',weather:'+14 °C, без осадков',measures:'Особые условия отсутствуют',unit:'м',quantity:'24',control:'Отклонение от проектной отметки: 2 мм. Проверка креплений.',persons:'Производитель работ Иванов И. И.; инженер Петров П. П.'}))await page.locator('#doc-row-'+key).fill(value);
    await page.locator('#docs-row-reviewed').check();await page.locator('#docs-row-form [type=submit]').click();
    const firstRow=page.locator('.document-journal tbody > tr').first();
    await firstRow.locator('[id$="-quantity"]').fill('25');
    assert.equal(await firstRow.locator('.document-row-state').innerText(),'Черновик','inline edit must reset review');
    await firstRow.getByRole('button',{name:'Изменить запись',exact:true}).click();
    assert.equal(await page.locator('#doc-row-quantity').inputValue(),'25','inline value not passed to row editor');
    await page.locator('#docs-row-reviewed').check();await page.locator('#docs-row-form [type=submit]').click();
    await page.locator('.document-journal tbody > tr').nth(1).getByRole('button',{name:'Изменить запись',exact:true}).click();
    await page.locator('#doc-row-description').fill('Проверка переноса длинной записи на следующую страницу. '.repeat(85));
    await page.locator('#doc-row-control').fill('Измерения и замечания требуют ручной проверки. '.repeat(70));
    await page.locator('#docs-row-form [type=submit]').click();
    // Fill each remaining section, preserving independent manual fields.
    state=await get();
    for(const spec of state.schema.sections.filter(s=>s.number!==4)){
      await page.locator('#docs-section-select').selectOption(String(spec.number));await page.getByRole('button',{name:'Добавить запись',exact:true}).click();
      for(const field of spec.fields){await page.locator('#doc-row-'+field.key).fill(field.type==='date'?'2026-10-21':({'person':'Инженер Иванов И. И., водоснабжение','act':`${act.content.number}. Прокладка трубопроводов в осях А-Б/1-4`,'signers':'Иванов И. И., прораб; Петров П. П., инженер технадзора','name':'Журнал сварочных работ','authority':'Инспекция государственного строительного надзора','inspector':'Инженер Петров П. П.','remark':'Проверить крепления участка А-Б','response':'Крепления проверены, отметка 21.10.2026'}[field.key]||'Тестовые сведения: '+field.label));}
      await page.locator('#docs-row-reviewed').check();await page.locator('#docs-row-form [type=submit]').click();
    }
    await submit();
    await page.locator('#docs-section-select').selectOption('4');
    const journalDownload=page.waitForEvent('download');await page.locator('[data-docs=pdf]').click();await (await journalDownload).saveAs(path.join(output,'journal.pdf'));
    for(const [width,height] of [[320,740],[390,844],[844,390],[1440,1000]])for(const theme of ['light','dark']){
      await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:theme,reducedMotion:'reduce'});
      await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
      await page.evaluate(async()=>{await Promise.all(document.getAnimations().filter(a=>Number.isFinite(a.effect.getTiming().iterations)).map(a=>a.finished.catch(()=>{})));});
      assert(await page.locator('#docs-editor').evaluate(e=>e.scrollWidth<=e.clientWidth+1),'editor overflow');
      await page.locator('#docs-editor-body').evaluate(e=>{e.scrollTop=0;});
      const rect=await page.locator('#docs-form > .docs-footer [type=submit]').boundingBox();assert(rect.y>=0&&rect.y+rect.height<=height+1,'save inaccessible');
      await page.screenshot({path:path.join(output,`journal-${width}-${theme}.png`)});
      if(process.env.AXE_PATH){await page.addScriptTag({path:process.env.AXE_PATH});const v=await page.evaluate(async()=>(await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>({target:n.target,summary:n.failureSummary}))})));assert.deepEqual(v,[],JSON.stringify(v));}
      await page.locator('.document-journal tbody > tr').first().getByRole('button',{name:'Изменить запись',exact:true}).click();
      assert(await page.locator('#docs-row-editor').evaluate(e=>e.scrollWidth<=e.clientWidth+1));
      await page.keyboard.press('Tab');assert(await page.evaluate(()=>!!document.activeElement.closest('#docs-row-editor')),'nested modal focus');
      await page.screenshot({path:path.join(output,`row-${width}-${theme}.png`)});await page.locator('[data-docs=close-row]').click();
    }
    await page.setViewportSize({width:390,height:844});
    await page.locator('#docs-section-select').selectOption('1');await page.locator('#doc-field-responsible').fill('НЕ ПОТЕРЯТЬ ИЗМЕНЕНИЕ');await page.locator('#docs-reason').fill('Проверка конфликта');
    await page.route('**/object/object/documents',route=>route.request().method()==='POST'?route.fulfill({status:409,contentType:'application/json',body:JSON.stringify({error:'Документ изменён в другой вкладке'})}):route.continue());
    await page.locator('#docs-form > .docs-footer [type=submit]').click();await page.locator('#docs-error:visible').waitFor();assert.equal(await page.locator('#doc-field-responsible').inputValue(),'НЕ ПОТЕРЯТЬ ИЗМЕНЕНИЕ');
    await page.unroute('**/object/object/documents');await page.locator('[data-docs=close]').click();await page.locator('#docs-confirm-yes').click();
    assert(!(await page.locator('#docs-list').innerText()).includes('undefined'),'undefined document count');
    await page.locator('.docs-document').filter({hasText:'Журнал производства работ'}).first().click();
    await page.locator('#docs-editor[open]').waitFor();await page.locator('#docs-section-select').selectOption('4');assert.equal(await page.locator('.document-journal tbody > tr').count(),2,'saved journal reopens');
    await page.locator('[data-docs=close]').click();
    const paged=await save({id:'',version:0,content:{kind:'journal',number:'Проверка-26-'+Date.now(),fields:{object_name:'Проверка большого журнала',actual_start:state.today,actual_end:state.today},rows:Array.from({length:26},(_,i)=>({id:'paging-'+i,section:4,fields:{date:state.today,description:'Запись '+(i+1)},reviewed:false,sources:[]})),archived:false},reason:''});
    await page.locator('[data-docs=refresh]').click();await page.locator('.docs-document').filter({hasText:paged.content.number}).click();
    await page.locator('#docs-section-select').selectOption('4');assert.equal(await page.locator('.document-journal tbody > tr').count(),25);
    await page.getByRole('button',{name:'Следующие',exact:true}).click();assert.equal(await page.locator('.document-journal tbody > tr').count(),1);
    await page.locator('[data-docs=preview]').click();assert.equal(await page.locator('.document-journal .document-input').count(),0);assert.equal(await page.locator('.document-row-tools').count(),0);
    await page.locator('[data-docs=close]').click();
    await page.screenshot({path:path.join(output,'object-mobile.png')});
    assert.deepEqual(errors,[]);
    console.log(JSON.stringify({passed:true,output,checks:'paper forms, preview/edit preservation, manual signing fields, inline cells and review reset, 7 sections, import/dedup, revisions/conflicts, 26-row paging, PDF, 8 responsive layouts, accessibility'}));
  } finally { await browser.close(); }
})().catch(e=>{console.error(e);process.exitCode=1;});
