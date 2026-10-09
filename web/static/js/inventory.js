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
  const drafts = new Map();
  let state = JSON.parse($('inventory-data').textContent);
  let editing = null;
  let deleting = null;
  let busy = false;
  let revision = 0;
  let editorDirty = false;
  const quantity = (n) => (n / 1000).toLocaleString('ru-RU', { maximumFractionDigits: 3, useGrouping: false });
  const normalize = (s) => s.trim().replace(/\s+/g, ' ').toLocaleLowerCase('ru');
  const itemByID = (id) => state.items.find((item) => item.id === id);
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
    stepper.append(minus, input, plus);
    const save = iconButton('check', `Сохранить количество: ${item.name}`, 'save');
    save.classList.add('inventory-save');
    const reset = iconButton('refresh', `Отменить ввод количества: ${item.name}`, 'reset');
    reset.classList.add('inventory-reset');
    controls.append(stepper, element('span', 'inventory-unit', units[item.unit]), save, reset);
    const actions = element('div', 'inventory-row-actions');
    actions.append(iconButton('pencil', `Изменить: ${item.name}`, 'edit'), iconButton('trash', `Удалить: ${item.name}`, 'delete'));
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
    const filter = normalize($('inventory-search').value);
    const visible = state.items.filter((item) => normalize(`${item.name} ${item.note}`).includes(filter));
    list.replaceChildren(...visible.map(row));
    $('inventory-count').textContent = String(state.items.length);
    const empty = $('inventory-empty');
    empty.hidden = visible.length !== 0;
    empty.textContent = state.items.length ? 'Ничего не найдено' : 'На объекте пока нет инвентаря';
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
    if (focusRow) {
      const currentRow = [...list.children].find((li) => li.dataset.id === focusRow);
      const target = currentRow?.querySelector(`[data-action="${action}"]`);
      if (target && !target.disabled && !target.hidden) target.focus({ preventScroll: true });
      else if (currentRow) currentRow.querySelector('input').focus({ preventScroll: true });
      else $('inventory-add').focus({ preventScroll: true });
    }
  }

  function setBusy(value) {
    busy = value;
    root.setAttribute('aria-busy', String(value));
    for (const id of ['inventory-add', 'inventory-refresh', 'inventory-submit', 'inventory-delete-confirm', 'inventory-reload-editor']) $(id).disabled = value;
    for (const field of form.querySelectorAll('input, select, textarea')) field.disabled = value;
    for (const button of root.querySelectorAll('[data-close-dialog]')) button.disabled = value;
    for (const li of list.children) updateRowControls(li, itemByID(li.dataset.id));
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
  }

  function fillEditor(item) {
    editor.querySelector('[data-page-toast]')?.remove();
    form.reset();
    editorDirty = false;
    editing = item ? { ...item } : null;
    $('inventory-reload-editor').hidden = true;
    $('inventory-submit').disabled = false;
    $('inventory-editor-title').textContent = item ? 'Изменить позицию' : 'Добавить позицию';
    $('inventory-submit').textContent = item ? 'Сохранить' : 'Добавить';
    $('inventory-name').value = item?.name || '';
    $('inventory-quantity').value = item ? quantity(item.quantity) : '1';
    $('inventory-unit').value = item?.unit || 'pcs';
    $('inventory-note').value = item?.note || '';
    existingHint();
  }

  async function openEditor(item, trigger) {
    fillEditor(item);
    editor.uiReturnFocus = trigger;
    editor.showModal();
    $('inventory-name').focus();
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
  $('inventory-search').addEventListener('input', render);
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
  $('inventory-name').addEventListener('input', () => {
    const matches = state.catalog.filter((entry) => normalize(entry.name) === normalize($('inventory-name').value));
    if (matches.length === 1 && !editing) $('inventory-unit').value = matches[0].unit;
    existingHint();
  });
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
