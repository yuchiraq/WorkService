package api

import (
	"project/internal/models"
	"testing"
)

func TestBusyWorkersOnDate(t *testing.T) {
	entries := []models.TimesheetEntry{
		{ID: "one", Date: "2026-10-09", WorkerIDs: []string{"one", "two"}},
		{ID: "period", Date: "2026-10-08", PeriodEnd: "2026-10-12", UserMark: "Б", WorkerIDs: []string{"three"}},
		{ID: "other", Date: "2026-10-10", WorkerIDs: []string{"four"}},
	}
	busy := busyWorkersOnDate(entries, "2026-10-09", "")
	if len(busy) != 3 || !busy["one"] || !busy["two"] || !busy["three"] {
		t.Fatal(busy)
	}
	busy = busyWorkersOnDate(entries, "2026-10-09", "one")
	if len(busy) != 1 || !busy["three"] {
		t.Fatal("editing assignment counted itself", busy)
	}
	for _, day := range []string{"2026-10-08", "2026-10-12"} {
		if !busyWorkersOnDate(entries, day, "")["three"] {
			t.Fatal("period boundary missing", day)
		}
	}
	if len(busyWorkersOnDate(entries, "2026-10-13", "")) != 0 {
		t.Fatal("old assignments carried forward")
	}
}
