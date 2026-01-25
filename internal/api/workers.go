package api

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
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
		if len(runes) > 0 { initials = string(runes[0]) }
		if len(runes) > 1 { initials = string(runes[0:2]) }

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
                        <span class="btn btn-secondary">Редакт.</span>
                        <span class="btn btn-danger">Удалить</span>
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
    <aside class="sidebar"> ... </aside>
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

	// For simplicity, sidebar is static here. You could use a template replacement for it.
	sidebarHTML := `... HTML ...` // assume this is populated
	finalHTML := fmt.Sprintf(pageTemplate, workersGridHTML.String())
    finalHTML = strings.Replace(finalHTML, "<aside class="sidebar"> ... </aside>", sidebarHTML, 1)

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

	// UTF-8 safe initials
	runes := []rune(worker.Name)
	initials := ""
	if len(runes) > 0 { initials = string(runes[0]) }
	if len(runes) > 1 { initials = string(runes[0:2]) }

	// Format birth date if not empty
	formattedBirthDate := "Не указана"
	if worker.BirthDate != "" {
		// This assumes date is in YYYY-MM-DD. A real app should parse and format it.
		parts := strings.Split(worker.BirthDate, "-")
		if len(parts) == 3 {
			formattedBirthDate = fmt.Sprintf("%s.%s.%s", parts[2], parts[1], parts[0])
		}
	}

    pageHTML := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>Профиль: %s</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    <aside class="sidebar"> ... </aside>
    <div class="main-content">
        <a href="/workers" class="back-link"><svg viewBox="0 0 16 16"><path d="M11.9,8.5H4.1l3.3,3.3c0.2,0.2,0.2,0.5,0,0.7s-0.5,0.2-0.7,0l-4-4C2.6,8.4,2.5,8.2,2.5,8s0.1-0.4,0.2-0.5l4-4c0.2-0.2,0.5-0.2,0.7,0s0.2,0.5,0,0.7L4.1,7.5H11.9c0.3,0,0.5,0.2,0.5,0.5S12.2,8.5,11.9,8.5z"/></svg>К списку работников</a>

        <div class="profile-header-container">
            <div class="profile-header">
                <div class="worker-avatar">%s</div>
                <div class="profile-header-info">
                    <h1>Профиль: %s</h1>
                    <p>%s</p>
                </div>
            </div>
            <div class="profile-actions">
                <div class="status-badge active"><svg viewBox="0 0 16 16"><path d="M8,0C3.6,0,0,3.6,0,8s3.6,8,8,8s8-3.6,8-8S12.4,0,8,0z M7,11.4L3.6,8L5,6.6l2,2l4-4L12.4,6L7,11.4z"/></svg>Активен</div>
                <a href="/workers/edit/%s" class="btn btn-secondary">Редактировать</a>
            </div>
        </div>

        <ul class="profile-details">
            <li><svg viewBox="0 0 24 24"><path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/></svg>Дата рождения: %s</li>
            <li><svg viewBox="0 0 24 24"><path d="M6.62 10.79c1.44 2.83 3.76 5.14 6.59 6.59l2.2-2.2c.27-.27.67-.36 1.02-.24 1.12.37 2.33.57 3.57.57.55 0 1 .45 1 1V20c0 .55-.45 1-1 1-9.39 0-17-7.61-17-17 0-.55.45-1 1-1h3.5c.55 0 1 .45 1 1 0 1.25.2 2.45.57 3.57.11.35.03.74-.25 1.02l-2.2 2.2z"/></svg>%s</li>
            <li><svg viewBox="0 0 24 24"><path d="M11.8 10.9c-2.27-.59-3-1.2-3-2.15 0-.96.74-1.75 2.2-1.75 1.45 0 2.2.8 2.2 1.75h2c0-1.93-1.57-3.5-4.2-3.5-2.64 0-4.2 1.57-4.2 3.5 0 1.93 1.57 3.5 4.2 3.5 2.64 0 4.2-1.57 4.2-3.5h-2c0 .96-.75 1.75-2.2 1.75-1.45 0-2.2-.8-2.2-1.75 0-.96.75-1.75 2.2-1.75s2.2.8 2.2 1.75h2c0-1.93-1.57-3.5-4.2-3.5z"/></svg>Ставка: %.2f руб/час</li>
        </ul>

        <div class="profile-grid">
            <div class="placeholder-card">
                 <div class="history-header"><h2>История назначений</h2></div>
                 <div class="icon">...</div> <h3>Назначений нет</h3> <p>Для этого работника нет назначений.</p>
            </div>
            <div class="placeholder-card">
                 <div class="history-header"><h2>История событий</h2></div>
                 <div class="icon">...</div> <h3>Событий не найдено</h3> <p>Для этого работника еще не было зарегистрировано событий.</p>
            </div>
        </div>
    </div>
</body>
</html>`,
        worker.Name,
        strings.ToUpper(initials),
        template.HTMLEscapeString(worker.Name),
        template.HTMLEscapeString(worker.Position),
        template.HTMLEscapeString(worker.ID),
        template.HTMLEscapeString(formattedBirthDate),
        template.HTMLEscapeString(worker.Phone),
        worker.HourlyRate,
    )

    // For simplicity, sidebar is static here. You could use a template replacement for it.
	sidebarHTML := `... HTML ...` // assume this is populated
	finalHTML := strings.Replace(pageHTML, "<aside class="sidebar"> ... </aside>", sidebarHTML, 1)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(finalHTML))
}

// Other functions remain unchanged ...
