package api

import (
	"net/http"

	"project/internal/models"
	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

// GetWorkers handles the API request to retrieve all workers.
func GetWorkers(c *gin.Context) {
	workers, err := storage.GetWorkers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve workers"})
		return
	}
	c.JSON(http.StatusOK, workers)
}

// CreateWorker handles the API request to add a new worker.
func CreateWorker(c *gin.Context) {
	var newWorker models.Worker
	if err := c.ShouldBindJSON(&newWorker); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
		return
	}

	// In a real application, user info would come from the session/token.
	// For demonstration, we're hardcoding it.
	newWorker.CreatedBy = "u1"
	newWorker.CreatedByName = "Admin"

	createdWorker, err := storage.CreateWorker(newWorker)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create worker"})
		return
	}

	c.JSON(http.StatusCreated, createdWorker)
}

// DeleteWorker handles the API request to delete a worker.
func DeleteWorker(c *gin.Context) {
	workerID := c.Param("id")

	if err := storage.DeleteWorker(workerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete worker"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Worker deleted successfully"})
}
