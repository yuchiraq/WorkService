package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	_ "golang.org/x/image/webp"
	"project/internal/models"
	"project/internal/storage"
)

// Decoding is memory intensive; bound it across simultaneous mobile uploads.
var photoProcessing = make(chan struct{}, 1)

func requirePhotoObject(c *gin.Context) (models.Object, bool) {
	object, err := storage.GetObjectByID(c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"error": "Объект не найден"})
		return object, false
	}
	if !canManageInventory(c, object) {
		c.JSON(403, gin.H{"error": "Нет доступа к фото этого объекта"})
		return object, false
	}
	return object, true
}

func photoCanEdit(c *gin.Context, object models.Object, photo models.ObjectPhoto) bool {
	return isAdmin(c) || object.ResponsibleUserID == c.GetString("userID") || photo.CreatedByID == c.GetString("userID")
}

func photosResponse(c *gin.Context, object models.Object) gin.H {
	type item struct {
		models.ObjectPhoto
		CanEdit bool `json:"canEdit"`
	}
	items := []item{}
	for _, photo := range storage.GetObjectPhotos(object.ID) {
		items = append(items, item{photo, photoCanEdit(c, object, photo)})
	}
	return gin.H{"photos": items}
}

func ObjectPhotos(c *gin.Context) {
	if object, ok := requirePhotoObject(c); ok {
		c.JSON(200, photosResponse(c, object))
	}
}

func optimizedObjectPhoto(reader io.Reader) ([]byte, []byte, int, int, error) {
	data, err := io.ReadAll(io.LimitReader(reader, (24<<20)+1))
	if err != nil || len(data) > 24<<20 {
		return nil, nil, 0, 0, errors.New("Файл превышает 24 МБ или не читается")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png" && format != "webp") {
		return nil, nil, 0, 0, errors.New("Выберите фото в формате JPEG, PNG или WebP")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 16000 || cfg.Height > 16000 || int64(cfg.Width)*int64(cfg.Height) > 50_000_000 {
		return nil, nil, 0, 0, errors.New("Максимальное разрешение фото: 50 мегапикселей")
	}
	decoded, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, nil, 0, 0, errors.New("Повреждённое изображение")
	}
	resized := imaging.Fit(decoded, 2560, 2560, imaging.Lanczos)
	flat := imaging.Overlay(imaging.New(resized.Bounds().Dx(), resized.Bounds().Dy(), color.White), resized, image.Pt(0, 0), 1)
	var full, thumb bytes.Buffer
	if err = jpeg.Encode(&full, flat, &jpeg.Options{Quality: 88}); err != nil {
		return nil, nil, 0, 0, err
	}
	if err = jpeg.Encode(&thumb, imaging.Fit(flat, 480, 480, imaging.Lanczos), &jpeg.Options{Quality: 82}); err != nil {
		return nil, nil, 0, 0, err
	}
	return full.Bytes(), thumb.Bytes(), flat.Bounds().Dx(), flat.Bounds().Dy(), nil
}

