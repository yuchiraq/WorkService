package router

import (
	"project/internal/api"

	"github.com/gin-gonic/gin"
)

func SetupRouter(r *gin.Engine) {
	// Load HTML templates
	r.LoadHTMLGlob("web/templates/*")

	// Serve static files (CSS, JS, images)
	r.Static("/static", "./web/static")

	// Handle 404 Not Found errors
	r.NoRoute(api.NotFoundPage)

	// Auth routes
	r.GET("/login", api.LoginPage)
	r.POST("/login", api.Login)
	r.GET("/logout", api.Logout)

	// Protected routes
	authRequired := r.Group("/")
	authRequired.Use(api.AuthRequired())
	{
		// Dashboard
		authRequired.GET("/dashboard", api.DashboardPage)

		// Workers
		authRequired.GET("/workers", api.WorkersPage)
		authRequired.GET("/worker/:id", api.WorkerProfilePage)
		authRequired.GET("/workers/new", api.AddWorkerPage)
		authRequired.POST("/workers/new", api.CreateWorker)
		authRequired.GET("/workers/edit/:id", api.EditWorkerPage)
		authRequired.POST("/workers/edit/:id", api.UpdateWorker)
		authRequired.POST("/workers/delete/:id", api.DeleteWorker)
	}

	// Redirect root to login
	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/login")
	})
}
