package api

import (
	"net/http"
	"strings"
	"time"

	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

// LoginPage renders the login page.
func LoginPage(c *gin.Context) {
	pageTemplate := `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>Вход в систему</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    <div class="center-page">
        <div class="card center-card" style="max-width: 400px;">
            <div style="text-align: left;">
                <h2>Вход в систему</h2>
                <p style="margin-bottom: 25px;">Пожалуйста, введите свои учетные данные для входа.</p>
                <form action="/login" method="POST">
                    <div class="form-group">
                        <label for="username">Имя пользователя</label>
                        <input type="text" id="username" name="username" required autofocus>
                    </div>
                    <div class="form-group">
                        <label for="password">Пароль</label>
                        <input type="password" id="password" name="password" required>
                    </div>
                    <button type="submit" class="btn btn-primary" style="width: 100%;">Войти</button>
                </form>
            </div>
        </div>
    </div>
</body>
</html>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(pageTemplate))
}

// Login handles the authentication logic.
func Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	user, err := storage.ValidateUser(username, password)
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=invalid_credentials")
		return
	}

	// Set session cookie
	cookieValue := user.ID + ":" + user.Username
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "session",
		Value:    cookieValue,
		Expires:  time.Now().Add(24 * time.Hour),
		Path:     "/",
		HttpOnly: true,
	})

	c.Redirect(http.StatusFound, "/dashboard")
}

// Logout clears the session cookie and redirects to the login page.
func Logout(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "session",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour), // Set to a past time to delete
		Path:     "/",
		HttpOnly: true,
	})
	c.Redirect(http.StatusFound, "/login")
}

// AuthRequired is a middleware to ensure the user is authenticated.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie("session")
		if err != nil || cookie == "" {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Simple validation of the cookie value format
		parts := strings.Split(cookie, ":")
		if len(parts) != 2 {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Set user info in context for other handlers to use
		c.Set("userID", parts[0])
		c.Set("userName", parts[1])

		c.Next()
	}
}
