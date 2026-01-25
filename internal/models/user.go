package models

import "gorm.io/gorm"

// User represents a user of the system
type User struct {
	gorm.Model
	Name     string `json:"name"`
	Email    string `json:"email" gorm:"unique"`
	Password string `json:"password"`
	Role     string `json:"role"` // Add Role field
}
