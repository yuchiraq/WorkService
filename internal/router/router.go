package router

import (
	"net/http"

	"project/internal/api"

	"github.com/gin-gonic/gin"
)

func SetupRouter(r *gin.Engine) {
	r.LoadHTMLGlob("web/templates/*")

	// API group
	apiGroup := r.Group("/api")
	{
		// Ping test
		apiGroup.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "pong",
			})
		})

		// User routes
		userRoutes := apiGroup.Group("/users")
		{
			userRoutes.POST("/", api.CreateUser)
		}

		// Article routes
		articleRoutes := apiGroup.Group("/articles")
		{
			articleRoutes.POST("/", api.CreateArticle)
			articleRoutes.GET("/:id", api.GetArticle)
			articleRoutes.PUT("/:id", api.UpdateArticle)
		}
	}

	// Web group
	webGroup := r.Group("/web")
	{
		webGroup.GET("/login", func(c *gin.Context) {
			c.HTML(http.StatusOK, "login.html", nil)
		})
		webGroup.POST("/login", api.Login)
		// Dashboard route
		webGroup.GET("/dashboard", api.Dashboard)
	}
}
