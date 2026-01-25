package models

import "gorm.io/gorm"

// User represents a user of the system
type User struct {
	gorm.Model
	Login    string `json:"login" gorm:"unique"`
	Password string `json:"password"`
	Role     string `json:"role"` // Add Role field
}
