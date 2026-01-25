package storage

import (
	"encoding/json"
	"errors"
	"os"
	"sync"

	"project/internal/models"
)

var (
	users      []models.User
	usersMutex sync.RWMutex
	usersFile  = "data/users.json"
)

// LoadUsers reads the users.json file and populates the users slice.
func LoadUsers() error {
	usersMutex.Lock()
	defer usersMutex.Unlock()

	file, err := os.ReadFile(usersFile)
	if err != nil {
		if os.IsNotExist(err) {
			users = []models.User{}
			return nil // File not existing is not an error; we start with an empty list.
		}
		return err
	}

	return json.Unmarshal(file, &users)
}

// GetUserByID retrieves a single user by their ID.
func GetUserByID(id string) (models.User, error) {
	usersMutex.RLock()
	defer usersMutex.RUnlock()

	for _, user := range users {
		if user.ID == id {
			return user, nil
		}
	}
	return models.User{}, errors.New("user not found")
}

// GetUserByUsername retrieves a single user by their username.
func GetUserByUsername(username string) (models.User, error) {
	usersMutex.RLock()
	defer usersMutex.RUnlock()

	for _, user := range users {
		if user.Username == username {
			return user, nil
		}
	}
	return models.User{}, errors.New("user not found")
}
