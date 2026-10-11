(() => {
  'use strict';
  const dial = document.querySelector('[data-quick-actions]');
  if (dial) {
    const trigger = dial.querySelector('#quick-create-toggle');
    const menu = dial.querySelector('#quick-create-menu');
    const items = [...menu.querySelectorAll('[role=menuitem]')];
    const picker = document.getElementById('quick-vehicle-picker');
    dial.querySelector('[data-quick-fallback]').hidden = true;
    trigger.hidden = false;

    function close(restore = false) {
      menu.hidden = true;
      trigger.setAttribute('aria-expanded', 'false');
      trigger.setAttribute('aria-label', 'Добавить');
      trigger.title = 'Добавить';
      if (restore) trigger.focus({preventScroll: true});
    }
    function open(index = 0) {
      menu.hidden = false;
      trigger.setAttribute('aria-expanded', 'true');
      trigger.setAttribute('aria-label', 'Закрыть быстрые действия');
      trigger.title = 'Закрыть быстрые действия';
      items[index].focus({preventScroll: true});
    }
    trigger.addEventListener('click', () => menu.hidden ? open() : close(true));
    trigger.addEventListener('keydown', event => {
      if (!['ArrowUp', 'ArrowDown'].includes(event.key)) return;
      event.preventDefault();
      open(event.key === 'ArrowUp' ? items.length - 1 : 0);
    });
    dial.addEventListener('keydown', event => {
      if (menu.hidden) return;
      if (event.key === 'Escape') {
        event.preventDefault();
        event.stopPropagation();
        close(true);
      } else if (event.key === 'Tab') {
        close(true);
      } else if (['ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) {
        event.preventDefault();
        const index = items.indexOf(document.activeElement);
        const next = event.key === 'Home' ? 0 : event.key === 'End' ? items.length - 1 : (index + (event.key === 'ArrowUp' ? -1 : 1) + items.length) % items.length;
        items[next].focus();
      } else if (event.key === ' ' && event.target.matches('a[role=menuitem]')) {
        event.preventDefault();
        event.target.click();
      }
    });
    menu.addEventListener('click', event => {
      const action = event.target.closest('[role=menuitem]');
      if (!action) return;
      close(true);
      if (action.hasAttribute('data-quick-vehicles') && picker) {
        picker.uiReturnFocus = trigger;
        picker.showModal();
        picker.querySelector('.quick-vehicle-choice').focus();
      } else if (action.hasAttribute('data-quick-upload')) {
        document.getElementById('cloud-floating-upload').click();
      }
    });
    document.addEventListener('pointerdown', event => {
      if (!menu.hidden && !dial.contains(event.target)) close(menu.contains(document.activeElement));
    });
    dial.addEventListener('focusout', () => {
      queueMicrotask(() => { if (!dial.contains(document.activeElement)) close(); });
    });
    window.addEventListener('pagehide', () => close());
    if (picker) {
      picker.querySelector('[data-quick-close]').addEventListener('click', () => picker.close());
      picker.addEventListener('click', event => {
        if (event.target !== picker) return;
        const rect = picker.getBoundingClientRect();
        if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) picker.close();
      });
    }
  }

  const record = document.getElementById('vehicle-record-form');
  const showRecord = () => {
    if (record && location.hash === '#vehicle-record-form') {
      record.scrollIntoView({block: 'start'});
      record.focus({preventScroll: true});
    }
  };
  window.addEventListener('hashchange', showRecord);
  if (record) requestAnimationFrame(showRecord);
})();
