package storage

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"sync"

	"project/internal/models"

	"github.com/google/uuid"
)

var (
	users      []models.User
	usersMutex sync.RWMutex
	usersFile  = "storage/users.json"
)

// LoadUsers reads users.json and populates users slice.
func LoadUsers() error {
	usersMutex.Lock()
	defer usersMutex.Unlock()

	file, err := os.ReadFile(usersFile)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(file, &users); err != nil {
		return err
	}

	normalizeUsers()
	return saveUsers()
}

func normalizeUsers() {
	for i := range users {
		users[i].Username = strings.TrimSpace(users[i].Username)
		users[i].Name = strings.TrimSpace(users[i].Name)
		users[i].Phone = strings.TrimSpace(users[i].Phone)
		users[i].Status = normalizeUserStatus(users[i].Status)
	}

	if len(users) > 0 && users[0].Status == "user" {
		users[0].Status = "admin"
	}
}

func normalizeUserStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "admin":
		return "admin"
	default:
		return "user"
	}
}

// saveUsers writes the current users slice.
func saveUsers() error {
	data, err := json.MarshalIndent(users, "", "    ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll("storage", 0o755); err != nil {
		return err
	}
	return os.WriteFile(usersFile, data, 0o644)
}

func GetUsers() ([]models.User, error) {
	usersMutex.RLock()
	defer usersMutex.RUnlock()

	usersCopy := make([]models.User, len(users))
	copy(usersCopy, users)
	sort.Slice(usersCopy, func(i, j int) bool {
		return usersCopy[i].Name < usersCopy[j].Name
	})
	return usersCopy, nil
}

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

func usernameExists(username, excludeUserID string) bool {
	for _, user := range users {
		if user.Username == username && user.ID != excludeUserID {
			return true
		}
	}
	return false
}

func CreateUser(user models.User) (models.User, error) {
	usersMutex.Lock()
	defer usersMutex.Unlock()

	user.Username = strings.TrimSpace(user.Username)
	user.Name = strings.TrimSpace(user.Name)
	user.Phone = strings.TrimSpace(user.Phone)
	user.Status = normalizeUserStatus(user.Status)
	if user.Username == "" || user.Password == "" || user.Name == "" {
		return models.User{}, errors.New("username, password and name are required")
	}
	if usernameExists(user.Username, "") {
		return models.User{}, errors.New("username already exists")
	}

	user.ID = uuid.New().String()
	users = append(users, user)
	if err := saveUsers(); err != nil {
		users = users[:len(users)-1]
		return models.User{}, err
	}
	return user, nil
}

func UpdateUser(updatedUser models.User) error {
	usersMutex.Lock()
	defer usersMutex.Unlock()

	updatedUser.Username = strings.TrimSpace(updatedUser.Username)
	updatedUser.Name = strings.TrimSpace(updatedUser.Name)
	updatedUser.Phone = strings.TrimSpace(updatedUser.Phone)
	updatedUser.Status = normalizeUserStatus(updatedUser.Status)

	if updatedUser.Username == "" || updatedUser.Password == "" || updatedUser.Name == "" {
		return errors.New("username, password and name are required")
	}
	if usernameExists(updatedUser.Username, updatedUser.ID) {
		return errors.New("username already exists")
	}

	for i, user := range users {
		if user.ID == updatedUser.ID {
			users[i] = updatedUser
			return saveUsers()
		}
	}

	return errors.New("user not found for update")
}

func DeleteUser(id string) error {
	usersMutex.Lock()
	defer usersMutex.Unlock()

	for i, user := range users {
		if user.ID == id {
			users = append(users[:i], users[i+1:]...)
			return saveUsers()
		}
	}

	return errors.New("user not found for deletion")
}
