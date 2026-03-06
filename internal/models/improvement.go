package models

import "time"

// ImprovementItem represents a bug report or improvement proposal.
type ImprovementItem struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"` // bug | improvement
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // open | done
	CreatedAt   time.Time `json:"createdAt"`
	CreatedByID string    `json:"createdById"`
	CreatedBy   string    `json:"createdBy"`
	DoneAt      time.Time `json:"doneAt,omitempty"`
	DoneByID    string    `json:"doneById,omitempty"`
	DoneBy      string    `json:"doneBy,omitempty"`
}
