package api

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// AuthRequired is a middleware to check if the user is authenticated.
// If they are not, it redirects them to the login page.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		user := session.Get("userName")
		if user == nil {
			// User is not logged in, redirect to login page.
			c.Redirect(http.StatusFound, "/login")
			c.Abort() // Stop processing the request
			return
		}
		// User is authenticated, continue to the next handler.
		c.Next()
	}
}
