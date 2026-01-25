package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Login handles user login.
func Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	// Hardcoded credentials for demonstration
	if username == "testuser" && password == "testpass" {
		// In a real application, you would set a session cookie here.
		c.Redirect(http.StatusFound, "/workers")
		return
	}

	// On failure, redirect back to login with an error message
	c.Redirect(http.StatusFound, "/login?error=Invalid+credentials")
}

// Dashboard renders the dashboard page.
func Dashboard(c *gin.Context) {
	c.HTML(http.StatusOK, "dashboard.html", nil)
}
