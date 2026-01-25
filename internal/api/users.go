package api

import (
	"net/http"

	"project/internal/models"
	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

func CreateUser(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	users, err := storage.ReadUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read users"})
		return
	}

	users = append(users, user)

	if err := storage.WriteUsers(users); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write users"})
		return
	}

	c.JSON(http.StatusCreated, user)
}
