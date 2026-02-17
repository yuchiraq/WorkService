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
	objects      []models.Object
	objectsMutex sync.RWMutex
	objectsFile  = "storage/objects.json"
)

func LoadObjects() error {
	objectsMutex.Lock()
	defer objectsMutex.Unlock()

	file, err := os.ReadFile(objectsFile)
	if err != nil {
		if os.IsNotExist(err) {
			objects = []models.Object{}
			return saveObjects()
		}
		return err
	}

	if err := json.Unmarshal(file, &objects); err != nil {
		return err
	}

	for i := range objects {
		objects[i].Status = NormalizeObjectStatus(objects[i].Status)
		objects[i].Name = strings.TrimSpace(objects[i].Name)
		objects[i].Address = strings.TrimSpace(objects[i].Address)
	}

	return saveObjects()
}

func saveObjects() error {
	data, err := json.MarshalIndent(objects, "", "    ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll("storage", 0o755); err != nil {
		return err
	}
	return os.WriteFile(objectsFile, data, 0o644)
}

func NormalizeObjectStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "paused":
		return "paused"
	case "completed":
		return "completed"
	default:
		return "in_progress"
	}
}

func GetObjects() ([]models.Object, error) {
	objectsMutex.RLock()
	defer objectsMutex.RUnlock()

	objectsCopy := make([]models.Object, len(objects))
	copy(objectsCopy, objects)
	sort.Slice(objectsCopy, func(i, j int) bool {
		return objectsCopy[i].Name < objectsCopy[j].Name
	})

	return objectsCopy, nil
}

func GetObjectByID(id string) (models.Object, error) {
	objectsMutex.RLock()
	defer objectsMutex.RUnlock()

	for _, object := range objects {
		if object.ID == id {
			return object, nil
		}
	}

	return models.Object{}, errors.New("object not found")
}

func CreateObject(object models.Object) (models.Object, error) {
	objectsMutex.Lock()
	defer objectsMutex.Unlock()

	object.Name = strings.TrimSpace(object.Name)
	object.Address = strings.TrimSpace(object.Address)
	object.ResponsibleUserID = strings.TrimSpace(object.ResponsibleUserID)
	object.Status = NormalizeObjectStatus(object.Status)

	if object.Name == "" || object.Address == "" || object.ResponsibleUserID == "" {
		return models.Object{}, errors.New("name, address and responsible user are required")
	}

	object.ID = uuid.New().String()
	objects = append(objects, object)
	if err := saveObjects(); err != nil {
		objects = objects[:len(objects)-1]
		return models.Object{}, err
	}

	return object, nil
}

func UpdateObject(updatedObject models.Object) error {
	objectsMutex.Lock()
	defer objectsMutex.Unlock()

	updatedObject.Name = strings.TrimSpace(updatedObject.Name)
	updatedObject.Address = strings.TrimSpace(updatedObject.Address)
	updatedObject.ResponsibleUserID = strings.TrimSpace(updatedObject.ResponsibleUserID)
	updatedObject.Status = NormalizeObjectStatus(updatedObject.Status)

	if updatedObject.Name == "" || updatedObject.Address == "" || updatedObject.ResponsibleUserID == "" {
		return errors.New("name, address and responsible user are required")
	}

	for i, object := range objects {
		if object.ID == updatedObject.ID {
			objects[i] = updatedObject
			return saveObjects()
		}
	}

	return errors.New("object not found for update")
}

func DeleteObject(id string) error {
	objectsMutex.Lock()
	defer objectsMutex.Unlock()

	for i, object := range objects {
		if object.ID == id {
			objects = append(objects[:i], objects[i+1:]...)
			return saveObjects()
		}
	}

	return errors.New("object not found for deletion")
}