func UploadObjectPhoto(c *gin.Context) {
	object, ok := requirePhotoObject(c)
	if !ok {
		return
	}
	select {
	case photoProcessing <- struct{}{}:
		defer func() { <-photoProcessing }()
	case <-c.Request.Context().Done():
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 25<<20)
	if err := c.Request.ParseMultipartForm(2 << 20); err != nil {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
		c.JSON(400, gin.H{"error": "Не удалось загрузить фото. Лимит файла: 24 МБ"})
		return
	}
	defer c.Request.MultipartForm.RemoveAll()
	headers := c.Request.MultipartForm.File["photo"]
	if len(headers) != 1 || headers[0].Size > 24<<20 {
		c.JSON(400, gin.H{"error": "Загрузите одно фото до 24 МБ"})
		return
	}
	file, err := headers[0].Open()
	if err != nil {
		c.JSON(400, gin.H{"error": "Не удалось прочитать фото"})
		return
	}
	defer file.Close()
	full, thumb, width, height, err := optimizedObjectPhoto(file)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	name := filepath.Base(strings.ReplaceAll(headers[0].Filename, `\`, "/"))
	if len([]rune(name)) > 180 {
		name = string([]rune(name)[:180])
	}
	photo, duplicate, err := storage.CreateObjectPhoto(models.ObjectPhoto{ObjectID: object.ID, Name: name, CreatedByID: c.GetString("userID"), CreatedByName: c.GetString("userName"), Width: width, Height: height}, full, thumb)
	if err != nil {
		photoError(c, err)
		return
	}
	response := photosResponse(c, object)
	response["duplicate"], response["id"] = duplicate, photo.ID
	c.JSON(200, response)
}

func photoError(c *gin.Context, err error) {
	status := 400
	switch {
	case errors.Is(err, storage.ErrPhotoNotFound):
		status = 404
	case errors.Is(err, storage.ErrPhotoConflict):
		status = 409
	case errors.Is(err, storage.ErrPhotoLimit), errors.Is(err, storage.ErrPhotoCaption):
	default:
		log.Printf("object photo: %v", err)
		c.JSON(500, gin.H{"error": "Не удалось сохранить фото. Повторите попытку"})
		return
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

func EditObjectPhoto(c *gin.Context) {
	object, ok := requirePhotoObject(c)
	if !ok {
		return
	}
	photo, err := storage.GetObjectPhoto(object.ID, c.Param("photoID"))
	if err != nil {
		photoError(c, err)
		return
	}
	if !photoCanEdit(c, object, photo) {
		c.JSON(403, gin.H{"error": "Изменять фото может автор, ответственный или администратор"})
		return
	}
	version, _ := strconv.ParseInt(c.PostForm("version"), 10, 64)
	if c.Param("action") == "delete" {
		err = storage.DeleteObjectPhoto(object.ID, photo.ID, version)
	} else if c.Param("action") == "caption" {
		err = storage.UpdateObjectPhoto(object.ID, photo.ID, c.PostForm("caption"), version)
	} else {
		c.JSON(404, gin.H{"error": "Действие не найдено"})
		return
	}
	if err != nil {
		photoError(c, err)
		return
	}
	c.JSON(200, photosResponse(c, object))
}

func ObjectPhotoImage(c *gin.Context) {
	object, ok := requirePhotoObject(c)
	if !ok {
		return
	}
	photo, err := storage.GetObjectPhoto(object.ID, c.Param("photoID"))
	if err != nil {
		photoError(c, err)
		return
	}
	file, err := storage.OpenPhotoFile(storage.ObjectPhotosDirectory(), photo.ID, c.Query("thumb") == "1")
	if err != nil {
		c.Status(404)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		c.Status(404)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Type", "image/jpeg")
	disposition := "inline"
	if c.Query("download") == "1" {
		disposition = "attachment"
	}
	c.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": storage.PhotoFilename(photo)}))
	http.ServeContent(c.Writer, c.Request, photo.Name, info.ModTime(), file)
}

var objectPhotosTemplate = template.Must(template.New("photos").Parse(`
<link rel="stylesheet" href="/static/css/object-photos.css?v=1">
<section id="object-photos" class="object-photos" aria-labelledby="photos-title" data-endpoint="/object/{{.ID}}/photos" data-csrf="{{.CSRF}}">
  <header class="photos-heading"><div><h2 id="photos-title">Фото объекта</h2><span id="photos-count"></span></div><button type="button" class="btn btn-primary" id="photos-add"><span class="photo-icon photo-plus" aria-hidden="true"></span>Добавить фото</button></header>
  <input id="photos-files" type="file" accept="image/jpeg,image/png,image/webp" multiple hidden aria-label="Фото для загрузки">
  <p class="photos-status" id="photos-status" role="status" aria-live="polite"></p>
  <div id="photos-upload" hidden><progress id="photos-progress" max="100" value="0" aria-label="Загрузка фото"></progress><button type="button" class="btn btn-secondary" id="photos-cancel">Остановить</button></div>
  <ul id="photos-errors" class="photos-errors" hidden></ul>
  <button type="button" class="btn btn-secondary" id="photos-retry" hidden>Повторить неудачные</button>
  <div id="photos-grid" class="photos-grid"></div>
  <button type="button" class="btn btn-secondary" id="photos-more" hidden>Показать ещё</button>
  <noscript>Для фототеки необходим JavaScript.</noscript>
  <dialog id="photo-viewer" class="confirmation-dialog photo-viewer" aria-labelledby="photo-viewer-title">
    <div class="modal-content">
      <header class="photos-heading"><h2 id="photo-viewer-title">Фото объекта</h2><button type="button" class="photo-button" data-close-dialog aria-label="Закрыть" title="Закрыть"><span class="photo-icon photo-close" aria-hidden="true"></span></button></header>
      <div class="photo-stage"><span id="photo-image-status" role="status">Загрузка фото...</span><img id="photo-image" alt="Фото объекта" hidden></div>
      <div class="photo-navigation"><button type="button" class="photo-button" id="photo-prev" aria-label="Предыдущее фото" title="Предыдущее фото"><span class="photo-icon photo-prev" aria-hidden="true"></span></button><span id="photo-position"></span><button type="button" class="photo-button" id="photo-next" aria-label="Следующее фото" title="Следующее фото"><span class="photo-icon photo-next" aria-hidden="true"></span></button><a class="photo-button" id="photo-download" aria-label="Скачать фото" title="Скачать фото"><span class="photo-icon photo-download" aria-hidden="true"></span></a></div>
      <p id="photo-meta"></p><p id="photo-caption-text"></p>
      <form id="photo-caption-form" method="post" data-native-submit><button type="button" class="btn btn-secondary" id="photo-reload" hidden>Загрузить сохранённую подпись</button><label for="photo-caption">Подпись</label><textarea id="photo-caption" name="caption" maxlength="300" rows="2"></textarea><div class="photo-actions"><button type="button" class="photo-button photo-danger" id="photo-delete" aria-label="Удалить фото" title="Удалить фото"><span class="photo-icon photo-trash" aria-hidden="true"></span></button><button type="submit" class="btn btn-primary">Сохранить подпись</button></div></form>
      <div id="photo-delete-confirmation" hidden><p>Удалить это фото?</p><div class="photo-actions"><button type="button" class="btn btn-secondary" id="photo-delete-cancel">Отмена</button><button type="button" class="btn btn-danger" id="photo-delete-confirm">Удалить</button></div></div>
    </div>
  </dialog>
  <script type="application/json" id="photos-data">{{.Data}}</script>
</section><script src="/static/js/object-photos.js?v=1" defer></script>`))

func renderObjectPhotos(c *gin.Context, object models.Object) string {
	if !canManageInventory(c, object) {
		return ""
	}
	data, err := json.Marshal(photosResponse(c, object))
	if err != nil {
		return ""
	}
	var output bytes.Buffer
	err = objectPhotosTemplate.Execute(&output, struct {
		ID, CSRF string
		Data     template.JS
	}{object.ID, c.GetString("csrfToken"), template.JS(data)})
	if err != nil {
		log.Printf("render photos: %v", err)
		return ""
	}
	return output.String()
}
