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

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":       "Панель управления",
		"active_page": "dashboard",
		"userName":    userName,
	})
}
