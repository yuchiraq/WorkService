package api

import (
	"bytes"
	"encoding/json"
	"html/template"

	"github.com/gin-gonic/gin"
	"project/internal/storage"
)

func CloudPage(c *gin.Context) {
	data, err := json.Marshal(storage.CloudState(c.GetString("userID")))
	if err != nil {
		cloudError(c, err)
		return
	}
	var output bytes.Buffer
	err = cloudTemplate.Execute(&output, struct {
		Sidebar template.HTML
		CSRF    string
		Data    template.JS
	}{template.HTML(RenderSidebar(c, "cloud")), c.GetString("csrfToken"), template.JS(data)})
	if err != nil {
		cloudError(c, err)
		return
	}
	c.Data(200, "text/html; charset=utf-8", output.Bytes())
}

var cloudTemplate = template.Must(template.New("cloud").Parse(`<!DOCTYPE html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover"><title>Моё облако</title><link rel="stylesheet" href="/static/css/style.css?v=20"><link rel="stylesheet" href="/static/css/cloud.css?v=2"></head><body>
{{.Sidebar}}
<main class="main-content cloud-page" id="main-content" data-csrf="{{.CSRF}}">
 <header class="cloud-header"><div><h1>Моё облако</h1><div class="cloud-capacity"><progress id="cloud-quota" value="0" max="1" aria-label="Использованное место"></progress><span id="cloud-usage"></span></div></div><div class="cloud-header-actions"><button type="button" class="btn btn-secondary" id="cloud-new-folder"><span class="cl-icon cl-folder-plus" aria-hidden="true"></span>Папка</button><button type="button" class="btn btn-primary" id="cloud-upload"><span class="cl-icon cl-upload" aria-hidden="true"></span>Загрузить</button></div></header>
 <input type="file" id="cloud-files" multiple hidden aria-label="Файлы для загрузки">
 <nav class="cloud-tabs" aria-label="Разделы облака"><button type="button" data-view="files" aria-current="page">Файлы</button><button type="button" data-view="recent">Недавние</button><button type="button" data-view="starred">Избранное</button><button type="button" data-view="trash">Корзина <span id="cloud-trash-count"></span></button></nav>
 <div class="cloud-toolbar"><label class="cloud-search"><span class="cl-icon cl-search" aria-hidden="true"></span><input type="search" id="cloud-search" placeholder="Поиск по всему облаку" aria-label="Поиск файлов" autocomplete="off"></label><label class="cloud-sort"><span class="sr-only">Сортировка</span><select id="cloud-sort" aria-label="Сортировка"><option value="name">По имени</option><option value="updated">По дате</option><option value="size">По размеру</option><option value="kind">По типу</option></select></label><button type="button" id="cloud-refresh" class="cl-button" aria-label="Обновить" title="Обновить"><span class="cl-icon cl-refresh" aria-hidden="true"></span></button></div>
 <nav class="cloud-breadcrumbs" id="cloud-breadcrumbs" aria-label="Путь к папке"></nav>
 <section id="cloud-queue" class="cloud-queue" aria-label="Загрузки" hidden><div class="cloud-queue-heading"><h2>Загрузки</h2><div><button type="button" class="btn btn-secondary" id="cloud-retry" hidden>Повторить ошибки</button><button type="button" class="cl-button" id="cloud-stop" title="Остановить загрузки" aria-label="Остановить загрузки"><span class="cl-icon cl-x" aria-hidden="true"></span></button></div></div><ul id="cloud-upload-list"></ul></section>
 <div class="cloud-selection" id="cloud-selection" hidden><strong id="cloud-selected-count"></strong><div class="cloud-selection-actions"><button type="button" class="cl-button" data-bulk="download" title="Скачать выбранное" aria-label="Скачать выбранное"><span class="cl-icon cl-download" aria-hidden="true"></span></button><button type="button" class="cl-button" data-bulk="move" title="Переместить выбранное" aria-label="Переместить выбранное"><span class="cl-icon cl-move" aria-hidden="true"></span></button><button type="button" class="cl-button" data-bulk="trash" title="В корзину" aria-label="В корзину"><span class="cl-icon cl-trash" aria-hidden="true"></span></button><button type="button" class="btn btn-secondary" data-bulk="restore" hidden>Восстановить</button><button type="button" class="cl-button cl-danger" data-bulk="purge" title="Удалить навсегда" aria-label="Удалить навсегда" hidden><span class="cl-icon cl-trash" aria-hidden="true"></span></button><button type="button" class="cl-button" data-bulk="clear" title="Снять выделение" aria-label="Снять выделение"><span class="cl-icon cl-x" aria-hidden="true"></span></button></div></div>
 <div class="cloud-list-heading"><label><input type="checkbox" id="cloud-select-all" aria-label="Выбрать все показанные элементы"> <span>Имя</span></label><span>Размер</span><span>Изменено</span><span class="sr-only">Действия</span></div>
 <div id="cloud-list" class="cloud-list" role="region" aria-label="Файлы и папки"></div><p id="cloud-empty" class="cloud-empty" role="status"></p><p id="cloud-status" class="cloud-status" role="status" aria-live="polite"></p><button type="button" class="btn btn-secondary" id="cloud-more" hidden>Показать ещё</button>
 <dialog id="cloud-editor" class="confirmation-dialog cloud-dialog" aria-labelledby="cloud-editor-title"><div class="modal-content"><header class="cloud-dialog-heading"><h2 id="cloud-editor-title">Новая папка</h2><button type="button" class="cl-button" data-close-dialog title="Закрыть" aria-label="Закрыть"><span class="cl-icon cl-x" aria-hidden="true"></span></button></header><form id="cloud-editor-form" data-native-submit method="post"><div id="cloud-name-field"><label for="cloud-name">Имя</label><input type="text" id="cloud-name" maxlength="180" autocomplete="off" required></div><div id="cloud-destination-field" hidden><label for="cloud-destination">Папка назначения</label><select id="cloud-destination"></select></div><p id="cloud-confirm-text" hidden></p><div class="cloud-dialog-actions"><button type="button" class="btn btn-secondary" data-close-dialog>Отмена</button><button type="submit" class="btn btn-primary" id="cloud-editor-submit">Создать</button></div></form></div></dialog>
 <dialog id="cloud-preview" class="confirmation-dialog cloud-preview" aria-labelledby="cloud-preview-title"><div class="modal-content"><header class="cloud-dialog-heading"><h2 id="cloud-preview-title">Просмотр файла</h2><button type="button" class="cl-button" data-close-dialog title="Закрыть" aria-label="Закрыть"><span class="cl-icon cl-x" aria-hidden="true"></span></button></header><p id="cloud-preview-meta"></p><div id="cloud-preview-body"></div><footer class="cloud-dialog-actions"><a id="cloud-preview-download" class="btn btn-primary"><span class="cl-icon cl-download" aria-hidden="true"></span>Скачать</a></footer></div></dialog>
 <noscript>Для работы с облаком включите JavaScript.</noscript><script id="cloud-data" type="application/json">{{.Data}}</script>
</main><script src="/static/js/cloud.js?v=2" defer></script></body></html>`))
