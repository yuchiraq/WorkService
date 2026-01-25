package storage

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"project/internal/models"

	"github.com/google/uuid"
)

const (
	articlesFile = "data/articles.json"
	usersFile    = "data/users.json"
	workersFile  = "data/workers.json"
)

// ReadArticles reads the JSON file and returns a slice of articles.
func ReadArticles() ([]models.Article, error) {
	data, err := ioutil.ReadFile(articlesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.Article{}, nil // Return empty slice if file doesn't exist
		}
		return nil, err
	}

	var articles []models.Article
	if err := json.Unmarshal(data, &articles); err != nil {
		return nil, err
	}
	return articles, nil
}

// WriteArticles writes a slice of articles to the JSON file.
func WriteArticles(articles []models.Article) error {
	data, err := json.MarshalIndent(articles, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(articlesFile, data, 0644)
}

// ReadUsers reads the JSON file and returns a slice of users.
func ReadUsers() ([]models.User, error) {
	data, err := ioutil.ReadFile(usersFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.User{}, nil // Return empty slice if file doesn't exist
		}
		return nil, err
	}

	var users []models.User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// WriteUsers writes a slice of users to the JSON file.
func WriteUsers(users []models.User) error {
	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(usersFile, data, 0644)
}

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
