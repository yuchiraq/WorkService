package storage

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"project/internal/models"

	"github.com/google/uuid"
)

const (
	workersFile = "data/workers.json"
)

// GetWorkers reads the workers data file and returns a slice of Worker structs.
func GetWorkers() ([]models.Worker, error) {
	data, err := ioutil.ReadFile(workersFile)
	if err != nil {
		// If the file doesn't exist, return an empty slice
		if os.IsNotExist(err) {
			return []models.Worker{}, nil
		}
		return nil, err
	}

	var workers []models.Worker
	if err := json.Unmarshal(data, &workers); err != nil {
		return nil, err
	}
	return workers, nil
}

// CreateWorker adds a new worker to the storage.
func CreateWorker(worker models.Worker) (models.Worker, error) {
	workers, err := GetWorkers()
	if err != nil {
		return models.Worker{}, err
	}

	// Generate a new unique ID
	worker.ID = uuid.New().String()

	workers = append(workers, worker)

	if err := saveWorkers(workers); err != nil {
		return models.Worker{}, err
	}
	return worker, nil
}

// DeleteWorker removes a worker by their ID.
func DeleteWorker(id string) error {
	workers, err := GetWorkers()
	if err != nil {
		return err
	}

	// Find the index of the worker to delete
	foundIndex := -1
	for i, worker := range workers {
		if worker.ID == id {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		// In a real application, you might return a specific "not found" error
		return nil // Or an error indicating not found
	}

	// Remove the worker from the slice
	workers = append(workers[:foundIndex], workers[foundIndex+1:]...)

	return saveWorkers(workers)
}

// saveWorkers writes the complete list of workers to the JSON file.
func saveWorkers(workers []models.Worker) error {
	data, err := json.MarshalIndent(workers, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(workersFile, data, 0644)
}
