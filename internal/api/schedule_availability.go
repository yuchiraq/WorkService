package api

import (
	"net/http"
	"time"

	"project/internal/models"
	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

func busyWorkersOnDate(entries []models.TimesheetEntry, date, excludeID string) map[string]bool {
	busy := map[string]bool{}
	for _, entry := range entries {
		if (excludeID != "" && entry.ID == excludeID) || date < entry.Date || date > entryEnd(entry) {
			continue
		}
		for _, id := range entry.WorkerIDs {
			busy[id] = true
		}
	}
	return busy
}

func ScheduleAvailability(c *gin.Context) {
	if !requireScheduleWorker(c) {
		return
	}
	date := c.Query("date")
	if _, err := time.Parse("2006-01-02", date); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Укажите корректную дату"})
		return
	}
	exclude := c.Query("exclude_id")
	if exclude != "" {
		entry, err := storage.GetTimesheetByID(exclude)
		if err != nil || !canEditSchedule(c, entry) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Нет доступа к назначению"})
			return
		}
	}
	entries, err := storage.GetTimesheets()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось проверить занятость"})
		return
	}
	// Expose only availability, never other workers' hours, objects or absence reasons.
	c.JSON(http.StatusOK, gin.H{"busy": busyWorkersOnDate(entries, date, exclude), "date": date})
}
