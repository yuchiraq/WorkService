package api

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"project/internal/models"
	"project/internal/storage"
	"project/internal/telegrambot"

	"github.com/gin-gonic/gin"
)

func userStatusLabel(status string) string {
	if status == "admin" {
		return "Админ"
	}
	return "Пользователь"
}

func formatLastLogin(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "—"
	}
	return t.Format("02.01.2006 15:04")
}

func UsersPage(c *gin.Context) {
	users, err := storage.GetUsers()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load users: %v", err)
		return
	}

	noticeBlock := ""
	switch c.Query("notice") {
	case "telegram_sent":
		noticeBlock = `<div class="dashboard-alert-item is-success"><strong>Учетка создана и сообщение отправлено</strong><p>Пользователь получил логин, адрес сайта, PWA-инструкцию и пароль в Telegram.</p></div>`
	case "telegram_chat_missing":
		noticeBlock = `<div class="dashboard-alert-item is-warning"><strong>Учетка создана, но Telegram не отправлен</strong><p>Для этого номера не найден подключенный Telegram-чат. Сотрудник должен открыть бота, нажать Start, отправить контакт и после этого нужно выполнить синхронизацию в настройках.</p></div>`
	case "telegram_bot_missing":
		noticeBlock = `<div class="dashboard-alert-item is-warning"><strong>Учетка создана, но бот не настроен</strong><p>Заполните токен, username бота и адрес сайта в настройках, чтобы отправлять данные в Telegram.</p></div>`
	case "telegram_failed":
		noticeBlock = `<div class="dashboard-alert-item is-warning"><strong>Учетка создана, но отправка в Telegram не удалась</strong><p>Проверьте настройки бота и синхронизацию контактов в разделе настроек.</p></div>`
	}

	var rows strings.Builder
	for _, user := range users {
		rows.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td><div class="table-actions"><a href="/users/edit/%s" class="btn btn-secondary" data-modal-url="/users/edit/%s" data-modal-title="Редактировать пользователя" data-modal-return="/users">Редактировать</a><form action="/users/delete/%s" method="POST" class="table-action-form"><button class="btn btn-danger" type="submit">Удалить</button></form></div></td></tr>`,
			template.HTMLEscapeString(user.Name),
			template.HTMLEscapeString(user.Username),
			template.HTMLEscapeString(user.Phone),
			template.HTMLEscapeString(userStatusLabel(user.Status)),
			template.HTMLEscapeString(formatLastLogin(user.LastLoginAt)),
			template.HTMLEscapeString(user.ID),
			template.HTMLEscapeString(user.ID),
			template.HTMLEscapeString(user.ID),
		))
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover"><title>Пользователи</title><link rel="stylesheet" href="/static/css/style.css"></head><body>
{{SIDEBAR_HTML}}
<div class="main-content">
<div class="page-header"><h1>Пользователи</h1><a href="/users/new" class="btn btn-primary" data-modal-url="/users/new" data-modal-title="Новый пользователь" data-modal-return="/users">Добавить пользователя</a></div>
<div class="card"><table class="table responsive-table users-table"><thead><tr><th>ФИО</th><th>Логин</th><th>Телефон</th><th>Статус</th><th>Последний вход</th><th>Действия</th></tr></thead><tbody>{{ROWS}}</tbody></table></div>
</div></body></html>`
	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "users"), 1)
	final = strings.Replace(final, "{{ROWS}}", rows.String(), 1)
	if noticeBlock != "" {
		final = strings.Replace(final, `<div class="card"><table class="table responsive-table users-table">`, noticeBlock+`<div class="card"><table class="table responsive-table users-table">`, 1)
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}

