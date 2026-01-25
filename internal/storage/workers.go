package storage

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"

	"project/internal/models"

	"github.com/google/uuid"
)

var (
	workers      []models.Worker
	workersMutex sync.RWMutex
	workersFile  = "data/workers.json"
)

// LoadWorkers reads the workers.json file and populates the workers slice.
func LoadWorkers() error {
	workersMutex.Lock()
	defer workersMutex.Unlock()

	file, err := os.ReadFile(workersFile)
	if err != nil {
		if os.IsNotExist(err) {
			workers = []models.Worker{}
			return nil // File not existing is not an error; we start with an empty list.
		}
		return err
	}

	return json.Unmarshal(file, &workers)
}

// saveWorkers writes the current state of the workers slice to the workers.json file.
func saveWorkers() error {
	// This function should be called with a Lock, not RLock
	data, err := json.MarshalIndent(workers, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(workersFile, data, 0644)
}

// GetWorkers returns all workers.
func GetWorkers() ([]models.Worker, error) {
	workersMutex.RLock()
	defer workersMutex.RUnlock()
	return workers, nil
}

// GetWorkerByID retrieves a single worker by their ID.
func GetWorkerByID(id string) (models.Worker, error) {
	workersMutex.RLock()
	defer workersMutex.RUnlock()

	for _, worker := range workers {
		if worker.ID == id {
			return worker, nil
		}
	}
	return models.Worker{}, errors.New("worker not found")
}

// CreateWorker adds a new worker to the list and saves it.
func CreateWorker(worker models.Worker) (models.Worker, error) {
	workersMutex.Lock()
	defer workersMutex.Unlock()

	worker.ID = uuid.New().String()
	worker.CreatedAt = time.Now()

	workers = append(workers, worker)

	if err := saveWorkers(); err != nil {
		// If saving fails, revert the addition to maintain consistency.
		workers = workers[:len(workers)-1]
		return models.Worker{}, err
	}

	return worker, nil
}

// UpdateWorker modifies an existing worker in the list and saves the changes.
func UpdateWorker(updatedWorker models.Worker) error {
	workersMutex.Lock()
	defer workersMutex.Unlock()

	for i, worker := range workers {
		if worker.ID == updatedWorker.ID {
			// Preserve original creation info
			updatedWorker.CreatedAt = worker.CreatedAt
			updatedWorker.CreatedBy = worker.CreatedBy
			updatedWorker.CreatedByName = worker.CreatedByName

			// Set new update info
			updatedWorker.UpdatedAt = time.Now()

			workers[i] = updatedWorker
			return saveWorkers()
		}
	}

	return errors.New("worker not found for update")
}


// DeleteWorker removes a worker from the list and saves the changes.
func DeleteWorker(id string) error {
	workersMutex.Lock()
	defer workersMutex.Unlock()

	for i, worker := range workers {
		if worker.ID == id {
			workers = append(workers[:i], workers[i+1:]...)
			return saveWorkers()
		}
	}

	return errors.New("worker not found for deletion")
}
