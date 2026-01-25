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

	r := gin.Default()

	// Use the simpler template loading method. This allows templates to be included in each other.
	r.LoadHTMLGlob("web/templates/*.html")

	// Setup all routes from the router package
	router.SetupRouter(r)

	log.Println("Starting HTTP server on port 8099")
	if err := r.Run(":8099"); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
