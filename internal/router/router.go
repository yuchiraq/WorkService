package router

import (
	"net/http"

	"project/internal/api"
	"project/internal/storage"

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
	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("mysession", store))

	// Serve static files
	r.Static("/static", "./web/static")

	// --- Public Routes ---
	r.GET("/", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/login") })
	r.GET("/login", func(c *gin.Context) {
		r.LoadHTMLFiles("web/templates/login.html") // Login page doesn't use the main layout
		c.HTML(http.StatusOK, "login.html", gin.H{"error": c.Query("error")})
	})
	r.POST("/login", api.Login)

	// --- Authenticated Routes ---
	authenticated := r.Group("/")
	authenticated.Use(AuthRequired)
	{
		// Dashboard
		authenticated.GET("/dashboard", func(c *gin.Context) {
			session := sessions.Default(c)
			userName := session.Get("userName")
			r.LoadHTMLFiles("web/templates/navigation.html", "web/templates/dashboard.html")
			c.HTML(http.StatusOK, "navigation.html", gin.H{
				"pageTitle": "Панель управления",
				"active":    "dashboard",
				"userName":  userName,
			})
		})

		// Worker Pages
		authenticated.GET("/workers", func(c *gin.Context) {
			workers, _ := storage.GetWorkers()
			userName := sessions.Default(c).Get("userName")
			r.LoadHTMLFiles("web/templates/navigation.html", "web/templates/workers.html")
			c.HTML(http.StatusOK, "navigation.html", gin.H{
				"pageTitle": "Работники",
				"active":    "workers",
				"workers":   workers,
				"userName":  userName,
			})
		})

		authenticated.GET("/workers/new", func(c *gin.Context) {
			userName := sessions.Default(c).Get("userName")
			r.LoadHTMLFiles("web/templates/navigation.html", "web/templates/add-worker.html")
			c.HTML(http.StatusOK, "navigation.html", gin.H{
				"pageTitle": "Добавить работника",
				"active":    "workers",
				"userName":  userName,
			})
		})

		authenticated.GET("/workers/edit/:id", func(c *gin.Context) {
			workerID := c.Param("id")
			worker, _ := storage.GetWorkerByID(workerID)
			userName := sessions.Default(c).Get("userName")
			r.LoadHTMLFiles("web/templates/navigation.html", "web/templates/edit-worker.html")
			c.HTML(http.StatusOK, "navigation.html", gin.H{
				"pageTitle": "Редактировать работника",
				"active":    "workers",
				"worker":    worker,
				"userName":  userName,
			})
		})
		authenticated.POST("/workers/edit/:id", api.UpdateWorker)
		authenticated.POST("/workers/delete/:id", api.DeleteWorker)


		// Logout
		authenticated.GET("/logout", api.Logout)
	}

	// --- API Endpoints ---
	apiGroup := r.Group("/api")
	apiGroup.Use(AuthRequired)
	{
		// Worker API routes
		workerApiRoutes := apiGroup.Group("/workers")
		{
			workerApiRoutes.POST("/", api.CreateWorker)
			workerApiRoutes.DELETE("/:id", api.DeleteWorker) // This is fine for an API
		}
	}
}
