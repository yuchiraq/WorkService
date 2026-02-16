package storage

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"project/internal/models"

	"github.com/google/uuid"
)

var (
	timesheets      []models.TimesheetEntry
	timesheetsMutex sync.RWMutex
	timesheetsFile  = "storage/timesheets.json"
)

func LoadTimesheets() error {
	timesheetsMutex.Lock()
	defer timesheetsMutex.Unlock()

	file, err := os.ReadFile(timesheetsFile)
	if err != nil {
		if os.IsNotExist(err) {
			timesheets = []models.TimesheetEntry{}
			return saveTimesheets()
		}
		return err
	}

	if err := json.Unmarshal(file, &timesheets); err != nil {
		return err
	}

	for i := range timesheets {
		normalizeTimesheet(&timesheets[i])
	}

	return saveTimesheets()
}

func saveTimesheets() error {
	data, err := json.MarshalIndent(timesheets, "", "    ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll("storage", 0o755); err != nil {
		return err
	}
	return os.WriteFile(timesheetsFile, data, 0o644)
}

func normalizeTimesheet(entry *models.TimesheetEntry) {
	entry.Date = strings.TrimSpace(entry.Date)
	entry.StartTime = strings.TrimSpace(entry.StartTime)
	entry.EndTime = strings.TrimSpace(entry.EndTime)
	entry.Notes = strings.TrimSpace(entry.Notes)
	entry.LunchBreakMinutes = normalizeLunchBreak(entry.LunchBreakMinutes)
	entry.WorkerIDs = cleanStringSlice(entry.WorkerIDs)
	entry.ObjectIDs = cleanStringSlice(entry.ObjectIDs)
}

func normalizeLunchBreak(minutes int) int {
	switch minutes {
	case 30, 60, 90:
		return minutes
	default:
		return 60
	}
}

func cleanStringSlice(values []string) []string {
	set := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := set[trimmed]; exists {
			continue
		}
		set[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func validateTimesheet(entry models.TimesheetEntry) error {
	if entry.Date == "" {
		return errors.New("date is required")
	}
	if _, err := time.Parse("2006-01-02", entry.Date); err != nil {
		return errors.New("invalid date format")
	}
	start, err := time.Parse("15:04", entry.StartTime)
	if err != nil {
		return errors.New("invalid start time")
	}
	end, err := time.Parse("15:04", entry.EndTime)
	if err != nil {
		return errors.New("invalid end time")
	}
	if !end.After(start) {
		return errors.New("end time must be after start time")
	}

	totalMinutes := int(end.Sub(start).Minutes())
	if entry.LunchBreakMinutes >= totalMinutes {
		return errors.New("lunch break must be shorter than work interval")
	}
	if len(entry.WorkerIDs) == 0 {
		return errors.New("at least one worker is required")
	}
	if len(entry.ObjectIDs) == 0 {
		return errors.New("at least one object is required")
	}

	for _, workerID := range entry.WorkerIDs {
		if _, err := GetWorkerByID(workerID); err != nil {
			return errors.New("one of selected workers does not exist")
		}
	}
	for _, objectID := range entry.ObjectIDs {
		if _, err := GetObjectByID(objectID); err != nil {
			return errors.New("one of selected objects does not exist")
		}
	}

	return nil
}

func GetTimesheets() ([]models.TimesheetEntry, error) {
	timesheetsMutex.RLock()
	defer timesheetsMutex.RUnlock()

	copyTimesheets := make([]models.TimesheetEntry, len(timesheets))
	copy(copyTimesheets, timesheets)
	sort.Slice(copyTimesheets, func(i, j int) bool {
		if copyTimesheets[i].Date == copyTimesheets[j].Date {
			return copyTimesheets[i].StartTime < copyTimesheets[j].StartTime
		}
		return copyTimesheets[i].Date > copyTimesheets[j].Date
	})
	return copyTimesheets, nil
}

func GetTimesheetByID(id string) (models.TimesheetEntry, error) {
	timesheetsMutex.RLock()
	defer timesheetsMutex.RUnlock()

	for _, entry := range timesheets {
		if entry.ID == id {
			return entry, nil
		}
	}
	return models.TimesheetEntry{}, errors.New("timesheet entry not found")
}

func CreateTimesheet(entry models.TimesheetEntry) (models.TimesheetEntry, error) {
	timesheetsMutex.Lock()
	defer timesheetsMutex.Unlock()

	normalizeTimesheet(&entry)
	if err := validateTimesheet(entry); err != nil {
		return models.TimesheetEntry{}, err
	}

	entry.ID = uuid.New().String()
	timesheets = append(timesheets, entry)
	if err := saveTimesheets(); err != nil {
		timesheets = timesheets[:len(timesheets)-1]
		return models.TimesheetEntry{}, err
	}
	return entry, nil
}

func UpdateTimesheet(entry models.TimesheetEntry) error {
	timesheetsMutex.Lock()
	defer timesheetsMutex.Unlock()

	normalizeTimesheet(&entry)
	if err := validateTimesheet(entry); err != nil {
		return err
	}

	for i := range timesheets {
		if timesheets[i].ID == entry.ID {
			timesheets[i] = entry
			return saveTimesheets()
		}
	}

	return errors.New("timesheet entry not found")
}

func DeleteTimesheet(id string) error {
	timesheetsMutex.Lock()
	defer timesheetsMutex.Unlock()

	for i := range timesheets {
		if timesheets[i].ID == id {
			timesheets = append(timesheets[:i], timesheets[i+1:]...)
			return saveTimesheets()
		}
	}
	return errors.New("timesheet entry not found")
}
