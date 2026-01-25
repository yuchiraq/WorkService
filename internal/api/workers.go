package api

import (
	"net/http"
	"strconv"

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

// AddWorkerPage displays the form to add a new worker.
func AddWorkerPage(c *gin.Context) {
	c.HTML(http.StatusOK, "add-worker.html", nil)
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

	// In a real app, you would get the user from the session/context
	// For now, let's keep it simple
	currentUserLogin := "user-login"
	currentUserName := "Admin Name"

	hourlyRate, _ := strconv.ParseFloat(c.PostForm("hourlyRate"), 64)

	updatedWorker := models.Worker{
		ID:            workerID, // Keep the original ID
		Name:          c.PostForm("name"),
		Position:      c.PostForm("position"),
		Phone:         c.PostForm("phone"),
		HourlyRate:    hourlyRate,
		BirthDate:     c.PostForm("birthDate"),
		CreatedBy:     currentUserLogin, // This should not be updated, but for now we will keep it
		CreatedByName: currentUserName,
	}

	if err := storage.UpdateWorker(updatedWorker); err != nil {
		c.String(http.StatusInternalServerError, "Failed to update worker: %v", err)
		return
	}

	c.Redirect(http.StatusFound, "/workers")
}


// CreateWorker handles the API request to add a new worker.
func CreateWorker(c *gin.Context) {
	// In a real app, you would get the user from the session/context
	currentUserLogin := "user-login" // Example user login
	currentUserName := "Admin Name"   // Example user name

	hourlyRate, _ := strconv.ParseFloat(c.PostForm("hourlyRate"), 64)

	newWorker := models.Worker{
		Name:          c.PostForm("name"),
		Position:      c.PostForm("position"),
		Phone:         c.PostForm("phone"),
		HourlyRate:    hourlyRate,
		BirthDate:     c.PostForm("birthDate"),
		CreatedBy:     currentUserLogin,
		CreatedByName: currentUserName,
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

	// Redirect back to the workers list after deletion
	c.Redirect(http.StatusFound, "/workers")
}
