package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// DashboardPage renders the main dashboard after login.
func DashboardPage(c *gin.Context) {
	userName, _ := c.Get("userName")

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
            <h1>Добро пожаловать, %s!</h1>
        </div>
        <div class="card">
            <p>Это ваша панель управления. Здесь будет отображаться общая информация и статистика.</p>
        </div>
    </div>
</body>
</html>`

	sidebar := RenderSidebar("dashboard")
	finalHTML := fmt.Sprintf(pageTemplate, userName)
	finalHTML = strings.Replace(finalHTML, "{{SIDEBAR_HTML}}", sidebar, 1)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(finalHTML))
}

// NotFoundPage renders a 404 error page.
func NotFoundPage(c *gin.Context) {
	pageTemplate := `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>404 - Страница не найдена</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    <div class="not-found-container">
        <h1>404</h1>
        <p>Страница не найдена.</p>
        <a href="/dashboard" class="btn btn-primary">На главную</a>
    </div>
</body>
</html>`
	c.Data(http.StatusNotFound, "text/html; charset=utf-8", []byte(pageTemplate))
}
