package api

import (
	"project/internal/models"
	"testing"
)

func TestAbsencePeriodsAndMonthlyWorkers(t *testing.T) {
	entries := []models.TimesheetEntry{
		{ID: "period", Date: "2026-01-30", PeriodEnd: "2026-02-03", UserMark: "ПР", WorkerIDs: []string{"fired"}},
		{ID: "work", Date: "2026-02-10", WorkerIDs: []string{"active"}},
		{ID: "old", Date: "2026-01-02", WorkerIDs: []string{"idle"}},
	}
	workers := []models.Worker{{ID: "fired", Name: "Борис", IsFired: true}, {ID: "active", Name: "Антон"}, {ID: "idle", Name: "Виктор"}}
	visible := workersWithEntries(workers, entries, "2026-02")
	if len(visible) != 2 || visible[0].ID != "active" || visible[1].ID != "fired" {
		t.Fatalf("workers: %+v", visible)
	}
	expanded := monthEntries(entries, "2026-02")
	if len(expanded) != 4 || expanded[0].Date != "2026-02-01" || expanded[2].Date != "2026-02-03" {
		t.Fatalf("expanded: %+v", expanded)
	}
	if entries[0].Date != "2026-01-30" {
		t.Fatal("modified persisted entry")
	}
	boundaries := scheduleBoundaries(entries[:1])
	if len(boundaries) != 2 || boundaries[0].Date != "2026-01-30" || boundaries[1].Date != "2026-02-03" || boundaries[0].ID != boundaries[1].ID {
		t.Fatalf("boundaries: %+v", boundaries)
	}
	if len(monthEntries(entries, "2026-03")) != 0 {
		t.Fatal("entries leaked into empty month")
	}
}

func TestOneDayAbsenceHasOneBoundary(t *testing.T) {
	entry := models.TimesheetEntry{Date: "2026-02-01", PeriodEnd: "2026-02-01", UserMark: "Б"}
	if len(scheduleBoundaries([]models.TimesheetEntry{entry})) != 1 {
		t.Fatal("duplicate boundary")
	}
}
