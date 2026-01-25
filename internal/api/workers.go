package api

import (
	"net/http"

	"project/internal/models"
	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

// GetWorkers handles the API request to get all workers.
func GetWorkers(c *gin.Context) {
	workers, err := storage.GetWorkers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch workers"})
		return
	}
	c.JSON(http.StatusOK, workers)
}

// CreateWorker handles the API request to create a new worker.
func CreateWorker(c *gin.Context) {
	var worker models.Worker
	if err := c.ShouldBindJSON(&worker); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// In a real app, you'd get the user from the session.
	// For now, we'll hardcode it.
	worker.CreatedBy = "u1"
	worker.CreatedByName = "Admin"

	newWorker, err := storage.CreateWorker(worker)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create worker"})
		return
	}

	c.JSON(http.StatusCreated, newWorker)
}
