package main

import (
	"log"
	"net/http"

	"project/internal/database"
	"project/internal/router"

	"github.com/gin-gonic/gin"
)

func main() {
	// Connect to the database
	database.Connect()

	r := gin.Default()

	// Serve static files
	r.Static("/static", "./web/static")

	// Load HTML templates
	r.LoadHTMLGlob("web/templates/*")

	// Redirect root to login
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/login")
	})

	// Login route
	r.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", nil)
	})

	router.SetupRouter(r)

	log.Println("Запуск HTTP-сервера на порту 8099")
	if err := r.Run(":8099"); err != nil {
		log.Fatalf("Ошибка запуска HTTP-сервера: %v", err)
	}
}
