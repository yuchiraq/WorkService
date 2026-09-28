package api

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"project/internal/security"
	"project/internal/storage"
	"project/internal/telegrambot"

	"github.com/gin-gonic/gin"
)

func copyFile(src, dst string) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, content, 0o644); err != nil {
		return err
	}
	return nil
}

func CreateBackup(c *gin.Context) {
	backupDir := filepath.Join("storage", "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		c.String(http.StatusInternalServerError, "backup dir error: %v", err)
		return
	}
	ts := time.Now().Format("20060102-150405")
	files := []string{"users.json", "workers.json", "objects.json", "timesheets.json", "vehicles.json"}
	for _, f := range files {
		src := filepath.Join("storage", f)
		dst := filepath.Join(backupDir, strings.TrimSuffix(f, ".json")+"-"+ts+".json")
		if err := copyFile(src, dst); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			c.String(http.StatusInternalServerError, "backup failed for %s: %v", f, err)
			return
		}
	}
	security.LogEvent("backup_created", fmt.Sprintf("user=%s time=%s", c.GetString("userName"), ts))
	c.Redirect(http.StatusFound, "/settings?ok=backup")
}

func SaveTelegramSettings(c *gin.Context) {
	settings, err := storage.GetAppSettings()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load app settings: %v", err)
		return
	}
	settings.TelegramBotToken = c.PostForm("telegram_bot_token")
	settings.TelegramBotUsername = c.PostForm("telegram_bot_username")
	settings.TelegramSiteURL = c.PostForm("telegram_site_url")
	if err := storage.UpdateAppSettings(settings); err != nil {
		c.String(http.StatusInternalServerError, "Failed to save telegram settings: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/settings?ok=telegram_saved")
}

func SyncTelegramContacts(c *gin.Context) {
	summary, err := telegrambot.SyncContacts()
	if err != nil {
		c.Redirect(http.StatusFound, "/settings?telegram_error="+template.URLQueryEscaper(err.Error()))
		return
	}
	c.Redirect(http.StatusFound, "/settings?ok=telegram_synced&processed="+strconv.Itoa(summary.Processed)+"&linked="+strconv.Itoa(summary.Linked))
}

func DeleteTelegramContact(c *gin.Context) {
	if err := storage.DeleteTelegramContactByPhone(c.PostForm("phone")); err != nil {
		c.Redirect(http.StatusFound, "/settings?telegram_error="+template.URLQueryEscaper(err.Error()))
		return
	}
	c.Redirect(http.StatusFound, "/settings?ok=telegram_deleted")
}

