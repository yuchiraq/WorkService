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

// WorkersPage displays the list of workers using manual HTML generation.
func WorkersPage(c *gin.Context) {
	workers, err := storage.GetWorkers()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load workers: %v", err)
		return
	}

	// --- Manually generate the HTML for the workers grid ---
	var workersGridHTML strings.Builder
	for _, worker := range workers {
		// UTF-8 safe initials
		runes := []rune(worker.Name)
		initials := ""
		if len(runes) > 0 {
			initials = string(runes[0])
		}
		if len(runes) > 1 {
			initials = string(runes[0:2])
		}

		cardHTML := fmt.Sprintf(`
			<div class="worker-card">
				<div class="worker-card-header">
					<div class="worker-avatar">%s</div>
					<div class="worker-info">
						<h3>%s</h3>
						<p>%s</p>
					</div>
				</div>
				<div class="worker-card-footer">
					<a href="/workers/edit/%s" class="btn btn-secondary">Редакт.</a>
					<form action="/workers/delete/%s" method="POST" style="display: inline;">
						<button type="submit" class="btn btn-danger">Удалить</button>
					</form>
				</div>
			</div>`,
			strings.ToUpper(initials),
			emplate.HTMLEscapeString(worker.Name),
			emplate.HTMLEscapeString(worker.Position),
			emplate.HTMLEscapeString(worker.ID),
			emplate.HTMLEscapeString(worker.ID),
		)
		workersGridHTML.WriteString(cardHTML)
	}

	// --- Define the full page structure ---
	pageTemplate := `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>Работники</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    <aside class="sidebar">
		<div class="sidebar-header"><h2>Управление</h2></div>
		<nav><ul>
			<li><a href="/dashboard">Главная</a></li>
			<li class="active"><a href="/workers">Работники</a></li>
		</ul></nav>
		<div class="sidebar-footer"><a href="/logout">Выход</a></div>
	</aside>
    <div class="main-content">
        <div class="page-header">
            <h1>Работники</h1>
            <a href="/workers/new" class="btn btn-primary">Добавить работника</a>
        </div>
        <div class="card">
            <p>Просмотр, добавление, редактирование или увольнение работников.</p>
            <div class="workers-grid">{{WORKERS_GRID}}</div>
        </div>
    </div>
</body>
</html>`

	// --- Assemble the final page ---
	finalHTML := strings.Replace(pageTemplate, "{{WORKERS_GRID}}", workersGridHTML.String(), 1)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(finalHTML))
}

// AddWorkerPage displays the form to add a new worker.
func AddWorkerPage(c *gin.Context) {
	c.HTML(http.StatusOK, "add-worker.html", gin.H{
		"title":       "Добавить работника",
		"active_page": "workers",
	})
}

// EditWorkerPage displays the form to edit an existing worker.
func EditWorkerPage(c *gin.Context) {
	workerID := c.Param("id")
	worker, err := storage.GetWorkerByID(workerID)
	if err != nil {
		c.String(http.StatusNotFound, "Worker not found")
		return
	}

	c.HTML(http.StatusOK, "edit-worker.html", gin.H{
		"title":       "Редактировать работника",
		"worker":      worker,
		"active_page": "workers",
	})
}

// UpdateWorker handles the submission of the worker edit form.
func UpdateWorker(c *gin.Context) {
	workerID := c.Param("id")

	hourlyRate, _ := strconv.ParseFloat(c.PostForm("hourlyRate"), 64)

	updatedWorker := models.Worker{
		ID:         workerID,
		Name:       c.PostForm("name"),
		Position:   c.PostForm("position"),
		Phone:      c.PostForm("phone"),
		HourlyRate: hourlyRate,
		BirthDate:  c.PostForm("birthDate"),
	}

	if err := storage.UpdateWorker(updatedWorker); err != nil {
		c.String(http.StatusInternalServerError, "Failed to update worker: %v", err)
		return
	}

	c.Redirect(http.StatusFound, "/workers")
}

// CreateWorker handles the API request to add a new worker.
func CreateWorker(c *gin.Context) {
	hourlyRate, _ := strconv.ParseFloat(c.PostForm("hourlyRate"), 64)

	newWorker := models.Worker{
		Name:       c.PostForm("name"),
		Position:   c.PostForm("position"),
		Phone:      c.PostForm("phone"),
		HourlyRate: hourlyRate,
		BirthDate:  c.PostForm("birthDate"),
	}

	_, err := storage.CreateWorker(newWorker)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to create worker: %v", err)
		return
	}

	c.Redirect(http.StatusFound, "/workers")
}

// DeleteWorker handles the API request to delete a worker.
func DeleteWorker(c *gin.Context) {
	workerID := c.Param("id")

	if err := storage.DeleteWorker(workerID); err != nil {
		c.String(http.StatusInternalServerError, "Failed to delete worker: %v", err)
		return
	}

	c.Redirect(http.StatusFound, "/workers")
}
