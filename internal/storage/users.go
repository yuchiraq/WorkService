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
	usersFile  = "storage/users.json" // Corrected path
)

// LoadUsers reads the users.json file and populates the users slice.
func LoadUsers() error {
	usersMutex.Lock()
	defer usersMutex.Unlock()

	file, err := os.ReadFile(usersFile)
	if err != nil {
		if os.IsNotExist(err) {
			users = []models.User{
				{ID: "1", Username: "testuser", Password: "password", Name: "Test User"},
			}
			return saveUsers()
		}
		return err
	}

	return json.Unmarshal(file, &users)
}

// saveUsers writes the current state of the users slice to the users.json file.
func saveUsers() error {
	data, err := json.MarshalIndent(users, "", "    ")
	if err != nil {
		return err
	}
	// Ensure the directory exists
	if err := os.MkdirAll("storage", 0755); err != nil {
		return err
	}
	return os.WriteFile(usersFile, data, 0644)
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

// ValidateUser checks if a username and password combination is valid.
func ValidateUser(username, password string) (models.User, error) {
	usersMutex.RLock()
	defer usersMutex.RUnlock()

	for _, user := range users {
		if user.Username == username && user.Password == password {
			return user, nil
		}
	}
	return models.User{}, errors.New("invalid credentials")
}