func SettingsPage(c *gin.Context) {
	stats := GetSecurityStats()
	logs := security.ReadRecent(20)
	settings, _ := storage.GetAppSettings()
	telegramContacts, _ := storage.GetTelegramContacts()

	var logsHTML strings.Builder
	if len(logs) == 0 {
		logsHTML.WriteString("<li>Событий безопасности пока нет.</li>")
	} else {
		for _, line := range logs {
			logsHTML.WriteString("<li>" + template.HTMLEscapeString(line) + "</li>")
		}
	}

	statusBlock := ""
	switch c.Query("ok") {
	case "backup":
		statusBlock = `<div class="dashboard-alert-item is-success"><strong>Резервная копия создана</strong><p>Пользователи, работники, объекты, табель и транспорт сохранены в локальный backup.</p></div>`
	case "telegram_saved":
		statusBlock = `<div class="dashboard-alert-item is-success"><strong>Настройки Telegram сохранены</strong><p>Токен, username бота и адрес сайта обновлены.</p></div>`
	case "telegram_synced":
		statusBlock = `<div class="dashboard-alert-item is-success"><strong>Контакты Telegram синхронизированы</strong><p>Обновлений обработано: ` + template.HTMLEscapeString(c.Query("processed")) + `. Привязок по телефону обновлено: ` + template.HTMLEscapeString(c.Query("linked")) + `.</p></div>`
	case "telegram_deleted":
		statusBlock = `<div class="dashboard-alert-item is-success"><strong>Привязка Telegram удалена</strong><p>Сотрудник сможет отправить контакт в бота повторно, если привязку нужно восстановить.</p></div>`
	}
	if errMsg := strings.TrimSpace(c.Query("telegram_error")); errMsg != "" {
		statusBlock += `<div class="dashboard-alert-item is-warning"><strong>Telegram не синхронизирован</strong><p>` + template.HTMLEscapeString(errMsg) + `</p></div>`
	}

	startBotLink := ""
	if strings.TrimSpace(settings.TelegramBotUsername) != "" {
		startBotLink = `<a class="btn btn-secondary" href="https://t.me/` + template.HTMLEscapeString(settings.TelegramBotUsername) + `" target="_blank" rel="noreferrer">Открыть бота</a>`
	}

	var contactsHTML strings.Builder
	if len(telegramContacts) == 0 {
		contactsHTML.WriteString(`<div class="dashboard-list-item"><strong>Пока нет привязок</strong><p>После того как сотрудник откроет бота и отправит свой контакт, здесь появится связка телефона и Telegram-чата.</p></div>`)
	} else {
		for i, contact := range telegramContacts {
			if i >= 6 {
				break
			}
			label := strings.TrimSpace(strings.TrimSpace(contact.FirstName + " " + contact.LastName))
			if label == "" {
				label = contact.Phone
			}
			meta := contact.Phone
			if strings.TrimSpace(contact.Username) != "" {
				meta += " · @" + contact.Username
			}
			contactsHTML.WriteString(`<div class="dashboard-list-item telegram-contact-item"><div><strong>` + template.HTMLEscapeString(label) + `</strong><p>` + template.HTMLEscapeString(meta) + `</p></div><form method="POST" action="/settings/telegram/delete" class="table-action-form">` + CSRFHiddenInput(c) + `<input type="hidden" name="phone" value="` + template.HTMLEscapeString(contact.Phone) + `"><button type="submit" class="btn btn-danger btn-compact">Удалить</button></form></div>`)
		}
	}

	page := `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
    <title>Настройки</title>
    <link rel="stylesheet" href="/static/css/style.css?v=17">
</head>
<body>
{{SIDEBAR_HTML}}
<div class="main-content">
    <div class="page-header">
        <h1>Настройки</h1>

    </div>

    {{STATUS_BLOCK}}

    <div class="compact-grid dashboard-panels">
        <div class="info-card">
            <div class="info-card-header">
                <h2>Резервное копирование</h2>

            </div>
            <form method="POST" action="/settings/backup">
                <button type="submit" class="btn btn-primary">Создать резервную копию</button>
            </form>
        </div>

        <div class="info-card">
            <div class="info-card-header">
                <h2>Установка на телефон</h2>
                <span class="status-badge">PWA</span>
            </div>
            <details class="settings-disclosure"><summary>Установка приложения</summary><div class="pwa-steps">
                <div class="pwa-step">
                    <strong>iPhone / Safari</strong>
                    <p>Safari → «Поделиться» → «На экран Домой».</p>
                </div>
                <div class="pwa-step">
                    <strong>Android / Chrome</strong>
                    <p>Меню Chrome → «Установить приложение».</p>
                </div>
            </div></details>
            <div class="site-notifications-panel"><button type="button" class="btn btn-secondary" data-enable-site-notifications>Включить уведомления сайта</button><span class="status-badge" data-site-notification-status>Проверка...</span></div>
        </div>
    </div>

    <div class="compact-grid dashboard-panels">
        <div class="info-card">
            <div class="info-card-header">
                <h2>Telegram-бот</h2>
                <span class="status-badge">` + template.HTMLEscapeString(strconv.Itoa(len(telegramContacts))) + ` контактов</span>
            </div>
            <form method="POST" action="/settings/telegram" class="form-grid-edit">
                <div class="form-group-edit form-group-name"><label for="telegram_bot_token">Токен бота</label><input type="password" autocomplete="off" id="telegram_bot_token" name="telegram_bot_token" value="` + template.HTMLEscapeString(settings.TelegramBotToken) + `" placeholder="123456:ABC..."></div>
                <div class="form-group-edit form-group-position"><label for="telegram_bot_username">Username бота</label><input type="text" id="telegram_bot_username" name="telegram_bot_username" value="` + template.HTMLEscapeString(settings.TelegramBotUsername) + `" placeholder="my_company_bot"></div>
                <div class="form-group-edit timesheet-span-2"><label for="telegram_site_url">Адрес сайта</label><input type="url" id="telegram_site_url" name="telegram_site_url" value="` + template.HTMLEscapeString(settings.TelegramSiteURL) + `" placeholder="https://example.com"></div>
                <div class="form-actions-edit"><button type="submit" class="btn btn-primary">Сохранить настройки бота</button></div>
            </form>
            <div class="info-card-actions">
                <form method="POST" action="/settings/telegram/sync"><button type="submit" class="btn btn-secondary">Синхронизировать контакты из бота</button></form>
                ` + startBotLink + `
            </div>
            <div class="dashboard-list">` + contactsHTML.String() + `</div>
        </div>

        <details class="settings-disclosure"><summary>Подключение Telegram</summary>
            <div class="pwa-steps">
                <div class="pwa-step">
                    <strong>1. Открыть бота</strong>
                    <p>Сотрудник открывает вашего бота в Telegram и нажимает Start.</p>
                </div>
                <div class="pwa-step">
                    <strong>2. Отправить контакт</strong>
                    <p>После Start бот покажет кнопку «Отправить контакт». Сотрудник нажимает её и подтверждает отправку своего номера. Номер должен совпадать с телефоном в карточке работника.</p>
                </div>
                <div class="pwa-step">
                    <strong>3. Синхронизировать</strong>
                    <p>Нажмите «Синхронизировать контакты из бота».</p>
                </div>
            </div>
        </details>
    </div>

    <div class="card">
        <h2>Мониторинг безопасности</h2>
        <p><strong>Активные сессии:</strong> {{ACTIVE}}</p>
        <p><strong>Заблокированные попытки входа:</strong> {{LOCKED}}</p>
        <details class="settings-disclosure"><summary>Последние события</summary><ul>{{LOGS}}</ul></details>
    </div>
</div>
</body>
</html>`

	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "settings"), 1)
	final = strings.Replace(final, "{{STATUS_BLOCK}}", statusBlock, 1)
	final = strings.Replace(final, "{{ACTIVE}}", fmt.Sprintf("%d", stats.ActiveSessions), 1)
	final = strings.Replace(final, "{{LOCKED}}", fmt.Sprintf("%d", stats.LockedAttempts), 1)
	final = strings.Replace(final, "{{LOGS}}", logsHTML.String(), 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}
