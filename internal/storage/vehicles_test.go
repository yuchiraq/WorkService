package storage

import (
	"path/filepath"
	"testing"

	"project/internal/models"
)

func TestVehicleLifecycleAndRecords(t *testing.T) {
	oldFile := vehiclesFile
	oldVehicles := vehicles
	oldRecords := vehicleRecords
	defer func() {
		vehiclesFile = oldFile
		vehicles = oldVehicles
		vehicleRecords = oldRecords
	}()

	vehiclesFile = filepath.Join(t.TempDir(), "vehicles.json")
	vehicles = nil
	vehicleRecords = nil
	if err := LoadVehicles(); err != nil {
		t.Fatalf("LoadVehicles() error = %v", err)
	}

	vehicle, err := CreateVehicle(models.Vehicle{Name: "Ford Transit", RegistrationNumber: "а123вс-7", CreatedByUserID: "admin"})
	if err != nil {
		t.Fatalf("CreateVehicle() error = %v", err)
	}
	if vehicle.RegistrationNumber != "А123ВС-7" {
		t.Fatalf("registration number = %q", vehicle.RegistrationNumber)
	}
	if _, err := CreateVehicle(models.Vehicle{Name: "Duplicate", RegistrationNumber: "А123ВС-7"}); err == nil {
		t.Fatal("CreateVehicle() accepted a duplicate registration number")
	}

	if err := AssignVehicle(vehicle.ID, "user-1"); err != nil {
		t.Fatalf("AssignVehicle() error = %v", err)
	}
	assigned, err := GetVehicleByID(vehicle.ID)
	if err != nil || assigned.AssignedUserID != "user-1" {
		t.Fatalf("assigned vehicle = %#v, error = %v", assigned, err)
	}

	if _, err := CreateVehicleRecord(models.VehicleRecord{VehicleID: vehicle.ID, Type: "fuel", Date: "2026-08-01"}); err == nil {
		t.Fatal("CreateVehicleRecord() accepted fuel without liters")
	}
	mileageRecord, err := CreateVehicleRecord(models.VehicleRecord{VehicleID: vehicle.ID, Type: "mileage", Date: "2026-08-01", Mileage: 125400, Liters: 50, Amount: 120})
	if err != nil {
		t.Fatalf("CreateVehicleRecord() error = %v", err)
	}
	if mileageRecord.Liters != 0 || mileageRecord.Amount != 0 {
		t.Fatalf("mileage record kept fuel values: %#v", mileageRecord)
	}
	maintenanceRecord, err := CreateVehicleRecord(models.VehicleRecord{VehicleID: vehicle.ID, Type: "maintenance", Date: "2026-08-02", Mileage: 125500, Liters: 10, Amount: 90, Notes: "Замена масла"})
	if err != nil {
		t.Fatalf("maintenance CreateVehicleRecord() error = %v", err)
	}
	if maintenanceRecord.Liters != 0 || maintenanceRecord.Amount != 0 {
		t.Fatalf("maintenance record kept fuel values: %#v", maintenanceRecord)
	}
	if !HasMileageRecordForMonth(vehicle.ID, "2026-08") {
		t.Fatal("HasMileageRecordForMonth() = false")
	}

	if err := MarkVehicleMileageReminder(vehicle.ID, "2026-08"); err != nil {
		t.Fatalf("MarkVehicleMileageReminder() error = %v", err)
	}
	updated, _ := GetVehicleByID(vehicle.ID)
	if updated.LastMileageReminderMonth != "2026-08" {
		t.Fatalf("reminder month = %q", updated.LastMileageReminderMonth)
	}
}
