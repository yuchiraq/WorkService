package models

// User represents a user in the system.
// Note: this is a simplified model. In production, passwords should be hashed.
type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Name        string `json:"name"`
	Phone       string `json:"phone,omitempty"`
	Status      string `json:"status"` // admin | user
	LastLoginAt string `json:"lastLoginAt,omitempty"`
}