func userWorkerOptions(userID, selectedWorkerID string) (string, string, error) {
	workers, err := storage.GetWorkers()
	if err != nil {
		return "", "", err
	}

	if selectedWorkerID == "" {
		if linkedWorker, err := storage.GetWorkerByUserID(userID); err == nil {
			selectedWorkerID = linkedWorker.ID
		}
	}

	var options strings.Builder
	options.WriteString(`<option value="">Создать нового работника автоматически</option>`)
	selectedLabel := "Автосоздание"
	for _, worker := range workers {
		if worker.IsFired {
			continue
		}
		if worker.UserID != "" && worker.UserID != userID {
			continue
		}
		selected := ""
		if worker.ID == selectedWorkerID {
			selected = " selected"
			selectedLabel = worker.Name
		}
		options.WriteString(fmt.Sprintf(`<option value="%s"%s>%s</option>`, template.HTMLEscapeString(worker.ID), selected, template.HTMLEscapeString(worker.Name)))
	}

	return options.String(), selectedLabel, nil
}

func selectedWorkerPhone(workerID string) string {
	if strings.TrimSpace(workerID) == "" {
		return ""
	}
	worker, err := storage.GetWorkerByID(workerID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(worker.Phone)
}

func syncWorkerPhoneByID(workerID, phone string) error {
	if strings.TrimSpace(workerID) == "" {
		return nil
	}
	worker, err := storage.GetWorkerByID(workerID)
	if err != nil {
		return err
	}
	worker.Phone = strings.TrimSpace(phone)
	return storage.UpdateWorker(worker)
}

func syncLinkedWorkerPhone(userID, phone string) error {
	worker, err := storage.GetWorkerByUserID(userID)
	if err != nil {
		return nil
	}
	worker.Phone = strings.TrimSpace(phone)
	return storage.UpdateWorker(worker)
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

	workerOptions, selectedWorkerName, err := userWorkerOptions(user.ID, "")
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load workers: %v", err)
		return
	}

	workerField := `<input type="hidden" name="worker_id" value="">`
	if user.Status != "admin" || user.ID == "" {
		workerField = `<label for="worker_id">Связанный работник</label><select id="worker_id" name="worker_id">` + workerOptions + `</select><small class="text-muted">Текущее значение: ` + template.HTMLEscapeString(selectedWorkerName) + `</small>`
	}

	isModal := IsModalRequest(c)
	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover"><title>{{TITLE}}</title><link rel="stylesheet" href="/static/css/style.css"></head><body>
{{LAYOUT_START}}
<div class="main-content{{MAIN_CONTENT_CLASS}}">
{{BACK_LINK}}
<div class="page-header"><h1>{{TITLE}}</h1></div>
<div class="card{{CARD_CLASS}}">
<form action="{{ACTION_URL}}" method="POST" class="form-grid-edit">
{{CSRF_FIELD}}
<div class="form-group-edit form-group-name"><label for="name">ФИО</label><input type="text" id="name" name="name" value="{{NAME}}" required></div>
<div class="form-group-edit form-group-position"><label for="username">Логин</label><input type="text" id="username" name="username" value="{{USERNAME}}" required></div>
<div class="form-group-edit form-group-phone"><label for="password">Пароль</label><input type="password" id="password" name="password" value="" placeholder="Оставьте пустым, чтобы не менять"></div>
<div class="form-group-edit form-group-rate"><label for="phone">Контактный номер</label><input type="tel" id="phone" name="phone" value="{{PHONE}}"></div>
<div class="form-group-edit form-group-rate">{{STATUS_FIELD}}</div>
<div class="form-group-edit form-group-rate">{{WORKER_FIELD}}</div>
<div class="form-actions-edit"><button type="submit" class="btn btn-primary">{{SUBMIT_LABEL}}</button><a href="{{BACK_URL}}" class="btn btn-secondary">Отмена</a></div>
</form>
</div>
</div>
{{LAYOUT_END}}
</body></html>`

	layoutStart := RenderSidebar(c, "users")
	layoutEnd := ""
	mainClass := ""
	backLink := `<a href="{{BACK_URL}}" class="back-link">← Назад</a>`
	cardClass := ""
	if isModal {
		layoutStart = `<div class="modal-form-layout">`
		layoutEnd = `</div>`
		mainClass = " modal-form-content"
		backLink = ""
		cardClass = " modal-form-card"
	}

	final := strings.Replace(page, "{{LAYOUT_START}}", layoutStart, 1)
	final = strings.Replace(final, "{{LAYOUT_END}}", layoutEnd, 1)
	final = strings.Replace(final, "{{MAIN_CONTENT_CLASS}}", mainClass, 1)
	final = strings.Replace(final, "{{BACK_LINK}}", backLink, 1)
	final = strings.Replace(final, "{{CARD_CLASS}}", cardClass, 1)
	final = strings.Replace(final, "{{TITLE}}", template.HTMLEscapeString(title), -1)
	final = strings.Replace(final, "{{ACTION_URL}}", template.HTMLEscapeString(actionURL), 1)
	final = strings.Replace(final, "{{CSRF_FIELD}}", CSRFHiddenInput(c), -1)
	final = strings.Replace(final, "{{NAME}}", template.HTMLEscapeString(user.Name), 1)
	final = strings.Replace(final, "{{USERNAME}}", template.HTMLEscapeString(user.Username), 1)
	final = strings.Replace(final, "{{PHONE}}", template.HTMLEscapeString(user.Phone), 1)
	final = strings.Replace(final, "{{CSRF_FIELD}}", CSRFHiddenInput(c), 1)
	final = strings.Replace(final, "{{STATUS_FIELD}}", statusField, 1)
	final = strings.Replace(final, "{{WORKER_FIELD}}", workerField, 1)
	final = strings.Replace(final, "{{SUBMIT_LABEL}}", template.HTMLEscapeString(submitLabel), 1)
	final = strings.Replace(final, "{{BACK_URL}}", "/users", -1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}

func AddUserPage(c *gin.Context) {
	renderUserForm(c, models.User{Status: "user"}, "/users/new", "Новый пользователь", "Сохранить", true)
}

func CreateUser(c *gin.Context) {
	plainPassword := c.PostForm("password")
	selectedWorkerID := c.PostForm("worker_id")
	phone := strings.TrimSpace(c.PostForm("phone"))
	if phone == "" {
		phone = selectedWorkerPhone(selectedWorkerID)
	}

	newUser := models.User{
		Name:     c.PostForm("name"),
		Username: c.PostForm("username"),
		Password: plainPassword,
		Phone:    phone,
		Status:   c.PostForm("status"),
	}

	createdUser, err := storage.CreateUser(newUser)
	if err != nil {
		c.String(http.StatusBadRequest, "Failed to create user: %v", err)
		return
	}
	if createdUser.Status == "user" {
		if selectedWorkerID != "" {
			if err := storage.LinkWorkerToUser(selectedWorkerID, createdUser.ID); err != nil {
				_ = storage.DeleteUser(createdUser.ID)
				c.String(http.StatusBadRequest, "Failed to link worker: %v", err)
				return
			}
			_ = syncWorkerPhoneByID(selectedWorkerID, createdUser.Phone)
		} else {
			_, _ = storage.CreateWorker(models.Worker{
				Name:          createdUser.Name,
				Position:      "Сотрудник",
				Phone:         createdUser.Phone,
				CreatedBy:     c.GetString("userID"),
				CreatedByName: c.GetString("userName"),
				UserID:        createdUser.ID,
			})
		}
	}

	redirectURL := "/users"
	if createdUser.Status == "user" {
		notifyErr := telegrambot.SendAccountCreatedNotification(createdUser, plainPassword)
		switch {
		case notifyErr == nil:
			redirectURL += "?notice=telegram_sent"
		case errors.Is(notifyErr, telegrambot.ErrChatNotLinked):
			redirectURL += "?notice=telegram_chat_missing"
		case errors.Is(notifyErr, telegrambot.ErrBotNotConfigured):
			redirectURL += "?notice=telegram_bot_missing"
		default:
			redirectURL += "?notice=telegram_failed"
		}
	}

	c.Redirect(http.StatusFound, redirectURL)
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
	newPassword := c.PostForm("password")
	if newPassword != "" {
		user.Password = newPassword
	}
	user.Phone = c.PostForm("phone")
	user.Status = c.PostForm("status")
	selectedWorkerID := c.PostForm("worker_id")

	if err := storage.UpdateUser(user); err != nil {
		c.String(http.StatusBadRequest, "Failed to update user: %v", err)
		return
	}
	if user.Status == "user" {
		if selectedWorkerID != "" {
			if err := storage.LinkWorkerToUser(selectedWorkerID, user.ID); err != nil {
				c.String(http.StatusBadRequest, "Failed to link worker: %v", err)
				return
			}
			_ = syncWorkerPhoneByID(selectedWorkerID, user.Phone)
		} else {
			if _, err := storage.GetWorkerByUserID(user.ID); err != nil {
				_, _ = storage.CreateWorker(models.Worker{
					Name:          user.Name,
					Position:      "Сотрудник",
					Phone:         user.Phone,
					CreatedBy:     c.GetString("userID"),
					CreatedByName: c.GetString("userName"),
					UserID:        user.ID,
				})
			} else {
				_ = syncLinkedWorkerPhone(user.ID, user.Phone)
			}
		}
	} else {
		_ = storage.ClearWorkerLinkByUserID(user.ID)
	}
	c.Redirect(http.StatusFound, "/users")
}

func DeleteUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == c.GetString("userID") {
		c.String(http.StatusBadRequest, "Нельзя удалить текущего пользователя")
		return
	}
	_ = storage.ClearWorkerLinkByUserID(userID)
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

	workerBlock := `
<div class="form-group-edit form-group-name"><label for="name">ФИО</label><input type="text" id="name" name="name" value="{{NAME}}" required></div>
<div class="form-group-edit form-group-position"><label for="username">Логин</label><input type="text" id="username" name="username" value="{{USERNAME}}" required></div>
<div class="form-group-edit form-group-phone"><label for="password">Пароль</label><input type="password" id="password" name="password" value="" placeholder="Оставьте пустым, чтобы не менять"></div>
<div class="form-group-edit form-group-rate"><label for="phone">Телефон</label><input type="tel" id="phone" name="phone" value="{{PHONE}}"></div>
<div class="form-group-edit form-group-position"><label for="position">Должность</label><input type="text" id="position" name="position" value="{{POSITION}}"></div>
<div class="form-group-edit form-group-rate"><label for="birth_date">Дата рождения</label><input type="date" id="birth_date" name="birth_date" value="{{BIRTH_DATE}}"></div>
<div class="form-group-edit form-group-rate"><label for="hourly_rate">Ставка, руб/час</label><input type="number" step="0.01" min="0" id="hourly_rate" name="hourly_rate" value="{{RATE}}"></div>`
	if isAdmin(c) {
		workerBlock = `
<div class="form-group-edit form-group-name"><label for="name">ФИО</label><input type="text" id="name" name="name" value="{{NAME}}" required></div>
<div class="form-group-edit form-group-position"><label for="username">Логин</label><input type="text" id="username" name="username" value="{{USERNAME}}" required></div>
<div class="form-group-edit form-group-phone"><label for="password">Пароль</label><input type="password" id="password" name="password" value="" placeholder="Оставьте пустым, чтобы не менять"></div>
<div class="form-group-edit form-group-rate"><label for="phone">Контактный номер</label><input type="tel" id="phone" name="phone" value="{{PHONE}}"></div>`
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover"><title>Мой профиль</title><link rel="stylesheet" href="/static/css/style.css"></head><body>
{{SIDEBAR_HTML}}
<div class="main-content"><div class="page-header"><h1>Мой профиль</h1></div>
<div class="card"><form action="/profile" method="POST" class="form-grid-edit">
{{CSRF_FIELD}}
{{PROFILE_FIELDS}}
<div class="form-actions-edit"><button type="submit" class="btn btn-primary">Сохранить</button></div>
</form></div></div>
</body></html>`
	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "my-profile"), 1)
	final = strings.Replace(final, "{{PROFILE_FIELDS}}", workerBlock, 1)
	final = strings.Replace(final, "{{NAME}}", template.HTMLEscapeString(user.Name), 1)
	final = strings.Replace(final, "{{USERNAME}}", template.HTMLEscapeString(user.Username), 1)
	final = strings.Replace(final, "{{PHONE}}", template.HTMLEscapeString(user.Phone), 1)
	final = strings.Replace(final, "{{POSITION}}", "", 1)
	final = strings.Replace(final, "{{BIRTH_DATE}}", "", 1)
	final = strings.Replace(final, "{{RATE}}", "0", 1)
	if !isAdmin(c) {
		worker, err := storage.GetWorkerByUserID(userID)
		if err != nil {
			worker, _ = storage.CreateWorker(models.Worker{
				Name:          user.Name,
				Position:      "Сотрудник",
				Phone:         user.Phone,
				CreatedBy:     userID,
				CreatedByName: user.Name,
				UserID:        userID,
			})
		}
		final = strings.Replace(final, "{{NAME}}", template.HTMLEscapeString(worker.Name), 1)
		final = strings.Replace(final, "{{PHONE}}", template.HTMLEscapeString(worker.Phone), 1)
		final = strings.Replace(final, "{{POSITION}}", template.HTMLEscapeString(worker.Position), 1)
		final = strings.Replace(final, "{{BIRTH_DATE}}", template.HTMLEscapeString(worker.BirthDate), 1)
		final = strings.Replace(final, "{{RATE}}", fmt.Sprintf("%.2f", worker.HourlyRate), 1)
	}
	final = strings.Replace(final, "{{CSRF_FIELD}}", CSRFHiddenInput(c), 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}

