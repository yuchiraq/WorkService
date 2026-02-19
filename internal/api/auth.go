package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"project/internal/security"
	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

type authSession struct {
	UserID    string
	ExpiresAt time.Time
	CSRFToken string
}

type loginAttempt struct {
	Count     int
	FirstFail time.Time
	LockUntil time.Time
}

var (
	sessionMutex  sync.RWMutex
	sessions      = map[string]authSession{}
	attemptsMutex sync.Mutex
	loginAttempts = map[string]loginAttempt{}
	maxLoginFails = 5
	lockDuration  = 15 * time.Minute
	attemptWindow = 15 * time.Minute
	sessionCookie = "session_token"
	sessionMaxAge = 24 * time.Hour
)

func randomToken(lengthBytes int) string {
	buf := make([]byte, lengthBytes)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func cookieSecure() bool {
	return strings.EqualFold(os.Getenv("APP_COOKIE_SECURE"), "true")
}

func setSessionCookie(c *gin.Context, token string, expires time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Expires:  expires,
		Path:     "/",
		HttpOnly: true,
		Secure:   cookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		Path:     "/",
		HttpOnly: true,
		Secure:   cookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func cleanExpiredSessions() {
	now := time.Now()
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	for k, v := range sessions {
		if now.After(v.ExpiresAt) {
			delete(sessions, k)
		}
	}
}

func getAttemptKey(c *gin.Context, username string) string {
	return strings.TrimSpace(strings.ToLower(username)) + "|" + c.ClientIP()
}

func checkLock(attemptKey string) (bool, time.Time) {
	attemptsMutex.Lock()
	defer attemptsMutex.Unlock()
	a := loginAttempts[attemptKey]
	if time.Now().Before(a.LockUntil) {
		return true, a.LockUntil
	}
	return false, time.Time{}
}

func registerFail(attemptKey string) {
	attemptsMutex.Lock()
	defer attemptsMutex.Unlock()

	now := time.Now()
	a := loginAttempts[attemptKey]
	if a.FirstFail.IsZero() || now.Sub(a.FirstFail) > attemptWindow {
		a.FirstFail = now
		a.Count = 0
	}
	a.Count++
	if a.Count >= maxLoginFails {
		a.LockUntil = now.Add(lockDuration)
		a.Count = 0
		a.FirstFail = now
	}
	loginAttempts[attemptKey] = a
}

func registerSuccess(attemptKey string) {
	attemptsMutex.Lock()
	defer attemptsMutex.Unlock()
	delete(loginAttempts, attemptKey)
}

// LoginPage renders the login page.
func LoginPage(c *gin.Context) {
	errorBlock := ""
	switch c.Query("error") {
	case "invalid_credentials":
		errorBlock = `<div style="margin-bottom: 16px; color: #b42318; background: #fee4e2; border: 1px solid #fecdca; border-radius: 8px; padding: 10px 12px; font-size: 14px;">Неверное имя пользователя или пароль.</div>`
	}

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
                {{ERROR_BLOCK}}
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
	final := strings.Replace(pageTemplate, "{{ERROR_BLOCK}}", errorBlock, 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}

// Login handles the authentication logic.
func Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	attemptKey := getAttemptKey(c, username)

	if locked, until := checkLock(attemptKey); locked {
		security.LogEvent("login_locked", fmt.Sprintf("user=%s ip=%s until=%s", username, c.ClientIP(), until.Format(time.RFC3339)))
		c.String(http.StatusTooManyRequests, "Слишком много попыток входа. Попробуйте позже.")
		return
	}

	user, err := storage.ValidateUser(username, password)
	if err != nil {
		registerFail(attemptKey)
		security.LogEvent("login_failed", fmt.Sprintf("user=%s ip=%s", username, c.ClientIP()))
		c.Redirect(http.StatusFound, "/login?error=invalid_credentials")
		return
	}

	registerSuccess(attemptKey)
	_ = storage.UpdateUserLastLogin(user.ID, time.Now())
	cleanExpiredSessions()
	token := randomToken(32)
	csrf := randomToken(24)
	expires := time.Now().Add(sessionMaxAge)

	sessionMutex.Lock()
	sessions[token] = authSession{UserID: user.ID, ExpiresAt: expires, CSRFToken: csrf}
	sessionMutex.Unlock()

	setSessionCookie(c, token, expires)
	security.LogEvent("login_success", fmt.Sprintf("user=%s ip=%s", username, c.ClientIP()))
	c.Redirect(http.StatusFound, "/dashboard")
}

// Logout clears the session cookie and redirects to the login page.
func Logout(c *gin.Context) {
	token, _ := c.Cookie(sessionCookie)
	if token != "" {
		sessionMutex.Lock()
		delete(sessions, token)
		sessionMutex.Unlock()
	}
	clearSessionCookie(c)
	c.Redirect(http.StatusFound, "/login")
}

// AuthRequired is a middleware to ensure the user is authenticated.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		cleanExpiredSessions()
		token, err := c.Cookie(sessionCookie)
		if err != nil || token == "" {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		sessionMutex.RLock()
		sess, exists := sessions[token]
		sessionMutex.RUnlock()
		if !exists || time.Now().After(sess.ExpiresAt) {
			clearSessionCookie(c)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		user, err := storage.GetUserByID(sess.UserID)
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Set("userID", user.ID)
		c.Set("userName", user.Name)
		c.Set("userStatus", user.Status)
		c.Set("csrfToken", sess.CSRFToken)

		c.Next()
	}
}

func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost && c.Request.Method != http.MethodPut && c.Request.Method != http.MethodDelete && c.Request.Method != http.MethodPatch {
			c.Next()
			return
		}
		if c.Request.URL.Path == "/login" {
			c.Next()
			return
		}
		csrfValue, ok := c.Get("csrfToken")
		if !ok {
			c.String(http.StatusForbidden, "CSRF token missing")
			c.Abort()
			return
		}
		token := c.PostForm("_csrf_token")
		if token == "" {
			token = c.GetHeader("X-CSRF-Token")
		}
		expected := csrfValue.(string)
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			security.LogEvent("csrf_failed", fmt.Sprintf("path=%s ip=%s", c.Request.URL.Path, c.ClientIP()))
			c.String(http.StatusForbidden, "CSRF token invalid")
			c.Abort()
			return
		}
		c.Next()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		statusValue, ok := c.Get("userStatus")
		if !ok || statusValue.(string) != "admin" {
			c.String(http.StatusForbidden, "Доступ запрещен")
			c.Abort()
			return
		}
		c.Next()
	}
}

type SecurityStats struct {
	ActiveSessions int
	LockedAttempts int
}

func GetSecurityStats() SecurityStats {
	sessionMutex.RLock()
	active := len(sessions)
	sessionMutex.RUnlock()

	attemptsMutex.Lock()
	locked := 0
	now := time.Now()
	for _, a := range loginAttempts {
		if now.Before(a.LockUntil) {
			locked++
		}
	}
	attemptsMutex.Unlock()

	return SecurityStats{ActiveSessions: active, LockedAttempts: locked}
}
