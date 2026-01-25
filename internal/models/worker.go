package models

// Worker represents a single worker in the system.
// Note the use of pointers for optional fields to allow them to be omitted
// from JSON if they are not set.

type Worker struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Position      string   `json:"position"`
	Phone         *string  `json:"phone,omitempty"`
	HourlyRate    *float64 `json:"hourlyRate,omitempty"`
	Status        string   `json:"status"` // "active" | "dismissed"
	BirthDate     *string  `json:"birthDate,omitempty"` // Format: YYYY-MM-DD
	CreatedBy     string   `json:"createdBy"`
	CreatedByName string   `json:"createdByName"`
}
