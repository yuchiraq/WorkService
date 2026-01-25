package api

import (
	"net/http"

	"project/internal/models"
	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

// WorkersPage displays the list of workers.
func WorkersPage(c *gin.Context) {
	workers, err := storage.GetWorkers()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load workers: %v", err)
		return
	}

	c.HTML(http.StatusOK, "workers.html", gin.H{
		"workers": workers,
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
		"worker": worker,
	})
}

// UpdateWorker handles the submission of the worker edit form.
func UpdateWorker(c *gin.Context) {
	workerID := c.Param("id")
	var updatedWorker models.Worker

	// Bind the form data to the struct
	if err := c.ShouldBind(&updatedWorker); err != nil {
		c.String(http.StatusBadRequest, "Invalid form data: %v", err)
		return
	}

	updatedWorker.ID = workerID // Ensure the ID is set for the update

	// In a real app, you would also update the 'UpdatedBy' fields, etc.

	if err := storage.UpdateWorker(updatedWorker); err != nil {
		c.String(http.StatusInternalServerError, "Failed to update worker: %v", err)
		return
	}

	c.Redirect(http.StatusFound, "/workers")
}


// CreateWorker handles the API request to add a new worker.
func CreateWorker(c *gin.Context) {
	var newWorker models.Worker
	if err := c.ShouldBindJSON(&newWorker); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
		return
	}

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

	// Redirect back to the workers list after deletion
	c.Redirect(http.StatusFound, "/workers")
}
