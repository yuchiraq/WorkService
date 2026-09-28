package storage

import (
	"path/filepath"
	"project/internal/models"
	"testing"
)

func TestAbsencePeriodLifecycle(t *testing.T) {
	oldFile, oldEntries, oldWorkers := timesheetsFile, timesheets, workers
	t.Cleanup(func() { timesheetsFile, timesheets, workers = oldFile, oldEntries, oldWorkers })
	timesheetsFile = filepath.Join(t.TempDir(), "timesheets.json")
	timesheets = nil
	workers = []models.Worker{{ID: "w"}}
	for _, mark := range []string{"Б", "ПР", "ОТ"} {
		entry, err := CreateTimesheet(models.TimesheetEntry{Date: "2026-01-30", PeriodEnd: "2026-02-04", UserMark: mark, WorkerIDs: []string{"w"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(timesheets) != 1 {
			t.Fatal("period was split into daily records")
		}
		entry.PeriodEnd = "2026-02-06"
		if err = UpdateTimesheet(entry); err != nil {
			t.Fatal(err)
		}
		if err = LoadTimesheets(); err != nil {
			t.Fatal(err)
		}
		if timesheets[0].PeriodEnd != entry.PeriodEnd {
			t.Fatal("period not persisted")
		}
		if err = DeleteTimesheet(entry.ID); err != nil {
			t.Fatal(err)
		}
	}
	for _, end := range []string{"invalid", "2026-01-01", "2028-01-01"} {
		_, err := CreateTimesheet(models.TimesheetEntry{Date: "2026-01-30", PeriodEnd: end, UserMark: "ПР", WorkerIDs: []string{"w"}})
		if err == nil || len(timesheets) != 0 {
			t.Fatalf("invalid period saved: %s", end)
		}
	}
}
