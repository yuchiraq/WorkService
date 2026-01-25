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
	user := session.Get("user")
	if user == nil {
		// User is not logged in, redirect to login page
		c.Redirect(http.StatusFound, "/login?error=Требуется авторизация")
		c.Abort() // Stop processing the request
		return
	}
	// User is authenticated, continue
	c.Next()
}

func SetupRouter(r *gin.Engine) {
	// Session management
	store := cookie.NewStore([]byte("secret")) // Use a long, random secret key in a real app
	r.Use(sessions.Sessions("mysession", store))

	// Serve static files
	r.Static("/static", "./web/static")

	// --- Public Routes ---

	// Redirect root to login
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/login")
	})

	// Login page
	r.LoadHTMLFiles("web/templates/login.html")
	r.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{"error": c.Query("error")})
	})
	r.POST("/login", api.Login)

	// --- Authenticated Routes ---
	authenticated := r.Group("/")
	authenticated.Use(AuthRequired)
	{
		// Dashboard
		authenticated.GET("/dashboard", func(c *gin.Context) {
			r.LoadHTMLFiles("web/templates/navigation.html", "web/templates/dashboard.html")
			c.HTML(http.StatusOK, "navigation.html", gin.H{"active": "dashboard"})
		})

		// Worker Pages
		authenticated.GET("/workers", func(c *gin.Context) {
			r.LoadHTMLFiles("web/templates/navigation.html", "web/templates/workers.html")
			c.HTML(http.StatusOK, "navigation.html", gin.H{"active": "workers"})
		})
		authenticated.GET("/workers/new", func(c *gin.Context) {
			r.LoadHTMLFiles("web/templates/navigation.html", "web/templates/add-worker.html")
			c.HTML(http.StatusOK, "navigation.html", gin.H{"active": "workers"})
		})

		// Logout
		authenticated.GET("/logout", api.Logout)
	}

	// --- API Endpoints (also authenticated) ---
	apiGroup := r.Group("/api")
	apiGroup.Use(AuthRequired)
	{
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
			workerApiRoutes.DELETE("/:id", api.DeleteWorker)
		}
	}
}
