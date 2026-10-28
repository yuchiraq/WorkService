(() => {
  'use strict';
  const root = document.querySelector('.cloud-connections');
  if (!root) return;
  const el = id => document.getElementById(id);
  let revokeID = '';
  const date = value => value && !value.startsWith('0001') ? new Date(value).toLocaleDateString('ru-RU') : 'ещё не подключалось';
  async function api(url, data) {
    const response = await fetch(url, {method: data === undefined ? 'GET' : 'POST', credentials: 'same-origin', cache: 'no-store', headers: {'Content-Type': 'application/json', 'X-CSRF-Token': root.dataset.csrf}, body: data === undefined ? undefined : JSON.stringify(data)});
    if (response.redirected) throw new Error('Сессия завершилась. Обновите страницу.');
    const result = response.status === 204 ? {} : await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(result.error || (response.status === 429 ? 'Слишком много попыток. Подождите минуту.' : 'Не удалось выполнить действие. Повторите попытку.'));
    return result;
  }
  async function load() {
    const data = await api('/cloud/devices');
    el('device-url').value = data.enabled ? data.url : '';
    el('device-username').value = data.username;
    if (data.enabled) {
      const url = new URL(data.url);
      el('device-windows').value = '\\\\' + url.hostname + '@SSL' + (url.port ? '@' + url.port : '') + '\\DavWWWRoot\\dav';
    }
    el('device-new').disabled = !data.secure;
    el('device-status').textContent = !data.enabled ? 'Подключения ещё не включены на сервере.' : !data.secure ? 'Откройте защищённый адрес сервиса для создания доступа.' : 'Защищённое подключение · HTTPS';
    el('device-list').replaceChildren();
    for (const device of data.devices) {
      const li = document.createElement('li');
      const info = document.createElement('div');
      const name = document.createElement('strong');
      name.textContent = device.name;
      const scope = document.createElement('span');
      const expired = new Date(device.expiresAt) <= new Date();
      scope.textContent = (expired ? 'Срок истёк' : device.readOnly ? 'Только чтение' : 'Чтение и изменение') + ' · до ' + date(device.expiresAt);
      const used = document.createElement('span');
      used.textContent = 'Последний доступ: ' + date(device.lastUsedAt);
      info.append(name, scope, used);
      const button = document.createElement('button');
      button.type = 'button'; button.className = 'btn btn-secondary'; button.textContent = 'Отключить';
      button.setAttribute('aria-label', 'Отключить ' + device.name);
      button.addEventListener('click', () => {
        revokeID = device.id;
        el('device-revoke-name').textContent = device.name;
        el('device-revoke-error').textContent = '';
        el('device-revoke-dialog').showModal();
      });
      li.append(info, button); el('device-list').append(li);
    }
    el('device-empty').hidden = data.devices.length !== 0;
  }
  root.querySelectorAll('[data-close]').forEach(button => button.addEventListener('click', () => el(button.dataset.close).close()));
  root.querySelectorAll('[data-copy]').forEach(button => button.addEventListener('click', async () => {
    const field = el(button.dataset.copy);
    const status = button.dataset.copy === 'device-secret' ? el('device-secret-status') : el('device-status');
    try { await navigator.clipboard.writeText(field.value); status.textContent = 'Скопировано'; }
    catch { field.focus(); field.select(); status.textContent = 'Не удалось скопировать автоматически'; }
  }));
  el('device-secret-dialog').addEventListener('close', () => { el('device-secret').value = ''; el('device-secret-status').textContent = ''; });
  el('device-editor').addEventListener('close', () => { el('device-confirm-password').value = ''; });
  window.addEventListener('pagehide', () => { el('device-secret').value = ''; el('device-confirm-password').value = ''; });
  el('device-new').addEventListener('click', () => { el('device-form').reset(); el('device-form-error').textContent = ''; el('device-editor').showModal(); });
  el('device-form').addEventListener('submit', async event => {
    event.preventDefault();
    const submit = event.submitter;
    submit.disabled = true; el('device-form-error').textContent = '';
    try {
      const result = await api('/cloud/devices', {name: el('device-name').value, days: Number(el('device-days').value), readOnly: el('device-readonly').checked, password: el('device-confirm-password').value});
      el('device-editor').close();
      el('device-secret').value = result.password;
      el('device-secret-dialog').showModal();
      await load();
    } catch (error) { el('device-form-error').textContent = error.message; }
    finally { submit.disabled = false; el('device-confirm-password').value = ''; }
  });
  el('device-revoke-confirm').addEventListener('click', async event => {
    event.currentTarget.disabled = true;
    try { await api('/cloud/devices/' + encodeURIComponent(revokeID) + '/revoke', {}); el('device-revoke-dialog').close(); await load(); }
    catch (error) { el('device-revoke-error').textContent = error.message; }
    finally { el('device-revoke-confirm').disabled = false; }
  });
  load().catch(error => { el('device-status').textContent = error.message; });
})();
