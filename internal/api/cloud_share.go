package api

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
	"project/internal/storage"
)

func SetCloudShare(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var request struct {
		Version int64 `json:"version"`
		Enabled bool  `json:"enabled"`
		Days    int   `json:"days"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": "Некорректные настройки доступа"})
		return
	}
	if err := storage.SetCloudShare(c.GetString("userID"), c.Param("id"), request.Version, request.Enabled, request.Days); err != nil {
		cloudError(c, err)
		return
	}
	CloudItems(c)
}

func sharedCloudHeaders(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Robots-Tag", "noindex, nofollow, noarchive")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'self'; font-src 'self'; img-src 'self'; media-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
}

func cloudShareOwnerActive(id string) bool {
	user, err := storage.GetUserByID(id)
	return err == nil && user.Status == "admin"
}

func SharedCloudPage(c *gin.Context) {
	sharedCloudHeaders(c)
	view, err := storage.GetSharedCloud(c.Param("token"), c.Query("item"))
	available := err == nil && cloudShareOwnerActive(view.OwnerID)
	var out bytes.Buffer
	data := struct {
		Token     string
		Available bool
		View      storage.SharedCloudView
	}{c.Param("token"), available, view}
	if err = sharedCloudTemplate.Execute(&out, data); err != nil {
		c.Status(500)
		return
	}
	status := 200
	if !available {
		status = 404
	}
	c.Data(status, "text/html; charset=utf-8", out.Bytes())
}

func SharedCloudFile(c *gin.Context) {
	sharedCloudHeaders(c)
	item, file, err := storage.OpenSharedCloudFile(c.Param("token"), c.Param("id"))
	if err != nil {
		c.String(404, "Ссылка недоступна")
		return
	}
	defer file.Close()
	if !cloudShareOwnerActive(item.OwnerID) {
		c.String(404, "Ссылка недоступна")
		return
	}
	serveCloudFile(c, item, file)
}

var sharedCloudTemplate = template.Must(template.New("shared-cloud").Funcs(template.FuncMap{
	"size": func(n int64) string {
		if n < 1024 {
			return fmt.Sprintf("%d Б", n)
		}
		if n < 1048576 {
			return fmt.Sprintf("%.1f КБ", float64(n)/1024)
		}
		return fmt.Sprintf("%.1f МБ", float64(n)/1048576)
	},
	"image": func(m string) bool {
		return m == "image/jpeg" || m == "image/png" || m == "image/webp" || m == "image/gif"
	},
}).Parse(`<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta name="referrer" content="no-referrer"><title>{{if .Available}}{{.View.Current.Name}}{{else}}Ссылка недоступна{{end}}</title><link rel="stylesheet" href="/static/css/style.css?v=20"><link rel="stylesheet" href="/static/css/cloud.css?v=3"></head><body class="cloud-share-page"><main class="cloud-public">
<header class="cloud-public-header"><span class="cl-icon cl-folder" aria-hidden="true"></span><strong>Общий доступ</strong><span>Только просмотр</span></header>
{{if .Available}}
<nav class="cloud-breadcrumbs" aria-label="Путь к папке">{{range .View.Breadcrumbs}}<a href="/s/{{$.Token}}?item={{.ID}}">{{.Name}}</a>{{end}}</nav>
<h1>{{.View.Current.Name}}</h1>
{{if eq .View.Current.Kind "folder"}}<ul class="cloud-public-list">{{range .View.Items}}<li><a class="cloud-public-open" href="/s/{{$.Token}}?item={{.ID}}"><span class="cl-icon {{if eq .Kind "folder"}}cl-folder{{else}}cl-file{{end}}" aria-hidden="true"></span><span>{{.Name}}</span><small>{{if eq .Kind "folder"}}Папка{{else}}{{size .Size}}{{end}}</small></a>{{if eq .Kind "file"}}<a class="cl-button" href="/s/{{$.Token}}/file/{{.ID}}" download title="Скачать {{.Name}}" aria-label="Скачать {{.Name}}"><span class="cl-icon cl-download" aria-hidden="true"></span></a>{{end}}</li>{{else}}<li class="cloud-empty">Папка пуста</li>{{end}}</ul>
{{else}}<div class="cloud-public-file"><p>{{size .View.Current.Size}}</p>{{if image .View.Current.MIME}}<img src="/s/{{.Token}}/file/{{.View.Current.ID}}?preview=1" alt="{{.View.Current.Name}}">{{end}}<a class="btn btn-primary" href="/s/{{.Token}}/file/{{.View.Current.ID}}" download><span class="cl-icon cl-download" aria-hidden="true"></span>Скачать файл</a></div>{{end}}
{{else}}<h1>Ссылка недоступна</h1><p>Владелец закрыл доступ, срок действия истёк или файл удалён.</p>{{end}}
</main></body></html>`))
