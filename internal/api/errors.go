package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// NotFoundPage renders a user-friendly 404 error page.
func NotFoundPage(c *gin.Context) {
	pageHTML := `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>Ошибка 404 - Страница не найдена</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    <div class="center-page">
        <div class="center-card">
            <h1>404</h1>
            <h2>Страница не найдена</h2>
            <p>К сожалению, страница, которую вы ищете, не существует или была перемещена.</p>
            <a href="/dashboard" class="btn btn-primary">Вернуться на главную</a>
        </div>
    </div>
</body>
</html>`

	c.Data(http.StatusNotFound, "text/html; charset=utf-8", []byte(pageHTML))
}
