package api

import (
	"net/http"

	"project/internal/storage"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// LoginPage renders the login page.
func LoginPage(c *gin.Context) {
	// This page does not use the main layout
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "Вход",
		"error": c.Query("error"),
	})
}

// Login handles the authentication logic.
func Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	user, err := storage.ValidateUser(username, password)
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=Неверный логин или пароль")
		return
	}

	// Set session
	session := sessions.Default(c)
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
	session.Clear()
	_ = session.Save() // Try to save but ignore errors on logout
	c.Redirect(http.StatusFound, "/login")
}
