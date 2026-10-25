(() => {
  'use strict';
  const root = document.querySelector('.cloud-page');
  if (!root) return;
  const $ = id => document.getElementById(id);
  const editor = $('cloud-editor'), preview = $('cloud-preview');
  let state = JSON.parse($('cloud-data').textContent), index = new Map();
  let view = 'files', folder = '', visible = 100, selected = new Set(), busy = false, uploading = false;
  let editing = null, sequence = 0, previewSequence = 0, queue = [], stopped = false, activeXHR = null;
  let pdfDispose = null;
  const share = $('cloud-share');
  let sharing = null;
  const shared = item => !!item.shareToken && (!item.shareExpiresAt || new Date(item.shareExpiresAt) > new Date());
  const collator = new Intl.Collator('ru', {numeric:true, sensitivity:'base'});
  const feedback = (message, error = true) => {
    window.WorkServiceUI.feedback(message, error);
    if(!error) { const toast=document.querySelector('[data-page-toast].is-success'); setTimeout(()=>toast?.remove(),3500); }
  };
  const size = value => value < 1024 ? `${value} Б` : value < 1048576 ? `${(value/1024).toLocaleString('ru',{maximumFractionDigits:1})} КБ` : value < 1073741824 ? `${(value/1048576).toLocaleString('ru',{maximumFractionDigits:1})} МБ` : `${(value/1073741824).toLocaleString('ru',{maximumFractionDigits:2})} ГБ`;
  const date = value => new Date(value).toLocaleDateString('ru-RU', {day:'numeric', month:'short',year:'numeric'});
  const el = (tag, className, text) => { const node=document.createElement(tag); if(className) node.className=className; if(text!==undefined) node.textContent=text; return node; };
  const icon = name => { const node=el('span',`cl-icon cl-${name}`); node.setAttribute('aria-hidden','true'); return node; };
  const button = (label, symbol, action) => { const node=el('button','cl-button'); node.type='button'; node.title=label; node.setAttribute('aria-label',label); node.append(icon(symbol)); node.addEventListener('click',action); return node; };
  const refs = items => items.map(item=>({id:item.id,version:item.version}));
  const fileURL = item => `/cloud/file/${encodeURIComponent(item.id)}`;
  const uploadID = () => {
    if (crypto.randomUUID) return crypto.randomUUID();
    const bytes=crypto.getRandomValues(new Uint8Array(16)); bytes[6]=(bytes[6]&15)|64; bytes[8]=(bytes[8]&63)|128;
    const hex=Array.from(bytes,value=>value.toString(16).padStart(2,'0')).join('');
    return `${hex.slice(0,8)}-${hex.slice(8,12)}-${hex.slice(12,16)}-${hex.slice(16,20)}-${hex.slice(20)}`;
  };
  function reindex() { index=new Map(state.items.map(item=>[item.id,item])); }
  function folderPath(id) {
    const names=[]; let steps=0;
    while(id && steps++<34) { const item=index.get(id); if(!item) break; names.unshift(item.name); id=item.parentId; }
    return names.join(' / ');
  }
  function descendants(id, ancestor) {
    let steps=0; while(id && steps++<34) { if(id===ancestor) return true; id=index.get(id)?.parentId; } return false;
  }
  function shownItems() {
    const query=$('cloud-search').value.trim().toLocaleLowerCase('ru');
    const items=state.items.filter(item=> {
      if(view==='trash') return item.deletedAt && item.trashRootId===item.id && (!query || item.name.toLocaleLowerCase('ru').includes(query));
      if(item.deletedAt) return false;
      if(view==='starred' && !item.starred) return false;
      if(view==='recent' && item.kind!=='file') return false;
      if(query) return item.name.toLocaleLowerCase('ru').includes(query);
      return view!=='files' || item.parentId===folder;
    });
    const sort=$('cloud-sort').value;
    items.sort((a,b)=>{
      if(view!=='recent' && a.kind!==b.kind) return a.kind==='folder'?-1:1;
      if(view==='recent' || sort==='updated') return b.updatedAt.localeCompare(a.updatedAt) || collator.compare(a.name,b.name);
      if(sort==='size') return b.size-a.size || collator.compare(a.name,b.name);
      if(sort==='kind') return (a.mime||'').localeCompare(b.mime||'') || collator.compare(a.name,b.name);
      return collator.compare(a.name,b.name);
    });
    return items;
  }
  function saveLocation(replace=false) {
    const url=new URL(location.href); url.search='';
    if(view!=='files') url.searchParams.set('view',view); else if(folder) url.searchParams.set('folder',folder);
    if($('cloud-search').value) url.searchParams.set('q',$('cloud-search').value);
    history[replace?'replaceState':'pushState']({},'',url);
  }
  function readLocation() {
    const params=new URL(location.href).searchParams;
    view=['recent','starred','trash'].includes(params.get('view'))?params.get('view'):'files';
    folder=params.get('folder')||''; $('cloud-search').value=params.get('q')||'';
    if(folder && (!index.has(folder) || index.get(folder).deletedAt || index.get(folder).kind!=='folder')) folder='';
  }
  function navigate(nextFolder='',nextView='files') {
    folder=nextFolder; view=nextView; selected.clear(); visible=100; $('cloud-search').value=''; saveLocation(); render();
  }
  function breadcrumbs() {
    const nav=$('cloud-breadcrumbs'); nav.replaceChildren();
    const link=(name,id)=>{ const a=el('a','',name); a.href=id?`/cloud?folder=${encodeURIComponent(id)}`:'/cloud'; a.addEventListener('click',event=>{event.preventDefault();navigate(id);}); return a; };
    nav.append(link('Моё облако',''));
    if(view!=='files' || $('cloud-search').value.trim()) { nav.append(el('span','','/'),el('strong','',$('cloud-search').value.trim()?'Поиск':({recent:'Недавние',starred:'Избранное',trash:'Корзина'})[view])); return; }
    const parts=[]; let id=folder,steps=0;
    while(id && steps++<34) { const item=index.get(id); if(!item) break; parts.unshift(item); id=item.parentId; }
    for(const item of parts) nav.append(el('span','','/'),item.id===folder?el('strong','',item.name):link(item.name,item.id));
  }
  function selectionUI(items=shownItems().slice(0,visible)) {
    $('cloud-selection').hidden=!selected.size;
    $('cloud-selected-count').textContent=`Выбрано: ${selected.size}`;
    $('cloud-select-all').checked=!!items.length && items.every(item=>selected.has(item.id));
    $('cloud-select-all').indeterminate=items.some(item=>selected.has(item.id)) && !$('cloud-select-all').checked;
    $('cloud-select-all').disabled=busy || !items.length;
    for(const btn of root.querySelectorAll('[data-bulk]')) {
      btn.disabled=busy || uploading;
      if(btn.dataset.bulk!=='clear') btn.hidden=(['restore','purge'].includes(btn.dataset.bulk)) !== (view==='trash');
    }
  }
  function row(item) {
    const node=el('div','cloud-row'); node.dataset.id=item.id; node.classList.toggle('is-selected',selected.has(item.id));
    const label=el('label','cloud-check'); const check=el('input'); check.type='checkbox'; check.checked=selected.has(item.id); check.disabled=busy; check.setAttribute('aria-label',`Выбрать ${item.name}`);
    check.addEventListener('change',()=>{ if(check.checked && selected.size>=500) { check.checked=false; feedback('Выберите не более 500 элементов'); return; } check.checked?selected.add(item.id):selected.delete(item.id); node.classList.toggle('is-selected',check.checked); selectionUI(); }); label.append(check);
    const open=el('button','cloud-row-open'); open.type='button'; open.title=item.name; open.disabled=busy; open.append(icon(item.kind==='folder'?'folder':item.mime?.startsWith('image/')?'image':'file'));
    const copy=el('span','cloud-row-copy'); copy.append(el('span','cloud-row-name',item.name));
    if(shared(item)) { const badge=el('span','cloud-shared-badge','Доступ по ссылке'); badge.prepend(icon('link')); copy.append(badge); }
    node.classList.toggle('is-starred',item.starred);
    const detail=item.kind==='folder'?'Папка':size(item.size);
    copy.append(el('span','cloud-row-meta',`${detail} · ${date(item.deletedAt||item.updatedAt)}`)); open.append(copy);
    open.addEventListener('click',()=> item.deletedAt?openMenu(item,open):item.kind==='folder'?navigate(item.id):showPreview(item,open));
    const actions=el('div','cloud-row-actions');
    if(!item.deletedAt) { const star=button(item.starred?'Убрать из избранного':'В избранное','star',()=>mutate({action:'star',items:refs([item]),starred:!item.starred})); star.classList.add('cloud-star'); star.setAttribute('aria-pressed',String(item.starred)); star.disabled=busy || uploading; actions.append(star); }
    const more=button(`Действия: ${item.name}`,'more',()=>openMenu(item,more)); more.disabled=busy || uploading; actions.append(more);
    node.append(label,open,el('span','cloud-row-size',detail),el('span','cloud-row-date',date(item.deletedAt||item.updatedAt)),actions);
    return node;
  }
  function render() {
    reindex();
    if(folder && (!index.has(folder) || index.get(folder).deletedAt)) folder='';
    for(const id of selected) if(!index.has(id)) selected.delete(id);
    const items=shownItems(), visibleItems=items.slice(0,visible);
    for(const id of selected) if(!items.some(item=>item.id===id)) selected.delete(id);
    $('cloud-list').replaceChildren(...visibleItems.map(row));
    $('cloud-empty').textContent=items.length?'':$('cloud-search').value.trim()?'Ничего не найдено':view==='trash'?'Корзина пуста':view==='starred'?'В избранном пока ничего нет':view==='recent'?'Файлов пока нет':'В этой папке пока пусто';
    $('cloud-more').hidden=items.length<=visible;
    $('cloud-status').textContent=items.length?`${Math.min(visible,items.length)} из ${items.length}`:'';
    $('cloud-quota').max=state.quotaBytes; $('cloud-quota').value=Math.min(state.usedBytes,state.quotaBytes);
    $('cloud-usage').textContent=`${size(state.usedBytes)} из ${size(state.quotaBytes)}${state.trashBytes?` · в корзине ${size(state.trashBytes)}`:''}`;
    $('cloud-trash-count').textContent=state.items.filter(item=>item.deletedAt && item.trashRootId===item.id).length || '';
    const canAdd=view==='files' && !$('cloud-search').value.trim();
    $('cloud-upload').disabled=!canAdd || uploading || busy; $('cloud-new-folder').disabled=!canAdd || busy || uploading;
    $('cloud-upload').title=`До ${size(state.maxFileBytes)} на файл`;
    const fab=$('cloud-floating-upload'); if(fab) fab.disabled=$('cloud-upload').disabled;
    $('cloud-refresh').disabled=busy;
    for(const btn of root.querySelectorAll('[data-view]')) { if(btn.dataset.view===view) btn.setAttribute('aria-current','page'); else btn.removeAttribute('aria-current'); }
    selectionUI(visibleItems); breadcrumbs();
  }
  async function json(response) {
    if(response.redirected) throw new Error('Сессия завершилась. Войдите в аккаунт снова');
    let data; try { data=await response.json(); } catch { throw new Error('Сервер не ответил. Проверьте соединение'); }
    if(!response.ok) { const error=new Error(data.error || 'Операция недоступна'); error.status=response.status; throw error; }
    return data;
  }
  async function refresh() {
    if(busy || uploading || editor.open || preview.open || menu.open || share.open) return;
    const request=++sequence;
    try { const data=await json(await fetch('/cloud/items',{cache:'no-store'})); if(request===sequence && !busy && !uploading && !editor.open && !preview.open && !menu.open) { state=data; render(); } }
    catch(error) { feedback(error.message); }
  }
  async function mutate(payload,finish) {
    if(busy || uploading) return;
    busy=true; sequence++; render(); editor.querySelectorAll('button,input,select').forEach(node=>node.disabled=true);
    try {
      state=await json(await fetch('/cloud/action',{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':root.dataset.csrf},body:JSON.stringify(payload)}));
      selected.clear(); if(finish) finish(); feedback(({folder:'Папка создана',rename:'Имя изменено',move:'Перемещено',trash:'Перемещено в корзину',restore:'Восстановлено',purge:'Удалено навсегда',star:'Избранное обновлено'})[payload.action],false);
    } catch(error) { feedback(error.message); }
    finally { busy=false; editor.querySelectorAll('button,input,select').forEach(node=>node.disabled=false); $('cloud-name').disabled=!['folder','rename'].includes(editing?.action); render(); }
  }
  function openEditor(action,items=[],trigger=document.activeElement) {
    if(busy || uploading) return;
    editing={action,items:refs(items),parentId:folder};
    const titles={folder:'Новая папка',rename:'Переименовать',move:'Переместить',trash:'Переместить в корзину?',restore:'Восстановить?',purge:'Удалить навсегда?'};
    $('cloud-editor-title').textContent=titles[action];
    const named=action==='folder'||action==='rename';
    $('cloud-name-field').hidden=!named; $('cloud-name').disabled=!named; $('cloud-name').value=action==='rename'?items[0].name:'';
    $('cloud-destination-field').hidden=action!=='move'; $('cloud-destination').replaceChildren(new Option('Моё облако',''));
    if(action==='move') {
      const folders=state.items.filter(item=>item.kind==='folder'&&!item.deletedAt&&!items.some(root=>descendants(item.id,root.id)));
      folders.sort((a,b)=>collator.compare(folderPath(a.id),folderPath(b.id)));
      for(const item of folders) $('cloud-destination').add(new Option(folderPath(item.id),item.id));
      if(folders.some(item=>item.id===folder)) $('cloud-destination').value=folder;
    }
    $('cloud-confirm-text').hidden=named||action==='move';
    $('cloud-confirm-text').textContent=`${items.length===1?items[0].name:`Элементов: ${items.length}`}${action==='purge'?'\nФайлы и содержимое папок будут удалены без возможности восстановления.':action==='trash'?'\nСодержимое папок тоже попадёт в корзину.':''}`;
    $('cloud-editor-submit').textContent=({folder:'Создать',rename:'Сохранить',move:'Переместить',trash:'В корзину',restore:'Восстановить',purge:'Удалить навсегда'})[action];
    $('cloud-editor-submit').className=`btn ${action==='purge'?'btn-danger':'btn-primary'}`;
    editor.uiReturnFocus=trigger; editor.showModal();
    if(named) { $('cloud-name').focus(); $('cloud-name').select(); } else editor.querySelector('[data-close-dialog]').focus();
  }
  const menu=el('dialog','confirmation-dialog cloud-dialog'); menu.id='cloud-actions'; menu.setAttribute('aria-labelledby','cloud-actions-title');
  const menuBody=el('div','modal-content'), menuHeading=el('header','cloud-dialog-heading'), menuTitle=el('h2'); menuTitle.id='cloud-actions-title';
  menuHeading.append(menuTitle,button('Закрыть','x',()=>menu.close())); const commands=el('div','cloud-menu-commands'); menuBody.append(menuHeading,commands); menu.append(menuBody); root.append(menu);
  window.WorkServiceUI.prepare(menu);
  function openMenu(item,trigger) {
    menuTitle.textContent=item.name; commands.replaceChildren();
    const add=(label,symbol,fn)=>{ const btn=el('button'); btn.type='button'; btn.append(icon(symbol),el('span','',label)); btn.addEventListener('click',()=>{menu.close();fn();}); commands.append(btn); };
    if(item.deletedAt) { add('Восстановить','restore',()=>openEditor('restore',[item],trigger)); add('Удалить навсегда','trash',()=>openEditor('purge',[item],trigger)); }
    else {
      add('Скачать','download',()=>download([item]));
      add('Доступ по ссылке','link',()=>openShare(item,trigger));
      add('Переименовать','pencil',()=>openEditor('rename',[item],trigger));
      add('Переместить','move',()=>openEditor('move',[item],trigger));
      add(item.starred?'Убрать из избранного':'В избранное','star',()=>mutate({action:'star',items:refs([item]),starred:!item.starred}));
      add('В корзину','trash',()=>openEditor('trash',[item],trigger));
    }
    menu.uiReturnFocus=trigger; menu.showModal();
  }
  function renderShare() {
    if(!sharing) return;
    const active=shared(sharing);
    $('cloud-share-name').textContent=sharing.name;
    $('cloud-share-status').textContent=active ? (sharing.shareExpiresAt ? `Доступ открыт до ${date(sharing.shareExpiresAt)}` : 'Доступ открыт: только просмотр') : 'Доступ закрыт';
    $('cloud-share-expiry-field').hidden=active;
    $('cloud-share-link-field').hidden=!active;
    $('cloud-share-revoke').hidden=!sharing.shareToken;
    $('cloud-share-create').hidden=active;
    $('cloud-share-link').value=active?new URL(`/s/${sharing.shareToken}`,location.origin).href:'';
  }
  async function openShare(item,trigger) {
    sharing={...item}; share.uiReturnFocus=trigger; $('cloud-share-expiry').value='0'; renderShare(); share.showModal();
    const id=item.id;
    const request=++sequence;
    try {
      const next=await json(await fetch('/cloud/items',{cache:'no-store'}));
      if(!share.open || sharing?.id!==id || request!==sequence || busy) return;
      state=next; render(); sharing=index.get(id);
      if(!sharing || sharing.deletedAt) {share.close();feedback('Файл или папка недоступны');return;}
      renderShare();
    } catch(error) {feedback(error.message);}
  }
  async function changeShare(enabled) {
    if(busy || !sharing) return;
    busy=true; sequence++; render();
    share.querySelectorAll('button,select').forEach(node=>node.disabled=true);
    try {
      state=await json(await fetch(`/cloud/share/${encodeURIComponent(sharing.id)}`,{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':root.dataset.csrf},body:JSON.stringify({version:sharing.version,enabled,days:Number($('cloud-share-expiry').value)})}));
      reindex(); sharing=index.get(sharing.id); renderShare();
      feedback(enabled?'Ссылка создана':'Доступ по ссылке закрыт',false);
    } catch(error) {
      if(error.status===409) {
        try {state=await json(await fetch('/cloud/items',{cache:'no-store'}));reindex();sharing=index.get(sharing.id);if(sharing&&!sharing.deletedAt)renderShare();else share.close();} catch(_) {}
      }
      feedback(error.message);
    } finally {busy=false;share.querySelectorAll('button,select').forEach(node=>node.disabled=false);render();}
  }
  $('cloud-share-create').addEventListener('click',()=>changeShare(true));
  $('cloud-share-revoke').addEventListener('click',()=>changeShare(false));
  $('cloud-share-copy').addEventListener('click',async()=>{
    const field=$('cloud-share-link');
    try {
      if(navigator.clipboard && isSecureContext) await navigator.clipboard.writeText(field.value);
      else {field.focus();field.select();if(!document.execCommand('copy'))throw new Error();}
      feedback('Ссылка скопирована',false);
    } catch(_) {field.focus();field.select();feedback('Не удалось скопировать автоматически. Ссылка выделена для копирования.');}
  });
  share.addEventListener('cancel',event=>{if(busy)event.preventDefault();});
  async function showPreview(item,trigger) {
    const request=++previewSequence; const body=$('cloud-preview-body');
    $('cloud-preview-title').textContent=item.name; $('cloud-preview-meta').textContent=`${size(item.size)} · ${date(item.updatedAt)}`;
    $('cloud-preview-download').href=fileURL(item); $('cloud-preview-download').download=item.name;
    body.replaceChildren(el('p','','Загрузка...')); preview.uiReturnFocus=trigger; preview.showModal();
    if(item.mime?.startsWith('text/plain')) {
      try { const data=await json(await fetch(`${fileURL(item)}?preview=text`,{cache:'no-store'})); if(request!==previewSequence) return; body.replaceChildren(el('pre','',data.text)); if(data.truncated) body.append(el('p','','Показаны первые 512 КБ')); }
      catch(error) { if(request===previewSequence) body.replaceChildren(el('p','',error.message)); }
      return;
    }
    if(item.mime==='application/pdf') {
      try { const {mountPDF}=await import('/static/js/cloud-pdf.mjs?v=2'); if(request===previewSequence) pdfDispose=mountPDF(body,`${fileURL(item)}?preview=1`); }
      catch { if(request===previewSequence) body.replaceChildren(el('p','','Не удалось открыть PDF. Файл можно скачать')); }
      return;
    }
    let media;
    if(['image/jpeg','image/png','image/gif','image/webp'].includes(item.mime)) { media=el('img'); media.alt=item.name; }
    else if(item.mime?.startsWith('video/')) { media=el('video'); media.controls=true; media.preload='metadata'; }
    else if(item.mime?.startsWith('audio/')) { media=el('audio'); media.controls=true; media.preload='metadata'; }
    if(!media) { body.replaceChildren(el('p','','Предпросмотр недоступен. Файл можно скачать')); return; }
    media.src=`${fileURL(item)}?preview=1`; media.addEventListener('error',()=>{if(request===previewSequence) body.replaceChildren(el('p','','Не удалось открыть файл. Попробуйте скачать его'));}); body.replaceChildren(media);
  }
  async function download(items) {
    if(!items.length || busy) return;
    const single=items.length===1&&items[0].kind==='file';
    busy=true; sequence++; render();
    try {
      const url=single?fileURL(items[0]):(await json(await fetch('/cloud/archive',{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':root.dataset.csrf},body:JSON.stringify({ids:items.map(item=>item.id)})}))).url;
      const link=el('a'); link.href=url; link.download=single?items[0].name:'cloud.zip'; document.body.append(link); link.click(); link.remove();
    } catch(error) { feedback(error.message); }
    finally { busy=false;render(); }
  }
  function renderQueue() {
    $('cloud-queue').hidden=!queue.length;
    $('cloud-upload-list').replaceChildren(...queue.map(job=>{
      const row=el('li','cloud-upload-item'); row.append(el('strong','',job.file.name),el('span','',job.status==='done'?'Готово':job.status==='error'?'Ошибка':job.status==='uploading'?`${job.progress}%`:'В очереди'));
      if(job.error) row.append(el('p','',job.error));
      if(job.status==='uploading') { const progress=el('progress'); progress.max=100; progress.value=job.progress; progress.setAttribute('aria-label',`Загрузка ${job.file.name}`); row.append(progress); }
      return row;
    }));
    $('cloud-retry').hidden=uploading || !queue.some(job=>job.status==='error');
    $('cloud-stop').title=uploading?'Остановить загрузки':'Закрыть список загрузок'; $('cloud-stop').setAttribute('aria-label',$('cloud-stop').title);
  }
  function sendFile(job) {
    return new Promise((resolve,reject)=>{
      const xhr=new XMLHttpRequest(); activeXHR=xhr;
      xhr.open('POST',`/cloud/upload?parent=${encodeURIComponent(job.parentId)}`); xhr.timeout=600000;
      xhr.setRequestHeader('X-CSRF-Token',root.dataset.csrf); xhr.setRequestHeader('X-Upload-ID',job.id);
      xhr.upload.onprogress=event=>{if(event.lengthComputable) {job.progress=Math.round(event.loaded/event.total*100);renderQueue();}};
      xhr.onerror=()=>reject(new Error('Нет соединения. Повторите загрузку')); xhr.ontimeout=()=>reject(new Error('Сервер не ответил. Повторите загрузку')); xhr.onabort=()=>reject(new Error('Загрузка остановлена'));
      xhr.onload=()=>{try {const data=JSON.parse(xhr.responseText);if(xhr.status<200||xhr.status>=300) throw new Error(data.error||'Не удалось загрузить файл');resolve(data);}catch(error){reject(new Error(error instanceof SyntaxError?'Сессия завершилась или сервер недоступен':error.message));}};
      const body=new FormData(); body.append('file',job.file); xhr.send(body);
    });
  }
  async function processQueue() {
    if(uploading) return;
    uploading=true; stopped=false; sequence++; render(); renderQueue();
    try {
      for(const job of queue) {
        if(job.status!=='pending') continue;
        if(stopped) {job.status='error';job.error='Загрузка остановлена';continue;}
        job.status='uploading';job.error='';renderQueue();
        try {
          if(job.file.size>state.maxFileBytes) throw new Error(`Максимальный размер: ${size(state.maxFileBytes)}`);
          const result=await sendFile(job); state=result.state;job.status='done'; render();
        } catch(error) {job.status='error';job.error=error.message;}
        finally {activeXHR=null;renderQueue();}
      }
    } finally {uploading=false; renderQueue();render();$('cloud-files').value='';}
  }
  function enqueue(files) {
    if(uploading || busy) return;
    if(view!=='files' || $('cloud-search').value.trim()) {feedback('Откройте папку для загрузки');return;}
    if(files.length>100) {feedback('За один раз можно выбрать до 100 файлов');return;}
    if(!files.length) return;
    const failed=queue.filter(job=>job.status==='error');
    if(failed.length+files.length>100) {feedback('Сначала повторите неудачные загрузки или закройте их список');return;}
    queue=[...failed,...files.map(file=>({file,parentId:folder,id:uploadID(),status:'pending',progress:0,error:''}))];
    processQueue();
  }
  $('cloud-files').addEventListener('change',()=>enqueue(Array.from($('cloud-files').files)));
  $('cloud-upload').addEventListener('click',()=>$('cloud-files').click());
  $('cloud-floating-upload')?.addEventListener('click',()=>$('cloud-files').click());
  $('cloud-new-folder').addEventListener('click',()=>openEditor('folder'));
  $('cloud-refresh').addEventListener('click',refresh);
  $('cloud-retry').addEventListener('click',()=>{for(const job of queue) if(job.status==='error') {job.status='pending';job.error='';job.progress=0;}processQueue();});
  $('cloud-stop').addEventListener('click',()=>{if(uploading){stopped=true;activeXHR?.abort();}else{queue=[];renderQueue();}});
  $('cloud-editor-form').addEventListener('submit',event=>{event.preventDefault(); const payload={...editing}; if(payload.action==='folder'||payload.action==='rename') payload.name=$('cloud-name').value; if(payload.action==='move') payload.parentId=$('cloud-destination').value; mutate(payload,()=>editor.close());});
  for(const btn of root.querySelectorAll('[data-close-dialog]')) btn.addEventListener('click',()=>{if(!busy) btn.closest('dialog').close();});
  editor.addEventListener('cancel',event=>{if(busy)event.preventDefault();});
  preview.addEventListener('close',()=>{previewSequence++; pdfDispose?.();pdfDispose=null;$('cloud-preview-body').replaceChildren();});
  for(const dialog of [editor,preview,menu,share]) dialog.addEventListener('close',()=>{if(!dialog.uiReturnFocus?.isConnected)$('cloud-refresh').focus({preventScroll:true});});
  for(const btn of root.querySelectorAll('[data-view]')) btn.addEventListener('click',()=>navigate('',btn.dataset.view));
  $('cloud-search').addEventListener('input',()=>{selected.clear();visible=100;saveLocation(true);render();});
  $('cloud-sort').addEventListener('change',render);
  $('cloud-more').addEventListener('click',()=>{visible+=100;render();});
  $('cloud-select-all').addEventListener('change',()=>{const items=shownItems().slice(0,Math.min(visible,500)); const checked=$('cloud-select-all').checked; selected.clear();if(checked)items.forEach(item=>selected.add(item.id));render();});
  for(const btn of root.querySelectorAll('[data-bulk]')) btn.addEventListener('click',()=>{const items=[...selected].map(id=>index.get(id)).filter(Boolean);const action=btn.dataset.bulk;if(action==='clear'){selected.clear();render();}else if(action==='download')download(items);else openEditor(action,items,btn);});
  root.addEventListener('dragover',event=>{if(event.dataTransfer.types.includes('Files')){event.preventDefault();root.classList.add('is-dragging');}});
  root.addEventListener('dragleave',event=>{if(!root.contains(event.relatedTarget))root.classList.remove('is-dragging');});
  root.addEventListener('drop',event=>{event.preventDefault();root.classList.remove('is-dragging');if([...event.dataTransfer.items].some(item=>item.webkitGetAsEntry?.()?.isDirectory)){feedback('Папки загружайте файлами или одним ZIP-архивом');return;}enqueue(Array.from(event.dataTransfer.files));});
  window.addEventListener('focus',refresh);
  window.addEventListener('popstate',()=>{readLocation();selected.clear();visible=100;render();});
  window.addEventListener('beforeunload',event=>{if(uploading){event.preventDefault();event.returnValue='';}});
  reindex();readLocation();render();
})();
