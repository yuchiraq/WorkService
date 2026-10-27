package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"project/internal/cloudvideo"
	"project/internal/models"
	"project/internal/storage"
)

var cloudVideos = cloudvideo.New(os.Getenv("WORKSERVICE_FFMPEG"))
var cloudThumbnails = cloudvideo.NewThumbnails(os.Getenv("WORKSERVICE_FFMPEG"))

func serveCloudThumbnail(c *gin.Context, item models.CloudItem, original *os.File) {
	var header [512]byte
	n, _ := original.ReadAt(header[:], 0)
	image := item.MIME == "image/jpeg" || item.MIME == "image/png" || item.MIME == "image/gif" || item.MIME == "image/webp"
	if !image && !storage.CloudVideoCandidate(item.Name, item.MIME, header[:n]) {
		c.JSON(415, gin.H{"error": "Миниатюра для этого формата недоступна"})
		return
	}
	dir, _ := filepath.Abs(storage.CloudDirectory())
	key := fmt.Sprintf("%s\x00%s\x00%s", dir, item.ID, item.SHA256)
	result := cloudThumbnails.Prepare(key, original.Name())
	if result.State == "processing" {
		select {
		case <-c.Request.Context().Done():
			return
		case <-time.After(80 * time.Millisecond):
		}
		result = cloudThumbnails.Prepare(key, original.Name())
	}
	if result.State != "ready" {
		status := 202
		if result.State == "error" || result.State == "unavailable" {
			status = 422
		}
		c.Header("Retry-After", "1")
		c.JSON(status, result)
		return
	}
	f, err := cloudThumbnails.Open(key)
	if err != nil {
		c.JSON(404, gin.H{"error": "Миниатюра недоступна"})
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		cloudError(c, err)
		return
	}
	c.Header("Content-Type", "image/jpeg")
	c.Header("Content-Disposition", "inline; filename=thumbnail.jpg")
	http.ServeContent(c.Writer, c.Request, "thumbnail.jpg", info.ModTime(), f)
}

func serveCloudVideo(c *gin.Context, item models.CloudItem, original *os.File) {
	var header [512]byte
	n, _ := original.ReadAt(header[:], 0)
	if !storage.CloudVideoCandidate(item.Name, item.MIME, header[:n]) {
		c.JSON(415, gin.H{"error": "Этот файл не является поддерживаемым видео"})
		return
	}
	dir, _ := filepath.Abs(storage.CloudDirectory())
	key := fmt.Sprintf("%s\x00%s\x00%s", dir, item.ID, item.SHA256)
	if c.Query("preview") == "video" {
		result := cloudVideos.Prepare(key, original.Name())
		status := 200
		switch result.State {
		case "processing", "busy":
			status = 202
			c.Header("Retry-After", "2")
		case "unavailable":
			status = 503
		case "error":
			status = 422
		}
		c.JSON(status, result)
		return
	}
	file, err := cloudVideos.Open(key)
	if err != nil {
		c.JSON(404, gin.H{"error": "Предпросмотр ещё не готов. Откройте видео снова"})
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		cloudError(c, err)
		return
	}
	c.Header("Content-Type", "video/mp4")
	c.Header("Content-Disposition", "inline; filename=preview.mp4")
	http.ServeContent(c.Writer, c.Request, "preview.mp4", info.ModTime(), file)
}
