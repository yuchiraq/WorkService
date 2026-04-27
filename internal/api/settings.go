package api

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"project/internal/security"

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

func SettingsPage(c *gin.Context) {
	stats := GetSecurityStats()
	logs := security.ReadRecent(20)

	var logsHTML strings.Builder
	if len(logs) == 0 {
		logsHTML.WriteString("<li>Событий безопасности пока нет.</li>")
	} else {
		for _, line := range logs {
			logsHTML.WriteString("<li>" + template.HTMLEscapeString(line) + "</li>")
		}
	}

	okMsg := ""
	if c.Query("ok") == "backup" {
		okMsg = `<div class="dashboard-alert-item is-success"><strong>Резервная копия создана</strong><p>Файлы users, workers, objects и timesheets сохранены в локальный backup.</p></div>`
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
        <p>Резервирование данных, установка приложения на телефон и контроль безопасности.</p>
    </div>

    <div class="compact-grid dashboard-panels">
        <div class="info-card">
            <div class="info-card-header">
                <h2>Резервное копирование</h2>
                <span class="status-badge">backup</span>
            </div>
            <p>Создайте локальную копию файлов данных перед крупными изменениями или обновлениями.</p>
            {{OK_MSG}}
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
	final = strings.Replace(final, "{{ACTIVE}}", fmt.Sprintf("%d", stats.ActiveSessions), 1)
	final = strings.Replace(final, "{{LOCKED}}", fmt.Sprintf("%d", stats.LockedAttempts), 1)
	final = strings.Replace(final, "{{LOGS}}", logsHTML.String(), 1)
	final = strings.Replace(final, "{{OK_MSG}}", okMsg, 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}
