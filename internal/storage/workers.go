package storage

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"project/internal/models"

	"github.com/google/uuid"
)

const workersFile = "data/workers.json"

// GetWorkers reads all workers from the JSON file.
func GetWorkers() ([]models.Worker, error) {
	bytes, err := ioutil.ReadFile(workersFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.Worker{}, nil // Return empty list if file doesn't exist
		}
		return nil, err
	}

	var workers []models.Worker
	err = json.Unmarshal(bytes, &workers)
	if err != nil {
		return nil, err
	}
	return workers, nil
}

// CreateWorker adds a new worker to the JSON file.
func CreateWorker(worker models.Worker) (*models.Worker, error) {
	workers, err := GetWorkers()
	if err != nil {
		return nil, err
	}

	worker.ID = "w" + uuid.New().String()[:8] // Short, unique ID
	workers = append(workers, worker)

	err = saveWorkersToFile(workers)
	if err != nil {
		return nil, err
	}
	return &worker, nil
}

// saveWorkersToFile is a helper function to write the worker list to the file.
func saveWorkersToFile(workers []models.Worker) error {
	bytes, err := json.MarshalIndent(workers, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(workersFile, bytes, 0644)
}
