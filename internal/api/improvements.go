package api

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"project/internal/models"
	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

func ImprovementsPage(c *gin.Context) {
	items, err := storage.GetImprovements()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load improvements: %v", err)
		return
	}

	var rows strings.Builder
	if len(items) == 0 {
		rows.WriteString(`<div class="info-card"><p>Пока нет записей. Добавьте первое улучшение или ошибку.</p></div>`)
	} else {
		for _, item := range items {
			typeLabel := "Улучшение"
			if item.Type == "bug" {
				typeLabel = "Ошибка"
			}
			statusLabel := "Открыто"
			statusClass := "status-warning"
			actionHTML := ""
			if item.Status == "done" {
				statusLabel = "Завершено"
				statusClass = "status-active"
				if strings.TrimSpace(item.DoneBy) != "" {
					actionHTML = `<p class="text-muted">Закрыл: ` + template.HTMLEscapeString(item.DoneBy) + `</p>`
				}
			} else {
				actionHTML = `<form method="POST" action="/improvements/complete/` + template.HTMLEscapeString(item.ID) + `">` + CSRFHiddenInput(c) + `<button type="submit" class="btn btn-success btn-compact">Отметить завершённым</button></form>`
			}

			rows.WriteString(fmt.Sprintf(`<article class="assignment-card"><div class="assignment-head"><strong>%s</strong><span class="status-badge %s">%s</span></div><div class="assignment-body"><div class="assignment-section"><div class="assignment-meta"><span>Тип</span><p>%s</p></div></div><div class="assignment-section"><div class="assignment-meta"><span>Описание</span><p>%s</p></div></div><div class="assignment-section"><div class="assignment-meta"><span>Создал</span><p>%s</p></div></div>%s</div></article>`,
				template.HTMLEscapeString(item.Title),
				template.HTMLEscapeString(statusClass),
				template.HTMLEscapeString(statusLabel),
				template.HTMLEscapeString(typeLabel),
				template.HTMLEscapeString(item.Description),
				template.HTMLEscapeString(item.CreatedBy),
				actionHTML,
			))
		}
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover"><title>Улучшения и ошибки</title><link rel="stylesheet" href="/static/css/style.css"></head><body>
{{SIDEBAR_HTML}}
<div class="main-content">
  <div class="page-header"><h1>Улучшения и ошибки</h1></div>
  <div class="card">
    <form method="POST" action="/improvements/new" class="form-grid">
      ` + CSRFHiddenInput(c) + `
      <div class="form-group">
        <label for="kind">Тип записи</label>
        <select id="kind" name="kind" required>
          <option value="improvement">Улучшение</option>
          <option value="bug">Ошибка</option>
        </select>
      </div>
      <div class="form-group">
        <label for="title">Заголовок</label>
        <input id="title" name="title" required placeholder="Коротко о проблеме/идее">
      </div>
      <div class="form-group timesheet-span-2">
        <label for="description">Описание</label>
        <textarea id="description" name="description" required placeholder="Опишите, что нужно исправить или улучшить"></textarea>
      </div>
      <div class="timesheet-span-2 form-actions"><button class="btn btn-primary" type="submit">Добавить запись</button></div>
    </form>
  </div>
  <div class="schedule-vertical">{{ROWS}}</div>
</div></body></html>`

	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "improvements"), 1)
	final = strings.Replace(final, "{{ROWS}}", rows.String(), 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}

func CreateImprovement(c *gin.Context) {
	kind := strings.TrimSpace(c.PostForm("kind"))
	if kind != "bug" {
		kind = "improvement"
	}
	title := strings.TrimSpace(c.PostForm("title"))
	description := strings.TrimSpace(c.PostForm("description"))
	if title == "" || description == "" {
		c.Redirect(http.StatusFound, "/improvements")
		return
	}

	item := models.ImprovementItem{
		Type:        kind,
		Title:       title,
		Description: description,
		Status:      "open",
		CreatedByID: c.GetString("userID"),
		CreatedBy:   c.GetString("userName"),
	}
	if item.CreatedBy == "" {
		item.CreatedBy = "Пользователь"
	}

	if err := storage.AddImprovement(item); err != nil {
		c.String(http.StatusInternalServerError, "Failed to add improvement: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/improvements")
}

func CompleteImprovement(c *gin.Context) {
	id := c.Param("id")
	if err := storage.MarkImprovementDone(id, c.GetString("userID"), c.GetString("userName")); err != nil {
		c.String(http.StatusNotFound, "Not found")
		return
	}
	c.Redirect(http.StatusFound, "/improvements")
}
