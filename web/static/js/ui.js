/* Shared interaction behavior; business rules remain on the server. */
(() => {
  'use strict';
  const root = document.documentElement;
  const modal = document.getElementById('app-action-modal');
  const sidebar = document.querySelector('.side-nav');
  const desktop = matchMedia('(min-width: 1024px)');
  const focusable = 'a[href],button:not(:disabled),input:not(:disabled):not([type=hidden]),select:not(:disabled),textarea:not(:disabled),[tabindex="0"]';
  const pending = new WeakMap();
  let fieldSequence = 0;
  let invalidFocusScheduled = false;

  function visible(element) {
    return !!element.getClientRects().length && getComputedStyle(element).visibility !== 'hidden' && !element.closest('[inert],[hidden]');
  }

  function activeLayer() {
    const confirmation = document.querySelector('dialog[open]');
    if (confirmation) return confirmation;
    if (modal?.classList.contains('visible')) return modal;
    if (!desktop.matches && document.body.classList.contains('nav-open')) return sidebar;
    return null;
  }

  function syncLayers() {
    const layer = activeLayer();
    document.querySelectorAll('body > .main-content,body > .top-nav,body > .floating-create-wrap,body > .skip-link,body > .site-toast').forEach(element => {
      element.inert = !!layer;
    });
    if (sidebar) {
      sidebar.inert = layer === modal || (!desktop.matches && layer !== sidebar);
      if (!desktop.matches) sidebar.setAttribute('aria-hidden', layer === sidebar ? 'false' : 'true');
      else sidebar.removeAttribute('aria-hidden');
    }
    if (modal) modal.inert = layer !== modal && !modal.contains(layer);
  }

  function feedback(message, error = true) {
    const host = document.querySelector('dialog[open] .modal-content') || (modal?.classList.contains('visible') ? modal.querySelector('.action-modal-body') : document.body);
    host.querySelector('[data-page-toast]')?.remove();
    const toast = document.createElement('div');
    toast.className = 'site-toast ' + (error ? 'is-error' : 'is-success');
    toast.dataset.pageToast = '';
    toast.setAttribute('role', error ? 'alert' : 'status');
    toast.setAttribute('aria-atomic', 'true');
    const copy = document.createElement('div');
    const title = document.createElement('strong');
    title.textContent = error ? 'Не удалось выполнить действие' : 'Готово';
    const text = document.createElement('p');
    text.textContent = message;
    copy.append(title, text);
    const close = document.createElement('button');
    close.type = 'button';
    close.textContent = '\u00d7';
    close.setAttribute('aria-label', 'Закрыть уведомление');
    close.addEventListener('click', () => toast.remove());
    toast.append(copy, close);
    host.prepend(toast);
  }

  function responseError(response, text) {
    if (response.status === 401) return 'Сессия завершена. Откройте сайт в новой вкладке и войдите, затем вернитесь к заполненной форме.';
    if (response.status === 403) return 'Недостаточно прав или сессия устарела. Проверьте вход в аккаунт. Введённые данные сохранены в форме.';
    const destination = new URL(response.url, location.origin);
    if (response.redirected && destination.pathname === '/login') return 'Сессия завершена. Войдите в аккаунт в новой вкладке, затем вернитесь к заполненной форме.';
    if (destination.searchParams.has('error')) return destination.searchParams.get('error');
    if ((response.headers.get('content-type') || '').includes('text/html')) {
      const doc = new DOMParser().parseFromString(text, 'text/html');
      const detail = doc.querySelector('.form-error,[data-page-toast].is-error p,.alert-danger,.error-message');
      if (detail?.textContent.trim()) return detail.textContent.trim();
    }
    if (!response.ok) {
      if (response.status >= 500 || (response.headers.get('content-type') || '').includes('text/html')) return 'Не удалось сохранить изменения. Данные остались в форме. Повторите попытку позже.';
      return text.trim().slice(0, 600) || 'Сервер не принял изменения. Проверьте данные.';
    }
    return '';
  }

  function rememberSuccess(destination) {
    try { sessionStorage.setItem('workservice-feedback', new URL(destination, location.href).pathname); } catch (_) {}
  }

  function beginSubmission(form, submitter) {
    if (pending.has(form)) return false;
    const buttons = [...form.querySelectorAll('button[type=submit],input[type=submit],button:not([type])')];
    const states = buttons.map(button => [button, button.disabled]);
    pending.set(form, states);
    form.setAttribute('aria-busy', 'true');
    buttons.forEach(button => { button.disabled = true; });
    if (submitter?.matches('button')) {
      submitter.dataset.pendingLabel = submitter.textContent;
      submitter.dataset.pendingAria = submitter.getAttribute('aria-label') || '';
      submitter.setAttribute('aria-label', submitter.textContent + '. Сохранение');
      submitter.classList.add('is-loading');
    }
    return true;
  }

  function endSubmission(form) {
    const states = pending.get(form);
    if (!states) return;
    for (const [button, disabled] of states) {
      button.disabled = disabled;
      if (button.dataset.pendingLabel !== undefined) {
        if (button.dataset.pendingAria) button.setAttribute('aria-label',button.dataset.pendingAria);
        else button.removeAttribute('aria-label');
        delete button.dataset.pendingLabel;
        delete button.dataset.pendingAria;
      }
      button.classList.remove('is-loading');
    }
    form.removeAttribute('aria-busy');
    pending.delete(form);
  }

  function validationMessage(field) {
    const state = field.validity;
    if (state.valid) return '';
    if (state.valueMissing) return field.tagName === 'SELECT' ? 'Выберите значение.' : 'Заполните это поле.';
    if (state.typeMismatch) return field.type === 'email' ? 'Введите адрес электронной почты.' : 'Проверьте формат значения.';
    if (state.rangeUnderflow) return 'Минимальное значение: ' + field.min + '.';
    if (state.rangeOverflow) return 'Максимальное значение: ' + field.max + '.';
    if (state.badInput || state.stepMismatch) return 'Введите допустимое число.';
    if (state.tooShort) return 'Введите не менее ' + field.minLength + ' символов.';
    return field.validationMessage || 'Проверьте значение.';
  }

  function validateField(field) {
    if (!field.willValidate) return;
    const message = validationMessage(field);
    let note = field.parentElement.querySelector('[data-error-for="' + field.dataset.uiField + '"]');
    if (!note && message) {
      note = document.createElement('small');
      note.className = 'field-feedback';
      note.dataset.errorFor = field.dataset.uiField;
      note.id = field.dataset.uiField + '-error';
      field.insertAdjacentElement('afterend', note);
      field.setAttribute('aria-describedby', [field.getAttribute('aria-describedby'), note.id].filter(Boolean).join(' '));
    }
    if (note) {
      note.textContent = message;
      note.hidden = !message;
    }
    if (message) field.setAttribute('aria-invalid', 'true');
    else field.removeAttribute('aria-invalid');
  }

  function prepare(scope = document) {
    scope.querySelectorAll('dialog').forEach(dialog => {
      if (dialog.dataset.uiDialog) return;
      dialog.dataset.uiDialog = 'true';
      new MutationObserver(syncLayers).observe(dialog, {attributes: true, attributeFilter: ['open']});
      dialog.addEventListener('close', () => {
        syncLayers();
        if (dialog.uiReturnFocus?.isConnected) dialog.uiReturnFocus.focus({preventScroll:true});
      });
    });
    scope.querySelectorAll('input:not([type=hidden]),select,textarea').forEach(field => {
      if (field.dataset.uiField) return;
      field.dataset.uiField = 'ui-field-' + (++fieldSequence);
      if (!field.id) field.id = field.dataset.uiField;
      const group = field.closest('.form-group,.form-group-edit');
      const label = group?.querySelector('label');
      if (label && !label.htmlFor && !label.contains(field)) label.htmlFor = field.id;
      if (field.type === 'number' && !field.inputMode) field.inputMode = field.step && field.step !== '1' ? 'decimal' : 'numeric';
      const ownProfile = field.form && new URL(field.form.action, location.href).pathname === '/profile';
      if (field.name === 'phone') { field.type = 'tel'; field.autocomplete = ownProfile ? 'tel' : 'off'; }
      if (field.name === 'username') { field.autocomplete = ownProfile ? 'username' : 'off'; field.autocapitalize = 'none'; field.spellcheck = false; }
      if (field.type === 'password' && !field.autocomplete) field.autocomplete = 'new-password';
    });
    scope.querySelectorAll('.table-scroll').forEach(table => {
      table.tabIndex = 0;
      table.setAttribute('role', 'region');
      table.setAttribute('aria-label', 'Таблица');
    });
  }

  document.addEventListener('invalid', event => {
    const field = event.target;
    if (!field.matches('input,select,textarea')) return;
    event.preventDefault();
    if (!field.dataset.uiField) prepare(field.form || document);
    validateField(field);
    if (!invalidFocusScheduled) {
      invalidFocusScheduled = true;
      requestAnimationFrame(() => {
        const first = field.form?.querySelector('[aria-invalid="true"]');
        first?.focus();
        first?.scrollIntoView({block: 'nearest'});
        invalidFocusScheduled = false;
      });
    }
  }, true);
  document.addEventListener('focusout', event => {
    const field = event.target;
    if (field.matches('input,select,textarea') && field.dataset.uiField && (field.value || field.hasAttribute('aria-invalid'))) validateField(field);
  });
  for (const type of ['input', 'change']) document.addEventListener(type, event => {
    const field = event.target;
    if (field.matches('[aria-invalid="true"]')) validateField(field);
  });

  document.addEventListener('submit', async event => {
    const form = event.target;
    if (event.defaultPrevented || form.method.toLowerCase() !== 'post' || (form.target && form.target !== '_self') || form.hasAttribute('data-native-submit')) return;
    const destination = new URL(form.action, location.href);
    if (destination.origin !== location.origin) return;
    event.preventDefault();
    const data = new FormData(form);
    if (event.submitter?.name) data.append(event.submitter.name, event.submitter.value);
    if (!beginSubmission(form, event.submitter)) return;
    try {
      const response = await fetch(destination, { method: 'POST', body: data, credentials: 'same-origin', cache: 'no-store' });
      const text = await response.text();
      const error = responseError(response, text);
      if (error) { feedback(error); return; }
      if (response.redirected) { rememberSuccess(response.url); location.assign(response.url); return; }
      if (response.ok) { rememberSuccess(location.href); location.reload(); return; }
    } catch (_) {
      feedback('Нет ответа от сервера. Данные остались в форме. Проверьте, сохранилась ли запись, прежде чем отправлять её повторно.');
    } finally { endSubmission(form); }
  });

  document.addEventListener('keydown', event => {
    const layer = activeLayer();
    if (event.key !== 'Tab' || !layer) return;
    const items = [...layer.querySelectorAll(focusable)].filter(visible);
    const first = items[0];
    const last = items[items.length - 1];
    if (!first) { event.preventDefault(); layer.focus(); return; }
    if (!layer.contains(document.activeElement) || (event.shiftKey && document.activeElement === first) || (!event.shiftKey && document.activeElement === last)) {
      event.preventDefault();
      (event.shiftKey ? last : first).focus();
    }
  });

  function measureHeader() {
    const header = document.querySelector('.top-nav');
    if (header) root.style.setProperty('--content-top', Math.ceil(header.getBoundingClientRect().bottom + 20) + 'px');
  }
  const header = document.querySelector('.top-nav');
  if (header && window.ResizeObserver) new ResizeObserver(measureHeader).observe(header);
  window.addEventListener('resize', measureHeader);
  window.addEventListener('pageshow', () => document.querySelectorAll('form[aria-busy]').forEach(endSubmission));
  desktop.addEventListener('change', syncLayers);
  new MutationObserver(syncLayers).observe(document.body, { attributes: true, attributeFilter: ['class'] });
  const main = document.querySelector('body > .main-content');
  if (main) { main.id = 'main-content'; main.tabIndex = -1; main.setAttribute('role', 'main'); }
  window.WorkServiceUI = { prepare, feedback, responseError, beginSubmission, endSubmission, syncLayers, rememberSuccess };
  prepare();
  measureHeader();
  syncLayers();
  try {
    const saved = sessionStorage.getItem('workservice-feedback');
    sessionStorage.removeItem('workservice-feedback');
    if (saved === location.pathname && !document.querySelector('[data-page-toast]')) feedback('Изменения сохранены.', false);
  } catch (_) {}
})();
