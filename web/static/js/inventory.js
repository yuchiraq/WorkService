(() => {
  'use strict';
  const root = document.getElementById('object-inventory');
  if (!root) return;
  const $ = (id) => document.getElementById(id);
  const units = { pcs: 'шт.', set: 'компл.', pack: 'упак.', m: 'м', m2: 'м²', m3: 'м³', kg: 'кг', l: 'л' };
  const list = $('inventory-items');
  const editor = $('inventory-editor');
  const deletion = $('inventory-delete');
  const form = $('inventory-form');
  const search = $('inventory-search');
  const materialFilter = $('inventory-filter-material');
  const stockFilter = $('inventory-filter-stock');
  const drafts = new Map();
  const collapsed = new Map();
  let state = JSON.parse($('inventory-data').textContent);
  let editing = null;
  let deleting = null;
  let busy = false;
  let revision = 0;
  let editorDirty = false;
  let materialDirty = false;
  const quantity = (n) => (n / 1000).toLocaleString('ru-RU', { maximumFractionDigits: 3, useGrouping: false });
  const normalize = (s) => s.trim().replace(/\s+/g, ' ').toLocaleLowerCase('ru');
  const itemByID = (id) => state.items.find((item) => item.id === id);
  const materialOf = item => item.material || '';
  const touchInput = () => matchMedia('(pointer: coarse)').matches;
  function suggestMaterial(name) {
    const aliases = { 'нерж':'Нержавейка', 'нержавейка':'Нержавейка', 'нержавеющая':'Нержавейка', 'пвх':'ПВХ', 'pvc':'ПВХ', 'оцинк':'Оцинковка', 'оцинковка':'Оцинковка', 'оцинкованная':'Оцинковка' };
    const matches = new Set((normalize(name).match(/\p{L}+/gu) || []).map(word => aliases[word]).filter(Boolean));
    return matches.size === 1 ? [...matches][0] : '';
  }
  const feedback = (text, error = true) => window.WorkServiceUI.feedback(text, error);

  function element(tag, className, text) {
    const el = document.createElement(tag);
    if (className) el.className = className;
    if (text !== undefined) el.textContent = text;
    return el;
  }

  function iconButton(icon, label, action) {
    const button = element('button', 'inv-button');
    button.type = 'button';
    button.title = label;
    button.setAttribute('aria-label', label);
    button.dataset.action = action;
    const symbol = element('span', `inv-icon inv-${icon}`);
    symbol.setAttribute('aria-hidden', 'true');
    button.append(symbol);
    return button;
  }

  function row(item) {
    const li = element('li', 'inventory-row');
    li.dataset.id = item.id;
    const info = element('div', 'inventory-info');
    const name = element('h3', '', item.name);
    name.id = `inventory-label-${item.id}`;
    info.append(name);
    if (item.note) info.append(element('p', 'inventory-note', item.note));
    if (!item.quantity) info.append(element('span', 'inventory-zero', 'Нет в наличии'));
    const controls = element('div', 'inventory-row-controls');
    const stepper = element('div', 'inventory-stepper');
    const minus = iconButton('minus', `Уменьшить на 1: ${item.name}`, 'minus');
    const input = element('input', 'inventory-amount');
    input.type = 'text';
    input.inputMode = 'decimal';
    input.maxLength = 11;
    input.dataset.action = 'amount';
    input.setAttribute('aria-label', `Количество: ${item.name}, ${units[item.unit]}`);
    input.value = drafts.get(item.id)?.value ?? quantity(item.quantity);
    input.autocomplete = 'off';
    const plus = iconButton('plus', `Увеличить на 1: ${item.name}`, 'plus');
    const amount = element('div', 'inventory-amount-field');
    amount.append(input, element('span', 'inventory-unit', units[item.unit]));
    const save = iconButton('check', `Сохранить количество: ${item.name}`, 'save');
    save.classList.add('inventory-save');
    const reset = iconButton('refresh', `Отменить ввод количества: ${item.name}`, 'reset');
    reset.classList.add('inventory-reset');
    stepper.append(minus, reset, amount, plus, save);
    controls.append(stepper);
    const actions = element('details', 'inventory-row-actions');
    const menu = element('summary', 'inv-button');
    menu.dataset.action = 'menu';
    menu.title = `Действия: ${item.name}`;
    menu.setAttribute('aria-label', menu.title);
    const dots = element('span', 'inv-icon inv-more');
    dots.setAttribute('aria-hidden', 'true');
    menu.append(dots);
    const commands = element('div', 'inventory-row-menu');
    for (const [symbol, label, action] of [['pencil','Изменить','edit'],['trash','Удалить','delete']]) {
      const button = iconButton(symbol, `${label}: ${item.name}`, action);
      button.append(element('span', '', label));
      commands.append(button);
    }
    actions.append(menu, commands);
    actions.addEventListener('toggle', () => {
      if (actions.open) {
        list.querySelectorAll('.inventory-row-actions[open]').forEach(other => { if (other !== actions) other.open = false; });
        const bounds = menu.getBoundingClientRect();
        actions.classList.toggle('opens-up', bounds.bottom + commands.offsetHeight + 8 > window.innerHeight && bounds.top > commands.offsetHeight + 8);
      }
    });
    li.append(info, controls, actions);
    const conflict = element('p', 'inventory-conflict');
    li.append(conflict);
    updateRowControls(li, item);
    return li;
  }

  function updateRowControls(li, item) {
    const draft = drafts.get(item.id);
    const conflict = draft && draft.version !== item.version;
    li.querySelector('[data-action="minus"]').disabled = busy || !!draft || item.quantity < 1000;
    li.querySelector('[data-action="plus"]').disabled = busy || !!draft || item.quantity >= 1000000000;
    li.querySelector('[data-action="save"]').disabled = busy || !draft || !!conflict;
    li.querySelector('[data-action="save"]').hidden = !draft;
    li.querySelector('[data-action="minus"]').hidden = !!draft;
    li.querySelector('[data-action="plus"]').hidden = !!draft;
    li.querySelector('[data-action="reset"]').hidden = !draft;
    li.querySelector('[data-action="reset"]').disabled = busy;
    li.querySelector('[data-action="edit"]').disabled = busy || !!draft;
    li.querySelector('[data-action="delete"]').disabled = busy || !!draft;
    li.querySelector('input').disabled = busy;
    const message = li.querySelector('.inventory-conflict');
    if (message) {
      message.hidden = !conflict;
      message.textContent = conflict ? `Остаток изменён. Сейчас: ${quantity(item.quantity)} ${units[item.unit]}. Ваш ввод не сохранён.` : '';
    }
    li.classList.toggle('has-draft', !!draft);
  }

  function render() {
    const focused = document.activeElement;
    const focusRow = focused?.closest('.inventory-row')?.dataset.id;
    const action = focused?.dataset.action;
    const filter = normalize(search.value);
    const terms = filter.split(' ').filter(Boolean);
    const selectedMaterial = materialFilter.value;
    const selectedStock = stockFilter.value;
    const filtered = !!(filter || selectedMaterial || selectedStock);
    const visible = state.items.filter(item => {
      const text = normalize(`${item.name} ${item.note} ${materialOf(item)}`);
      return terms.every(term => text.includes(term)) &&
        (!selectedMaterial || selectedMaterial === `material:${materialOf(item)}`) &&
        (!selectedStock || (selectedStock === 'available' ? item.quantity > 0 : item.quantity === 0));
    });
    const groups = new Map();
    visible.forEach(item => {
      const material = materialOf(item);
      if (!groups.has(material)) groups.set(material, []);
      groups.get(material).push(item);
    });
    const grouped = [...groups].sort(([a], [b]) => !a ? 1 : !b ? -1 : a.localeCompare(b, 'ru', {sensitivity:'base'}));
    list.replaceChildren(...grouped.map(([material, items]) => {
      const section = element('details', 'inventory-group');
      section.dataset.material = material;
      section.open = filtered || !collapsed.get(material);
      const heading = element('summary', 'inventory-group-heading');
      const chevron = element('span', 'inv-icon inv-chevron');
      chevron.setAttribute('aria-hidden', 'true');
      heading.append(chevron, element('span', 'inventory-group-name', material || 'Без раздела'), element('span', 'inventory-group-count', String(items.length)));
      const rows = element('ul', 'inventory-group-rows');
      rows.setAttribute('aria-label', material || 'Без раздела');
      rows.replaceChildren(...items.sort((a,b) => a.name.localeCompare(b.name, 'ru', {numeric:true,sensitivity:'base'})).map(row));
      section.append(heading, rows);
      section.addEventListener('toggle', () => {
        if (!filtered && section.isConnected) { collapsed.set(material, !section.open); syncCollapse(); }
      });
      return section;
    }));
    syncCollapse();
    $('inventory-count').textContent = filtered ? `${visible.length} из ${state.items.length}` : String(state.items.length);
    $('inventory-count').setAttribute('aria-label', `Показано позиций: ${visible.length} из ${state.items.length}`);
    $('inventory-search-clear').hidden = !search.value;
    $('inventory-filter-reset').hidden = !(selectedMaterial || selectedStock);
    const empty = $('inventory-empty');
    empty.hidden = visible.length !== 0;
    empty.textContent = state.items.length ? 'Нет позиций по выбранным условиям' : 'На объекте пока нет инвентаря';
    const create = $('inventory-search-create');
    create.hidden = !filter || [...search.value.trim()].length > 120 || state.items.some(item => normalize(item.name) === filter || normalize(materialOf(item)) === filter);
    create.querySelector('span:last-child').textContent = `Добавить «${search.value.trim()}»`;
    create.disabled = busy;
    const history = $('inventory-history');
    history.replaceChildren(...state.history.map((change) => {
      const li = element('li');
      const item = change.after || change.before;
      const label = change.action === 'delete' ? 'Удалено' : !change.before ? 'Добавлено' : 'Изменено';
      li.append(element('strong', '', item.name));
      let detail = `${label}: ${quantity(item.quantity)} ${units[item.unit]}`;
      if (change.before && change.after) {
        detail = `${quantity(change.before.quantity)} ${units[change.before.unit]} → ${quantity(change.after.quantity)} ${units[change.after.unit]}`;
        if (change.before.name !== change.after.name) detail = `${change.before.name} → ${change.after.name} · ${detail}`;
        if (change.before.note !== change.after.note) detail += ' · Примечание изменено';
        if (materialOf(change.before) !== materialOf(change.after)) detail += ` · Материал: ${materialOf(change.after) || 'без раздела'}`;
      }
      li.append(element('span', 'inventory-history-detail', detail));
      const date = new Date(change.at).toLocaleString('ru-RU', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' });
      li.append(element('span', 'inventory-history-meta', `${change.actor_name || 'Пользователь'} · ${date}`));
      return li;
    }));
    if (!state.history.length) history.append(element('li', 'inventory-history-meta', 'Изменений пока нет'));
    const names = [...new Set(state.catalog.map((entry) => entry.name))];
    $('inventory-names').replaceChildren(...names.map((name) => {
      const option = element('option');
      option.value = name;
      option.label = state.catalog.filter((entry) => entry.name === name).map((entry) => units[entry.unit]).join(', ');
      return option;
    }));
    const materials = [...new Set(['Нержавейка','ПВХ','Оцинковка', ...state.catalog.map(materialOf), ...state.items.map(materialOf)].filter(Boolean))].sort((a,b)=>a.localeCompare(b,'ru'));
    $('inventory-materials').replaceChildren(...materials.map(material => { const option = element('option'); option.value = material; return option; }));
    const objectMaterials = new Set(state.items.map(materialOf));
    if (selectedMaterial) objectMaterials.add(selectedMaterial.slice('material:'.length));
    const allMaterials = element('option', '', 'Все материалы');
    allMaterials.value = '';
    materialFilter.replaceChildren(allMaterials, ...[...objectMaterials].sort((a,b) => !a ? 1 : !b ? -1 : a.localeCompare(b,'ru')).map(material => {
      const option = element('option', '', material || 'Без раздела');
      option.value = `material:${material}`;
      return option;
    }));
    materialFilter.value = selectedMaterial;
    if (focusRow) {
      const currentRow = [...list.querySelectorAll('.inventory-row')].find((li) => li.dataset.id === focusRow);
      const target = currentRow?.querySelector(`[data-action="${action}"]`);
      if (target && !target.disabled && !target.hidden) target.focus({ preventScroll: true });
      else if (currentRow) currentRow.querySelector(touchInput() ? '[data-action="menu"]' : 'input').focus({ preventScroll: true });
      else $('inventory-add').focus({ preventScroll: true });
    }
  }

  function syncCollapse() {
    const sections = [...list.querySelectorAll('.inventory-group')];
    const allClosed = sections.length > 0 && sections.every(section => !section.open);
    const button = $('inventory-collapse');
    button.title = allClosed ? 'Развернуть разделы' : 'Свернуть разделы';
    button.setAttribute('aria-label', button.title);
    button.classList.toggle('is-collapsed', allClosed);
    button.disabled = busy || !sections.length || !!(search.value.trim() || materialFilter.value || stockFilter.value);
  }

  function setBusy(value) {
    busy = value;
    root.setAttribute('aria-busy', String(value));
    for (const id of ['inventory-add', 'inventory-search-create', 'inventory-refresh', 'inventory-submit', 'inventory-delete-confirm', 'inventory-reload-editor']) $(id).disabled = value;
    for (const field of form.querySelectorAll('input, select, textarea')) field.disabled = value;
    for (const button of root.querySelectorAll('[data-close-dialog]')) button.disabled = value;
    for (const li of list.querySelectorAll('.inventory-row')) updateRowControls(li, itemByID(li.dataset.id));
    syncCollapse();
  }

  async function request(path = '', values) {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 30000);
    try {
      const response = await fetch(root.dataset.endpoint + path, {
        method: values ? 'POST' : 'GET', credentials: 'same-origin', cache: 'no-store',
        headers: { 'X-CSRF-Token': root.dataset.csrf, Accept: 'application/json' },
        body: values ? new URLSearchParams(values) : undefined,
        signal: controller.signal,
      });
      if (response.redirected || !response.headers.get('content-type')?.includes('application/json')) {
        throw new Error(response.status === 403 ? 'Сеанс устарел. Обновите страницу и повторите действие.' : 'Сеанс завершён. Войдите на сайт заново.');
      }
      const data = await response.json();
      if (!response.ok) {
        const error = new Error(data.error || 'Не удалось сохранить изменения');
        error.status = response.status;
        throw error;
      }
      return data;
    } finally { clearTimeout(timeout); }
  }

  function acceptState(next) {
    state = next;
    let removedDraft = false;
    for (const id of drafts.keys()) {
      if (!itemByID(id)) { drafts.delete(id); removedDraft = true; }
    }
    if (removedDraft) feedback('Редактируемая позиция удалена другим пользователем');
    render();
  }

  async function mutate(path, values, success, done = () => {}) {
    if (busy) return;
    revision++;
    setBusy(true);
    let next;
    try {
      next = await request(path, values);
      done();
      $('inventory-status').textContent = success;
    } catch (error) {
      const message = error instanceof TypeError || error.name === 'AbortError' ? 'Нет связи с сервером. Обновите список перед повторной попыткой: изменение могло сохраниться.' : error.message;
      if (error.status === 409 || error.status === 404) {
        try { next = await request(); } catch (_) { /* Keep the form and local values on a failed refresh. */ }
        if (editor.open && editing) {
          $('inventory-reload-editor').hidden = false;
          editing.conflict = true;
        }
        if (deletion.open) { deletion.close(); deleting = null; }
      }
      feedback(message);
    } finally {
      setBusy(false);
      if (next) acceptState(next);
      if (editor.open) existingHint();
      if (editing?.conflict) $('inventory-submit').disabled = true;
    }
  }

  function existingHint() {
    const match = state.items.find((item) => normalize(item.name) === normalize($('inventory-name').value) && item.unit === $('inventory-unit').value);
    const existing = !editing && match;
    $('inventory-existing').hidden = !existing;
    $('inventory-existing').textContent = existing ? `Уже на объекте: ${quantity(match.quantity)} ${units[match.unit]}. Количество будет добавлено к остатку.` : '';
    $('inventory-note').disabled = busy || !!existing;
    $('inventory-material').disabled = busy || !!existing;
    if (existing) $('inventory-material').value = materialOf(match);
  }

  function fillEditor(item) {
    editor.querySelector('[data-page-toast]')?.remove();
    form.reset();
    editorDirty = false;
    materialDirty = false;
    editing = item ? { ...item } : null;
    $('inventory-reload-editor').hidden = true;
    $('inventory-submit').disabled = false;
    $('inventory-editor-title').textContent = item ? 'Изменить позицию' : 'Добавить позицию';
    $('inventory-submit').textContent = item ? 'Сохранить' : 'Добавить';
    $('inventory-name').value = item?.name || '';
    $('inventory-material').value = item ? materialOf(item) : '';
    $('inventory-quantity').value = item ? quantity(item.quantity) : '1';
    $('inventory-unit').value = item?.unit || 'pcs';
    $('inventory-note').value = item?.note || '';
    existingHint();
  }

  async function openEditor(item, trigger, prefill = '') {
    fillEditor(item);
    if (!item && materialFilter.value) {
      $('inventory-material').value = materialFilter.value.slice('material:'.length);
      materialDirty = true;
    }
    if (prefill) {
      $('inventory-name').value = prefill;
      suggestFields();
    }
    editor.uiReturnFocus = trigger;
    editor.showModal();
    (touchInput() ? $('inventory-editor-title') : prefill ? $('inventory-quantity') : $('inventory-name')).focus();
    const currentRevision = ++revision;
    try {
      const next = await request();
      if (revision !== currentRevision || busy || !editor.open) return;
      acceptState(next);
      existingHint();
    } catch (_) {
      if (revision === currentRevision && editor.open) feedback('Не удалось обновить список. Показаны последние загруженные данные.');
    }
  }

  $('inventory-add').addEventListener('click', (event) => openEditor(null, event.currentTarget));
  $('inventory-search-create').addEventListener('click', event => openEditor(null, event.currentTarget, search.value.trim()));
  search.addEventListener('input', render);
  search.addEventListener('keydown', event => {
    if (event.key === 'Enter' && touchInput()) { event.preventDefault(); search.blur(); }
  });
  $('inventory-search-clear').addEventListener('click', () => {
    search.value = '';
    render();
    search.focus({preventScroll:true});
  });
  for (const select of [materialFilter, stockFilter]) select.addEventListener('change', render);
  $('inventory-filter-reset').addEventListener('click', () => {
    search.value = materialFilter.value = stockFilter.value = '';
    render();
    materialFilter.focus({preventScroll:true});
  });
  $('inventory-collapse').addEventListener('click', () => {
    const sections = [...list.querySelectorAll('.inventory-group')];
    const open = sections.every(section => !section.open);
    sections.forEach(section => { section.open = open; collapsed.set(section.dataset.material, !open); });
    syncCollapse();
  });
  $('inventory-refresh').addEventListener('click', async () => {
    if (busy) return;
    revision++;
    setBusy(true);
    try {
      const next = await request();
      setBusy(false);
      acceptState(next);
      $('inventory-status').textContent = 'Инвентарь обновлён';
    } catch (error) { feedback(error instanceof TypeError || error.name === 'AbortError' ? 'Не удалось обновить инвентарь. Проверьте соединение.' : error.message); }
    finally { setBusy(false); }
  });
  for (const button of root.querySelectorAll('[data-close-dialog]')) button.addEventListener('click', () => button.closest('dialog').close());
  for (const dialog of [editor, deletion]) dialog.addEventListener('cancel', (event) => { if (busy) event.preventDefault(); });
  for (const dialog of [editor, deletion]) dialog.addEventListener('close', () => {
    if (!dialog.uiReturnFocus?.isConnected) $('inventory-add').focus({ preventScroll: true });
  });
  function suggestFields() {
    const matches = state.catalog.filter((entry) => normalize(entry.name) === normalize($('inventory-name').value));
    if (matches.length === 1 && !editing) $('inventory-unit').value = matches[0].unit;
    if (!materialDirty && !editing) $('inventory-material').value = matches.length === 1 ? materialOf(matches[0]) : suggestMaterial($('inventory-name').value);
    existingHint();
  }
  $('inventory-name').addEventListener('input', suggestFields);
  $('inventory-material').addEventListener('input', () => { materialDirty = true; });
  $('inventory-unit').addEventListener('change', existingHint);
  $('inventory-reload-editor').addEventListener('click', () => {
    const item = itemByID(editing?.id);
    if (item) fillEditor(item);
    else { editor.close(); feedback('Позиция уже удалена'); }
  });

  form.addEventListener('submit', (event) => {
    event.preventDefault();
    if (busy || editing?.conflict || !form.reportValidity()) return;
    const values = Object.fromEntries(new FormData(form));
    if (editing) values.version = editing.version;
    const id = editing?.id;
    mutate(id ? `/${encodeURIComponent(id)}/update` : '', values, id ? 'Позиция сохранена' : 'Позиция добавлена', () => {
      editor.close();
      editing = null;
    });
  });
  form.addEventListener('input', () => { editorDirty = true; });
  form.addEventListener('change', () => { editorDirty = true; });

  list.addEventListener('input', (event) => {
    if (!event.target.matches('.inventory-amount')) return;
    const li = event.target.closest('.inventory-row');
    const item = itemByID(li.dataset.id);
    const value = event.target.value;
    if (value === quantity(item.quantity)) drafts.delete(item.id);
    else drafts.set(item.id, { value, version: item.version });
    updateRowControls(li, item);
  });

  function saveQuantity(item) {
    const draft = drafts.get(item.id);
    if (!draft || draft.version !== item.version) return;
    mutate(`/${encodeURIComponent(item.id)}/quantity`, { quantity: draft.value, version: draft.version }, 'Количество сохранено', () => drafts.delete(item.id));
  }

  list.addEventListener('keydown', (event) => {
    if (event.key === 'Enter' && event.target.matches('.inventory-amount')) {
      event.preventDefault();
      saveQuantity(itemByID(event.target.closest('.inventory-row').dataset.id));
    }
    if (event.key === 'Escape' && event.target.matches('.inventory-amount')) {
      drafts.delete(event.target.closest('.inventory-row').dataset.id);
      render();
    }
  });

  list.addEventListener('click', (event) => {
    const button = event.target.closest('button[data-action]');
    if (!button || busy) return;
    const item = itemByID(button.closest('.inventory-row').dataset.id);
    button.closest('.inventory-row-actions')?.removeAttribute('open');
    const path = `/${encodeURIComponent(item.id)}`;
    switch (button.dataset.action) {
      case 'plus': case 'minus':
        mutate(`${path}/adjust`, { delta: button.dataset.action === 'plus' ? 1000 : -1000 }, 'Количество сохранено');
        break;
      case 'save': saveQuantity(item); break;
      case 'reset': drafts.delete(item.id); render(); break;
      case 'edit': openEditor(item, button); break;
      case 'delete':
        deletion.querySelector('[data-page-toast]')?.remove();
        deleting = { ...item };
        $('inventory-delete-name').textContent = `${item.name} · ${quantity(item.quantity)} ${units[item.unit]}`;
        deletion.uiReturnFocus = button;
        deletion.showModal();
        break;
    }
  });
  document.addEventListener('pointerdown', event => {
    list.querySelectorAll('.inventory-row-actions[open]').forEach(menu => { if (!menu.contains(event.target)) menu.open = false; });
  });
  list.addEventListener('keydown', event => {
    const menu = event.target.closest('.inventory-row-actions[open]');
    if (event.key === 'Escape' && menu) { event.preventDefault(); menu.open = false; menu.querySelector('summary').focus(); }
  });
  $('inventory-delete-confirm').addEventListener('click', () => {
    if (!deleting) return;
    mutate(`/${encodeURIComponent(deleting.id)}/delete`, { version: deleting.version }, 'Позиция удалена', () => {
      deletion.close();
      deleting = null;
    });
  });
  window.addEventListener('beforeunload', (event) => {
    if (drafts.size || busy || (editor.open && editorDirty)) { event.preventDefault(); event.returnValue = ''; }
  });
  render();
  window.WorkServiceUI?.prepare(root);
})();
