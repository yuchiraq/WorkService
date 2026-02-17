package api

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"
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

	selectedTab := c.DefaultQuery("tab", "active")

	visibleObjects := make([]models.Object, 0, len(objects))
	for _, object := range objects {
		if selectedTab == "completed" {
			if object.Status == "completed" {
				visibleObjects = append(visibleObjects, object)
			}
			continue
		}
		if object.Status != "completed" {
			visibleObjects = append(visibleObjects, object)
		}
	}

	var cards strings.Builder
	for _, object := range visibleObjects {
		responsible := userNames[object.ResponsibleUserID]
		if responsible == "" {
			responsible = "Не назначен"
		}
		statusClass := "active"
		if object.Status == "completed" {
			statusClass = "warning"
		}
		cards.WriteString(fmt.Sprintf(`<article class="info-card object-card"><div class="info-card-header"><h3><a class="entity-link" href="/object/%s">%s</a></h3><span class="status-badge %s">%s</span></div><div class="details-list"><div class="detail-row"><span>Адрес</span><strong>%s</strong></div><div class="detail-row"><span>Ответственный</span><strong>%s</strong></div></div><div class="info-card-actions"><a class="btn btn-secondary" href="/object/%s">Открыть</a><a class="btn btn-secondary" href="/objects/edit/%s" data-modal-url="/objects/edit/%s" data-modal-title="Редактировать объект" data-modal-return="/objects?tab=%s">Редактировать</a></div></article>`,
			template.HTMLEscapeString(object.ID),
			template.HTMLEscapeString(object.Name),
			template.HTMLEscapeString(statusClass),
			template.HTMLEscapeString(objectStatusLabel(object.Status)),
			template.HTMLEscapeString(object.Address),
			template.HTMLEscapeString(responsible),
			template.HTMLEscapeString(object.ID),
			template.HTMLEscapeString(object.ID),
			template.HTMLEscapeString(object.ID),
			template.HTMLEscapeString(selectedTab),
		))
	}
	if cards.Len() == 0 {
		cards.WriteString(`<div class="info-card"><p>Объекты пока не добавлены.</p></div>`)
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
            <a href="/objects/new" class="btn btn-primary" data-modal-url="/objects/new" data-modal-title="Новый объект" data-modal-return="/objects?tab={{TAB}}">Добавить объект</a>
        </div>
        <div class="card"><div class="tab-switcher" style="margin-bottom:10px;display:flex;gap:8px;"><a class="btn btn-secondary{{TAB_ACTIVE_CLASS}}" href="/objects?tab=active">В работе</a><a class="btn btn-secondary{{TAB_COMPLETED_CLASS}}" href="/objects?tab=completed">Оконченные</a></div><div class="compact-grid">{{CARDS}}</div></div>
    </div>
</body>
</html>`

	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "objects"), 1)
	final = strings.Replace(final, "{{TAB}}", template.HTMLEscapeString(selectedTab), -1)
	tabActiveClass := ""
	tabCompletedClass := ""
	if selectedTab == "completed" {
		tabCompletedClass = " active"
	} else {
		tabActiveClass = " active"
	}
	final = strings.Replace(final, "{{TAB_ACTIVE_CLASS}}", tabActiveClass, 1)
	final = strings.Replace(final, "{{TAB_COMPLETED_CLASS}}", tabCompletedClass, 1)
	final = strings.Replace(final, "{{CARDS}}", cards.String(), 1)
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
	isModal := IsModalRequest(c)
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
    {{LAYOUT_START}}
    <div class="main-content{{MAIN_CONTENT_CLASS}}">
        {{BACK_LINK}}
        <div class="page-header"><h1>{{TITLE}}</h1></div>
        <div class="card{{CARD_CLASS}}">
            <form action="{{ACTION_URL}}" method="POST" class="form-grid-edit">
{{CSRF_FIELD}}
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
{{LAYOUT_END}}

    <div id="deleteModal" class="modal" style="display:none;">
        <div class="modal-content">
            <span class="close-button" onclick="closeDeleteModal()">&times;</span>
            <h2>Подтверждение удаления</h2>
            <p>Вы уверены, что хотите удалить объект?</p>
            <form action="/objects/delete/{{OBJECT_ID}}" method="POST">
                {{CSRF_FIELD}}
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

	layoutStart := RenderSidebar(c, "objects")
	layoutEnd := ""
	mainClass := ""
	tab := c.DefaultQuery("tab", "active")
	backLink := `<a href="/objects?tab=` + template.HTMLEscapeString(tab) + `" class="back-link">← К списку объектов</a>`
	cardClass := ""
	if isModal {
		layoutStart = `<div class="modal-form-layout">`
		layoutEnd = `</div>`
		mainClass = " modal-form-content"
		backLink = ""
		cardClass = " modal-form-card"
	}

	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "objects"), 1)
	final = strings.Replace(final, "{{LAYOUT_START}}", layoutStart, 1)
	final = strings.Replace(final, "{{LAYOUT_END}}", layoutEnd, 1)
	final = strings.Replace(final, "{{MAIN_CONTENT_CLASS}}", mainClass, 1)
	final = strings.Replace(final, "{{BACK_LINK}}", backLink, 1)
	final = strings.Replace(final, "{{CARD_CLASS}}", cardClass, 1)
	final = strings.Replace(final, "{{TITLE}}", template.HTMLEscapeString(title), -1)
	final = strings.Replace(final, "{{ACTION_URL}}", template.HTMLEscapeString(actionURL), 1)
	final = strings.Replace(final, "{{CSRF_FIELD}}", CSRFHiddenInput(c), -1)
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

func ObjectProfilePage(c *gin.Context) {
	objectID := c.Param("id")
	object, err := storage.GetObjectByID(objectID)
	if err != nil {
		c.String(http.StatusNotFound, "Object not found")
		return
	}

	users, _ := storage.GetUsers()
	responsible := "Не назначен"
	for _, u := range users {
		if u.ID == object.ResponsibleUserID {
			responsible = u.Name
			break
		}
	}

	entries, _ := storage.GetTimesheets()
	workersMap, _ := buildWorkersMap()
	related := make([]models.TimesheetEntry, 0)
	for _, entry := range entries {
		for _, oid := range entry.ObjectIDs {
			if oid == object.ID {
				related = append(related, entry)
				break
			}
		}
	}
	sort.Slice(related, func(i, j int) bool {
		if related[i].Date == related[j].Date {
			return related[i].StartTime > related[j].StartTime
		}
		return related[i].Date > related[j].Date
	})

	var assignments strings.Builder
	if len(related) == 0 {
		assignments.WriteString(`<div class="info-card"><p>По объекту пока нет назначений.</p></div>`)
	} else {
		for _, entry := range related {
			commentHTML := ""
			if strings.TrimSpace(entry.Notes) != "" {
				commentHTML = `<div class="assignment-note"><span>Комментарий</span><p>` + template.HTMLEscapeString(entry.Notes) + `</p></div>`
			}
			assignments.WriteString(fmt.Sprintf(`<article class="schedule-entry-vertical assignment-card"><div class="assignment-head"><strong>%s · %s — %s</strong><span>%s ч</span></div><div class="assignment-body"><div class="assignment-meta"><span>Работники</span><p>%s</p></div>%s</div></article>`, template.HTMLEscapeString(formatScheduleDateLabel(entry.Date)), template.HTMLEscapeString(entry.StartTime), template.HTMLEscapeString(entry.EndTime), template.HTMLEscapeString(formatWorkHours(entry.StartTime, entry.EndTime, entry.LunchBreakMinutes)), joinMappedLinks(entry.WorkerIDs, workersMap, "/worker"), commentHTML))
		}
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><title>Объект: {{OBJECT_NAME}}</title><link rel="stylesheet" href="/static/css/style.css"></head><body>
{{SIDEBAR_HTML}}
<div class="main-content">
  <a href="/objects" class="back-link">← К списку объектов</a>
  <div class="profile-header-container">
    <div class="profile-header">
      <div class="worker-avatar">🏗</div>
      <div class="profile-header-info"><h1>{{OBJECT_NAME}}</h1><p>{{OBJECT_STATUS}}</p></div>
    </div>
    <div class="profile-actions"><a class="btn btn-secondary" href="/objects/edit/{{OBJECT_ID}}" data-modal-url="/objects/edit/{{OBJECT_ID}}" data-modal-title="Редактировать объект" data-modal-return="/object/{{OBJECT_ID}}">Редактировать</a></div>
  </div>
  <ul class="profile-details"><li><strong>Адрес:</strong> {{OBJECT_ADDRESS}}</li><li><strong>Ответственный:</strong> {{RESPONSIBLE}}</li></ul>
  <div class="card"><div class="history-header"><h2>Назначения по объекту</h2></div><div class="schedule-vertical">{{ASSIGNMENTS}}</div></div>
</div></body></html>`

	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "objects"), 1)
	final = strings.Replace(final, "{{OBJECT_NAME}}", template.HTMLEscapeString(object.Name), -1)
	final = strings.Replace(final, "{{OBJECT_STATUS}}", template.HTMLEscapeString(objectStatusLabel(object.Status)), 1)
	final = strings.Replace(final, "{{OBJECT_ADDRESS}}", template.HTMLEscapeString(object.Address), 1)
	final = strings.Replace(final, "{{RESPONSIBLE}}", template.HTMLEscapeString(responsible), 1)
	final = strings.Replace(final, "{{OBJECT_ID}}", template.HTMLEscapeString(object.ID), -1)
	final = strings.Replace(final, "{{ASSIGNMENTS}}", assignments.String(), 1)
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
