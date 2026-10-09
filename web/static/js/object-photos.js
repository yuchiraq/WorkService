(() => {
  'use strict';
  const root = document.getElementById('object-photos');
  if (!root) return;
  const $ = (id) => document.getElementById(id);
  const viewer = $('photo-viewer');
  const feedback = (message, error = true) => window.WorkServiceUI.feedback(message, error);
  let photos = JSON.parse($('photos-data').textContent).photos;
  let visible = 12, current = null, uploading = false, saving = false, stopped = false, failed = [];
  const imageURL = (photo, suffix = '') => `${root.dataset.endpoint}/${encodeURIComponent(photo.id)}/image${suffix}`;
  const selected = () => photos.find((photo) => photo.id === current);
  const el = (tag, text, className) => {
    const node = document.createElement(tag);
    if (text !== undefined) node.textContent = text;
    if (className) node.className = className;
    return node;
  };
  const date = (value) => new Date(value).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short', year: 'numeric' });

  function render() {
    $('photos-count').textContent = photos.length || '';
    $('photos-grid').replaceChildren(...photos.slice(0, visible).map((photo) => {
      const button = el('button', undefined, 'photo-tile');
      button.type = 'button'; button.dataset.id = photo.id;
      button.setAttribute('aria-label', `Открыть фото: ${photo.caption || photo.name}`);
      const img = el('img'); img.src = imageURL(photo, '?thumb=1'); img.alt = ''; img.loading = 'lazy'; img.width = 480; img.height = 360;
      const label = el('span', photo.caption || date(photo.createdAt));
      button.append(img, label);
      button.addEventListener('click', () => {
        current = photo.id; showPhoto(); viewer.uiReturnFocus = button; viewer.showModal();
      });
      return button;
    }));
    $('photos-more').hidden = photos.length <= visible;
    if (!uploading) $('photos-status').textContent = photos.length ? '' : 'Фото пока нет';
  }

  function showPhoto() {
    const photo = selected();
    if (!photo) { viewer.close(); return; }
    const index = photos.indexOf(photo);
    $('photo-image').hidden = true;
    $('photo-image-status').hidden = false;
    $('photo-image-status').textContent = 'Загрузка фото...';
    $('photo-image').src = imageURL(photo);
    $('photo-image').alt = photo.caption || photo.name;
    $('photo-position').textContent = `${index + 1} / ${photos.length}`;
    $('photo-prev').disabled = index === 0;
    $('photo-next').disabled = index === photos.length - 1;
    $('photo-download').href = imageURL(photo, '?download=1');
    $('photo-meta').textContent = `${photo.createdByName} · ${date(photo.createdAt)} · ${photo.width} × ${photo.height}`;
    $('photo-caption').value = photo.caption;
    $('photo-caption-form').hidden = !photo.canEdit;
    $('photo-caption-text').textContent = photo.caption;
    $('photo-caption-text').hidden = photo.canEdit || !photo.caption;
    $('photo-delete-confirmation').hidden = true;
    $('photo-reload').hidden = true;
  }

  async function responseJSON(response) {
    if (response.redirected || response.status === 401) throw new Error('Сессия завершилась. Войдите в аккаунт снова');
    let result;
    try { result = await response.json(); } catch { throw new Error('Сервер не ответил. Повторите попытку'); }
    if (!response.ok) { const error = new Error(result.error || 'Операция не выполнена'); error.status = response.status; throw error; }
    return result;
  }

  async function refresh() {
    if (uploading || saving || viewer.open) return;
    try {
      const result = await responseJSON(await fetch(root.dataset.endpoint, { cache: 'no-store' }));
      if (!uploading && !saving && !viewer.open) { photos = result.photos; render(); }
    } catch (error) { feedback(error.message); }
  }

  function uploadFile(file, index, total) {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open('POST', root.dataset.endpoint);
      xhr.setRequestHeader('X-CSRF-Token', root.dataset.csrf);
      xhr.timeout = 180000;
      xhr.upload.onprogress = (event) => {
        if (event.lengthComputable) $('photos-progress').value = (index + event.loaded / event.total) * 100 / total;
      };
      xhr.onerror = () => reject(new Error('Нет соединения. Фото можно отправить повторно'));
      xhr.ontimeout = () => reject(new Error('Время загрузки истекло. Повторите попытку'));
      xhr.onload = () => {
        try {
          const result = JSON.parse(xhr.responseText);
          if (xhr.status < 200 || xhr.status >= 300) throw new Error(result.error || 'Не удалось загрузить фото');
          resolve(result);
        } catch (error) { reject(new Error(error instanceof SyntaxError ? 'Сессия завершилась или сервер недоступен' : error.message)); }
      };
      const data = new FormData(); data.append('photo', file); xhr.send(data);
    });
  }

  async function upload(files) {
    if (uploading || !files.length) return;
    if (files.length > 30) { feedback('Выберите не более 30 фото за один раз'); return; }
    uploading = true; stopped = false; failed = [];
    $('photos-errors').replaceChildren(); $('photos-errors').hidden = true;
    $('photos-retry').hidden = true; $('photos-add').disabled = true; $('photos-upload').hidden = false;
    $('photos-cancel').disabled = false; $('photos-progress').value = 0;
    let added = 0, duplicates = 0;
    try {
      for (let i = 0; i < files.length; i++) {
        if (stopped) { failed.push(...files.slice(i)); break; }
        const file = files[i];
        $('photos-status').textContent = `Загрузка ${i + 1} из ${files.length}`;
        try {
          if (file.size > 24 * 1024 * 1024) throw new Error('Файл превышает 24 МБ');
          const result = await uploadFile(file, i, files.length);
          photos = result.photos; render();
          if (result.duplicate) duplicates++; else added++;
        } catch (error) {
          failed.push(file);
          $('photos-errors').append(el('li', `${file.name}: ${error.message}`));
          $('photos-errors').hidden = false;
        }
      }
    } finally {
      uploading = false; $('photos-add').disabled = false; $('photos-upload').hidden = true;
      $('photos-retry').hidden = !failed.length; $('photos-files').value = '';
      $('photos-status').textContent = `Добавлено: ${added}${duplicates ? ` · Уже есть: ${duplicates}` : ''}${failed.length ? ` · Не загружено: ${failed.length}` : ''}`;
      if (added) feedback(`Добавлено фото: ${added}`, false);
    }
  }

  function dirty() { return viewer.open && selected() && $('photo-caption').value !== selected().caption; }
  function move(direction) {
    if (saving || !selected() || dirty()) { if (dirty()) feedback('Сохраните подпись перед переходом к другому фото'); return; }
    const photo = photos[photos.indexOf(selected()) + direction];
    if (photo) { current = photo.id; showPhoto(); }
  }

  async function mutate(action) {
    if (saving) return;
    const photo = selected(); if (!photo) return;
    saving = true;
    const controls = [...viewer.querySelectorAll('button,textarea')]; controls.forEach((control) => { control.disabled = true; });
    try {
      const result = await responseJSON(await fetch(`${root.dataset.endpoint}/${photo.id}/${action}`, {
        method: 'POST', headers: { 'X-CSRF-Token': root.dataset.csrf },
        body: new URLSearchParams({ version: photo.version, caption: $('photo-caption').value })
      }));
      photos = result.photos; render();
      if (action === 'delete') { viewer.close(); feedback('Фото удалено', false); }
      else { showPhoto(); feedback('Подпись сохранена', false); }
    } catch (error) {
      if (error.status === 409 || error.status === 404) $('photo-reload').hidden = false;
      feedback(error.message);
    }
    finally { saving = false; controls.forEach((control) => { control.disabled = false; });
      if (selected()) { const i = photos.indexOf(selected()); $('photo-prev').disabled = i === 0; $('photo-next').disabled = i === photos.length - 1; }
    }
  }

  $('photos-add').addEventListener('click', () => $('photos-files').click());
  $('photos-files').addEventListener('change', () => upload(Array.from($('photos-files').files)));
  $('photos-retry').addEventListener('click', () => upload([...failed]));
  $('photos-cancel').addEventListener('click', () => { stopped = true; $('photos-cancel').disabled = true; $('photos-status').textContent = 'Остановка после текущего фото'; });
  $('photos-more').addEventListener('click', () => { visible += 12; render(); });
  $('photo-prev').addEventListener('click', () => move(-1));
  $('photo-next').addEventListener('click', () => move(1));
  $('photo-image').addEventListener('load', () => { $('photo-image').hidden = false; $('photo-image-status').hidden = true; });
  $('photo-image').addEventListener('error', () => { $('photo-image').hidden = true; $('photo-image-status').hidden = false; $('photo-image-status').textContent = 'Фото недоступно. Проверьте соединение и откройте его снова'; });
  $('photo-reload').addEventListener('click', async () => {
    if (saving) return;
    saving = true; $('photo-reload').disabled = true;
    try {
      const result = await responseJSON(await fetch(root.dataset.endpoint, {cache: 'no-store'}));
      photos = result.photos; render(); showPhoto();
    } catch (error) { feedback(error.message); }
    finally { saving = false; $('photo-reload').disabled = false; }
  });
  $('photo-caption-form').addEventListener('submit', (event) => { event.preventDefault(); mutate('caption'); });
  $('photo-delete').addEventListener('click', () => { $('photo-delete-confirmation').hidden = false; $('photo-delete-cancel').focus(); });
  $('photo-delete-cancel').addEventListener('click', () => { $('photo-delete-confirmation').hidden = true; $('photo-delete').focus(); });
  $('photo-delete-confirm').addEventListener('click', () => mutate('delete'));
  viewer.querySelector('[data-close-dialog]').addEventListener('click', () => viewer.close());
  viewer.addEventListener('cancel', (event) => { if (saving) event.preventDefault(); });
  viewer.addEventListener('close', () => { if (!viewer.uiReturnFocus?.isConnected) $('photos-add').focus({ preventScroll: true }); refresh(); });
  viewer.addEventListener('keydown', (event) => {
    if (event.target.matches('input,textarea')) return;
    if (event.key === 'ArrowLeft') { event.preventDefault(); move(-1); }
    if (event.key === 'ArrowRight') { event.preventDefault(); move(1); }
  });
  window.addEventListener('focus', refresh);
  window.addEventListener('beforeunload', (event) => { if (uploading || dirty()) { event.preventDefault(); event.returnValue = ''; } });
  render();
})();
