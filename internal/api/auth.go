package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Login handles user login
func Login(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Login endpoint"})
}
