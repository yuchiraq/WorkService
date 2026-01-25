package api

import (
	"net/http"

	"project/internal/database"
	"project/internal/models"

	"github.com/gin-gonic/gin"
)

// CreateUser creates a new user
func CreateUser(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	database.DB.Create(&user)
	c.JSON(http.StatusOK, user)
}

// Login authenticates a user
func Login(c *gin.Context) {
	var user models.User
	if err := c.ShouldBind(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var foundUser models.User
	if err := database.DB.First(&foundUser, "login = ?", user.Login).Error; err != nil {
		c.Redirect(http.StatusMovedPermanently, "/login")
		return
	}

	if foundUser.Password != user.Password {
		c.Redirect(http.StatusMovedPermanently, "/login")
		return
	}

	c.Redirect(http.StatusMovedPermanently, "/dashboard")
}

// Dashboard is a placeholder for the main application page
func Dashboard(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Welcome to the dashboard!"})
}
