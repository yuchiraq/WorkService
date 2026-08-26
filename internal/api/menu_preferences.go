package api

import (
	"html/template"
	"net/http"
	"strings"

	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

func MenuPreferencesPage(c *gin.Context) {
	user, err := storage.GetUserByID(c.GetString("userID"))
	if err != nil {
		c.String(http.StatusNotFound, "Пользователь не найден")
		return
	}

	hidden := make(map[string]struct{}, len(user.HiddenMenuItems))
	for _, item := range user.HiddenMenuItems {
		hidden[item] = struct{}{}
	}

	var options strings.Builder
	for _, item := range navItemsForStatus(user.Status) {
		checked := " checked"
		if _, isHidden := hidden[item.PageID]; isHidden {
			checked = ""
		}
		options.WriteString(`<label class="menu-preference-item"><input type="checkbox" name="visible_items" value="` + template.HTMLEscapeString(item.PageID) + `"` + checked + `><span><strong>` + template.HTMLEscapeString(item.Label) + `</strong><small>` + template.HTMLEscapeString(item.Path) + `</small></span></label>`)
	}

	status := ""
	if c.Query("saved") == "1" {
		status = `<div class="dashboard-alert-item is-success"><strong>Меню сохранено</strong><p>Изменения уже применены в боковой панели.</p></div>`
	}
	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover"><title>Настройка меню</title><link rel="stylesheet" href="/static/css/style.css?v=9"></head><body>
{{SIDEBAR_HTML}}
<main class="main-content"><div class="page-header"><div><h1>Настройка меню</h1><p class="text-muted">Оставьте только разделы, которыми пользуетесь. Скрытые страницы по-прежнему можно открыть по прямой ссылке.</p></div></div>
{{STATUS}}<section class="card"><form method="POST" action="/profile/menu" class="menu-preferences-form">{{CSRF_FIELD}}<div class="menu-preferences-grid">{{OPTIONS}}</div><div class="form-actions-edit"><button class="btn btn-primary" type="submit">Сохранить меню</button><a class="btn btn-secondary" href="/profile">Назад к профилю</a></div></form></section></main>
</body></html>`
	page = strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "menu-settings"), 1)
	page = strings.Replace(page, "{{CSRF_FIELD}}", CSRFHiddenInput(c), 1)
	page = strings.Replace(page, "{{OPTIONS}}", options.String(), 1)
	page = strings.Replace(page, "{{STATUS}}", status, 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
}

func UpdateMenuPreferences(c *gin.Context) {
	user, err := storage.GetUserByID(c.GetString("userID"))
	if err != nil {
		c.String(http.StatusNotFound, "Пользователь не найден")
		return
	}

	visible := make(map[string]struct{})
	for _, item := range c.PostFormArray("visible_items") {
		visible[item] = struct{}{}
	}
	hidden := make([]string, 0)
	for _, item := range navItemsForStatus(user.Status) {
		if _, isVisible := visible[item.PageID]; !isVisible {
			hidden = append(hidden, item.PageID)
		}
	}
	if err := storage.UpdateUserHiddenMenuItems(user.ID, hidden); err != nil {
		c.String(http.StatusInternalServerError, "Не удалось сохранить меню: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/profile/menu?saved=1")
}
