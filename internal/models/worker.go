package models

import "time"

// Worker represents a worker in the system.
type Worker struct {
	ID            string    `json:"id"`
	FirstName     string    `json:"firstName"`
	LastName      string    `json:"lastName"`
	Position      string    `json:"position"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt,omitempty"`
	CreatedBy     string    `json:"createdBy"`
	CreatedByName string    `json:"createdByName"` // e.g., "Admin User"
}
