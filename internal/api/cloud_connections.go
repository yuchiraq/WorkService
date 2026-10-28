package api

import (
	"bytes"
	"html/template"

	"github.com/gin-gonic/gin"
)

func CloudConnectionsPage(c *gin.Context) {
	var output bytes.Buffer
	if err := cloudConnectionsTemplate.Execute(&output, struct {
		Sidebar template.HTML
		CSRF    string
	}{template.HTML(RenderSidebar(c, "cloud")), c.GetString("csrfToken")}); err != nil {
		cloudError(c, err)
		return
	}
	c.Data(200, "text/html; charset=utf-8", output.Bytes())
}

var cloudConnectionsTemplate = template.Must(template.New("cloud-connections").Parse(`<!DOCTYPE html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover"><title>Подключения к облаку</title><link rel="stylesheet" href="/static/css/style.css?v=20"><link rel="stylesheet" href="/static/css/cloud.css?v=6"><link rel="stylesheet" href="/static/css/cloud-connections.css?v=1"></head><body>
{{.Sidebar}}
<main class="main-content cloud-page cloud-connections" id="main-content" data-csrf="{{.CSRF}}">
<header class="cloud-header"><div><a href="/cloud">Моё облако</a><h1>Подключения</h1></div><button type="button" class="btn btn-primary" id="device-new" disabled><span class="cl-icon cl-plus" aria-hidden="true"></span>Добавить устройство</button></header>
<p id="device-status" role="status" aria-live="polite">Загрузка…</p>
<section class="device-addresses" aria-label="Адреса подключения">
<label>Сервер WebDAV<div class="device-copy"><input id="device-url" readonly type="url"><button class="cl-button" type="button" data-copy="device-url" aria-label="Копировать адрес сервера" title="Копировать адрес сервера"><span class="cl-icon cl-copy" aria-hidden="true"></span></button></div></label>
<label>Логин<div class="device-copy"><input id="device-username" readonly><button class="cl-button" type="button" data-copy="device-username" aria-label="Копировать логин" title="Копировать логин"><span class="cl-icon cl-copy" aria-hidden="true"></span></button></div></label>
<label>Сетевой диск Windows<div class="device-copy"><input id="device-windows" readonly><button class="cl-button" type="button" data-copy="device-windows" aria-label="Копировать путь Windows" title="Копировать путь Windows"><span class="cl-icon cl-copy" aria-hidden="true"></span></button></div></label>
</section>
<section aria-labelledby="device-heading"><h2 id="device-heading">Мои устройства</h2><ul id="device-list" class="device-list"></ul><p id="device-empty" hidden>Нет подключённых устройств</p></section>
<dialog id="device-editor" class="confirmation-dialog cloud-dialog" aria-labelledby="device-title"><div class="modal-content"><header class="cloud-dialog-heading"><h2 id="device-title">Новое устройство</h2><button class="cl-button" type="button" data-close="device-editor" aria-label="Закрыть" title="Закрыть"><span class="cl-icon cl-x" aria-hidden="true"></span></button></header>
<form id="device-form" data-native-submit>
<label for="device-name">Название</label><input id="device-name" maxlength="80" placeholder="Например, мой iPhone" required autocomplete="off">
<label for="device-days">Срок доступа</label><select id="device-days"><option value="30">30 дней</option><option value="90" selected>90 дней</option><option value="365">1 год</option></select>
<label class="device-check"><input id="device-readonly" type="checkbox" checked>Только просмотр и скачивание</label>
<label for="device-confirm-password">Текущий пароль аккаунта</label><input id="device-confirm-password" type="password" required autocomplete="current-password">
<p id="device-form-error" role="alert"></p><div class="cloud-dialog-actions"><button type="button" class="btn btn-secondary" data-close="device-editor">Отмена</button><button type="submit" class="btn btn-primary">Создать доступ</button></div></form>
</div></dialog>
<dialog id="device-secret-dialog" class="confirmation-dialog cloud-dialog" aria-labelledby="device-secret-title"><div class="modal-content"><header class="cloud-dialog-heading"><h2 id="device-secret-title">Пароль устройства</h2><button type="button" class="cl-button" data-close="device-secret-dialog" aria-label="Закрыть" title="Закрыть"><span class="cl-icon cl-x" aria-hidden="true"></span></button></header>
<p>Пароль показан только сейчас. Он даёт доступ к вашему облаку. Не передавайте его другим людям.</p><label for="device-secret">Пароль</label><div class="device-copy"><input id="device-secret" readonly autocomplete="off" spellcheck="false"><button type="button" class="cl-button" data-copy="device-secret" aria-label="Копировать пароль" title="Копировать пароль"><span class="cl-icon cl-copy" aria-hidden="true"></span></button></div><p id="device-secret-status" role="status"></p><button type="button" class="btn btn-primary" data-close="device-secret-dialog">Готово</button></div></dialog>
<dialog id="device-revoke-dialog" class="confirmation-dialog cloud-dialog" aria-labelledby="device-revoke-title"><div class="modal-content"><h2 id="device-revoke-title">Отключить устройство?</h2><p id="device-revoke-name"></p><p>Новые обращения будут запрещены. Уже скачанные копии останутся на устройстве.</p><p id="device-revoke-error" role="alert"></p><div class="cloud-dialog-actions"><button type="button" class="btn btn-secondary" data-close="device-revoke-dialog">Отмена</button><button type="button" class="btn btn-danger" id="device-revoke-confirm">Отключить</button></div></div></dialog>
</main><script src="/static/js/cloud-connections.js?v=1" defer></script></body></html>`))
