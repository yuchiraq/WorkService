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
		okMsg = `<p style="color:#1f9d55;">Резервная копия успешно создана.</p>`
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><title>Настройки</title><link rel="stylesheet" href="/static/css/style.css"></head><body>
{{SIDEBAR_HTML}}
<div class="main-content">
<div class="page-header"><h1>Настройки</h1></div>
<div class="card" style="margin-bottom:14px;"><h2>Резервное копирование</h2><p>Создание локальной резервной копии файлов данных (users/workers/objects/timesheets).</p>{{OK_MSG}}<form method="POST" action="/settings/backup"><button type="submit" class="btn btn-primary">Создать резервную копию</button></form></div>
<div class="card"><h2>Мониторинг безопасности</h2><p><strong>Активные сессии:</strong> {{ACTIVE}}</p><p><strong>Заблокированные попытки входа:</strong> {{LOCKED}}</p><h3 style="margin-top:12px;">Последние события security.log</h3><ul>{{LOGS}}</ul></div>
</div></body></html>`
	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "settings"), 1)
	final = strings.Replace(final, "{{ACTIVE}}", fmt.Sprintf("%d", stats.ActiveSessions), 1)
	final = strings.Replace(final, "{{LOCKED}}", fmt.Sprintf("%d", stats.LockedAttempts), 1)
	final = strings.Replace(final, "{{LOGS}}", logsHTML.String(), 1)
	final = strings.Replace(final, "{{OK_MSG}}", okMsg, 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}
