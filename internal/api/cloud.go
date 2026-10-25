package api

import (
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"project/internal/models"
	"project/internal/storage"
)

var cloudUploadSlots = make(chan struct{}, 2)
var cloudArchiveSlots = make(chan struct{}, 1)

type cloudArchiveJob struct {
	Owner, Path string
	InUse       bool
}

var cloudArchiveMutex sync.Mutex
var cloudArchives = map[string]cloudArchiveJob{}

func removeCloudArchive(token string, force bool) {
	cloudArchiveMutex.Lock()
	defer cloudArchiveMutex.Unlock()
	job, ok := cloudArchives[token]
	if !ok || (job.InUse && !force) {
		return
	}
	_ = os.Remove(job.Path)
	delete(cloudArchives, token)
}

func cloudError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, storage.ErrCloudNotFound):
		status = http.StatusNotFound
	case errors.Is(err, storage.ErrCloudConflict), errors.Is(err, storage.ErrCloudDuplicate):
		status = http.StatusConflict
	case errors.Is(err, storage.ErrCloudQuota), errors.Is(err, storage.ErrCloudSize):
		status = http.StatusRequestEntityTooLarge
	case errors.Is(err, storage.ErrCloudName), errors.Is(err, storage.ErrCloudMove), errors.Is(err, storage.ErrCloudLimit), errors.Is(err, storage.ErrCloudArchiveLimit):
	default:
		log.Printf("cloud operation: %v", err)
		c.JSON(500, gin.H{"error": "Не удалось выполнить операцию. Повторите попытку"})
		return
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

func CloudItems(c *gin.Context) { c.JSON(200, storage.CloudState(c.GetString("userID"))) }

func MutateCloud(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var mutation storage.CloudMutation
	if err := c.ShouldBindJSON(&mutation); err != nil {
		c.JSON(400, gin.H{"error": "Некорректные данные операции"})
		return
	}
	if err := storage.ChangeCloud(c.GetString("userID"), mutation); err != nil {
		cloudError(c, err)
		return
	}
	CloudItems(c)
}

func UploadCloudFile(c *gin.Context) {
	select {
	case cloudUploadSlots <- struct{}{}:
		defer func() { <-cloudUploadSlots }()
	default:
		c.JSON(429, gin.H{"error": "Сервер занят загрузками. Повторите отправку через несколько секунд"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, storage.CloudMaxFileBytes()+(1<<20))
	reader, err := c.Request.MultipartReader()
	if err != nil {
		c.JSON(400, gin.H{"error": "Некорректная загрузка"})
		return
	}
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "file" || part.FileName() == "" {
		c.JSON(400, gin.H{"error": "Выберите один файл"})
		return
	}
	defer part.Close()
	item, err := storage.AddCloudFile(c.GetString("userID"), c.Query("parent"), part.FileName(), c.GetHeader("X-Upload-ID"), part)
	if err != nil {
		cloudError(c, err)
		return
	}
	c.JSON(200, gin.H{"item": item, "state": storage.CloudState(c.GetString("userID"))})
}

func cloudHeaders(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox; default-src 'none'; frame-ancestors 'self'")
	c.Header("Cross-Origin-Resource-Policy", "same-origin")
	c.Header("Referrer-Policy", "no-referrer")
}

func CloudFile(c *gin.Context) {
	item, file, err := storage.OpenCloudFile(c.GetString("userID"), c.Param("id"))
	if err != nil {
		cloudError(c, err)
		return
	}
	defer file.Close()
	serveCloudFile(c, item, file)
}

func serveCloudFile(c *gin.Context, item models.CloudItem, file *os.File) {
	cloudHeaders(c)
	if c.Query("preview") == "text" {
		if !strings.HasPrefix(item.MIME, "text/plain") {
			c.JSON(415, gin.H{"error": "Для этого файла доступно только скачивание"})
			return
		}
		data, err := io.ReadAll(io.LimitReader(file, (512<<10)+1))
		if err != nil {
			cloudError(c, err)
			return
		}
		truncated := len(data) > 512<<10
		if truncated {
			data = data[:512<<10]
			for len(data) > 0 && !utf8.Valid(data) && len(data) > 512*1024-4 {
				data = data[:len(data)-1]
			}
		}
		if !utf8.Valid(data) {
			c.JSON(415, gin.H{"error": "Текст не в кодировке UTF-8. Скачайте файл"})
			return
		}
		c.JSON(200, gin.H{"text": string(data), "truncated": truncated})
		return
	}
	disposition, contentType := "attachment", "application/octet-stream"
	if c.Query("preview") == "1" {
		switch item.MIME {
		case "image/jpeg", "image/png", "image/gif", "image/webp", "application/pdf", "video/mp4", "video/webm", "audio/mpeg", "audio/wav", "audio/ogg", "video/ogg":
			disposition, contentType = "inline", item.MIME
		default:
			c.JSON(415, gin.H{"error": "Для этого формата доступно скачивание"})
			return
		}
	}
	c.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": item.Name}))
	c.Header("Content-Type", contentType)
	info, err := file.Stat()
	if err != nil {
		cloudError(c, err)
		return
	}
	http.ServeContent(c.Writer, c.Request, item.Name, info.ModTime(), file)
}

func CloudArchive(c *gin.Context) {
	select {
	case cloudArchiveSlots <- struct{}{}:
		defer func() { <-cloudArchiveSlots }()
	default:
		c.JSON(429, gin.H{"error": "Архив уже формируется. Повторите попытку позже"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var request struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": "Выберите файлы для архива"})
		return
	}
	cloudArchiveMutex.Lock()
	full := len(cloudArchives) >= 3
	cloudArchiveMutex.Unlock()
	if full {
		c.JSON(429, gin.H{"error": "Дождитесь завершения текущих скачиваний архивов"})
		return
	}
	file, err := os.CreateTemp("", "workservice-cloud-*.zip")
	if err != nil {
		cloudError(c, err)
		return
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(file.Name())
		}
	}()
	defer file.Close()
	if err = storage.WriteCloudArchive(c.GetString("userID"), request.IDs, file); err != nil {
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
	token := uuid.NewString()
	cloudArchiveMutex.Lock()
	cloudArchives[token] = cloudArchiveJob{Owner: c.GetString("userID"), Path: file.Name()}
	cloudArchiveMutex.Unlock()
	keep = true
	time.AfterFunc(10*time.Minute, func() { removeCloudArchive(token, false) })
	c.JSON(200, gin.H{"url": "/cloud/archive/" + token})
}

func DownloadCloudArchive(c *gin.Context) {
	token := c.Param("token")
	cloudArchiveMutex.Lock()
	job, ok := cloudArchives[token]
	if !ok || job.Owner != c.GetString("userID") || job.InUse {
		cloudArchiveMutex.Unlock()
		c.JSON(404, gin.H{"error": "Архив недоступен. Создайте его снова"})
		return
	}
	job.InUse = true
	cloudArchives[token] = job
	cloudArchiveMutex.Unlock()
	defer removeCloudArchive(token, true)
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
	cloudHeaders(c)
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="cloud.zip"`)
	http.ServeContent(c.Writer, c.Request, "cloud.zip", info.ModTime(), file)
}
