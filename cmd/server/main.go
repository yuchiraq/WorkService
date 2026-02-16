package main

import (
	"log"

	"project/internal/router"
	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load initial data
	if err := storage.LoadUsers(); err != nil {
		log.Fatalf("Failed to load users: %v", err)
	}
	if err := storage.LoadWorkers(); err != nil {
		log.Fatalf("Failed to load workers: %v", err)
	}
	if err := storage.LoadObjects(); err != nil {
		log.Fatalf("Failed to load objects: %v", err)
	}
	if err := storage.LoadTimesheets(); err != nil {
		log.Fatalf("Failed to load timesheets: %v", err)
	}

	r := gin.Default()

	// Setup all routes from the router package
	router.SetupRouter(r)

	log.Println("Starting HTTP server on port 8099")
	if err := r.Run(":8099"); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
