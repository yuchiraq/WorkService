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
	files := []string{"users.json", "workers.json", "objects.json", "timesheets.json"}
	for _, f := range files {
		src := filepath.Join("storage", f)
		dst := filepath.Join(backupDir, strings.TrimSuffix(f, ".json")+"-"+ts+".json")
		if err := copyFile(src, dst); err != nil {
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
		statusBlock = `<div class="dashboard-alert-item is-success"><strong>Резервная копия создана</strong><p>Файлы users, workers, objects и timesheets сохранены в локальный backup.</p></div>`
	case "telegram_saved":
		statusBlock = `<div class="dashboard-alert-item is-success"><strong>Настройки Telegram сохранены</strong><p>Токен, username бота и адрес сайта обновлены.</p></div>`
	case "telegram_synced":
		statusBlock = `<div class="dashboard-alert-item is-success"><strong>Контакты Telegram синхронизированы</strong><p>Обновлений обработано: ` + template.HTMLEscapeString(c.Query("processed")) + `. Привязок по телефону обновлено: ` + template.HTMLEscapeString(c.Query("linked")) + `.</p></div>`
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
			contactsHTML.WriteString(`<div class="dashboard-list-item"><strong>` + template.HTMLEscapeString(label) + `</strong><p>` + template.HTMLEscapeString(meta) + `</p></div>`)
		}
	}

	page := `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
    <title>Настройки</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
{{SIDEBAR_HTML}}
<div class="main-content">
    <div class="page-header">
        <h1>Настройки</h1>
        <p>Резервирование данных, установка приложения на телефон, Telegram-бот и контроль безопасности.</p>
    </div>

    {{STATUS_BLOCK}}

    <div class="compact-grid dashboard-panels">
        <div class="info-card">
            <div class="info-card-header">
                <h2>Резервное копирование</h2>
                <span class="status-badge">backup</span>
            </div>
            <p>Создайте локальную копию файлов данных перед крупными изменениями или обновлениями.</p>
            <form method="POST" action="/settings/backup">
                <button type="submit" class="btn btn-primary">Создать резервную копию</button>
            </form>
        </div>

        <div class="info-card">
            <div class="info-card-header">
                <h2>Установка на телефон</h2>
                <span class="status-badge">PWA</span>
            </div>
            <p>Сервис «АВАЮССТРОЙ» можно поставить как обычное приложение: иконка появится на главном экране, а система будет открываться без лишних вкладок браузера.</p>
            <div class="pwa-steps">
                <div class="pwa-step">
                    <strong>iPhone / Safari</strong>
                    <p>Откройте сайт в Safari, нажмите «Поделиться», выберите «На экран Домой», при наличии включите «Открывать как веб-приложение», затем нажмите «Добавить».</p>
                </div>
                <div class="pwa-step">
                    <strong>Android / Chrome</strong>
                    <p>Откройте сайт в Chrome, откройте меню браузера и выберите «Добавить на главный экран» или «Установить приложение», затем подтвердите установку.</p>
                </div>
            </div>
            <p class="pwa-meta">Если кнопка не появилась, откройте сайт именно в Safari или Chrome, дождитесь полной загрузки страницы и попробуйте снова.</p>
        </div>
    </div>

    <div class="compact-grid dashboard-panels">
        <div class="info-card">
            <div class="info-card-header">
                <h2>Telegram-бот</h2>
                <span class="status-badge">` + template.HTMLEscapeString(strconv.Itoa(len(telegramContacts))) + ` контактов</span>
            </div>
            <p>Telegram не позволяет боту написать человеку первым только по номеру телефона. Рабочая схема такая: сотрудник сам открывает бота, нажимает Start, отправляет свой контакт, после этого система сможет сопоставить его телефон и отправлять сообщение о создании учётки.</p>
            <form method="POST" action="/settings/telegram" class="form-grid-edit">
                <div class="form-group-edit form-group-name"><label for="telegram_bot_token">Токен бота</label><input type="text" id="telegram_bot_token" name="telegram_bot_token" value="` + template.HTMLEscapeString(settings.TelegramBotToken) + `" placeholder="123456:ABC..."></div>
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

        <div class="info-card">
            <div class="info-card-header">
                <h2>Как подключить сотрудника</h2>
                <span class="status-badge">bot flow</span>
            </div>
            <div class="pwa-steps">
                <div class="pwa-step">
                    <strong>1. Открыть бота</strong>
                    <p>Сотрудник открывает вашего бота в Telegram и нажимает Start.</p>
                </div>
                <div class="pwa-step">
                    <strong>2. Отправить контакт</strong>
                    <p>Сотрудник отправляет в бот свой контакт с тем же номером, который указан у него в системе.</p>
                </div>
                <div class="pwa-step">
                    <strong>3. Синхронизировать</strong>
                    <p>В настройках нажмите «Синхронизировать контакты из бота». После этого при создании учётки система сможет отправить логин, сайт, PWA-инструкцию и пароль в этот Telegram-чат.</p>
                </div>
            </div>
        </div>
    </div>

    <div class="card">
        <h2>Мониторинг безопасности</h2>
        <p><strong>Активные сессии:</strong> {{ACTIVE}}</p>
        <p><strong>Заблокированные попытки входа:</strong> {{LOCKED}}</p>
        <h3 style="margin-top:12px;">Последние события security.log</h3>
        <ul>{{LOGS}}</ul>
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
