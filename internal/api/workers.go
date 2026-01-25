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

// WorkersPage renders the list of workers with clickable cards.
func WorkersPage(c *gin.Context) {
	workers, err := storage.GetWorkers()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load workers: %v", err)
		return
	}

	var workersGridHTML strings.Builder
	for _, worker := range workers {
		runes := []rune(worker.Name)
		initials := ""
		if len(runes) > 1 {
			initials = string(runes[0:2])
		} else if len(runes) > 0 {
			initials = string(runes[0])
		}

		cardHTML := fmt.Sprintf(`
            <a href="/worker/%s" class="worker-card-link-wrapper">
                <div class="worker-card">
                    <div class="worker-card-header">
                        <div class="worker-avatar">%s</div>
                        <div class="worker-info">
                            <h3>%s</h3>
                            <p>%s</p>
                        </div>
                    </div>
                    <div class="worker-card-footer">
                        <span class="btn btn-secondary">Просмотр</span>
                    </div>
                </div>
            </a>`,
			template.HTMLEscapeString(worker.ID),
			strings.ToUpper(initials),
			template.HTMLEscapeString(worker.Name),
			template.HTMLEscapeString(worker.Position),
		)
		workersGridHTML.WriteString(cardHTML)
	}

	pageTemplate := `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>Работники</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    {{SIDEBAR_HTML}}
    <div class="main-content">
        <div class="page-header">
            <h1>Работники</h1>
            <a href="/workers/new" class="btn btn-primary">Добавить работника</a>
        </div>
        <div class="card">
            <p>Просмотр, добавление, редактирование или увольнение работников.</p>
            <div class="workers-grid">%s</div>
        </div>
    </div>
</body>
</html>`

	// Build the final HTML by replacing placeholders
	sidebar := RenderSidebar("workers")
	finalHTML := fmt.Sprintf(pageTemplate, workersGridHTML.String())
	finalHTML = strings.Replace(finalHTML, "{{SIDEBAR_HTML}}", sidebar, 1)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(finalHTML))
}

