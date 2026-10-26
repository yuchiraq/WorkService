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
  let opener;
  const el = (tag, text, cls) => { const node = document.createElement(tag); if (text != null) node.textContent = text; if (cls) node.className = cls; return node; };
  const icon = name => { const n = el('span', null, 'docs-icon'); n.dataset.icon = name; n.setAttribute('aria-hidden', 'true'); return n; };
  const button = (label, handler, iconName, cls = 'btn btn-secondary') => {
    const b = el('button', null, cls); b.type = 'button';
    if (iconName) b.append(icon(iconName)); b.append(document.createTextNode(label));
    b.addEventListener('click', handler); return b;
  };
  function notify(text) {
    const dialog = document.activeElement?.closest('.docs-dialog[open]') || [...root.querySelectorAll('.docs-dialog[open]')].at(-1);
    if (!dialog) { window.WorkServiceUI?.feedback(text); return; }
    dialog.querySelector('[data-doc-feedback]')?.remove();
    const message = el('p', text, 'docs-error'); message.dataset.docFeedback = ''; message.setAttribute('role', 'alert'); dialog.append(message);
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
    if (!id || !state) await refresh();
    opener = document.activeElement;
    if (id) current = await request('/' + encodeURIComponent(id));
    else {
      const defaults = state.documents.find(d => d.content.kind === 'requisites')?.content.fields || {};
      const fields = {};
      state.schema[kind].forEach(f => { if (defaults[f.key]) fields[f.key] = defaults[f.key]; });
      fields.object_name ||= state.object.name; fields.address ||= state.object.address;
      if (kind === 'act') { fields.act_date = state.today; fields.performer = defaults.contractor || ''; }
      if (kind === 'journal') fields.issue_date = state.today;
      current = {id: '', version: 0, content: {kind, number: '', fields, rows: [], archived: false}, history: []};
    }
    section = 1; dirty = !id; $('docs-reason').value = ''; $('docs-error').hidden = true;
    $('docs-number').value = current.content.number;
    $('docs-number').closest('label').hidden = current.content.kind === 'requisites';
    $('docs-editor-title').textContent = names[current.content.kind];
    $('docs-section-select').replaceChildren();
    const sections = [{number: 1, title: current.content.kind === 'journal' ? 'Общие сведения' : current.content.kind === 'act' ? 'Содержание акта' : 'Реквизиты'}];
    if (current.content.kind === 'journal') sections.push(...state.schema.sections);
    sections.forEach(s => { const o = el('option', `${s.number}. ${s.title}`); o.value = s.number; $('docs-section-select').append(o); });
    $('docs-section-select').parentElement.hidden = sections.length === 1;
    editor.querySelector('[data-docs=pdf]').hidden = current.content.kind === 'requisites';
    updateMeta(); renderEditor(); openDialog(editor);
  }
  function updateMeta() {
    $('docs-editor-meta').textContent = current.content.archived ? 'В архиве · сохранена история' : current.version ? `Версия ${current.version} · ${current.updatedBy}` : 'Новый черновик';
    $('docs-save-state').textContent = dirty ? 'Есть несохранённые изменения' : 'Сохранено';
    $('docs-reason-label').hidden = !current.version;
  }
  function fieldsForm(fields, values, onChange, prefix) {
    const grid = el('div', null, 'docs-fields');
    fields.forEach(f => {
      const wrap = el('div', null, 'docs-field' + (f.type === 'textarea' ? ' docs-field-wide' : ''));
      const label = el('label', f.label + (f.required ? ' *' : '')); label.htmlFor = `${prefix}-${f.key}`;
      let input;
      if (f.type === 'violations') {
        input = el('select'); [['', 'Не проверено'], ['none', 'Отсутствуют'], ['present', 'Допущены']].forEach(([value, text]) => { const o = el('option', text); o.value = value; input.append(o); });
      } else if (f.type === 'textarea') { input = el('textarea'); input.rows = 3; }
      else { input = el('input'); input.type = f.type; }
      input.id = label.htmlFor; input.value = values[f.key] || ''; input.maxLength = 5000;
      if (f.type === 'date') { input.min = '1900-01-01'; input.max = '2200-12-31'; }
      input.addEventListener('input', () => { values[f.key] = input.value; onChange(); });
      wrap.append(label, input); grid.append(wrap);
    });
    return grid;
  }
  function renderEditor() {
    const body = $('docs-editor-body'); body.replaceChildren();
    const kind = current.content.kind, toolbar = el('div', null, 'docs-inline-tools');
    if (section === 1) {
      if (kind === 'act') toolbar.append(button('Из назначений', () => openImport('assignments'), 'plus'));
      if (kind !== 'requisites') toolbar.append(button('Подставить пустые реквизиты', () => {
        const defaults = state.documents.find(d => d.content.kind === 'requisites')?.content.fields || {};
        state.schema[kind].forEach(f => { if (!current.content.fields[f.key]?.trim() && defaults[f.key]) current.content.fields[f.key] = defaults[f.key]; });
        markDirty(); renderEditor();
      }, 'copy'));
      body.append(toolbar, fieldsForm(state.schema[kind], current.content.fields, markDirty, 'doc-field'));
      const note = el('p', '', 'docs-note'); note.id = 'docs-completeness'; completeness(note); body.append(note);
    } else {
      const spec = state.schema.sections.find(s => s.number === section);
      toolbar.append(button('Добавить запись', () => editRow(), 'plus', 'btn btn-primary'));
      if (section === 4) toolbar.append(button('Из назначений', () => openImport('assignments'), 'copy'));
      if (section === 3) toolbar.append(button('Из актов объекта', () => openImport('acts'), 'copy'));
      body.append(el('h3', spec.title), toolbar);
      const rows = (current.content.rows || []).filter(r => r.section === section);
      if (!rows.length) body.append(el('p', 'Записей в этом разделе пока нет', 'docs-empty'));
      const ul = el('ul', null, 'docs-rows'); body.append(ul);
      rows.forEach(r => {
        const li = el('li'), text = el('div', null, 'docs-row-description');
        const date = r.fields.date || r.fields.start || r.fields.signed_date || r.fields.issued || '';
        const description = r.fields.description || r.fields.person || r.fields.act || r.fields.name || r.fields.remark || r.fields.authority || 'Без описания';
        text.append(el('strong', date ? dateLabel(date) : 'Дата не указана'), el('p', description), el('span', r.reviewed ? 'Данные проверены' : 'Черновик · требует проверки', 'docs-note'));
        const open = button('Изменить', () => editRow(r), 'pencil');
        const remove = button('', async () => {
          if (await confirmAction('Удалить запись из текущей версии? В сохранённых версиях она останется.')) { current.content.rows = current.content.rows.filter(item => item.id !== r.id); markDirty(); renderEditor(); }
        }, 'trash-2', 'docs-icon-button'); remove.title = 'Удалить запись'; remove.setAttribute('aria-label', 'Удалить запись');
        li.append(text, open, remove); ul.append(li);
      });
    }
    if (current.id && kind !== 'requisites') {
      const tools = el('div', null, 'docs-document-actions');
      tools.append(button('Создать копию', async () => {
        if (!await confirmAction('Создать новый черновик на основе открытого документа? Несохранённые изменения войдут в копию.')) return;
        current = {id:'', version:0, content:copy(current.content), history:[]}; current.content.number = ''; current.content.archived = false;
        (current.content.rows || []).forEach(r => { r.id = uid(); r.reviewed = false; });
        if (kind === 'act') { current.content.fields.act_date = state.today; current.content.fields.violations = ''; current.content.fields.deviations = ''; }
        $('docs-number').value = ''; $('docs-reason').value = ''; markDirty(); updateMeta(); renderEditor();
      }, 'copy'));
      tools.append(button(current.content.archived ? 'Вернуть из архива' : 'В архив', async () => {
        if (!await confirmAction(current.content.archived ? 'Вернуть документ в текущие?' : 'Перенести документ в архив? Версии и PDF останутся доступны.')) return;
        current.content.archived = !current.content.archived; $('docs-reason').value ||= current.content.archived ? 'Перенос в архив' : 'Возврат из архива'; markDirty(); updateMeta(); renderEditor();
      }, 'folder')); body.append(tools);
    }
  }
  function editRow(existing) {
    row = existing ? copy(existing) : {id: uid(), section, fields: {}, reviewed: false, sources: []};
    if (!existing && [4,6].includes(section)) row.fields.date = state.today;
    rowDirty = false;
    const spec = state.schema.sections.find(s => s.number === row.section);
    $('docs-row-title').textContent = spec.title;
    $('docs-row-fields').replaceChildren(fieldsForm(spec.fields, row.fields, () => { rowDirty = true; row.reviewed = false; $('docs-row-reviewed').checked = false; }, 'doc-row'));
    if (row.sources?.length) $('docs-row-fields').prepend(el('p', 'Источник: данные сервиса. Фактическое выполнение и результаты контроля требуют подтверждения.', 'docs-note'));
    $('docs-row-reviewed').checked = row.reviewed; $('docs-row-error').hidden = true; openDialog(rowEditor);
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
    if (!dirty && current.id) return true;
    if (current.version && !$('docs-reason').value.trim()) { showError('Укажите основание изменения'); $('docs-reason').focus(); return false; }
    saving = true; $('docs-error').hidden = true;
    current.content.number = $('docs-number').value;
    // Freeze controls so a response cannot overwrite typing that happened in flight.
    const controls = [...editor.querySelectorAll('input,textarea,select,button')]; controls.forEach(n => n.disabled = true);
    $('docs-save-state').textContent = 'Сохранение…';
    try {
      current = await request('', {id:current.id, version:current.version, content:current.content, reason:$('docs-reason').value});
      dirty = false; $('docs-number').value = current.content.number; $('docs-reason').value = ''; updateMeta();
      try { await refresh(); } catch { notify('Документ сохранён. Обновить список пока не удалось.'); }
      renderEditor();
      return true;
    } catch (error) { showError(error); $('docs-save-state').textContent = 'Не сохранено'; return false; }
    finally { saving = false; controls.forEach(n => n.disabled = false); }
  }
  $('docs-form').addEventListener('submit', async e => { e.preventDefault(); await save(); });
  $('docs-number').addEventListener('input', markDirty);
  $('docs-section-select').addEventListener('change', () => { section = Number($('docs-section-select').value); renderEditor(); });
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
    if (!response.ok || !response.headers.get('content-type')?.includes('application/pdf')) throw new Error('Не удалось получить PDF. Проверьте сессию и повторите попытку.');
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
      const text = el('span'); text.append(el('strong', r.fields.date ? dateLabel(r.fields.date) : r.fields.act || 'Акт'), el('span', check.disabled ? 'Уже добавлено' : (r.fields.description || 'Описание необходимо заполнить вручную')));
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
    try { const result = await request('/sources?' + new URLSearchParams(form)); if (seq !== importSequence || !importer.open) return; sources = result.rows; sourceList(); }
    catch (e) { if (seq === importSequence) $('docs-source-list').textContent = e.message; }
  });
  function applyImport() {
    const selected = [...$('docs-source-list').querySelectorAll('input:checked:not(:disabled)')].map(input => sources[Number(input.value)]);
    if (!selected.length) return;
    if (current.content.kind === 'act') {
      const f = current.content.fields, descriptions = [...new Set(selected.map(r => r.fields.description).filter(Boolean))];
      f.works = [f.works, ...descriptions].filter(Boolean).join('\n');
      const dates = selected.map(r => r.fields.date).sort(); f.start_date ||= dates[0]; f.end_date ||= dates.at(-1);
    } else { current.content.rows ||= []; current.content.rows.push(...copy(selected)); }
    markDirty(); importer.close(); renderEditor();
  }
  const actions = {
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
