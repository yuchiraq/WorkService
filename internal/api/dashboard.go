package api

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Dashboard handles the dashboard page
func Dashboard(c *gin.Context) {
	session := sessions.Default(c)
	userName := session.Get("userName")

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"userName": userName,
	})
}
