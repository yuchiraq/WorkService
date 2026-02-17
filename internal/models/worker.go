package models

// Worker represents a worker in the system.
type Worker struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Position      string  `json:"position"`
	Phone         string  `json:"phone,omitempty"`
	HourlyRate    float64 `json:"hourlyRate,omitempty"`
	BirthDate     string  `json:"birthDate,omitempty"`
	CreatedBy     string  `json:"createdBy"`
	CreatedByName string  `json:"createdByName"`
	UserID        string  `json:"userId,omitempty"`
	IsFired       bool    `json:"isFired,omitempty"`
	FiredAt       string  `json:"firedAt,omitempty"`
}
