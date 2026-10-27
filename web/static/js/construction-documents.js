(() => {
  'use strict';
  const root = document.getElementById('object-documents');
  if (!root) return;
  const $ = id => document.getElementById(id);
  const editor = $('docs-editor'), rowEditor = $('docs-row-editor'), importer = $('docs-import');
  const copy = value => JSON.parse(JSON.stringify(value));
  const uid = () => [...crypto.getRandomValues(new Uint32Array(4))].map(n => n.toString(16).padStart(8, '0')).join('');
  const names = {act: 'Акт скрытых работ', journal: 'Журнал производства работ', requisites: 'Реквизиты объекта'};
  let state, current, section = 1, dirty = false, saving = false, row, rowDirty = false, sources = [], sourceKind = 'assignments', importSequence = 0;
  let opener, paperPreview = false, rowPage = 0;
  const weatherRequests=new Map(), rowRequests=new WeakMap(), rowAutofill=new WeakMap(), assignmentRequests=new WeakMap();
  let assignmentNotice='',assignmentError=false,editSequence=0;
  const el = (tag, text, cls) => { const node = document.createElement(tag); if (text != null) node.textContent = text; if (cls) node.className = cls; return node; };
  const icon = name => { const n = el('span', null, 'docs-icon'); n.dataset.icon = name; n.setAttribute('aria-hidden', 'true'); return n; };
  const button = (label, handler, iconName, cls = 'btn btn-secondary') => {
    const b = el('button', null, cls); b.type = 'button';
    if (iconName) b.append(icon(iconName)); b.append(document.createTextNode(label));
    b.addEventListener('click', handler); return b;
  };
  function notify(text,error=true) {
    const dialog = document.activeElement?.closest('.docs-dialog[open]') || [...root.querySelectorAll('.docs-dialog[open]')].at(-1);
    if (!dialog) { window.WorkServiceUI?.feedback(text,error); return; }
    dialog.querySelector('[data-doc-feedback]')?.remove();
    const message = el('p', text, error?'docs-error':'docs-local-status'); message.dataset.docFeedback = ''; message.setAttribute('role',error?'alert':'status'); dialog.append(message);
  }
  function showError(error) { $('docs-error').textContent = error.message || error; $('docs-error').hidden = false; }
  function markDirty() {
    dirty = true; $('docs-save-state').textContent = 'Есть несохранённые изменения';
    const note = $('docs-completeness');
    if (note) completeness(note);
  }
  function completeness(node) {
    const count = missing(current.content.fields, state.schema[current.content.kind]).length;
    node.textContent = count ? `Обязательных полей осталось: ${count}. Черновик можно сохранить сейчас.` : 'Обязательные поля заполнены. Перед подписанием проверьте сведения.';
  }
  function dateLabel(value) { const date = new Date(value + 'T12:00:00Z'); return Number.isNaN(date.getTime()) ? value : date.toLocaleDateString('ru-RU', {timeZone:'UTC'}); }
  function actDateLabel(value) {
    const date=new Date(value+'T12:00:00Z');if(Number.isNaN(date.getTime()))return value;
    const parts=new Intl.DateTimeFormat('ru-RU',{timeZone:'UTC',day:'2-digit',month:'long',year:'numeric'}).formatToParts(date);
    return `«${parts.find(p=>p.type==='day').value}» ${parts.find(p=>p.type==='month').value} ${parts.find(p=>p.type==='year').value} г.`;
  }
  function missing(values, fields) {
    const result = fields.filter(f => f.required && !(values[f.key] || '').trim()).map(f => f.label);
    if (values.violations === 'present' && !values.deviations?.trim()) result.push('Согласование нарушений');
    return result;
  }
  async function request(path = '', body) {
    const controller = new AbortController(), timer = setTimeout(() => controller.abort(), 30000);
    try {
      const response = await fetch(root.dataset.endpoint + path, {method: body ? 'POST' : 'GET', cache: 'no-store', credentials: 'same-origin', signal: controller.signal,
        headers: {Accept: 'application/json', ...(body ? {'Content-Type': 'application/json', 'X-CSRF-Token': root.dataset.csrf} : {})}, ...(body ? {body: JSON.stringify(body)} : {})});
      if (response.redirected || !response.headers.get('content-type')?.includes('application/json')) throw new Error('Сессия истекла. Откройте вход в другой вкладке; текст в редакторе не потерян.');
      const data = await response.json(); if (!response.ok) throw new Error(data.error || 'Не удалось выполнить запрос'); return data;
    } catch (error) { if (error.name === 'AbortError') throw new Error('Сервер не ответил. Проверьте соединение и обновите список перед повторным сохранением.'); throw error; }
    finally { clearTimeout(timer); }
  }
  function confirmAction(text) {
    return new Promise(resolve => {
      const dialog = $('docs-confirm'); $('docs-confirm-text').textContent = text;
      const finish = answer => { dialog.close(); resolve(answer); };
      $('docs-confirm-yes').onclick = () => finish(true); $('docs-confirm-no').onclick = () => finish(false);
      dialog.oncancel = event => { event.preventDefault(); finish(false); }; dialog.showModal(); $('docs-confirm-no').focus();
    });
  }
  function openDialog(dialog) { dialog.querySelector('[data-doc-feedback]')?.remove(); dialog.showModal(); dialog.querySelector('button')?.focus({preventScroll: true}); dialog.querySelectorAll('.docs-scroll').forEach(node => { node.scrollTop = 0; }); }
  async function refresh() {
    state = await request(); renderList();
  }
  function renderList() {
    $('docs-font-status').hidden=state.font?.available!==false;$('docs-font-status').textContent=state.font?.reason||'';
    $('docs-weather-attribution').hidden=!state.weather?.enabled;
    const list = $('docs-list'); list.replaceChildren();
    const query = $('docs-search').value.trim().toLocaleLowerCase('ru');
    const archived = $('docs-filter').value === 'archived';
    const docs = state.documents.filter(d => d.content.kind !== 'requisites' && d.content.archived === archived && `${d.content.number} ${d.content.fields.work_name || ''} ${names[d.content.kind]}`.toLocaleLowerCase('ru').includes(query));
    if (!docs.length) { list.append(el('p', query ? 'Документы не найдены' : archived ? 'В архиве пока пусто' : 'Актов и журналов пока нет', 'docs-empty')); return; }
    const ul = el('ul', null, 'docs-list'); list.append(ul);
    docs.forEach(d => {
      const li = el('li'), b = button('', async () => { try { await edit(d.id); } catch (e) { notify(e.message); } }, null, 'docs-document');
      const symbol = icon(d.content.kind === 'journal' ? 'file' : 'check');
      const info = el('span', null, 'docs-document-info');
      info.append(el('strong', `${d.content.number} · ${d.content.fields.work_name || names[d.content.kind]}`));
      const count = state.rowCounts[d.id] || 0, noun = {one:'запись',few:'записи',many:'записей',other:'записи'}[new Intl.PluralRules('ru').select(count)];
      info.append(el('span', `${d.content.kind === 'journal' ? `${count} ${noun} · ` : ''}Версия ${d.version} · ${new Date(d.updatedAt).toLocaleDateString('ru-RU')}`));
      b.append(symbol, info, icon('chevron-right')); li.append(b); ul.append(li);
    });
  }
  async function edit(id, kind) {
    const editing=++editSequence;
    if (!id || !state) await refresh();
    if(editing!==editSequence)return;
    opener = document.activeElement;
    if (id) {const document=await request('/' + encodeURIComponent(id));if(editing!==editSequence)return;current=document;}
    else {
      const defaults = state.automation?.defaults || state.documents.find(d => d.content.kind === 'requisites')?.content.fields || {};
      const fields = {};
      state.schema[kind].forEach(f => { if (defaults[f.key]) fields[f.key] = defaults[f.key]; });
      fields.object_name ||= state.object.name; fields.address ||= state.object.address;
      if (kind === 'act') { fields.act_date = state.today; fields.performer = defaults.contractor || ''; }
      if(kind==='act')for(const s of state.layouts.signatories){const matches=(state.automation?.people||[]).filter(p=>p.value===fields[s.representative]||p.name===fields[s.representative]);if(matches.length===1&&!fields[s.name])fields[s.name]=matches[0].name;}
      if (kind === 'journal') fields.issue_date = state.today;
      current = {id: '', version: 0, content: {kind, number: '', fields, rows: [], archived: false}, history: []};
    }
    let refreshedDefaults=false;
    if(current.content.kind==='requisites'){
      for(const f of state.schema.requisites){const value=state.automation?.defaults[f.key];if(value&&(!current.content.fields[f.key]?.trim()||(f.key==='responsible'&&state.object.responsibleUserId))&&current.content.fields[f.key]!==value){current.content.fields[f.key]=value;refreshedDefaults=true;}}
    }
    section = current.content.kind==='journal'?0:1; rowPage = 0; paperPreview = false; dirty = !id||refreshedDefaults; $('docs-reason').value = ''; $('docs-error').hidden = true;
    $('docs-number').value = current.content.number;
    $('docs-number').closest('label').hidden = true;
    editor.querySelector('[data-docs=preview]').hidden = current.content.kind === 'requisites';
    $('docs-editor-title').textContent = names[current.content.kind];
    $('docs-section-select').replaceChildren();
    const sections = [{number: 1, title: current.content.kind === 'journal' ? 'Общие сведения' : current.content.kind === 'act' ? 'Содержание акта' : 'Реквизиты'}];
    if (current.content.kind === 'journal') sections.unshift({number:0,title:'Титульный лист'});
    if (current.content.kind === 'journal') sections.push(...state.schema.sections);
    sections.forEach(s => { const o = el('option', s.number===0?s.title:`${s.number}. ${s.title}`); o.value = s.number; $('docs-section-select').append(o); });
    $('docs-section-select').parentElement.hidden = sections.length === 1;
    editor.querySelector('[data-docs=pdf]').hidden = current.content.kind === 'requisites';
    assignmentNotice='';assignmentError=false;
    const added=current.content.kind==='journal'?await syncJournalAssignments({render:false,quiet:true}):[];
    if(editing!==editSequence||!root.isConnected)return;
    updateMeta(); renderEditor(); openDialog(editor);
    if(added.length)fillWeatherRows(added,true);
  }
  function updateMeta() {
    $('docs-editor-meta').textContent = current.content.archived ? 'В архиве · сохранена история' : current.version ? `Версия ${current.version} · ${current.updatedBy}` : 'Новый черновик';
    $('docs-save-state').textContent = dirty ? 'Есть несохранённые изменения' : 'Сохранено';
    $('docs-reason-label').hidden = !current.version;
  }
  function fieldsForm(fields, values, onChange, prefix) {
    const grid = el('div', null, 'docs-fields');
    fields.forEach(f => {
      if(f.type==='hidden'||f.type==='annotation')return;
      const wrap = el('div', null, 'docs-field' + (f.type === 'textarea' ? ' docs-field-wide' : ''));
      const label = el('label', f.label + (f.required ? ' *' : '')); label.htmlFor = `${prefix}-${f.key}`;
      let input;
      if (f.type === 'violations') {
        input = el('select'); [['', 'Не проверено'], ['none', 'Отсутствуют'], ['present', 'Допущены']].forEach(([value, text]) => { const o = el('option', text); o.value = value; input.append(o); });
      } else if (f.type === 'textarea') { input = el('textarea'); input.rows = 3; }
      else { input = el('input'); input.type = f.type==='city'?'text':f.type; }
      input.id = label.htmlFor; input.value = values[f.key] || ''; input.maxLength = 5000;
      if(f.key==='responsible'&&current.content.kind==='requisites'&&state.object.responsibleUserId){input.readOnly=true;label.textContent+=' (из карточки объекта)';}
      if (f.type === 'date') { input.min = '1900-01-01'; input.max = '2200-12-31'; }
      input.addEventListener('input', () => { values[f.key] = input.value; onChange(); });
      wrap.append(label, input); grid.append(wrap);
      wireSuggestions(input,f,values);
      if(f.type==='city'&&!state.weather.enabled)wrap.append(el('small',state.weather.reason,'docs-note'));
    });
    return grid;
  }
  function fitPaperFields(scope) {
    const resize = () => scope.querySelectorAll('textarea.document-input').forEach(resizePaperInput);
    requestAnimationFrame(resize); document.fonts?.ready.then(resize);
  }
  function resizePaperInput(input) {
    input.style.height='0';const full=Math.max(36,input.scrollHeight+2),height=input.closest('.document-journal')?Math.min(full,300):full;
    input.style.height=height+'px';input.style.overflowY=full>height?'auto':'hidden';
  }
  function paperField(field, values, onChange, prefix, hint) {
    const wrap = el('span', null, 'document-variable');
    if(field.type==='annotation'){const note=el('small',values[field.key]||'','document-weather-source');note.id=prefix+'-'+field.key;wrap.append(note);return wrap;}
    if (paperPreview) {
      const text = field.type === 'date' ? (field.key==='act_date'?actDateLabel:dateLabel)(values[field.key] || '') : field.type === 'datetime-local' ? (values[field.key]?dateLabel(values[field.key].split('T')[0])+'\n'+values[field.key].split('T')[1]:'') : field.type === 'violations' ? ({none:'отсутствуют',present:'допущены'}[values[field.key]] || '') : values[field.key];
      wrap.append(el('span', text || (field.key==='number'?'______':'\u00a0'), 'document-value'));
      return wrap;
    }
    const label = el('label',field.label,'sr-only'), id = prefix + '-' + field.key; label.htmlFor = id;
    if(field.type==='datetime-local'){
      wrap.classList.add('document-signing-time');const parts=(values[field.key]||'').split('T');
      const date=el('input'),time=el('input');date.type='date';time.type='time';date.id=id+'-date';time.id=id+'-time';date.value=parts[0]||'';time.value=parts[1]||'';
      date.className=time.className='document-input';date.min='1900-01-01';date.max='2200-12-31';
      date.setAttribute('aria-label',field.label+' — дата');time.setAttribute('aria-label',field.label+' — время');
      const change=()=>{values[field.key]=date.value||time.value?date.value+'T'+time.value:'';onChange();};date.addEventListener('input',change);time.addEventListener('input',change);
      wrap.append(date,time);return wrap;
    }
    let input;
    if (field.type === 'violations') {
      input = el('select'); [['','Выберите результат'],['none','отсутствуют'],['present','допущены']].forEach(([v,t]) => { const option=el('option',t); option.value=v; input.append(option); });
    } else if (['date','datetime-local'].includes(field.type)) { input=el('input');input.type=field.type;input.min=field.type==='date'?'1900-01-01':'1900-01-01T00:00';input.max=field.type==='date'?'2200-12-31':'2200-12-31T23:59'; }
    else { input=el('textarea');input.rows=1;input.maxLength=field.key==='number'?80:field.key.endsWith('_signer')?150:5000;input.placeholder=hint || field.label; }
    input.className='document-input';input.id=id;input.value=values[field.key] || '';input.setAttribute('aria-required',String(!!field.required));
    input.addEventListener('input',()=>{values[field.key]=input.value;onChange();if(input.tagName==='TEXTAREA')resizePaperInput(input);});
    wrap.append(label,input);wireSuggestions(input,field,values);return wrap;
  }
  const peopleKeys=new Set(['responsible','contractor_rep','subcontractor_rep','technical_rep','author_rep','quality_rep','person','persons','crew','inspector','signers']);
  function wireSuggestions(input,field,values){
    if(input.readOnly||!window.WorkServiceDocumentSuggestions||!['text','textarea','city'].includes(field.type))return;
    if(field.type==='city'){
      input.addEventListener('input',()=>{delete values.weather_latitude;delete values.weather_longitude;});
      window.WorkServiceDocumentSuggestions.attach(input,{
        label:field.label,
        items:async value=>state.weather.enabled?(await request('/cities?q='+encodeURIComponent(value))).cities.map(c=>({value:c.label,label:c.label,city:c})):[],
        select:item=>{input.value=item.value;input.dispatchEvent(new Event('input',{bubbles:true}));values.weather_latitude=String(item.city.latitude);values.weather_longitude=String(item.city.longitude);}
      });return;
    }
    const person=peopleKeys.has(field.key)||field.key.endsWith('_signer'),multi=['crew','persons','signers'].includes(field.key);
    window.WorkServiceDocumentSuggestions.attach(input,{
      label:field.label,
      items:value=>{
        const term=(multi?value.split('\n').at(-1):value).trim().toLocaleLowerCase('ru');
        const saved=state.automation?.suggestions[field.key]||[];
        const suggestions=(person?saved.flatMap(text=>text.split('\n').map(line=>line.trim()).filter(Boolean)):saved).map(text=>({value:text,label:text,detail:'Из сохранённых документов'}));
        if(person)suggestions.unshift(...(state.automation?.people||[]).map(p=>({value:field.key.endsWith('_signer')?p.name:p.value,label:field.key.endsWith('_signer')?p.name:p.value,detail:p.assigned?'Сотрудник объекта':'Сотрудник сервиса'})));
        if(!person&&term.length<2)return [];
        const seen=new Set();return suggestions.filter(item=>{
          const key=item.value.toLocaleLowerCase('ru');if(seen.has(key)||!term.split(/\s+/).every(word=>key.includes(word))||item.value===value)return false;seen.add(key);return true;
        }).slice(0,12);
      },
      select:item=>{const lines=multi?input.value.split('\n'):[];if(multi)lines.pop();input.value=multi?[...lines,item.value].join('\n'):item.value;input.dispatchEvent(new Event('input',{bubbles:true}));}
    });
  }
  function defaultsForRow(r){
    const d=state.automation?.defaults||{};
    if(r.section===4)r.fields.persons||=[d.responsible,d.quality_rep].filter((v,i,a)=>v&&a.indexOf(v)===i).join('\n');
    if(r.section===6)r.fields.inspector||=d.quality_rep||'';
  }
  function liveRow(r){return (rowEditor.open&&row===r)||(current?.content.rows||[]).includes(r);}
  function updateWeatherUI(r){
    for(const prefix of ['doc-cell-'+r.id,...(row===r&&rowEditor.open?['doc-row']:[])]){
      const input=$(prefix+'-weather');if(input&&input.matches('input,textarea')){input.value=r.fields.weather||'';if(input.tagName==='TEXTAREA')resizePaperInput(input);}
      const note=$(prefix+'-weather_source');if(note)note.textContent=r.fields.weather_source||'';
    }
  }
  function rowChanged(r,key,changed){
    r.reviewed=false;
    const automatic=rowAutofill.get(r);
    if(automatic&&['crew','description','persons','shift'].includes(key))delete automatic.values[key];
    if(key==='weather'){delete r.fields.weather_source;delete r.fields.weather_date;updateWeatherUI(r);}
    if(key==='date'){
      if(automatic&&automatic.date!==r.fields.date){
        for(const [name,value]of Object.entries(automatic.values))if(r.fields[name]===value)delete r.fields[name];
        r.sources=(r.sources||[]).filter(source=>!automatic.sources.includes(source));rowAutofill.delete(r);updateAssignmentUI(r);
      }
      if(r.fields.weather_date&&r.fields.weather_date!==r.fields.date){delete r.fields.weather;delete r.fields.weather_source;delete r.fields.weather_date;updateWeatherUI(r);}
      if(/^\d{4}-\d{2}-\d{2}$/.test(r.fields.date||'')){fillRowFromAssignments(r);fillWeatherRows([r],true);}
    }
    changed();
  }
  function updateAssignmentUI(r){
    for(const prefix of ['doc-cell-'+r.id,...(row===r&&rowEditor.open?['doc-row']:[])])for(const key of ['crew','description','persons','shift']){const node=$(prefix+'-'+key);if(node&&node.matches('input,textarea')){node.value=r.fields[key]||'';if(node.tagName==='TEXTAREA')resizePaperInput(node);}}
  }
  async function fillRowFromAssignments(r){
    if(r.section!==4||!r.fields.date)return;
    const date=r.fields.date,doc=current,seq=(rowRequests.get(r)||0)+1;rowRequests.set(r,seq);
    try{
      const result=await request('/sources?'+new URLSearchParams({from:date,to:date,section:'4'}));
      if(current!==doc||!liveRow(r)||rowRequests.get(r)!==seq||r.fields.date!==date)return;
      const relevant=result.rows.filter(s=>!r.fields.shift||s.fields.shift===r.fields.shift);
      const merge=key=>[...new Set(relevant.flatMap(s=>(s.fields[key]||'').split('\n')).filter(Boolean))].join('\n');
      let changed=false;
      const automatic=rowAutofill.get(r)||{date,values:{},sources:[]};
      for(const key of ['crew','description','persons','shift']){const value=merge(key);if(!r.fields[key]?.trim()&&value){r.fields[key]=value;automatic.values[key]=value;changed=true;}}
      if(!changed)return;
      const added=relevant.flatMap(s=>s.sources||[]).filter(source=>!(r.sources||[]).includes(source));
      automatic.sources=[...new Set([...automatic.sources,...added])];rowAutofill.set(r,automatic);
      r.sources=[...new Set([...(r.sources||[]),...added])];updateAssignmentUI(r);
      r.reviewed=false;if(rowEditor.open&&row===r){rowDirty=true;$('docs-row-reviewed').checked=false;}else markDirty();
    }catch{}
  }
  async function syncJournalAssignments({render=true,quiet=false}={}){
    const doc=current;if(doc?.content.kind!=='journal'||doc.content.archived)return [];
    if(assignmentRequests.has(doc))return assignmentRequests.get(doc);
    const job=(async()=>{
      try{
        const fields=doc.content.fields;
        const excluded=[...new Set((doc.content.rows||[]).flatMap(r=>r.sources||[]).filter(id=>id.startsWith('assignment:')))];
        const result=await request('/assignments',{from:fields.actual_start||fields.planned_start||'',to:fields.actual_end||'',excluded});
        if(current!==doc)return [];
        const seen=new Set((doc.content.rows||[]).flatMap(r=>r.sources||[]));
        const added=result.rows.filter(r=>r.sources?.length&&r.sources.every(id=>!seen.has(id)));
        if((doc.content.rows?.length||0)+added.length>1500)throw new Error('В журнале допускается 1500 записей. Уточните период или создайте следующий том.');
        if(added.length){
          doc.content.rows||=[];doc.content.rows.push(...added);markDirty();
          if(doc.version&&!$('docs-reason').value.trim())$('docs-reason').value='Добавление новых назначений объекта';
        }
        assignmentError=false;
        assignmentNotice=added.length?`Добавлено записей из назначений: ${added.length}. Раздел 4 · черновики для проверки.`:quiet?'':'Новых назначений за период журнала нет.';
        if(render&&editor.open){renderEditor();if(added.length)fillWeatherRows(added,true);}
        return added;
      }catch(error){if(current===doc){assignmentNotice=error.message;assignmentError=true;if(render&&editor.open)renderEditor();}return [];}
    })();
    assignmentRequests.set(doc,job);
    try{return await job;}finally{assignmentRequests.delete(doc);}
  }
  async function weatherLocation(){
    const values=current?.content.kind==='requisites'?current.content.fields:state.automation.defaults;
    let city=values.weather_city||'';
    if(!city){city=state.object.address.split(',')[0].trim().replace(/^(г\.?|город)\s+/i,'');}
    if(!city)throw new Error('Укажите город для погоды в реквизитах объекта');
    if(!values.weather_latitude||!values.weather_longitude){
      const matches=(await request('/cities?q='+encodeURIComponent(city))).cities.filter(c=>c.name.toLocaleLowerCase('ru')===city.toLocaleLowerCase('ru'));
      if(matches.length!==1)throw new Error('Уточните город в реквизитах: найдено несколько вариантов или точное совпадение отсутствует');
      const match=matches[0];values.weather_city=match.label;values.weather_latitude=String(match.latitude);values.weather_longitude=String(match.longitude);city=match.label;
    }
    return {city,latitude:values.weather_latitude,longitude:values.weather_longitude};
  }
  async function fillWeatherRows(rows,quiet=false){
    rows=rows.filter(r=>r.section===4&&r.fields.date&&r.fields.date<=state.today&&!r.fields.weather?.trim());
    if(!rows.length){if(!quiet)notify('Погода уже указана или нет записей с подходящими датами',false);return;}
    if(!state.weather.enabled){if(!quiet)notify(state.weather.reason);return;}
    const doc=current,dates=rows.map(r=>r.fields.date).sort(),expected=new Map(rows.map(r=>[r,r.fields.date]));
    try{
      const location=await weatherLocation(),query=new URLSearchParams({...location,from:dates[0],to:dates.at(-1)}).toString();
      if(!weatherRequests.has(query))weatherRequests.set(query,request('/weather?'+query).finally(()=>weatherRequests.delete(query)));
      const result=await weatherRequests.get(query);if(current!==doc)return;
      let count=0;
      for(const r of rows){const day=result.days[expected.get(r)];if(!day||!liveRow(r)||r.fields.date!==expected.get(r)||r.fields.weather?.trim())continue;
        r.fields.weather=day.text;r.fields.weather_source=day.source;r.fields.weather_date=day.date;r.reviewed=false;
        if(rowEditor.open&&row===r){rowDirty=true;$('docs-row-reviewed').checked=false;}else markDirty();updateWeatherUI(r);count++;
      }
      if(!quiet)notify(count?'Погода подставлена. Источник указан в записи':'За выбранные даты данных нет или поля уже заполнены',false);
    }catch(error){if(current===doc&&!quiet)notify(error.message);}
  }
  function paperSignatures(values, onChange, prefix) {
    const table=el('table',null,'document-signatures');table.append(el('caption','Подписи представителей'));
    const head=el('thead'),tr=el('tr');['Представитель','Дата и время','Подпись','Фамилия, инициалы'].forEach(text=>{const th=el('th',text);th.scope='col';tr.append(th);});head.append(tr);table.append(head);
    const tbody=el('tbody');table.append(tbody);
    state.layouts.signatories.forEach(s=>{
      const row=el('tr'),role=el('th',s.role);role.scope='row';row.append(role);
      const at=el('td'),sign=el('td',null,'document-hand-sign'),name=el('td');sign.append(el('span',null,'document-signature-rule'));
      at.append(paperField(state.schema.act.find(f=>f.key===s.at),values,onChange,prefix));
      name.append(paperField(state.schema.act.find(f=>f.key===s.name),values,onChange,prefix,'Фамилия И. О.'));
      row.append(at,sign,name);tbody.append(row);
    });return table;
  }
  function paperDocument(kind) {
    const title=kind==='journal'&&section===0;
    const paper=el('article',null,'document-paper'+(title?' document-title-paper':''));paper.setAttribute('aria-label',title?'Титульный лист журнала':'Бланк: '+names[kind]);
    const fields=current.content.fields, schema=state.schema[kind];
    (title?state.layouts.journalTitle:state.layouts[kind]).forEach(block=>{
      if(block.kind==='cover-summary'){
        const summary=el('div',null,'document-cover-summary');
        for(const key of ['object_name','address','contractor'])if(fields[key]?.trim()){const text=Array.from(fields[key].trim());summary.append(el('p',text.length>260?text.slice(0,260).join('')+'…':fields[key]));}
        paper.append(summary);return;
      }
      if(block.optional && paperPreview && !fields[block.key])return;
      if(block.key==='deviations' && paperPreview && fields.violations!=='present')return;
      if(block.kind==='signatures'){paper.append(paperSignatures(fields,markDirty,'doc-field'));return;}
      if(block.kind==='signature-line'){paper.append(el('p','Подпись ____________________','document-signature-line'));return;}
      if(['title','subtitle','reference','text'].includes(block.kind)){
        const node=el(block.kind==='title'?'h3':'p',block.text,'document-'+block.kind);
        if((block.kind==='title'&&!title)||(block.kind==='subtitle'&&title)){
          const num={number:$('docs-number').value};const number=paperField({key:'number',label:'Номер документа',type:'text'},num,()=>{$('docs-number').value=num.number;markDirty();},'doc-paper','Автоматически');
          const line=el('span',null,'document-number');line.append(document.createTextNode(' № '),number);node.append(line);
        }
        paper.append(node);return;
      }
      const field=schema.find(f=>f.key===block.key),line=el('div',null,'document-line document-line-'+block.kind);
      if(block.text)line.append(el('span',block.text,'document-wording'));
      line.append(paperField(field,fields,()=>{
        markDirty();
        if(field.key==='violations'){
          const target=paper.querySelector('[data-deviations]');if(target)target.hidden=fields.violations!=='present';
        }
      },'doc-field',block.hint));
      if(block.hint&&block.kind!=='violations')line.append(el('small',block.hint,'document-undertext'));
      if(block.key==='deviations'){line.dataset.deviations='';line.hidden=fields.violations!=='present';}
      if(block.kind==='violations')line.append(el('span',block.hint,'document-wording'));
      paper.append(line);
    });
    return paper;
  }
  function journalPaper(rows,spec) {
    const paper=el('article',null,'document-paper document-paper-landscape');
    paper.append(el('p','Раздел '+section,'document-subtitle'),el('h3',state.layouts.titles[section],'document-section-title'),el('p',`Журнал № ${$('docs-number').value || '___'} · ${current.content.fields.object_name || ''}`,'document-object'));
    const region=el('div',null,'document-table-scroll');region.tabIndex=0;region.setAttribute('role','region');region.setAttribute('aria-label',spec.title+' — таблица журнала');paper.append(region);
    const table=el('table',null,'document-journal');const head=el('thead'),header=el('tr');
    const columns=state.layouts.columns[section];columns.forEach(c=>{const th=el('th',c.label);th.scope='col';th.style.width=(c.width/257*(paperPreview?100:90))+'%';header.append(th);});
    if(!paperPreview){const th=el('th','Действия');th.scope='col';th.style.width='10%';header.append(th);}head.append(header);table.append(head);region.append(table);
    const body=el('tbody');table.append(body);
    rows.slice(rowPage*25,rowPage*25+25).forEach((r,index)=>{
      const tr=el('tr');tr.dataset.rowId=r.id;
      columns.forEach(c=>{
        const td=el('td');c.keys.forEach(key=>{
          if(key==='_number'){td.append(el('span',String(rowPage*25+index+1)));return;}
          if(key==='_signature'){td.append(el('span','________________','document-hand-sign'));return;}
          const field=spec.fields.find(f=>f.key===key);
          td.append(paperField(field,r.fields,()=>rowChanged(r,key,()=>{markDirty();status.textContent='Черновик';}),'doc-cell-'+r.id));
        });tr.append(td);
      });
      const status=el('span',r.reviewed?'Проверено':'Черновик','document-row-state');
      if(!paperPreview){const td=el('td',null,'document-row-tools');const edit=button('',()=>editRow(r),'pencil','docs-icon-button');edit.title='Изменить запись';edit.setAttribute('aria-label','Изменить запись');
        const remove=button('',async()=>{if(await confirmAction('Удалить запись из текущей версии? В сохранённых версиях она останется.')){current.content.rows=current.content.rows.filter(x=>x.id!==r.id);markDirty();renderEditor();}},'trash-2','docs-icon-button');remove.title='Удалить запись';remove.setAttribute('aria-label','Удалить запись');td.append(status,edit,remove);tr.append(td);
      }body.append(tr);
    });
    if(!rows.length){const tr=el('tr'),td=el('td','Записей пока нет.');td.colSpan=columns.length+(paperPreview?0:1);tr.append(td);body.append(tr);}
    return paper;
  }
  function renderEditor() {
    window.WorkServiceDocumentSuggestions?.close();
    const body = $('docs-editor-body'); body.replaceChildren();
    const kind = current.content.kind, toolbar = el('div', null, 'docs-inline-tools');
    if(kind==='journal'){
      const count=(current.content.rows||[]).filter(r=>r.section===4).length,option=$('docs-section-select').querySelector('[value="4"]');
      if(option)option.textContent='4. Производство работ'+(count?' · '+count:'');
      if(assignmentNotice){const notice=el('p',assignmentNotice,assignmentError?'docs-error':'docs-note');notice.id='docs-assignment-notice';notice.setAttribute('role',assignmentError?'alert':'status');body.append(notice);}
    }
    body.classList.toggle('docs-paper-workspace',kind!=='requisites');
    body.classList.toggle('docs-paper-preview',paperPreview);
    editor.querySelector('[data-docs=preview]').setAttribute('aria-pressed',String(paperPreview));
    $('docs-preview-label').textContent=paperPreview?'Редактировать':'Просмотр бланка';
    if (section === 0) {
      toolbar.append(button('Общие сведения',()=>{section=1;$('docs-section-select').value='1';renderEditor();$('docs-editor-body').scrollTop=0;},'chevron-right'));
      if(!current.content.archived)toolbar.append(button('Обновить назначения',()=>syncJournalAssignments(),'refresh-cw'));
      body.append(toolbar,paperDocument(kind));
    } else if (section === 1) {
      if (kind === 'act') toolbar.append(button('Из назначений', () => openImport('assignments'), 'plus'));
      if (kind !== 'requisites') toolbar.append(button('Подставить пустые реквизиты', () => {
        const defaults = state.documents.find(d => d.content.kind === 'requisites')?.content.fields || {};
        state.schema[kind].forEach(f => { if (!current.content.fields[f.key]?.trim() && defaults[f.key]) current.content.fields[f.key] = defaults[f.key]; });
        markDirty(); renderEditor();
      }, 'copy'));
      body.append(toolbar,kind==='requisites'?fieldsForm(state.schema[kind],current.content.fields,markDirty,'doc-field'):paperDocument(kind));
      const note = el('p', '', 'docs-note'); note.id = 'docs-completeness'; completeness(note); body.append(note);
    } else {
      const spec = state.schema.sections.find(s => s.number === section);
      toolbar.append(button('Добавить запись', () => editRow(), 'plus', 'btn btn-primary'));
      if (section === 4||section===2) toolbar.append(button('Из назначений', () => openImport('assignments'), 'copy'));
      if(section===4&&!current.content.archived)toolbar.append(button('Обновить назначения',()=>syncJournalAssignments(),'refresh-cw'));
      if(section===4)toolbar.append(button('Заполнить погоду',()=>fillWeatherRows(current.content.rows||[]),'refresh-cw'));
      if (section === 3) toolbar.append(button('Из актов объекта', () => openImport('acts'), 'copy'));
      body.append(toolbar);
      const rows = (current.content.rows || []).filter(r => r.section === section);
      rowPage=Math.min(rowPage,Math.max(0,Math.ceil(rows.length/25)-1));
      if(rows.length>25){const pager=el('div',null,'docs-inline-tools');const prev=button('Предыдущие',()=>{rowPage--;renderEditor();},'chevron-left');prev.disabled=rowPage===0;const next=button('Следующие',()=>{rowPage++;renderEditor();},'chevron-right');next.disabled=(rowPage+1)*25>=rows.length;pager.append(prev,el('span',`${rowPage*25+1}–${Math.min(rows.length,(rowPage+1)*25)} из ${rows.length}`),next);body.append(pager);}
      body.append(journalPaper(rows,spec));
    }
    toolbar.hidden=paperPreview&&kind!=='requisites';
    if (current.id && kind !== 'requisites') {
      const tools = el('div', null, 'docs-document-actions');
      tools.append(button('Создать копию', async () => {
        if (!await confirmAction('Создать новый черновик на основе открытого документа? Несохранённые изменения войдут в копию.')) return;
        current = {id:'', version:0, content:copy(current.content), history:[]}; current.content.number = ''; current.content.archived = false;
        (current.content.rows || []).forEach(r => { r.id = uid(); r.reviewed = false; });
        if (kind === 'act') { current.content.fields.act_date = state.today; current.content.fields.violations = ''; current.content.fields.deviations = ''; }
        state.layouts.signatories.forEach(s=>{delete current.content.fields[s.at];});
        $('docs-number').value = ''; $('docs-reason').value = ''; markDirty(); updateMeta(); renderEditor();
      }, 'copy'));
      tools.append(button(current.content.archived ? 'Вернуть из архива' : 'В архив', async () => {
        if (!await confirmAction(current.content.archived ? 'Вернуть документ в текущие?' : 'Перенести документ в архив? Версии и PDF останутся доступны.')) return;
        current.content.archived = !current.content.archived; $('docs-reason').value ||= current.content.archived ? 'Перенос в архив' : 'Возврат из архива'; markDirty(); updateMeta(); renderEditor();
      }, 'folder')); body.append(tools);
    }
    fitPaperFields(body);
  }
  function editRow(existing) {
    window.WorkServiceDocumentSuggestions?.close();
    row = existing ? copy(existing) : {id: uid(), section, fields: {}, reviewed: false, sources: []};
    if (!existing && [4,6].includes(section)) row.fields.date = state.today;
    defaultsForRow(row);
    rowDirty = false;
    const spec = state.schema.sections.find(s => s.number === row.section);
    $('docs-row-title').textContent = spec.title;
    const paper=el('article',null,'document-paper document-row-paper');paper.append(el('p',`Раздел ${row.section}`,'document-subtitle'),el('h3',state.layouts.titles[row.section],'document-section-title'));
    const table=el('table',null,'document-row-sheet');const tbody=el('tbody');table.append(tbody);paper.append(table);
    spec.fields.forEach(field=>{if(field.type==='hidden')return;const tr=el('tr'),th=el('th',field.label),td=el('td');th.scope='row';td.append(paperField(field,row.fields,()=>rowChanged(row,field.key,()=>{rowDirty=true;$('docs-row-reviewed').checked=false;}),'doc-row'));tr.append(th,td);tbody.append(tr);});
    $('docs-row-fields').replaceChildren(paper);$('docs-row-fields').classList.add('docs-paper-workspace');
    if (row.sources?.length) $('docs-row-fields').prepend(el('p', 'Источник: данные сервиса. Фактическое выполнение и результаты контроля требуют подтверждения.', 'docs-note'));
    $('docs-row-reviewed').checked = row.reviewed; $('docs-row-error').hidden = true; openDialog(rowEditor);
    fitPaperFields($('docs-row-fields'));
    if(row.section===4){fillRowFromAssignments(row);fillWeatherRows([row],true);}
  }
  $('docs-row-reviewed').addEventListener('change', () => { rowDirty = true; });
  $('docs-row-form').addEventListener('submit', event => {
    event.preventDefault();
    const fields = state.schema.sections.find(s => s.number === row.section).fields;
    const required = missing(row.fields, fields);
    if ($('docs-row-reviewed').checked && required.length) { $('docs-row-error').textContent = 'Заполните: ' + required.join('; '); $('docs-row-error').hidden = false; return; }
    if (!$('docs-row-form').reportValidity()) return;
    row.reviewed = $('docs-row-reviewed').checked;
    current.content.rows ||= [];
    const index = current.content.rows.findIndex(r => r.id === row.id);
    if (index < 0) current.content.rows.push(row); else current.content.rows[index] = row;
    rowDirty = false; rowEditor.close(); markDirty(); renderEditor();
  });
  async function closeRow() { if (!rowDirty || await confirmAction('Закрыть запись без применения изменений?')) rowEditor.close(); }
  async function save() {
    if (saving) return false;
    if(assignmentRequests.has(current))await assignmentRequests.get(current);
    if(saving)return false;
    if (!dirty && current.id) return true;
    if (current.version && !$('docs-reason').value.trim()) { showError('Укажите основание изменения'); $('docs-reason').focus(); return false; }
    saving = true; $('docs-error').hidden = true;
    current.content.number = $('docs-number').value;
    // Freeze controls so a response cannot overwrite typing that happened in flight.
    const controls = [...editor.querySelectorAll('input,textarea,select,button')]; controls.forEach(n => n.disabled = true);
    $('docs-save-state').textContent = 'Сохранение…';
    try {
      current = await request('', {id:current.id, version:current.version, content:current.content, reason:$('docs-reason').value});
      assignmentNotice='';assignmentError=false;
      dirty = false; $('docs-number').value = current.content.number; $('docs-reason').value = ''; updateMeta();
      try { await refresh(); } catch { notify('Документ сохранён. Обновить список пока не удалось.'); }
      renderEditor();
      return true;
    } catch (error) { showError(error); $('docs-save-state').textContent = 'Не сохранено'; return false; }
    finally { saving = false; controls.forEach(n => n.disabled = false); }
  }
  $('docs-form').addEventListener('submit', async e => { e.preventDefault(); await save(); });
  $('docs-number').addEventListener('input', markDirty);
  $('docs-section-select').addEventListener('change', () => { section = Number($('docs-section-select').value);rowPage=0;renderEditor();$('docs-editor-body').scrollTop=0; });
  window.addEventListener('resize',()=>{if(editor.open)fitPaperFields(editor);if(rowEditor.open)fitPaperFields(rowEditor);});
  async function closeEditor() {
    if (saving) return;
    if (!dirty || await confirmAction('Закрыть документ без сохранения последних изменений?')) { dirty = false; editor.close(); opener?.focus({preventScroll:true}); }
  }
  editor.addEventListener('cancel', e => { e.preventDefault(); closeEditor(); });
  rowEditor.addEventListener('cancel', e => { e.preventDefault(); closeRow(); });
  window.addEventListener('beforeunload', e => { if (dirty || rowDirty && rowEditor.open || saving) { e.preventDefault(); e.returnValue = ''; } });
  editor.addEventListener('keydown', e => { if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') { e.preventDefault(); save(); } });
  async function downloadPDF(version) {
    if (!version && !await save()) return;
    const url = root.dataset.endpoint + '/' + encodeURIComponent(current.id) + '/pdf' + (version ? '?version=' + version : '');
    const response = await fetch(url, {cache: 'no-store', credentials: 'same-origin'});
    if (!response.ok || !response.headers.get('content-type')?.includes('application/pdf')) {let error;try{error=await response.json();}catch{}throw new Error(error?.error||'Не удалось получить PDF. Проверьте сессию и повторите попытку.');}
    const blob = await response.blob(), a = el('a'); const objectURL = URL.createObjectURL(blob);
    a.href = objectURL; a.download = `${current.content.number || 'Документ'}-v${version || current.version}.pdf`; a.click(); setTimeout(() => URL.revokeObjectURL(objectURL), 60000);
  }
  function showHistory() {
    const list = $('docs-history-list'); list.replaceChildren();
    if (!current.history?.length) list.append(el('p', 'Документ ещё не сохранён'));
    [...current.history || []].reverse().forEach(rev => {
      const details = el('details', null, 'docs-revision');
      details.append(el('summary', `Версия ${rev.version} · ${new Date(rev.at).toLocaleString('ru-RU')}`), el('p', rev.actor + ' · ' + rev.reason));
      if (current.content.kind !== 'requisites') details.append(button('PDF этой версии', () => downloadPDF(rev.version).catch(e => notify(e.message)), 'download'));
      const dl = el('dl');
      state.schema[rev.content.kind].forEach(f => { if (rev.content.fields[f.key]) { dl.append(el('dt', f.label), el('dd', f.type === 'violations' ? (rev.content.fields[f.key] === 'none' ? 'Отсутствуют' : 'Допущены') : rev.content.fields[f.key])); } });
      details.append(dl);
      (rev.content.rows || []).forEach(r => {
        const spec = state.schema.sections.find(s => s.number === r.section), group = el('dl');
        group.append(el('dt', `${r.section}. ${spec.title} · ${r.reviewed ? 'Проверено' : 'Черновик'}`));
        spec.fields.forEach(f => { if (r.fields[f.key]) group.append(el('dt', f.label), el('dd', r.fields[f.key])); }); details.append(group);
      }); list.append(details);
    }); openDialog($('docs-history'));
  }
  function sourceList() {
    const list = $('docs-source-list'); list.replaceChildren();
    const already = new Set((current.content.rows || []).flatMap(r => r.sources || []));
    if (!sources.length) list.append(el('p', 'Нет записей для добавления', 'docs-empty'));
    sources.forEach((r, index) => {
      const label = el('label', null, 'docs-source-row'), check = el('input'); check.type = 'checkbox'; check.value = index;
      check.disabled = r.sources.some(id => already.has(id));
      const text = el('span'); text.append(el('strong', r.fields.date ? dateLabel(r.fields.date) : r.fields.person||r.fields.act || 'Акт'), el('span', check.disabled ? 'Уже добавлено' : [r.fields.description,r.fields.crew].filter(Boolean).join('\n')||'Описание необходимо заполнить вручную'));
      label.append(check, text); list.append(label);
    });
    importer.querySelector('[data-docs=apply-import]').disabled = !sources.length;
  }
  async function openImport(kind) {
    sourceKind = kind; sources = []; importSequence++;
    $('docs-import-title').textContent = kind === 'acts' ? 'Акты объекта' : 'Назначения объекта';
    $('docs-source-form').hidden = kind === 'acts';
    $('docs-source-form').elements.from.value ||= state.today.slice(0,8) + '01';
    $('docs-source-form').elements.to.value ||= state.today;
    sourceList(); openDialog(importer);
    if (kind === 'acts') {
      try {
        const fresh = await request();
        if (!importer.open || sourceKind !== 'acts') return;
        sources = fresh.documents.filter(d => d.content.kind === 'act' && !d.content.archived).map(d => ({id:uid(), section:3, fields:{act:`${d.content.number}. ${d.content.fields.work_name || ''}\n${d.content.fields.works || ''}`.trim()}, reviewed:false, sources:['act:'+d.id]}));
        sourceList();
        $('docs-source-list').prepend(el('p', 'В реестр включают подписанные акты. Дату фактического подписания и состав подписавших укажите вручную.', 'docs-note'));
      } catch (e) { $('docs-source-list').textContent = e.message; }
    } else $('docs-source-form').requestSubmit();
  }
  $('docs-source-form').addEventListener('submit', async event => {
    event.preventDefault(); const seq = ++importSequence; sources = [];
    $('docs-source-list').textContent = 'Загрузка…'; importer.querySelector('[data-docs=apply-import]').disabled = true;
    const form = new FormData(event.currentTarget);
    form.set('section',String(current.content.kind==='journal'?section:4));
    try { const result = await request('/sources?' + new URLSearchParams(form)); if (seq !== importSequence || !importer.open) return; sources = result.rows; sourceList(); }
    catch (e) { if (seq === importSequence) $('docs-source-list').textContent = e.message; }
  });
  async function applyImport() {
    const selected = [...$('docs-source-list').querySelectorAll('input:checked:not(:disabled)')].map(input => sources[Number(input.value)]);
    if (!selected.length) return;
    if (current.content.kind === 'act') {
      const f = current.content.fields, descriptions = [...new Set(selected.map(r => r.fields.description).filter(Boolean))];
      f.works = [f.works, ...descriptions].filter(Boolean).join('\n');
      const dates = selected.map(r => r.fields.date).sort(); f.start_date ||= dates[0]; f.end_date ||= dates.at(-1);
    } else { current.content.rows ||= []; current.content.rows.push(...copy(selected)); }
    markDirty(); importer.close(); renderEditor();
    if(current.content.kind==='journal')await fillWeatherRows(selected,true);
  }
  const actions = {
    preview:()=>{paperPreview=!paperPreview;renderEditor();$('docs-editor-body').scrollTop=0;},
    refresh, act: () => edit('', 'act'), journal: () => edit('', 'journal'), requisites: async () => { await refresh(); const doc = state.documents.find(d => d.content.kind === 'requisites'); await edit(doc?.id || '', 'requisites'); },
    close: closeEditor, 'close-row': closeRow, 'close-import': () => { importSequence++; importer.close(); }, 'close-history': () => $('docs-history').close(), history: showHistory,
    pdf: () => downloadPDF(), 'apply-import': applyImport,
  };
  root.addEventListener('click', async event => {
    const action = event.target.closest('[data-docs]')?.dataset.docs;
    if (!actions[action] || saving) return;
    try { await actions[action](); } catch (error) { if (editor.open) showError(error); else { $('docs-list').textContent = error.message; notify(error.message); } }
  });
  $('docs-search').addEventListener('input', () => state && renderList()); $('docs-filter').addEventListener('change', () => state && renderList());
  refresh().catch(e => { $('docs-list').textContent = e.message; });
})();
