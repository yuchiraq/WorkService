package router

import (
	"net/http"

	"project/internal/api"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// Middleware to check if the user is authenticated
func AuthRequired(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("userID")
	if userID == nil {
		c.Redirect(http.StatusFound, "/login?error=Требуется авторизация")
		c.Abort()
		return
	}
	c.Next()
}

func SetupRouter(r *gin.Engine) {
	// Session management
	store := cookie.NewStore([]byte("secret")) // It's better to use a more secure secret in production
	r.Use(sessions.Sessions("mysession", store))

	// Serve static files (CSS, JS, images)
	r.Static("/static", "./web/static")

	// --- Public Routes ---
	// Redirect root to login page
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/login")
	})

	r.GET("/login", api.LoginPage)
	r.POST("/login", api.Login)

	// --- Authenticated Routes (protected by AuthRequired middleware) ---
	authenticated := r.Group("/")
	authenticated.Use(AuthRequired)
	{
		// Dashboard
		authenticated.GET("/dashboard", api.DashboardPage)

		// Worker Pages
		authenticated.GET("/workers", api.WorkersPage)
		authenticated.GET("/workers/new", api.AddWorkerPage) // Corrected function name
		authenticated.POST("/workers/new", api.CreateWorker)
		authenticated.GET("/workers/edit/:id", api.EditWorkerPage)
		authenticated.POST("/workers/edit/:id", api.UpdateWorker)
		authenticated.POST("/workers/delete/:id", api.DeleteWorker)

		// Logout
		authenticated.GET("/logout", api.Logout)
	}
}
