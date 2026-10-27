package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"project/internal/cloudvideo"
	"project/internal/models"
	"project/internal/storage"
)

var cloudVideos = cloudvideo.New(os.Getenv("WORKSERVICE_FFMPEG"))

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
