package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"project/internal/models"
)

var (
	improvementsFile  = "storage/improvements.json"
	improvements      []models.ImprovementItem
	improvementsMutex sync.Mutex
)

func LoadImprovements() error {
	improvementsMutex.Lock()
	defer improvementsMutex.Unlock()

	data, err := readData(improvementsFile)
	if os.IsNotExist(err) {
		improvements = []models.ImprovementItem{}
		return saveImprovementsLocked()
	}

	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		improvements = []models.ImprovementItem{}
		return nil
	}

	if err := json.Unmarshal(data, &improvements); err != nil {
		return err
	}
	return nil
}

func saveImprovementsLocked() error {
	data, err := json.MarshalIndent(improvements, "", "    ")
	if err != nil {
		return err
	}
	return writeData(improvementsFile, data)
}

func GetImprovements() ([]models.ImprovementItem, error) {
	improvementsMutex.Lock()
	defer improvementsMutex.Unlock()

	result := make([]models.ImprovementItem, len(improvements))
	copy(result, improvements)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Status != result[j].Status {
			return result[i].Status == "open"
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func AddImprovement(item models.ImprovementItem) error {
	improvementsMutex.Lock()
	defer improvementsMutex.Unlock()

	if strings.TrimSpace(item.ID) == "" {
		item.ID = fmt.Sprintf("imp-%d", time.Now().UnixNano())
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	if item.Status == "" {
		item.Status = "open"
	}
	improvements = append(improvements, item)
	if err := saveImprovementsLocked(); err != nil {
		improvements = improvements[:len(improvements)-1]
		return err
	}
	return nil
}

func MarkImprovementDone(id, doneByID, doneBy string) error {
	improvementsMutex.Lock()
	defer improvementsMutex.Unlock()

	for i := range improvements {
		if improvements[i].ID == id {
			previous := improvements[i]
			improvements[i].Status = "done"
			improvements[i].DoneAt = time.Now()
			improvements[i].DoneByID = doneByID
			improvements[i].DoneBy = doneBy
			if err := saveImprovementsLocked(); err != nil {
				improvements[i] = previous
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("improvement not found")
}
