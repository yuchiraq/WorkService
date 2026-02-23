package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

// DashboardPage renders the main dashboard page using manual HTML string building.
func DashboardPage(c *gin.Context) {
	userName, _ := c.Get("userName")

	workers, _ := storage.GetWorkers()
	objects, _ := storage.GetObjects()
	entries, _ := storage.GetTimesheets()

	activeWorkers := 0
	for _, w := range workers {
		if !w.IsFired {
			activeWorkers++
		}
	}
	activeObjects := 0
	for _, o := range objects {
		if o.Status != "completed" {
			activeObjects++
		}
	}
	today := time.Now().Format("2006-01-02")
	todayAssignments := 0
	for _, entry := range entries {
		if entry.Date == today {
			todayAssignments++
		}
	}

	pageTemplate := `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>Панель управления</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    {{SIDEBAR_HTML}}
    <div class="main-content">
        <div class="page-header">
            <h1>Добро пожаловать, %s!</h1>
        </div>
        <div class="workers-grid dashboard-stats">
            <a class="metric metric-link" href="/schedule"><div class="label">Назначений сегодня</div><div class="value">%d</div></a>
            <a class="metric metric-link" href="/workers"><div class="label">Рабочих в штате</div><div class="value">%d</div></a>
            <a class="metric metric-link" href="/objects"><div class="label">Объектов в работе</div><div class="value">%d</div></a>
        </div>
        <div class="card">
            <p>Вы находитесь в системе управления работами.</p>
			<p>Используйте меню слева для перехода в нужный раздел.</p>
        </div>
    </div>
</body>
</html>`

	sidebar := RenderSidebar(c, "dashboard")
	pageContent := fmt.Sprintf(pageTemplate, userName, todayAssignments, activeWorkers, activeObjects)
	finalHTML := strings.Replace(pageContent, "{{SIDEBAR_HTML}}", sidebar, 1)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(finalHTML))
}
