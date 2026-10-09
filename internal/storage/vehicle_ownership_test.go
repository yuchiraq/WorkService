package storage

import (
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"project/internal/models"
)

func TestVehicleOwnershipHistory(t *testing.T) {
	oldFile, oldVehicles, oldRecords := vehiclesFile, vehicles, vehicleRecords
	t.Cleanup(func() { vehiclesFile, vehicles, vehicleRecords = oldFile, oldVehicles, oldRecords })
	vehiclesFile = filepath.Join(t.TempDir(), "vehicles.json")
	if err := LoadVehicles(); err != nil {
		t.Fatal(err)
	}
	vehicle, err := CreateVehicle(models.Vehicle{Name: "Test", RegistrationNumber: "1000 AA-7", CreatedByUserID: "admin", CreatedByName: "Admin"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, id := range []string{"one", "two"} {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			results <- AssignVehicle(vehicle.ID, "", userID, "admin", "Admin")
		}(id)
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrVehicleOwnerConflict) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatal("concurrent claims overwrote one another")
	}
	current, _ := GetVehicleByID(vehicle.ID)
	if len(current.OwnerHistory) != 2 || current.OwnerHistory[1].ActorName != "Admin" {
		t.Fatalf("missing history: %+v", current)
	}
	if err := AssignVehicle(vehicle.ID, current.AssignedUserID, "third", "admin", "Admin"); !errors.Is(err, ErrVehicleAlreadyAssigned) {
		t.Fatal("direct reassignment accepted", err)
	}
	if err := AssignVehicle(vehicle.ID, current.AssignedUserID, "", "admin", "Admin"); err != nil {
		t.Fatal(err)
	}
	if err := AssignVehicle(vehicle.ID, "", "third", "admin", "Admin"); err != nil {
		t.Fatal(err)
	}
	if err := AssignVehicle(vehicle.ID, current.AssignedUserID, "", "admin", "Admin"); !errors.Is(err, ErrVehicleOwnerConflict) {
		t.Fatal("stale unassign accepted", err)
	}
	current, _ = GetVehicleByID(vehicle.ID)
	if len(current.OwnerHistory) != 4 || current.OwnerHistory[2].PreviousUserID == "" || current.OwnerHistory[2].UserID != "" {
		t.Fatal("unassignment not recorded", current)
	}
	copy, _ := GetVehicleByID(vehicle.ID)
	copy.OwnerHistory[1].UserID = "mutated"
	all, _ := GetVehicles()
	all[0].OwnerHistory[1].UserID = "mutated"
	if got, _ := GetVehicleByID(vehicle.ID); !reflect.DeepEqual(got, current) {
		t.Fatal("history exposed mutable storage")
	}
	if err := LoadVehicles(); err != nil {
		t.Fatal(err)
	}
	if got, _ := GetVehicleByID(vehicle.ID); !reflect.DeepEqual(got, current) {
		t.Fatal("history not persisted")
	}
	vehiclesFile = filepath.Join(vehiclesFile, "invalid.json")
	if err := AssignVehicle(vehicle.ID, "third", "", "admin", "Admin"); err == nil {
		t.Fatal("expected save failure")
	}
	if got, _ := GetVehicleByID(vehicle.ID); !reflect.DeepEqual(got, current) {
		t.Fatal("failed save changed owner/history")
	}
}
