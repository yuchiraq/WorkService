package router

import (
	"project/internal/api"

	"github.com/gin-gonic/gin"
)

func SetupRouter(r *gin.Engine) {
	r.Static("/static", "./web/static")
	r.NoRoute(api.NotFoundPage)

	r.GET("/login", api.LoginPage)
	r.POST("/login", api.Login)
	r.GET("/logout", api.Logout)

	authRequired := r.Group("/")
	authRequired.Use(api.AuthRequired())
	{
		authRequired.GET("/dashboard", api.DashboardPage)

		authRequired.GET("/workers", api.WorkersPage)
		authRequired.GET("/worker/:id", api.WorkerProfilePage)
		authRequired.GET("/workers/new", api.AddWorkerPage)
		authRequired.POST("/workers/new", api.CreateWorker)
		authRequired.GET("/workers/edit/:id", api.EditWorkerPage)
		authRequired.POST("/workers/edit/:id", api.UpdateWorker)
		authRequired.POST("/workers/delete/:id", api.DeleteWorker)

		authRequired.GET("/objects", api.ObjectsPage)
		authRequired.GET("/objects/new", api.AddObjectPage)
		authRequired.POST("/objects/new", api.CreateObject)
		authRequired.GET("/objects/edit/:id", api.EditObjectPage)
		authRequired.POST("/objects/edit/:id", api.UpdateObject)
		authRequired.POST("/objects/delete/:id", api.DeleteObject)

		// Timesheets (keep multiple aliases so old/bookmarked URLs never give 404)
		authRequired.GET("/timesheets", api.TimesheetsPage)
		authRequired.GET("/timesheets/", api.TimesheetsPage)
		authRequired.GET("/timesheet", api.TimesheetsPage)
		authRequired.GET("/timesheet/", api.TimesheetsPage)
		authRequired.GET("/tabel", api.TimesheetsPage)
		authRequired.GET("/tabel/", api.TimesheetsPage)
		authRequired.GET("/timesheets/new", api.AddTimesheetPage)
		authRequired.POST("/timesheets/new", api.CreateTimesheet)
		authRequired.GET("/timesheets/edit/:id", api.EditTimesheetPage)
		authRequired.POST("/timesheets/edit/:id", api.UpdateTimesheet)
		authRequired.POST("/timesheets/delete/:id", api.DeleteTimesheet)

		authRequired.GET("/profile", api.ProfilePage)
		authRequired.POST("/profile", api.UpdateProfile)
	}

	adminRequired := r.Group("/")
	adminRequired.Use(api.AuthRequired(), api.AdminRequired())
	{
		adminRequired.GET("/users", api.UsersPage)
		adminRequired.GET("/users/new", api.AddUserPage)
		adminRequired.POST("/users/new", api.CreateUser)
		adminRequired.GET("/users/edit/:id", api.EditUserPage)
		adminRequired.POST("/users/edit/:id", api.UpdateUser)
	}

	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/login")
	})
}
