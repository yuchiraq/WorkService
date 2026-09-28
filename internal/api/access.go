package api

import (
	"github.com/gin-gonic/gin"
	"html/template"
	"project/internal/models"
	"project/internal/storage"
)

func scheduleEditAction(c *gin.Context, entry models.TimesheetEntry, returnPath string) string {
	if !canEditSchedule(c, entry) {
		return ""
	}
	url := "/schedule/edit/" + template.HTMLEscapeString(entry.ID) + "?return=" + template.URLQueryEscaper(returnPath)
	return `<div class="assignment-actions"><a class="btn btn-secondary btn-compact" href="` + url + `" data-modal-url="` + url + `" data-modal-title="Редактировать запись" data-modal-return="` + template.HTMLEscapeString(returnPath) + `">Изменить</a></div>`
}

func isAdmin(c *gin.Context) bool {
	return c.GetString("userStatus") == "admin"
}

func requireAdmin(c *gin.Context) bool {
	if isAdmin(c) {
		return true
	}
	accessDenied(c)
	return false
}

func canEditSchedule(c *gin.Context, entry models.TimesheetEntry) bool {
	if isAdmin(c) {
		return true
	}
	worker, err := storage.GetWorkerByUserID(c.GetString("userID"))
	if err != nil || worker.IsFired || entry.CreatedByID != c.GetString("userID") {
		return false
	}
	for _, id := range entry.WorkerIDs {
		if id == worker.ID {
			return true
		}
	}
	return false
}

func requireScheduleWorker(c *gin.Context) bool {
	if isAdmin(c) {
		return true
	}
	worker, err := storage.GetWorkerByUserID(c.GetString("userID"))
	if err != nil || worker.IsFired {
		c.String(403, "Нет активного профиля работника")
		return false
	}
	return true
}
