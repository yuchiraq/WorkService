package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// DashboardPage renders the main dashboard page using manual HTML string building.
func DashboardPage(c *gin.Context) {
	session := sessions.Default(c)
	userName := session.Get("userName")

	pageTemplate := `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>Панель управления</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    {{SIDEBAR_HTML}}
    <div class="main-content">
        <div class="page-header">
            <h1>Панель управления</h1>
        </div>
        <div class="card">
            <p>Добро пожаловать, %s! Вы находитесь в системе управления работниками.</p>
			<p>Используйте меню слева для навигации по разделам.</p>
        </div>
    </div>
</body>
</html>`

	// Generate the sidebar with the active page set to "dashboard"
	sidebar := RenderSidebar("dashboard")

	// Create the final HTML by injecting the username and sidebar
	pageContent := fmt.Sprintf(pageTemplate, userName)
	finalHTML := strings.Replace(pageContent, "{{SIDEBAR_HTML}}", sidebar, 1)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(finalHTML))
}
