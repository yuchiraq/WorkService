package router

import (
	"net/http"

	"project/internal/api"

	"github.com/gin-gonic/gin"
)

func SetupRouter(r *gin.Engine) {
	// Load all HTML templates, including partials
	r.LoadHTMLGlob("web/templates/*")

	// Serve static files (CSS, JS, images)
	r.Static("/static", "./web/static")

	// Redirect root to the login page
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/login")
	})

	// --- Web Pages ---

	// Auth
	r.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{"error": c.Query("error")})
	})
	r.POST("/login", api.Login)

	// Dashboard
	r.GET("/dashboard", api.Dashboard)

	// Worker Pages
	workerPages := r.Group("/workers")
	{
		workerPages.GET("/", func(c *gin.Context) {
			c.HTML(http.StatusOK, "workers.html", nil)
		})
		workerPages.GET("/new", func(c *gin.Context) {
			c.HTML(http.StatusOK, "add-worker.html", nil)
		})
	}

	// --- API Endpoints ---
	apiGroup := r.Group("/api")
	{
		// Ping test
		apiGroup.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "pong"})
		})

		// User routes
		apiGroup.POST("/users", api.CreateUser)

		// Article routes
		articleRoutes := apiGroup.Group("/articles")
		{
			articleRoutes.POST("/", api.CreateArticle)
			articleRoutes.GET("/:id", api.GetArticle)
			articleRoutes.PUT("/:id", api.UpdateArticle)
		}

		// Worker API routes
		workerApiRoutes := apiGroup.Group("/workers")
		{
			workerApiRoutes.GET("/", api.GetWorkers)
			workerApiRoutes.POST("/", api.CreateWorker)
		}
	}
}
