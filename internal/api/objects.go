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

func objectStatusLabel(status string) string {
	switch status {
	case "paused":
		return "На паузе"
	case "completed":
		return "Окончен"
	default:
		return "В работе"
	}
}

func ObjectsPage(c *gin.Context) {
	objects, err := storage.GetObjects()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load objects: %v", err)
		return
	}

	users, err := storage.GetUsers()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load users: %v", err)
		return
	}

	userNames := make(map[string]string, len(users))
	for _, user := range users {
		userNames[user.ID] = user.Name
	}

	var rows strings.Builder
	for _, object := range objects {
		responsible := userNames[object.ResponsibleUserID]
		if responsible == "" {
			responsible = "Не назначен"
		}
		rows.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td><a class="btn btn-secondary" href="/objects/edit/%s">Редактировать</a></td></tr>`,
			template.HTMLEscapeString(object.Name),
			template.HTMLEscapeString(objectStatusLabel(object.Status)),
			template.HTMLEscapeString(object.Address),
			template.HTMLEscapeString(responsible),
			template.HTMLEscapeString(object.ID),
		))
	}

	if rows.Len() == 0 {
		rows.WriteString(`<tr><td colspan="5">Объекты пока не добавлены.</td></tr>`)
	}

	page := `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>Объекты</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    {{SIDEBAR_HTML}}
    <div class="main-content">
        <div class="page-header">
            <h1>Объекты</h1>
            <a href="/objects/new" class="btn btn-primary">Добавить объект</a>
        </div>
        <div class="card">
            <table class="table">
                <thead>
                    <tr><th>Название</th><th>Статус</th><th>Адрес</th><th>Ответственный</th><th>Действия</th></tr>
                </thead>
                <tbody>{{ROWS}}</tbody>
            </table>
        </div>
    </div>
</body>
</html>`

	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "objects"), 1)
	final = strings.Replace(final, "{{ROWS}}", rows.String(), 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}

func renderObjectForm(c *gin.Context, object models.Object, actionURL, title, submitLabel string, isEdit bool) {
	users, err := storage.GetUsers()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load users: %v", err)
		return
	}

	var responsibleOptions strings.Builder
	for _, user := range users {
		selected := ""
		if user.ID == object.ResponsibleUserID {
			selected = " selected"
		}
		responsibleOptions.WriteString(fmt.Sprintf(`<option value="%s"%s>%s</option>`, template.HTMLEscapeString(user.ID), selected, template.HTMLEscapeString(user.Name)))
	}

	statusOptions := map[string]string{"in_progress": "", "paused": "", "completed": ""}
	if _, exists := statusOptions[object.Status]; !exists {
		object.Status = "in_progress"
	}
	statusOptions[object.Status] = " selected"

	deleteSection := ""
	if isEdit {
		deleteSection = `<button type="button" class="btn btn-danger" onclick="showDeleteModal()">Удалить объект</button>`
	}

	page := `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>{{TITLE}}</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    {{SIDEBAR_HTML}}
    <div class="main-content">
        <a href="/objects" class="back-link">← К списку объектов</a>
        <div class="page-header"><h1>{{TITLE}}</h1></div>
        <div class="card">
            <form action="{{ACTION_URL}}" method="POST" class="form-grid-edit">
                <div class="form-group-edit form-group-name">
                    <label for="name">Название</label>
                    <input type="text" id="name" name="name" value="{{OBJECT_NAME}}" required>
                </div>
                <div class="form-group-edit form-group-position">
                    <label for="status">Статус</label>
                    <select id="status" name="status" required>
                        <option value="in_progress"{{STATUS_IN_PROGRESS}}>В работе</option>
                        <option value="paused"{{STATUS_PAUSED}}>На паузе</option>
                        <option value="completed"{{STATUS_COMPLETED}}>Окончен</option>
                    </select>
                </div>
                <div class="form-group-edit form-group-phone">
                    <label for="address">Адрес</label>
                    <input type="text" id="address" name="address" value="{{OBJECT_ADDRESS}}" required>
                </div>
                <div class="form-group-edit form-group-rate">
                    <label for="responsible_user_id">Ответственный</label>
                    <select id="responsible_user_id" name="responsible_user_id" required>
                        <option value="">Выберите пользователя</option>
                        {{RESPONSIBLE_OPTIONS}}
                    </select>
                </div>
                <div class="form-actions-edit">
                    <button type="submit" class="btn btn-primary">{{SUBMIT_LABEL}}</button>
                    <a href="/objects" class="btn btn-secondary">Отмена</a>
                    {{DELETE_SECTION}}
                </div>
            </form>
        </div>
    </div>

    <div id="deleteModal" class="modal" style="display:none;">
        <div class="modal-content">
            <span class="close-button" onclick="closeDeleteModal()">&times;</span>
            <h2>Подтверждение удаления</h2>
            <p>Вы уверены, что хотите удалить объект?</p>
            <form action="/objects/delete/{{OBJECT_ID}}" method="POST">
                <div class="form-actions">
                    <button type="submit" class="btn btn-danger">Да, удалить</button>
                    <button type="button" class="btn btn-secondary" onclick="closeDeleteModal()">Отмена</button>
                </div>
            </form>
        </div>
    </div>

    <script>
        function showDeleteModal(){document.getElementById('deleteModal').style.display='block';}
        function closeDeleteModal(){document.getElementById('deleteModal').style.display='none';}
    </script>
</body>
</html>`

	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "objects"), 1)
	final = strings.Replace(final, "{{TITLE}}", template.HTMLEscapeString(title), -1)
	final = strings.Replace(final, "{{ACTION_URL}}", template.HTMLEscapeString(actionURL), 1)
	final = strings.Replace(final, "{{OBJECT_NAME}}", template.HTMLEscapeString(object.Name), 1)
	final = strings.Replace(final, "{{OBJECT_ADDRESS}}", template.HTMLEscapeString(object.Address), 1)
	final = strings.Replace(final, "{{RESPONSIBLE_OPTIONS}}", responsibleOptions.String(), 1)
	final = strings.Replace(final, "{{STATUS_IN_PROGRESS}}", statusOptions["in_progress"], 1)
	final = strings.Replace(final, "{{STATUS_PAUSED}}", statusOptions["paused"], 1)
	final = strings.Replace(final, "{{STATUS_COMPLETED}}", statusOptions["completed"], 1)
	final = strings.Replace(final, "{{SUBMIT_LABEL}}", template.HTMLEscapeString(submitLabel), 1)
	final = strings.Replace(final, "{{DELETE_SECTION}}", deleteSection, 1)
	final = strings.Replace(final, "{{OBJECT_ID}}", template.HTMLEscapeString(object.ID), -1)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}

func AddObjectPage(c *gin.Context) {
	renderObjectForm(c, models.Object{Status: "in_progress"}, "/objects/new", "Новый объект", "Сохранить", false)
}

func CreateObject(c *gin.Context) {
	newObject := models.Object{
		Name:              c.PostForm("name"),
		Status:            c.PostForm("status"),
		Address:           c.PostForm("address"),
		ResponsibleUserID: c.PostForm("responsible_user_id"),
	}
	if _, err := storage.GetUserByID(newObject.ResponsibleUserID); err != nil {
		c.String(http.StatusBadRequest, "Invalid responsible user")
		return
	}
	if _, err := storage.CreateObject(newObject); err != nil {
		c.String(http.StatusBadRequest, "Failed to create object: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/objects")
}

func EditObjectPage(c *gin.Context) {
	object, err := storage.GetObjectByID(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "Object not found")
		return
	}
	renderObjectForm(c, object, "/objects/edit/"+object.ID, "Редактировать объект", "Сохранить изменения", true)
}

func UpdateObject(c *gin.Context) {
	object, err := storage.GetObjectByID(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "Object not found")
		return
	}

	object.Name = c.PostForm("name")
	object.Status = c.PostForm("status")
	object.Address = c.PostForm("address")
	object.ResponsibleUserID = c.PostForm("responsible_user_id")

	if _, err := storage.GetUserByID(object.ResponsibleUserID); err != nil {
		c.String(http.StatusBadRequest, "Invalid responsible user")
		return
	}
	if err := storage.UpdateObject(object); err != nil {
		c.String(http.StatusBadRequest, "Failed to update object: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/objects")
}

func DeleteObject(c *gin.Context) {
	if err := storage.DeleteObject(c.Param("id")); err != nil {
		c.String(http.StatusBadRequest, "Failed to delete object: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/objects")
}
