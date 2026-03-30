package api

import "github.com/gin-gonic/gin"

func isAdmin(c *gin.Context) bool {
	return c.GetString("userStatus") == "admin"
}

func requireAdmin(c *gin.Context) bool {
	if isAdmin(c) {
		return true
	}
	c.String(403, "Доступ запрещен")
	return false
}
