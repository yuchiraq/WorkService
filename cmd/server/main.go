package main

import (
	"log"

	"project/internal/router"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	// Setup all routes from the router package
	router.SetupRouter(r)

	log.Println("Starting HTTP server on port 8099")
	if err := r.Run(":8099"); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
