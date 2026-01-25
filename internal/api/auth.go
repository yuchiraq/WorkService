package api

import (
	"net/http"

	"project/internal/storage"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Login handles user login.
func Login(c *gin.Context) {
	session := sessions.Default(c)
	username := c.PostForm("username")
	password := c.PostForm("password")

	// Find user in storage
	user, err := storage.GetUserByUsername(username)
	if err != nil || user.Password != password { // In a real app, compare hashed passwords
		c.Redirect(http.StatusFound, "/login?error=Неверные учетные данные")
		return
	}

	// Set user session
	session.Set("userID", user.ID)
	session.Set("userName", user.Name)
	if err := session.Save(); err != nil {
		c.Redirect(http.StatusFound, "/login?error=Не удалось сохранить сессию")
		return
	}

	c.Redirect(http.StatusFound, "/dashboard")
}

// Logout handles user logout.
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Delete("userID")
	session.Delete("userName")
	if err := session.Save(); err != nil {
		// Log the error, but still redirect
	}
	c.Redirect(http.StatusFound, "/login")
}
