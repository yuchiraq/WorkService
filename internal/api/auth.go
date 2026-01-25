package api

import (
	"net/http"
	"strings"
	"time"

	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

// LoginPage renders the login page using an HTML template.
func LoginPage(c *gin.Context) {
	// The router is configured to load templates from "web/templates/*"
	// We render the "login.html" template.
	c.HTML(http.StatusOK, "login.html", gin.H{
		"Title": "Вход в систему",
	})
}

// Login handles the authentication logic.
func Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	user, err := storage.ValidateUser(username, password)
	if err != nil {
		// Redirect back to login page with an error
		c.Redirect(http.StatusFound, "/login?error=invalid_credentials")
		return
	}

	// Set session cookie
	cookieValue := user.ID + ":" + user.Username
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "session",
		Value:    cookieValue,
		Expires:  time.Now().Add(24 * time.Hour),
		Path:     "/",
		HttpOnly: true,
	})

	c.Redirect(http.StatusFound, "/dashboard")
}

// Logout clears the session cookie and redirects to the login page.
func Logout(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "session",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour), // Set to a past time to delete
		Path:     "/",
		HttpOnly: true,
	})
	c.Redirect(http.StatusFound, "/login")
}

// AuthRequired is a middleware to ensure the user is authenticated.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie("session")
		if err != nil || cookie == "" {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Simple validation of the cookie value format
		parts := strings.Split(cookie, ":")
		if len(parts) != 2 {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Set user info in context for other handlers to use
		c.Set("userID", parts[0])
		c.Set("userName", parts[1])

		c.Next()
	}
}
