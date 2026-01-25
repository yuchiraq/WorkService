package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Dashboard handles the dashboard page
func Dashboard(c *gin.Context) {
	c.HTML(http.StatusOK, "dashboard.html", nil)
}
