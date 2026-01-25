package database

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"project/internal/models"
)

var DB *gorm.DB

// Connect connects to the database
func Connect() {
	// In a real application, you would get these values from a config file
	dsn := "host=db user=user password=password dbname=yourdb port=5432 sslmode=disable"

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("Database connection successfully opened")

	// Migrate the schema
	if err := DB.AutoMigrate(&models.User{}, &models.Article{}); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	fmt.Println("Database migrated")

	// Create a default admin user if one doesn't exist
	var adminUser models.User
	if err := DB.First(&adminUser, "role = ?", "admin").Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			adminUser = models.User{
				Login:    "admin",
				Password: "password", // In a real app, hash this!
				Role:     "admin",
			}
			if err := DB.Create(&adminUser).Error; err != nil {
				log.Fatalf("Failed to create admin user: %v", err)
			}
			fmt.Println("Admin user created")
		} else {
			log.Fatalf("Failed to query for admin user: %v", err)
		}
	}
}
