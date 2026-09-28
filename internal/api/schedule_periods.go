package api

import (
	"sort"
	"strings"
	"time"

	"project/internal/models"
)

func entryEnd(entry models.TimesheetEntry) string {
	if isSpecialMark(entry.UserMark) && entry.PeriodEnd > entry.Date {
		return entry.PeriodEnd
	}
	return entry.Date
}

func entryInMonth(entry models.TimesheetEntry, month string) bool {
	return entry.Date < month+"-32" && entryEnd(entry) >= month+"-01"
}

// Expand only the selected month; persisted periods remain one editable record.
func monthEntries(entries []models.TimesheetEntry, month string) []models.TimesheetEntry {
	_, start, days := resolveSelectedMonth(month)
	result := make([]models.TimesheetEntry, 0)
	for _, entry := range entries {
		if !entryInMonth(entry, month) {
			continue
		}
		for day := 0; day < days; day++ {
			date := start.AddDate(0, 0, day).Format("2006-01-02")
			if date >= entry.Date && date <= entryEnd(entry) {
				copy := entry
				copy.Date = date
				result = append(result, copy)
			}
		}
	}
	return result
}

func workersWithEntries(workers []models.Worker, entries []models.TimesheetEntry, month string) []models.Worker {
	ids := map[string]bool{}
	for _, entry := range entries {
		if entryInMonth(entry, month) {
			for _, id := range entry.WorkerIDs {
				ids[id] = true
			}
		}
	}
	result := make([]models.Worker, 0)
	for _, worker := range workers {
		if ids[worker.ID] {
			result = append(result, worker)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result
}

func scheduleBoundaries(entries []models.TimesheetEntry) []models.TimesheetEntry {
	result := make([]models.TimesheetEntry, 0, len(entries))
	for _, entry := range entries {
		if entryEnd(entry) == entry.Date {
			result = append(result, entry)
			continue
		}
		start, _ := time.Parse("2006-01-02", entry.Date)
		end, _ := time.Parse("2006-01-02", entry.PeriodEnd)
		period := start.Format("02.01.2006") + " – " + end.Format("02.01.2006")
		first, last := entry, entry
		first.Notes = strings.TrimSpace("Начало: " + period + ". " + entry.Notes)
		last.Date = entry.PeriodEnd
		last.Notes = strings.TrimSpace("Окончание: " + period + ". " + entry.Notes)
		result = append(result, first, last)
	}
	return result
}
