package models

// Worker represents a worker in the organization.
type Worker struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Position      string  `json:"position"`
	Phone         *string `json:"phone,omitempty"`
	HourlyRate    *float64 `json:"hourlyRate,omitempty"`
	Status        string  `json:"status"`
	BirthDate     *string `json:"birthDate,omitempty"`
	CreatedBy     string  `json:"createdBy"`
	CreatedByName string  `json:"createdByName"`
}
