(() => {
  'use strict';
  document.querySelectorAll('.timesheet-form:not([data-worker-picker-ready])').forEach(form => {
    form.dataset.workerPickerReady = 'true';
    const group = form.querySelector('[data-required-worker-id]');
    const date = form.querySelector('[name=date]');
    if (!group || !date) return;
    let busy = {};
    let target = null;
    let loading = false;
    let requestID = 0;
    let requestController;
    const dialog = document.createElement('dialog');
    const titleID = 'worker-picker-' + Math.random().toString(36).slice(2);
    dialog.className = 'confirmation-dialog worker-picker-dialog';
    dialog.setAttribute('aria-labelledby', titleID);
    dialog.innerHTML = '<div class="modal-content"><button type="button" class="close-button" aria-label="Закрыть">&times;</button><h2 id="'+titleID+'">Выбрать работника</h2><input type="search" class="worker-picker-search" aria-label="Поиск работника" placeholder="Найти работника" autocomplete="off"><p class="worker-picker-status" role="status"></p><ul class="worker-picker-list"></ul></div>';
    form.after(dialog);
    const search = dialog.querySelector('input');
    const status = dialog.querySelector('[role=status]');
    const list = dialog.querySelector('ul');
    dialog.querySelector('button').addEventListener('click', () => dialog.close());
    search.addEventListener('input', renderChoices);
    function compare(a, b) {
      return Number(!!busy[a.value]) - Number(!!busy[b.value]) || a.textContent.localeCompare(b.textContent, 'ru', {sensitivity:'base'});
    }
    function sync() {
      group.querySelectorAll('select').forEach(select => {
        const value = select.value;
        const options = [...select.options].filter(option => option.value).sort(compare);
        const empty = [...select.options].find(option => !option.value);
        select.replaceChildren(...(empty ? [empty] : []), ...options);
        select.value = value;
        select.hidden = true;
        let button = select.parentElement.querySelector('.worker-picker-trigger');
        if (!button) {
          button = document.createElement('button');
          button.type = 'button';
          button.className = 'worker-picker-trigger';
          button.setAttribute('aria-haspopup','dialog');
          select.after(button);
          button.addEventListener('click', () => {
            target = select;
            search.value = '';
            dialog.uiReturnFocus = button;
            dialog.showModal();
            search.focus();
            renderChoices();
            refresh();
          });
        }
        const name = value ? select.selectedOptions[0]?.textContent || 'Работник' : 'Выберите работника';
        button.replaceChildren();
        const label = document.createElement('span');
        label.textContent = name;
        button.append(label);
        if (value && busy[value]) {
          const badge = document.createElement('small');
          badge.textContent = 'Занят';
          button.append(badge);
        }
        button.classList.toggle('is-busy', !!busy[value]);
        button.disabled = select.disabled || (!!value && value === group.dataset.requiredWorkerId);
        button.setAttribute('aria-label', value ? 'Работник: '+name+(busy[value] ? '. Уже есть запись на эту дату' : '') : name);
      });
      if (dialog.open) renderChoices();
    }
    function renderChoices() {
      if (!target?.isConnected) { if (dialog.open) dialog.close(); return; }
      const query = search.value.trim().toLocaleLowerCase('ru');
      const options = [...target.options].filter(option => option.value && option.textContent.toLocaleLowerCase('ru').includes(query)).sort(compare);
      list.replaceChildren(...options.map(option => {
        const li = document.createElement('li');
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'worker-choice' + (busy[option.value] ? ' is-busy' : '');
        button.dataset.workerId = option.value;
        button.disabled = loading;
        const name = document.createElement('span');
        name.textContent = option.textContent;
        button.append(name);
        if (busy[option.value]) {
          const label = document.createElement('small');
          label.textContent = 'Уже есть запись';
          button.append(label);
        }
        button.addEventListener('click', () => {
          target.value = option.value;
          target.dispatchEvent(new Event('change', {bubbles:true}));
          dialog.close();
        });
        li.append(button);
        return li;
      }));
      if (!options.length) { const li = document.createElement('li'); li.textContent = 'Работники не найдены'; list.append(li); }
    }
    async function refresh() {
      const current = ++requestID;
      requestController?.abort();
      requestController = new AbortController();
      const controller = requestController;
      const timer = setTimeout(() => controller.abort(), 15000);
      loading = true;
      status.textContent = 'Проверяем занятость...';
      renderChoices();
      try {
        const params = new URLSearchParams({date:date.value});
        if (form.dataset.entryId) params.set('exclude_id',form.dataset.entryId);
        const response = await fetch('/schedule/availability?'+params, {cache:'no-store',signal:controller.signal,headers:{Accept:'application/json'}});
        if (!response.ok || response.redirected) throw new Error('availability');
        const result = await response.json();
        if (current !== requestID || !form.isConnected) return;
        busy = result.busy;
        status.textContent = '';
      } catch (_) {
        if (current !== requestID) return;
        busy = {};
        status.textContent = 'Занятость не проверена. Проверьте соединение.';
        if (!dialog.open) window.WorkServiceUI?.feedback(status.textContent);
      } finally {
        clearTimeout(timer);
        if (current === requestID) { loading = false; sync(); }
      }
    }
    group.addEventListener('change', sync);
    form.querySelector('[name=entry_kind]')?.addEventListener('change', sync);
    date.addEventListener('change', refresh);
    new MutationObserver(mutations => {
      if (mutations.some(m => m.target === group || m.target.classList?.contains('dynamic-select-row'))) sync();
    }).observe(group, {childList:true,subtree:true});
    window.WorkServiceUI?.prepare(dialog.parentElement);
    sync();
    refresh();
  });
})();
