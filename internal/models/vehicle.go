package models

// Vehicle represents a vehicle that can be assigned to a user account.
type Vehicle struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	RegistrationNumber       string `json:"registrationNumber"`
	AssignedUserID           string `json:"assignedUserId,omitempty"`
	CreatedByUserID          string `json:"createdByUserId"`
	CreatedByName            string `json:"createdByName"`
	CreatedAt                string `json:"createdAt"`
	LastMileageReminderMonth string `json:"lastMileageReminderMonth,omitempty"`
}

// VehicleRecord stores fuel, odometer and maintenance events for a vehicle.
type VehicleRecord struct {
	ID            string  `json:"id"`
	VehicleID     string  `json:"vehicleId"`
	Type          string  `json:"type"` // fuel | mileage | maintenance
	Date          string  `json:"date"`
	Mileage       int     `json:"mileage,omitempty"`
	Liters        float64 `json:"liters,omitempty"`
	Amount        float64 `json:"amount,omitempty"`
	Notes         string  `json:"notes,omitempty"`
	CreatedByID   string  `json:"createdById"`
	CreatedByName string  `json:"createdByName"`
	CreatedAt     string  `json:"createdAt"`
}
