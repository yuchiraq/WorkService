package api

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Login handles user login.
func Login(c *gin.Context) {
	session := sessions.Default(c)
	username := c.PostForm("username")
	password := c.PostForm("password")

	// Hardcoded credentials for demonstration. In a real app, you'd check a database.
	if username == "testuser" && password == "testpass" {
		// Set user session
		session.Set("user", username)
		if err := session.Save(); err != nil {
			c.Redirect(http.StatusFound, "/login?error=Не удалось сохранить сессию")
			return
		}

		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	// On failure, redirect back to login with an error message
	c.Redirect(http.StatusFound, "/login?error=Неверные учетные данные")
}

// Logout handles user logout.
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Delete("user")
	if err := session.Save(); err != nil {
		// You might want to log this error
		// For simplicity, we redirect anyway
	}
	c.Redirect(http.StatusFound, "/login")
}
