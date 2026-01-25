package router

import (
	"net/http"

	"project/internal/api"

	"github.com/gin-gonic/gin"
)

func SetupRouter(r *gin.Engine) {
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
			userRoutes.GET("/:id", api.GetUser)
			userRoutes.PUT("/:id", api.UpdateUser)
		}

		// Article routes
		articleRoutes := apiGroup.Group("/articles")
		{
			articleRoutes.POST("/", api.CreateArticle)
			articleRoutes.GET("/:id", api.GetArticle)
			articleRoutes.PUT("/:id", api.UpdateArticle)
		}
	}

	// Login route
	r.POST("/login", api.Login)

	// Dashboard route
	r.GET("/dashboard", api.Dashboard)
}
