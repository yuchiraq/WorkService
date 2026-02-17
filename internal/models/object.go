package models

// Object represents a construction object/project managed in the system.
type Object struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Status            string `json:"status"` // in_progress | paused | completed
	Address           string `json:"address"`
	ResponsibleUserID string `json:"responsibleUserId"`
}
