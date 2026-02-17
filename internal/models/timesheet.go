package models

// TimesheetEntry describes a daily assignment for workers on objects.
type TimesheetEntry struct {
	ID                string   `json:"id"`
	Date              string   `json:"date"` // YYYY-MM-DD
	StartTime         string   `json:"startTime"`
	EndTime           string   `json:"endTime"`
	LunchBreakMinutes int      `json:"lunchBreakMinutes"` // 30 | 60 | 90
	WorkerIDs         []string `json:"workerIds"`
	ObjectIDs         []string `json:"objectIds"`
	Notes             string   `json:"notes,omitempty"`
	UserMark          string   `json:"userMark,omitempty"`
}
