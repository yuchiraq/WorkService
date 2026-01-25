package api

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// DashboardPage renders the main dashboard.
func DashboardPage(c *gin.Context) {
	session := sessions.Default(c)
	userName := session.Get("userName")
	userID := session.Get("userID")

	c.HTML(http.StatusOK, "layout.html", gin.H{
		"title":       "Панель управления",
		"content":     "dashboard.html", // Specify content template
		"active_page": "dashboard",
		"user": gin.H{
			"ID":   userID,
			"Name": userName,
		},
	})
}
