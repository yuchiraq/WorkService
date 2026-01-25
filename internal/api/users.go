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

	// In a real application, you would save the user to a database here.

	c.JSON(http.StatusOK, user)
}

// GetUser retrieves a user by ID
func GetUser(c *gin.Context) {
	// In a real application, you would retrieve the user from a database here.
	c.JSON(http.StatusOK, gin.H{"message": "GetUser not implemented"})
}

// UpdateUser updates a user's information
func UpdateUser(c *gin.Context) {
	// In a real application, you would update the user in a database here.
	c.JSON(http.StatusOK, gin.H{"message": "UpdateUser not implemented"})
}

// Login authenticates a user
func Login(c *gin.Context) {
	var user models.User
	if err := c.ShouldBind(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var foundUser models.User
	if err := database.DB.First(&foundUser, "email = ?", user.Email).Error; err != nil {
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