// WorkerProfilePage displays a single worker's profile.
func WorkerProfilePage(c *gin.Context) {
	workerID := c.Param("id")
	worker, err := storage.GetWorkerByID(workerID)
	if err != nil {
		c.String(http.StatusNotFound, "Worker not found: %v", err)
		return
	}

	runes := []rune(worker.Name)
	initials := ""
	if len(runes) > 1 {
		initials = string(runes[0:2])
	} else if len(runes) > 0 {
		initials = string(runes[0])
	}

	formattedBirthDate := "Не указана"
	if worker.BirthDate != "" {
		parts := strings.Split(worker.BirthDate, "-")
		if len(parts) == 3 {
			formattedBirthDate = fmt.Sprintf("%s.%s.%s", parts[2], parts[1], parts[0])
		}
	}

	pageTemplate := `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>Профиль: {{WORKER_NAME}}</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    {{SIDEBAR_HTML}}
    <div class="main-content"> 
        <a href="/workers" class="back-link"><svg viewBox="0 0 16 16"><path d="M11.9,8.5H4.1l3.3,3.3c0.2,0.2,0.2,0.5,0,0.7s-0.5,0.2-0.7,0l-4-4C2.6,8.4,2.5,8.2,2.5,8s0.1-0.4,0.2-0.5l4-4c0.2-0.2,0.5-0.2,0.7,0s0.2,0.5,0,0.7L4.1,7.5H11.9c0.3,0,0.5,0.2,0.5,0.5S12.2,8.5,11.9,8.5z"/></svg>К списку работников</a>

        <div class="profile-header-container">
            <div class="profile-header">
                <div class="worker-avatar">{{INITIALS}}</div>
                <div class="profile-header-info">
                    <h1>Профиль: {{WORKER_NAME}}</h1>
                    <p>{{POSITION}}</p>
                </div>
            </div>
            <div class="profile-actions">
                <div class="status-badge active"><svg viewBox="0 0 16 16"><path d="M8,0C3.6,0,0,3.6,0,8s3.6,8,8,8s8-3.6,8-8S12.4,0,8,0z M7,11.4L3.6,8L5,6.6l2,2l4-4L12.4,6L7,11.4z"/></svg>Активен</div>
                <a href="/workers/edit/{{WORKER_ID}}" class="btn btn-secondary">Редактировать</a>
            </div>
        </div>

        <ul class="profile-details">
            <li><svg fill="currentColor" viewBox="0 0 20 20"><path d="M6 8V7a4 4 0 118 0v1h2V7a6 6 0 10-12 0v1h2zm6 2H8v6h4v-6z"/></svg>Дата рождения: {{BIRTH_DATE}}</li>
            <li><svg fill="currentColor" viewBox="0 0 20 20"><path d="M2 3a1 1 0 011-1h2.153a1 1 0 01.986.836l.74 4.435a1 1 0 01-.54 1.06l-1.548.773a11.037 11.037 0 006.105 6.105l.774-1.548a1 1 0 011.059-.54l4.435.74a1 1 0 01.836.986V17a1 1 0 01-1 1h-2C7.82 18 2 12.18 2 5V3z"/></svg>{{PHONE}}</li>
            <li><svg fill="currentColor" viewBox="0 0 20 20"><path d="M10 2a8 8 0 100 16 8 8 0 000-16zm1 11a1 1 0 11-2 0v-2a1 1 0 112 0v2zm-1-4a1 1 0 01-1-1V7a1 1 0 112 0v1a1 1 0 01-1 1z"/></svg>Ставка: {{RATE}} руб/час</li>
        </ul>

        <div class="profile-grid">
            <div class="placeholder-card">
                 <div class="history-header"><h2>История назначений</h2></div>
                 <div class="icon"><svg fill="currentColor" viewBox="0 0 20 20"><path d="M3 3a1 1 0 000 2v8a1 1 0 001 1h12a1 1 0 001-1V5a1 1 0 000-2H3zm11 1H6v2h8V4zm-8 4H4v2h2V8zm0 3H4v2h2v-2zm4-3h4v2h-4V8zm0 3h4v2h-4v-2z"/></svg></div>
                 <h3>Назначений нет</h3>
                 <p>Для этого работника нет назначений.</p>
            </div>
            <div class="placeholder-card">
                 <div class="history-header"><h2>История событий</h2></div>
                 <div class="icon"><svg fill="currentColor" viewBox="0 0 20 20"><path d="M10 2a6 6 0 00-6 6v3.586l-1.707 1.707A1 1 0 003 15v1a1 1 0 001 1h12a1 1 0 001-1v-1a1 1 0 00-.293-.707L16 11.586V8a6 6 0 00-6-6zm0 14a2 2 0 110-4 2 2 0 010 4z"/></svg></div>
                 <h3>Событий не найдено</h3>
                 <p>Для этого работника еще не было зарегистрировано событий.</p>
            </div>
        </div>
    </div>
</body>
</html>`

	// Build the final HTML by replacing placeholders
	sidebar := RenderSidebar("workers")
	finalHTML := pageTemplate
	finalHTML = strings.Replace(finalHTML, "{{SIDEBAR_HTML}}", sidebar, -1)
	finalHTML = strings.Replace(finalHTML, "{{WORKER_NAME}}", template.HTMLEscapeString(worker.Name), -1)
	finalHTML = strings.Replace(finalHTML, "{{INITIALS}}", template.HTMLEscapeString(strings.ToUpper(initials)), -1)
	finalHTML = strings.Replace(finalHTML, "{{POSITION}}", template.HTMLEscapeString(worker.Position), -1)
	finalHTML = strings.Replace(finalHTML, "{{WORKER_ID}}", template.HTMLEscapeString(worker.ID), -1)
	finalHTML = strings.Replace(finalHTML, "{{BIRTH_DATE}}", template.HTMLEscapeString(formattedBirthDate), -1)
	finalHTML = strings.Replace(finalHTML, "{{PHONE}}", template.HTMLEscapeString(worker.Phone), -1)
	finalHTML = strings.Replace(finalHTML, "{{RATE}}", fmt.Sprintf("%.2f", worker.HourlyRate), -1)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(finalHTML))
}
