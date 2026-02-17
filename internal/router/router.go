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

		// Schedule (назначения)
		authRequired.GET("/schedule", api.SchedulePage)
		authRequired.GET("/schedule/", api.SchedulePage)
		authRequired.GET("/schedule/new", api.AddSchedulePage)
		authRequired.GET("/timesheets/new", api.AddSchedulePage)
		authRequired.POST("/schedule/new", api.CreateScheduleEntry)
		authRequired.POST("/timesheets/new", api.CreateScheduleEntry)
		authRequired.GET("/schedule/edit/:id", api.EditSchedulePage)
		authRequired.GET("/timesheets/edit/:id", api.EditSchedulePage)
		authRequired.POST("/schedule/edit/:id", api.UpdateScheduleEntry)
		authRequired.POST("/timesheets/edit/:id", api.UpdateScheduleEntry)
		authRequired.POST("/schedule/delete/:id", api.DeleteScheduleEntry)
		authRequired.POST("/timesheets/delete/:id", api.DeleteScheduleEntry)

		// Timesheet matrix (табель)
		authRequired.GET("/timesheets", api.TimesheetsPage)
		authRequired.GET("/timesheets/", api.TimesheetsPage)
		authRequired.GET("/timesheet", api.TimesheetsPage)
		authRequired.GET("/timesheet/", api.TimesheetsPage)
		authRequired.GET("/tabel", api.TimesheetsPage)
		authRequired.GET("/tabel/", api.TimesheetsPage)

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
		adminRequired.POST("/users/delete/:id", api.DeleteUser)
	}

	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/login")
	})
}
