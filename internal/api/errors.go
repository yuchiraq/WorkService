package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func accessDenied(c *gin.Context) {
	c.Data(http.StatusForbidden, "text/html; charset=utf-8", []byte(`<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Нет доступа</title><link rel="stylesheet" href="/static/css/style.css?v=17"></head><body><main class="center-page"><section class="center-card"><h1>Нет доступа</h1><p>Эта страница доступна владельцу или администратору.</p><a class="btn btn-secondary" href="/schedule">К расписанию</a></section></main></body></html>`))
}

// NotFoundPage renders a user-friendly 404 error page.
func NotFoundPage(c *gin.Context) {
	pageHTML := `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
    <title>Ошибка 404 - Страница не найдена</title>
    <link rel="stylesheet" href="/static/css/style.css?v=17">
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
