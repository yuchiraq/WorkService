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
	workersFile  = "storage/workers.json" // Corrected path
)

// LoadWorkers reads the workers.json file and populates the workers slice.
func LoadWorkers() error {
	workersMutex.Lock()
	defer workersMutex.Unlock()

	file, err := os.ReadFile(workersFile)
	if err != nil {
		if os.IsNotExist(err) {
			workers = []models.Worker{} // If file doesn't exist, start with an empty slice
			return nil
		}
		return err
	}

	return json.Unmarshal(file, &workers)
}

// saveWorkers writes the current state of the workers slice to the workers.json file.
func saveWorkers() error {
	data, err := json.MarshalIndent(workers, "", "    ")
	if err != nil {
		return err
	}
	// Ensure the directory exists
	if err := os.MkdirAll("storage", 0755); err != nil {
		return err
	}
	return os.WriteFile(workersFile, data, 0644)
}

// GetWorkers returns all workers.
func GetWorkers() ([]models.Worker, error) {
	workersMutex.RLock()
	defer workersMutex.RUnlock()

	workersCopy := make([]models.Worker, len(workers))
	copy(workersCopy, workers)

	return workersCopy, nil
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

// GetWorkerByUserID retrieves a worker linked to a user account.
func GetWorkerByUserID(userID string) (models.Worker, error) {
	workersMutex.RLock()
	defer workersMutex.RUnlock()

	for _, worker := range workers {
		if worker.UserID == userID {
			return worker, nil
		}
	}

	return models.Worker{}, errors.New("worker not found for user")
}

// CreateWorker adds a new worker to the list and saves it.
func CreateWorker(worker models.Worker) (models.Worker, error) {
	workersMutex.Lock()
	defer workersMutex.Unlock()

	worker.ID = uuid.New().String()

	workers = append(workers, worker)

	if err := saveWorkers(); err != nil {
		// If save fails, roll back the addition
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
			workers[i] = updatedWorker
			return saveWorkers()
		}
	}

	return errors.New("worker not found for update")
}

// DeleteWorker marks worker as fired instead of physical deletion.
func DeleteWorker(id string) error {
	workersMutex.Lock()
	defer workersMutex.Unlock()

	for i, worker := range workers {
		if worker.ID == id {
			if workers[i].IsFired {
				return nil
			}
			workers[i].IsFired = true
			workers[i].FiredAt = time.Now().Format(time.RFC3339)
			workers[i].UserID = ""
			return saveWorkers()
		}
	}

	return errors.New("worker not found for dismissal")
}

// LinkWorkerToUser links a worker to a user and clears previous links for both sides.
func LinkWorkerToUser(workerID, userID string) error {
	workersMutex.Lock()
	defer workersMutex.Unlock()

	workerIndex := -1
	for i, worker := range workers {
		if worker.ID == workerID {
			workerIndex = i
			break
		}
	}
	if workerIndex == -1 {
		return errors.New("worker not found")
	}
	if workers[workerIndex].IsFired {
		return errors.New("cannot link dismissed worker")
	}

	for i := range workers {
		if workers[i].UserID == userID {
			workers[i].UserID = ""
		}
	}
	workers[workerIndex].UserID = userID
	return saveWorkers()
}

// ClearWorkerLinkByUserID clears the worker-user link without deleting worker data.
func ClearWorkerLinkByUserID(userID string) error {
	workersMutex.Lock()
	defer workersMutex.Unlock()

	changed := false
	for i := range workers {
		if workers[i].UserID == userID {
			workers[i].UserID = ""
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return saveWorkers()
}

// DeleteWorkerByUserID unlinks user account without deleting worker history.
func DeleteWorkerByUserID(userID string) error {
	return ClearWorkerLinkByUserID(userID)
}

// LinkWorkerToUser links a worker to a user and clears previous links for both sides.
func LinkWorkerToUser(workerID, userID string) error {
	workersMutex.Lock()
	defer workersMutex.Unlock()

	workerIndex := -1
	for i, worker := range workers {
		if worker.ID == workerID {
			workerIndex = i
			break
		}
	}
	if workerIndex == -1 {
		return errors.New("worker not found")
	}

	for i := range workers {
		if workers[i].UserID == userID {
			workers[i].UserID = ""
		}
	}
	workers[workerIndex].UserID = userID
	return saveWorkers()
}

// ClearWorkerLinkByUserID clears the worker-user link without deleting worker data.
func ClearWorkerLinkByUserID(userID string) error {
	workersMutex.Lock()
	defer workersMutex.Unlock()

	changed := false
	for i := range workers {
		if workers[i].UserID == userID {
			workers[i].UserID = ""
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return saveWorkers()
}

// DeleteWorkerByUserID removes a worker linked to user account.
func DeleteWorkerByUserID(userID string) error {
	workersMutex.Lock()
	defer workersMutex.Unlock()

	for i, worker := range workers {
		if worker.UserID == userID {
			workers = append(workers[:i], workers[i+1:]...)
			return saveWorkers()
		}
	}

	return nil
}