func UpdateProfile(c *gin.Context) {
	userID := c.GetString("userID")
	user, err := storage.GetUserByID(userID)
	if err != nil {
		c.String(http.StatusNotFound, "User not found")
		return
	}

	if !isAdmin(c) {
		worker, err := storage.GetWorkerByUserID(userID)
		if err != nil {
			worker, err = storage.CreateWorker(models.Worker{
				Name:          user.Name,
				Position:      "Сотрудник",
				Phone:         user.Phone,
				CreatedBy:     userID,
				CreatedByName: user.Name,
				UserID:        userID,
			})
			if err != nil {
				c.String(http.StatusBadRequest, "Не удалось создать профиль работника: %v", err)
				return
			}
		}
		user.Username = c.PostForm("username")
		user.Name = c.PostForm("name")
		newPassword := c.PostForm("password")
		if newPassword != "" {
			user.Password = newPassword
		}
		user.Phone = c.PostForm("phone")
		if err := storage.UpdateUser(user); err != nil {
			c.String(http.StatusBadRequest, "Failed to update profile: %v", err)
			return
		}
		worker.Name = c.PostForm("name")
		worker.Position = c.PostForm("position")
		worker.Phone = c.PostForm("phone")
		worker.BirthDate = c.PostForm("birth_date")
		rate, _ := strconv.ParseFloat(strings.TrimSpace(c.PostForm("hourly_rate")), 64)
		worker.HourlyRate = rate
		if err := storage.UpdateWorker(worker); err != nil {
			c.String(http.StatusBadRequest, "Failed to update profile: %v", err)
			return
		}
		c.Redirect(http.StatusFound, "/profile")
		return
	}

	user.Name = c.PostForm("name")
	user.Username = c.PostForm("username")
	newPassword := c.PostForm("password")
	if newPassword != "" {
		user.Password = newPassword
	}
	user.Phone = c.PostForm("phone")

	if err := storage.UpdateUser(user); err != nil {
		c.String(http.StatusBadRequest, "Failed to update profile: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/profile")
}
