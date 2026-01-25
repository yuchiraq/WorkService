package api

import (
	"net/http"
	"strconv"

	"project/internal/models"
	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

// WorkersPage displays the list of workers using the new layout.
func WorkersPage(c *gin.Context) {
	workers, err := storage.GetWorkers()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load workers: %v", err)
		return
	}

	c.HTML(http.StatusOK, "workers.html", gin.H{
		"title":       "Работники",
		"workers":     workers,
		"active_page": "workers",
	})
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
