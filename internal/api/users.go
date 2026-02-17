package api

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"project/internal/models"
	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

func userStatusLabel(status string) string {
	if status == "admin" {
		return "Админ"
	}
	return "Пользователь"
}

func UsersPage(c *gin.Context) {
	users, err := storage.GetUsers()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load users: %v", err)
		return
	}

	var rows strings.Builder
	for _, user := range users {
		rows.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td><div class="table-actions"><a href="/users/edit/%s" class="btn btn-secondary">Редактировать</a><form action="/users/delete/%s" method="POST" style="display:inline;"><button class="btn btn-danger" type="submit">Удалить</button></form></div></td></tr>`,
			template.HTMLEscapeString(user.Name),
			template.HTMLEscapeString(user.Username),
			template.HTMLEscapeString(user.Phone),
			template.HTMLEscapeString(userStatusLabel(user.Status)),
			template.HTMLEscapeString(user.ID),
			template.HTMLEscapeString(user.ID),
		))
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><title>Пользователи</title><link rel="stylesheet" href="/static/css/style.css"></head><body>
{{SIDEBAR_HTML}}
<div class="main-content">
<div class="page-header"><h1>Пользователи</h1><a href="/users/new" class="btn btn-primary">Добавить пользователя</a></div>
<div class="card"><table class="table"><thead><tr><th>ФИО</th><th>Логин</th><th>Телефон</th><th>Статус</th><th>Действия</th></tr></thead><tbody>{{ROWS}}</tbody></table></div>
</div></body></html>`
	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "users"), 1)
	final = strings.Replace(final, "{{ROWS}}", rows.String(), 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}

