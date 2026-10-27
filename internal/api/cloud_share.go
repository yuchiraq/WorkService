package api

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	c.Header("Content-Security-Policy", "default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; connect-src 'self'; worker-src 'self' blob:; style-src 'self'; font-src 'self'; img-src 'self' blob:; media-src 'self' blob:; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
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

type cloudContextWriter struct {
	context.Context
	io.Writer
}

func (w cloudContextWriter) Write(b []byte) (int, error) {
	if err := w.Context.Err(); err != nil {
		return 0, err
	}
	return w.Writer.Write(b)
}

func SharedCloudArchive(c *gin.Context) {
	sharedCloudHeaders(c)
	if c.GetHeader("Sec-Fetch-Site") == "cross-site" {
		c.JSON(403, gin.H{"error": "Откройте ссылку перед скачиванием"})
		return
	}
	view, err := storage.GetSharedCloud(c.Param("token"), c.Query("item"))
	if err != nil || !cloudShareOwnerActive(view.OwnerID) {
		c.JSON(404, gin.H{"error": "Ссылка недоступна"})
		return
	}
	select {
	case cloudArchiveSlots <- struct{}{}:
		defer func() { <-cloudArchiveSlots }()
	default:
		c.JSON(429, gin.H{"error": "Сервер формирует другой архив. Повторите чуть позже"})
		return
	}
	cloudArchiveMutex.Lock()
	publicPending := 0
	for _, job := range cloudArchives {
		if job.ShareToken != "" {
			publicPending++
		}
	}
	full := len(cloudArchives) >= 3 || publicPending >= 1
	cloudArchiveMutex.Unlock()
	if full {
		c.JSON(429, gin.H{"error": "Дождитесь завершения текущих скачиваний"})
		return
	}
	file, err := os.CreateTemp("", "workservice-cloud-*.zip")
	if err != nil {
		cloudError(c, err)
		return
	}
	keep := false
	defer func() {
		file.Close()
		if !keep {
			os.Remove(file.Name())
		}
	}()
	if err = storage.WriteSharedCloudArchive(c.Param("token"), view.Current.ID, cloudContextWriter{c.Request.Context(), file}); err != nil {
		cloudError(c, err)
		return
	}
	if err = file.Close(); err != nil {
		cloudError(c, err)
		return
	}
	if c.Request.Context().Err() != nil {
		return
	}
	if _, err = storage.GetSharedCloud(c.Param("token"), view.Current.ID); err != nil || !cloudShareOwnerActive(view.OwnerID) {
		c.JSON(404, gin.H{"error": "Доступ уже закрыт"})
		return
	}
	id := uuid.NewString()
	cloudArchiveMutex.Lock()
	cloudArchives[id] = cloudArchiveJob{Owner: view.OwnerID, Path: file.Name(), ShareToken: c.Param("token"), ItemID: view.Current.ID, Name: view.Current.Name + ".zip"}
	cloudArchiveMutex.Unlock()
	keep = true
	time.AfterFunc(10*time.Minute, func() { removeCloudArchive(id, false) })
	c.JSON(200, gin.H{"url": "/s/" + c.Param("token") + "/archive/" + id})
}

func DownloadSharedCloudArchive(c *gin.Context) {
	sharedCloudHeaders(c)
	id := c.Param("archiveID")
	cloudArchiveMutex.Lock()
	job, ok := cloudArchives[id]
	if !ok || job.ShareToken != c.Param("token") || job.ShareToken == "" || job.InUse {
		cloudArchiveMutex.Unlock()
		c.JSON(404, gin.H{"error": "Архив недоступен. Создайте его снова"})
		return
	}
	job.InUse = true
	cloudArchives[id] = job
	cloudArchiveMutex.Unlock()
	defer removeCloudArchive(id, true)
	if _, err := storage.GetSharedCloud(c.Param("token"), job.ItemID); err != nil || !cloudShareOwnerActive(job.Owner) {
		c.JSON(404, gin.H{"error": "Ссылка недоступна"})
		return
	}
	file, err := os.Open(job.Path)
	if err != nil {
		cloudError(c, err)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		cloudError(c, err)
		return
	}
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": job.Name}))
	http.ServeContent(c.Writer, c.Request, job.Name, info.ModTime(), file)
}

//go:embed templates/cloud_shared.html
var sharedCloudHTML string

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
}).Parse(sharedCloudHTML))
