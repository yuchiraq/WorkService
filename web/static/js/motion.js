/* Animata-inspired motion, adapted to server-rendered pages without React. */
(() => {
  'use strict';
  const root = document.documentElement;
  const reduced = matchMedia('(prefers-reduced-motion: reduce)');
  const systemTheme = matchMedia('(prefers-color-scheme: dark)');
  const read = key => { try { return localStorage.getItem(key); } catch { return null; } };
  const write = (key, value) => { try { localStorage.setItem(key, value); } catch {} };
  const motionOff = () => root.dataset.motion === 'off';

  function syncPreferences() {
    const value = read('workservice-theme');
    const theme = ['light', 'dark'].includes(value) ? value : 'system';
    if (theme === 'system') root.removeAttribute('data-theme');
    else root.dataset.theme = theme;
    root.dataset.motion = reduced.matches || read('workservice-motion') === 'off' ? 'off' : 'on';
    document.querySelectorAll('[name="appearance-theme"]').forEach(input => { input.checked = input.value === theme; });
    const motion = document.getElementById('appearance-motion');
    if (motion) {
      motion.checked = !motionOff();
      motion.disabled = reduced.matches;
      motion.title = reduced.matches ? 'Отключено настройками устройства' : '';
    }
    const meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.content = getComputedStyle(root).getPropertyValue('--theme-color').trim();
  }
  syncPreferences();
  reduced.addEventListener('change', syncPreferences);
  systemTheme.addEventListener('change', syncPreferences);
  window.addEventListener('storage', event => {
    if (!event.key || ['workservice-theme', 'workservice-motion'].includes(event.key)) syncPreferences();
  });
  document.addEventListener('change', event => {
    if (event.target.matches('[name="appearance-theme"]')) write('workservice-theme', event.target.value);
    else if (event.target.id === 'appearance-motion') write('workservice-motion', event.target.checked ? 'on' : 'off');
    else return;
    syncPreferences();
  });
  const syncVisibility = () => { root.toggleAttribute('data-page-hidden', document.hidden); };
  document.addEventListener('visibilitychange', syncVisibility);
  syncVisibility();

  const rippleSelector = 'button.btn,a.btn,button.cl-button,button.photo-button,button.mobile-nav-toggle,.floating-create-btn';
  function ripple(event, keyboard = false) {
    if (motionOff() || (event.button !== undefined && event.button !== 0)) return;
    const control = event.target.closest(rippleSelector);
    if (!control || control.matches(':disabled,[aria-disabled="true"]') || control.closest('[inert]')) return;
    const rect = control.getBoundingClientRect();
    if (!rect.width || !rect.height) return;
    control.classList.add('ui-ripple-host');
    control.querySelector('.ui-ripple')?.remove();
    const wave = document.createElement('span');
    wave.className = 'ui-ripple';
    wave.setAttribute('aria-hidden', 'true');
    const diameter = Math.hypot(rect.width, rect.height) * 2;
    wave.style.width = wave.style.height = diameter + 'px';
    wave.style.left = (keyboard ? rect.width / 2 : event.clientX - rect.left) - diameter / 2 + 'px';
    wave.style.top = (keyboard ? rect.height / 2 : event.clientY - rect.top) - diameter / 2 + 'px';
    control.append(wave);
    wave.addEventListener('animationend', () => wave.remove(), {once: true});
    setTimeout(() => wave.remove(), 650);
  }
  document.addEventListener('pointerdown', ripple);
  document.addEventListener('keydown', event => {
    if (!event.repeat && ['Enter', ' '].includes(event.key)) ripple(event, true);
  });

  function fluidIndicator(group) {
    const indicator = document.createElement('span');
    indicator.className = 'ui-tab-indicator';
    indicator.setAttribute('aria-hidden', 'true');
    group.classList.add('has-fluid-indicator');
    group.append(indicator);
    let frame = 0;
    const update = () => {
      frame = 0;
      const active = group.querySelector(':scope > [aria-selected="true"],:scope > [aria-current]:not([aria-current="false"]),:scope > .active');
      if (!active) { indicator.hidden = true; return; }
      indicator.hidden = false;
      const box = group.getBoundingClientRect(), target = active.getBoundingClientRect();
      indicator.style.width = target.width + 'px';
      indicator.style.height = target.height + 'px';
      indicator.style.transform = `translate(${target.left-box.left-group.clientLeft+group.scrollLeft}px,${target.top-box.top-group.clientTop+group.scrollTop}px)`;
      if (!group.classList.contains('fluid-ready')) requestAnimationFrame(() => group.classList.add('fluid-ready'));
    };
    const schedule = () => { if (!frame) frame = requestAnimationFrame(update); };
    const observer = new MutationObserver(schedule);
    observer.observe(group, {attributes: true, attributeFilter: ['aria-current', 'aria-selected', 'class'], subtree: true, childList: true, characterData: true});
    if (window.ResizeObserver) {
      const resize = new ResizeObserver(schedule);
      resize.observe(group);
      group.querySelectorAll(':scope > button,:scope > a').forEach(item => resize.observe(item));
    }
    document.fonts?.ready.then(schedule);
    update();
  }

  const plainClick = event => event.button === 0 && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey;

  function objectTabs(group) {
    const tabs = [...group.querySelectorAll('a[href^="#"]')];
    const panels = tabs.map(tab => document.getElementById(tab.hash.slice(1)));
    if (panels.some(panel => !panel)) return;
    group.setAttribute('role', 'tablist');
    tabs.forEach((tab, index) => {
      tab.id = 'tab-' + panels[index].id;
      tab.setAttribute('role', 'tab');
      tab.setAttribute('aria-controls', panels[index].id);
      panels[index].setAttribute('role', 'tabpanel');
      panels[index].setAttribute('aria-labelledby', tab.id);
      panels[index].tabIndex = 0;
      panels[index].classList.add('object-tab-panel');
    });
    const select = (index, push = false) => {
      // A history change must not hide an open editor and its unsaved fields.
      if (panels.some(panel => panel.querySelector('dialog[open]'))) return;
      tabs.forEach((tab, i) => {
        const selected = i === index;
        tab.setAttribute('aria-selected', String(selected));
        tab.classList.toggle('active', selected);
        tab.tabIndex = selected ? 0 : -1;
        panels[i].hidden = !selected;
      });
      if (push && location.hash !== tabs[index].hash) history.pushState(null, '', tabs[index].hash);
    };
    const fromLocation = () => {
      const index = tabs.findIndex(tab => tab.hash === location.hash);
      select(index < 0 ? 0 : index);
    };
    group.addEventListener('click', event => {
      const index = tabs.indexOf(event.target.closest('a'));
      if (index < 0 || !plainClick(event)) return;
      event.preventDefault();
      select(index, true);
    });
    group.addEventListener('keydown', event => {
      const index = tabs.indexOf(event.target);
      if (index < 0) return;
      let next;
      if (event.key === 'ArrowRight') next = (index + 1) % tabs.length;
      else if (event.key === 'ArrowLeft') next = (index - 1 + tabs.length) % tabs.length;
      else if (event.key === 'Home') next = 0;
      else if (event.key === 'End') next = tabs.length - 1;
      else if (event.key === ' ') { event.preventDefault(); select(index, true); return; }
      else return;
      event.preventDefault();
      tabs.forEach((tab, i) => { tab.tabIndex = i === next ? 0 : -1; });
      tabs[next].focus();
    });
    window.addEventListener('hashchange', fromLocation);
    window.addEventListener('popstate', fromLocation);
    panels.forEach(panel => panel.querySelectorAll('dialog').forEach(dialog => dialog.addEventListener('close', fromLocation)));
    fromLocation();
  }

  // Only list fragments opt in; forms and other page scripts keep their normal lifecycle.
  function listTabs(group) {
    const scope = group.closest('[data-fluid-page]');
    const results = scope.querySelector('[data-fluid-results]');
    const tabs = [...group.querySelectorAll('a')];
    let committed = location.href, controller;
    const status = document.createElement('span');
    status.className = 'sr-only';
    status.setAttribute('role', 'status');
    group.after(status);
    function sync(target) {
      const url = new URL(target);
      const selected = url.searchParams.get('tab') || 'active';
      tabs.forEach(tab => {
        const name = new URL(tab.href).searchParams.get('tab');
        const destination = new URL(url);
        destination.searchParams.set('tab', name);
        tab.href = destination.pathname + destination.search;
        tab.classList.toggle('active', name === selected);
        if (name === selected) tab.setAttribute('aria-current', 'page');
        else tab.removeAttribute('aria-current');
      });
    }
    async function navigate(target, push = true) {
      controller?.abort();
      const request = new AbortController();
      controller = request;
      sync(target);
      results.setAttribute('aria-busy', 'true');
      results.inert = true;
      status.textContent = 'Загрузка списка';
      const timeout = setTimeout(() => request.abort('timeout'), 15000);
      try {
        const response = await fetch(target, {signal: request.signal, cache: 'no-store', credentials: 'same-origin'});
        const doc = new DOMParser().parseFromString(await response.text(), 'text/html');
        if (controller !== request) return;
        if (new URL(response.url).pathname === '/login') { location.assign(response.url); return; }
        const content = doc.querySelector('[data-fluid-results]');
        if (!response.ok || !content) throw new Error('Не удалось загрузить список. Повторите попытку.');
        results.replaceChildren(...content.childNodes);
        window.WorkServiceUI?.prepare(results);
        committed = target;
        if (push) history.pushState(null, '', target);
        document.querySelectorAll('[data-modal-return]').forEach(link => {
          const updated = new URL(link.dataset.modalReturn, location.origin);
          if (updated.pathname === location.pathname) link.dataset.modalReturn = location.pathname + location.search;
        });
        status.textContent = 'Список обновлён: ' + group.querySelector('[aria-current]').textContent;
      } catch (error) {
        if (controller !== request) return;
        sync(committed);
        if (!push) history.replaceState(null, '', committed);
        status.textContent = 'Не удалось загрузить список';
        window.WorkServiceUI?.feedback('Не удалось загрузить список. Проверьте соединение и повторите попытку.');
      } finally {
        clearTimeout(timeout);
        if (controller === request) {
          results.removeAttribute('aria-busy');
          results.inert = false;
          controller = null;
        }
      }
    }
    group.addEventListener('click', event => {
      const tab = event.target.closest('a');
      if (!tabs.includes(tab) || !plainClick(event)) return;
      event.preventDefault();
      if (tab.href !== committed || controller) navigate(tab.href);
    });
    group.addEventListener('keydown', event => {
      const index = tabs.indexOf(event.target);
      if (index < 0 || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
      event.preventDefault();
      const next = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : (index + (event.key === 'ArrowLeft' ? -1 : 1) + tabs.length) % tabs.length;
      tabs[next].focus();
    });
    window.addEventListener('popstate', () => { if (location.href !== committed) navigate(location.href, false); });
    window.addEventListener('pagehide', () => { controller?.abort(); });
    sync(committed);
  }

  document.querySelectorAll('[data-object-tabs]').forEach(objectTabs);
  document.querySelectorAll('[data-fluid-navigation]').forEach(listTabs);
  document.querySelectorAll('.cloud-tabs,.tab-switcher').forEach(fluidIndicator);
})();