func renderUserForm(c *gin.Context, user models.User, actionURL, title, submitLabel string, adminEditable bool) {
	statusAdmin := ""
	statusUser := ""
	if user.Status == "admin" {
		statusAdmin = " selected"
	} else {
		statusUser = " selected"
	}

	statusField := `<input type="hidden" name="status" value="user">`
	if adminEditable {
		statusField = `<label for="status">Статус</label><select id="status" name="status"><option value="user"` + statusUser + `>Пользователь</option><option value="admin"` + statusAdmin + `>Админ</option></select>`
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><title>{{TITLE}}</title><link rel="stylesheet" href="/static/css/style.css"></head><body>
{{SIDEBAR_HTML}}
<div class="main-content">
<a href="{{BACK_URL}}" class="back-link">← Назад</a>
<div class="page-header"><h1>{{TITLE}}</h1></div>
<div class="card">
<form action="{{ACTION_URL}}" method="POST" class="form-grid-edit">
<div class="form-group-edit form-group-name"><label for="name">ФИО</label><input type="text" id="name" name="name" value="{{NAME}}" required></div>
<div class="form-group-edit form-group-position"><label for="username">Логин</label><input type="text" id="username" name="username" value="{{USERNAME}}" required></div>
<div class="form-group-edit form-group-phone"><label for="password">Пароль</label><input type="text" id="password" name="password" value="{{PASSWORD}}" required></div>
<div class="form-group-edit form-group-rate"><label for="phone">Контактный номер</label><input type="tel" id="phone" name="phone" value="{{PHONE}}"></div>
<div class="form-group-edit form-group-rate">{{STATUS_FIELD}}</div>
<div class="form-actions-edit"><button type="submit" class="btn btn-primary">{{SUBMIT_LABEL}}</button><a href="{{BACK_URL}}" class="btn btn-secondary">Отмена</a></div>
</form>
</div>
</div>
</body></html>`

	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "users"), 1)
	final = strings.Replace(final, "{{TITLE}}", template.HTMLEscapeString(title), -1)
	final = strings.Replace(final, "{{ACTION_URL}}", template.HTMLEscapeString(actionURL), 1)
	final = strings.Replace(final, "{{NAME}}", template.HTMLEscapeString(user.Name), 1)
	final = strings.Replace(final, "{{USERNAME}}", template.HTMLEscapeString(user.Username), 1)
	final = strings.Replace(final, "{{PASSWORD}}", template.HTMLEscapeString(user.Password), 1)
	final = strings.Replace(final, "{{PHONE}}", template.HTMLEscapeString(user.Phone), 1)
	final = strings.Replace(final, "{{STATUS_FIELD}}", statusField, 1)
	final = strings.Replace(final, "{{SUBMIT_LABEL}}", template.HTMLEscapeString(submitLabel), 1)
	final = strings.Replace(final, "{{BACK_URL}}", "/users", -1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}

func AddUserPage(c *gin.Context) {
	renderUserForm(c, models.User{Status: "user"}, "/users/new", "Новый пользователь", "Сохранить", true)
}

func CreateUser(c *gin.Context) {
	newUser := models.User{
		Name:     c.PostForm("name"),
		Username: c.PostForm("username"),
		Password: c.PostForm("password"),
		Phone:    c.PostForm("phone"),
		Status:   c.PostForm("status"),
	}
	createdUser, err := storage.CreateUser(newUser)
	if err != nil {
		c.String(http.StatusBadRequest, "Failed to create user: %v", err)
		return
	}
	if createdUser.Status == "user" {
		_, _ = storage.CreateWorker(models.Worker{
			Name:          createdUser.Name,
			Position:      "Сотрудник",
			Phone:         createdUser.Phone,
			CreatedBy:     c.GetString("userID"),
			CreatedByName: c.GetString("userName"),
			UserID:        createdUser.ID,
		})
	}
	c.Redirect(http.StatusFound, "/users")
}

func EditUserPage(c *gin.Context) {
	user, err := storage.GetUserByID(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "User not found")
		return
	}
	renderUserForm(c, user, "/users/edit/"+user.ID, "Редактировать пользователя", "Сохранить изменения", true)
}

func UpdateUser(c *gin.Context) {
	user, err := storage.GetUserByID(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "User not found")
		return
	}

	user.Name = c.PostForm("name")
	user.Username = c.PostForm("username")
	user.Password = c.PostForm("password")
	user.Phone = c.PostForm("phone")
	user.Status = c.PostForm("status")

	if err := storage.UpdateUser(user); err != nil {
		c.String(http.StatusBadRequest, "Failed to update user: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/users")
}

func DeleteUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == c.GetString("userID") {
		c.String(http.StatusBadRequest, "Нельзя удалить текущего пользователя")
		return
	}
	_ = storage.DeleteWorkerByUserID(userID)
	if err := storage.DeleteUser(userID); err != nil {
		c.String(http.StatusBadRequest, "Failed to delete user: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/users")
}

func ProfilePage(c *gin.Context) {
	userID := c.GetString("userID")
	user, err := storage.GetUserByID(userID)
	if err != nil {
		c.String(http.StatusNotFound, "User not found")
		return
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><title>Мой профиль</title><link rel="stylesheet" href="/static/css/style.css"></head><body>
{{SIDEBAR_HTML}}
<div class="main-content"><div class="page-header"><h1>Мой профиль</h1></div>
<div class="card"><form action="/profile" method="POST" class="form-grid-edit">
<div class="form-group-edit form-group-name"><label for="name">ФИО</label><input type="text" id="name" name="name" value="{{NAME}}" required></div>
<div class="form-group-edit form-group-position"><label for="username">Логин</label><input type="text" id="username" name="username" value="{{USERNAME}}" required></div>
<div class="form-group-edit form-group-phone"><label for="password">Пароль</label><input type="text" id="password" name="password" value="{{PASSWORD}}" required></div>
<div class="form-group-edit form-group-rate"><label for="phone">Контактный номер</label><input type="tel" id="phone" name="phone" value="{{PHONE}}"></div>
<div class="form-actions-edit"><button type="submit" class="btn btn-primary">Сохранить</button></div>
</form></div></div>
</body></html>`
	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "my-profile"), 1)
	final = strings.Replace(final, "{{NAME}}", template.HTMLEscapeString(user.Name), 1)
	final = strings.Replace(final, "{{USERNAME}}", template.HTMLEscapeString(user.Username), 1)
	final = strings.Replace(final, "{{PASSWORD}}", template.HTMLEscapeString(user.Password), 1)
	final = strings.Replace(final, "{{PHONE}}", template.HTMLEscapeString(user.Phone), 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}

func UpdateProfile(c *gin.Context) {
	userID := c.GetString("userID")
	user, err := storage.GetUserByID(userID)
	if err != nil {
		c.String(http.StatusNotFound, "User not found")
		return
	}

	user.Name = c.PostForm("name")
	user.Username = c.PostForm("username")
	user.Password = c.PostForm("password")
	user.Phone = c.PostForm("phone")

	if err := storage.UpdateUser(user); err != nil {
		c.String(http.StatusBadRequest, "Failed to update profile: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/profile")
}
