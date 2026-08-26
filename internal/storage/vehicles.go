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

type fleetFileData struct {
	Vehicles []models.Vehicle       `json:"vehicles"`
	Records  []models.VehicleRecord `json:"records"`
}

var (
	ErrVehicleRecordNotFound      = errors.New("vehicle record not found")
	ErrVehicleRecordDeleteExpired = errors.New("vehicle record delete window expired")
)

var (
	vehicles       []models.Vehicle
	vehicleRecords []models.VehicleRecord
	vehiclesMutex  sync.RWMutex
	vehiclesFile   = "storage/vehicles.json"
)

func LoadVehicles() error {
	vehiclesMutex.Lock()
	defer vehiclesMutex.Unlock()

	vehicles = []models.Vehicle{}
	vehicleRecords = []models.VehicleRecord{}
	file, err := os.ReadFile(vehiclesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(file) == 0 {
		return nil
	}
	var data fleetFileData
	if err := json.Unmarshal(file, &data); err != nil {
		return err
	}
	vehicles = data.Vehicles
	vehicleRecords = data.Records
	return nil
}

func saveVehicles() error {
	data, err := json.MarshalIndent(fleetFileData{Vehicles: vehicles, Records: vehicleRecords}, "", "    ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll("storage", 0o755); err != nil {
		return err
	}
	return os.WriteFile(vehiclesFile, data, 0o644)
}

func GetVehicles() ([]models.Vehicle, error) {
	vehiclesMutex.RLock()
	defer vehiclesMutex.RUnlock()

	result := append([]models.Vehicle(nil), vehicles...)
	sort.SliceStable(result, func(i, j int) bool {
		left := strings.ToLower(result[i].Name + " " + result[i].RegistrationNumber)
		right := strings.ToLower(result[j].Name + " " + result[j].RegistrationNumber)
		return left < right
	})
	return result, nil
}

func GetVehicleByID(id string) (models.Vehicle, error) {
	vehiclesMutex.RLock()
	defer vehiclesMutex.RUnlock()
	for _, vehicle := range vehicles {
		if vehicle.ID == id {
			return vehicle, nil
		}
	}
	return models.Vehicle{}, errors.New("vehicle not found")
}

func CreateVehicle(vehicle models.Vehicle) (models.Vehicle, error) {
	vehiclesMutex.Lock()
	defer vehiclesMutex.Unlock()

	vehicle.Name = strings.TrimSpace(vehicle.Name)
	vehicle.RegistrationNumber = strings.ToUpper(strings.TrimSpace(vehicle.RegistrationNumber))
	if vehicle.Name == "" || vehicle.RegistrationNumber == "" {
		return models.Vehicle{}, errors.New("vehicle name and registration number are required")
	}
	for _, existing := range vehicles {
		if strings.EqualFold(existing.RegistrationNumber, vehicle.RegistrationNumber) {
			return models.Vehicle{}, errors.New("vehicle registration number already exists")
		}
	}
	vehicle.ID = uuid.New().String()
	vehicle.CreatedAt = time.Now().Format(time.RFC3339)
	vehicles = append(vehicles, vehicle)
	if err := saveVehicles(); err != nil {
		vehicles = vehicles[:len(vehicles)-1]
		return models.Vehicle{}, err
	}
	return vehicle, nil
}

func AssignVehicle(vehicleID, userID string) error {
	vehiclesMutex.Lock()
	defer vehiclesMutex.Unlock()
	for i := range vehicles {
		if vehicles[i].ID == vehicleID {
			vehicles[i].AssignedUserID = strings.TrimSpace(userID)
			return saveVehicles()
		}
	}
	return errors.New("vehicle not found")
}

func GetVehicleRecords(vehicleID string) ([]models.VehicleRecord, error) {
	vehiclesMutex.RLock()
	defer vehiclesMutex.RUnlock()

	result := make([]models.VehicleRecord, 0)
	for _, record := range vehicleRecords {
		if record.VehicleID == vehicleID {
			result = append(result, record)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Date == result[j].Date {
			return result[i].CreatedAt > result[j].CreatedAt
		}
		return result[i].Date > result[j].Date
	})
	return result, nil
}

func GetVehicleRecord(vehicleID, recordID string) (models.VehicleRecord, error) {
	vehiclesMutex.RLock()
	defer vehiclesMutex.RUnlock()
	for _, record := range vehicleRecords {
		if record.VehicleID == vehicleID && record.ID == recordID {
			return record, nil
		}
	}
	return models.VehicleRecord{}, ErrVehicleRecordNotFound
}

func CreateVehicleRecord(record models.VehicleRecord) (models.VehicleRecord, error) {
	vehiclesMutex.Lock()
	defer vehiclesMutex.Unlock()

	found := false
	for _, vehicle := range vehicles {
		if vehicle.ID == record.VehicleID {
			found = true
			break
		}
	}
	if !found {
		return models.VehicleRecord{}, errors.New("vehicle not found")
	}
	record.Type = strings.ToLower(strings.TrimSpace(record.Type))
	if record.Type != "fuel" && record.Type != "mileage" && record.Type != "maintenance" {
		return models.VehicleRecord{}, errors.New("invalid vehicle record type")
	}
	if _, err := time.Parse("2006-01-02", record.Date); err != nil {
		return models.VehicleRecord{}, errors.New("invalid record date")
	}
	if record.Mileage < 0 || record.Liters < 0 || record.Amount < 0 {
		return models.VehicleRecord{}, errors.New("record values cannot be negative")
	}
	if record.Type == "mileage" && record.Mileage == 0 {
		return models.VehicleRecord{}, errors.New("mileage is required")
	}
	if record.Type == "fuel" && record.Liters == 0 {
		return models.VehicleRecord{}, errors.New("fuel liters are required")
	}
	if record.Type != "fuel" {
		record.Liters = 0
		record.Amount = 0
	}
	record.Notes = strings.TrimSpace(record.Notes)
	if record.Type == "maintenance" && record.Notes == "" {
		return models.VehicleRecord{}, errors.New("maintenance description is required")
	}
	record.ID = uuid.New().String()
	record.CreatedAt = time.Now().Format(time.RFC3339)
	vehicleRecords = append(vehicleRecords, record)
	if err := saveVehicles(); err != nil {
		vehicleRecords = vehicleRecords[:len(vehicleRecords)-1]
		return models.VehicleRecord{}, err
	}
	return record, nil
}

func DeleteVehicleRecord(vehicleID, recordID string, now time.Time) error {
	vehiclesMutex.Lock()
	defer vehiclesMutex.Unlock()

	for index, record := range vehicleRecords {
		if record.VehicleID != vehicleID || record.ID != recordID {
			continue
		}
		createdAt, err := time.Parse(time.RFC3339, record.CreatedAt)
		if err != nil || now.After(createdAt.Add(24*time.Hour)) {
			return ErrVehicleRecordDeleteExpired
		}

		removed := record
		vehicleRecords = append(vehicleRecords[:index], vehicleRecords[index+1:]...)
		if err := saveVehicles(); err != nil {
			vehicleRecords = append(vehicleRecords, models.VehicleRecord{})
			copy(vehicleRecords[index+1:], vehicleRecords[index:])
			vehicleRecords[index] = removed
			return err
		}
		return nil
	}
	return ErrVehicleRecordNotFound
}

func HasMileageRecordForMonth(vehicleID, month string) bool {
	vehiclesMutex.RLock()
	defer vehiclesMutex.RUnlock()
	for _, record := range vehicleRecords {
		if record.VehicleID == vehicleID && record.Type == "mileage" && strings.HasPrefix(record.Date, month+"-") {
			return true
		}
	}
	return false
}

func MarkVehicleMileageReminder(vehicleID, month string) error {
	vehiclesMutex.Lock()
	defer vehiclesMutex.Unlock()
	for i := range vehicles {
		if vehicles[i].ID == vehicleID {
			vehicles[i].LastMileageReminderMonth = month
			return saveVehicles()
		}
	}
	return errors.New("vehicle not found")
}
