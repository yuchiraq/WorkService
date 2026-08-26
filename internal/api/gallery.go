package api

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"project/internal/security"
	"project/internal/storage"
	"project/internal/telegrambot"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	xdraw "golang.org/x/image/draw"
)

const (
	galleryMaxRequestSize = 64 << 20
	galleryMaxFileSize    = 24 << 20
	galleryMaxFiles       = 12
	galleryMaxDimension   = 2560
	galleryJPEGQuality    = 88
)

type galleryImageInfo struct {
	Name       string
	URLName    string
	Size       int64
	Width      int
	Height     int
	ModifiedAt time.Time
}

func galleryDirectory() (string, error) {
	settings, err := storage.GetAppSettings()
	if err != nil {
		return "", err
	}
	directory := strings.TrimSpace(settings.GalleryDirectory)
	if directory == "" {
		return "", errors.New("папка галереи не настроена")
	}
	if !filepath.IsAbs(directory) {
		return "", errors.New("путь к галерее должен быть абсолютным")
	}
	directory = filepath.Clean(directory)
	info, err := os.Stat(directory)
	if err != nil {
		return "", fmt.Errorf("папка галереи недоступна: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("путь галереи не является папкой")
	}
	return directory, nil
}

func isGalleryImageName(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png"
}

func galleryFilePath(directory, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, `/\`) || !isGalleryImageName(name) {
		return "", errors.New("недопустимое имя файла")
	}
	candidate := filepath.Join(directory, name)
	relative, err := filepath.Rel(directory, candidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("файл находится вне папки галереи")
	}
	return candidate, nil
}

func regularGalleryFile(directory, name string) (string, os.FileInfo, error) {
	path, err := galleryFilePath(directory, name)
	if err != nil {
		return "", nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", nil, errors.New("доступны только обычные файлы изображений")
	}
	return path, info, nil
}

func listGalleryImages(directory string) ([]galleryImageInfo, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	images := make([]galleryImageInfo, 0)
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() || !isGalleryImageName(entry.Name()) {
			continue
		}
		path, info, err := regularGalleryFile(directory, entry.Name())
		if err != nil {
			continue
		}
		item := galleryImageInfo{Name: entry.Name(), URLName: url.PathEscape(entry.Name()), Size: info.Size(), ModifiedAt: info.ModTime()}
		if file, openErr := os.Open(path); openErr == nil {
			if config, _, decodeErr := image.DecodeConfig(file); decodeErr == nil {
				item.Width = config.Width
				item.Height = config.Height
			}
			_ = file.Close()
		}
		images = append(images, item)
	}
	sortGalleryImages(images)
	return images, nil
}

func sortGalleryImages(images []galleryImageInfo) {
	for i := 1; i < len(images); i++ {
		for j := i; j > 0; j-- {
			left, right := images[j-1], images[j]
			if left.ModifiedAt.After(right.ModifiedAt) || (left.ModifiedAt.Equal(right.ModifiedAt) && strings.ToLower(left.Name) <= strings.ToLower(right.Name)) {
				break
			}
			images[j-1], images[j] = images[j], images[j-1]
		}
	}
}

func galleryFileSize(size int64) string {
	if size >= 1<<20 {
		return fmt.Sprintf("%.1f МБ", float64(size)/(1<<20))
	}
	return fmt.Sprintf("%.0f КБ", float64(size)/(1<<10))
}

func galleryFilesLabel(count int) string {
	lastTwo := count % 100
	if lastTwo >= 11 && lastTwo <= 14 {
		return strconv.Itoa(count) + " файлов"
	}

	switch count % 10 {
	case 1:
		return strconv.Itoa(count) + " файл"
	case 2, 3, 4:
		return strconv.Itoa(count) + " файла"
	default:
		return strconv.Itoa(count) + " файлов"
	}
}

func sanitizeGalleryBaseName(filename string) string {
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	var result strings.Builder
	previousDash := false
	for _, r := range base {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-':
			result.WriteRune(r)
			previousDash = r == '-'
		case unicode.IsSpace(r) && !previousDash:
			result.WriteRune('-')
			previousDash = true
		}
	}
	cleaned := strings.Trim(result.String(), "-_.")
	if cleaned == "" {
		return "image"
	}
	return cleaned
}

func uniqueGalleryPath(directory, base, extension string) (string, string, error) {
	for index := 0; index < 10000; index++ {
		name := base + extension
		if index > 0 {
			name = base + "-" + strconv.Itoa(index+1) + extension
		}
		path, err := galleryFilePath(directory, name)
		if err != nil {
			return "", "", err
		}
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			return path, name, nil
		} else if err != nil {
			return "", "", err
		}
	}
	return "", "", errors.New("не удалось подобрать свободное имя файла")
}

func resizedGalleryImage(source image.Image) image.Image {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= galleryMaxDimension && height <= galleryMaxDimension {
		return source
	}
	scale := float64(galleryMaxDimension) / float64(width)
	if height > width {
		scale = float64(galleryMaxDimension) / float64(height)
	}
	targetWidth := max(1, int(float64(width)*scale+0.5))
	targetHeight := max(1, int(float64(height)*scale+0.5))
	target := image.NewNRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	xdraw.CatmullRom.Scale(target, target.Bounds(), source, bounds, xdraw.Over, nil)
	return target
}

func saveOptimizedGalleryImage(source io.Reader, directory, originalName string) (string, error) {
	data, err := io.ReadAll(io.LimitReader(source, galleryMaxFileSize+1))
	if err != nil {
		return "", errors.New("не удалось прочитать изображение")
	}
	if len(data) > galleryMaxFileSize {
		return "", errors.New("файл превышает 24 МБ")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", errors.New("файл не является корректным JPEG или PNG")
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 12000 || config.Height > 12000 || int64(config.Width)*int64(config.Height) > 80_000_000 {
		return "", errors.New("слишком большое разрешение изображения")
	}
	decoded, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil || decodedFormat != format {
		return "", errors.New("не удалось декодировать изображение")
	}
	format = strings.ToLower(format)
	extension := ".jpg"
	if format == "png" {
		extension = ".png"
	} else if format != "jpeg" {
		return "", errors.New("поддерживаются только JPEG и PNG")
	}

	finalPath, finalName, err := uniqueGalleryPath(directory, sanitizeGalleryBaseName(originalName), extension)
	if err != nil {
		return "", err
	}
	temporaryPath := filepath.Join(directory, ".gallery-upload-"+uuid.NewString()+".tmp")
	temporary, err := os.OpenFile(temporaryPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	keepTemporary := true
	defer func() {
		_ = temporary.Close()
		if keepTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	optimized := resizedGalleryImage(decoded)
	if format == "png" {
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		err = encoder.Encode(temporary, optimized)
	} else {
		err = jpeg.Encode(temporary, optimized, &jpeg.Options{Quality: galleryJPEGQuality})
	}
	if err != nil {
		return "", err
	}
	if err := temporary.Sync(); err != nil {
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return "", err
	}
	keepTemporary = false
	return finalName, nil
}

func validateGalleryUpload(header *multipart.FileHeader) error {
	if header == nil || header.Size <= 0 {
		return errors.New("пустой файл")
	}
	if header.Size > galleryMaxFileSize {
		return errors.New("файл превышает 24 МБ")
	}
	if !isGalleryImageName(header.Filename) {
		return errors.New("поддерживаются только JPEG и PNG")
	}
	return nil
}

func notifyGalleryAdmins(actorName, action string, filenames []string) {
	go func() {
		if err := telegrambot.SendAdminGalleryChangeNotification(actorName, action, filenames); err != nil && !errors.Is(err, telegrambot.ErrBotNotConfigured) {
			log.Printf("gallery Telegram notification failed: %v", err)
		}
	}()
}

func GalleryPage(c *gin.Context) {
	settings, _ := storage.GetAppSettings()
	directory, directoryErr := galleryDirectory()
	images := []galleryImageInfo{}
	if directoryErr == nil {
		images, directoryErr = listGalleryImages(directory)
	}

	statusBlock := ""
	switch c.Query("ok") {
	case "path":
		statusBlock = `<div class="dashboard-alert-item is-success"><strong>Папка галереи сохранена</strong><p>Содержимое папки уже доступно ниже.</p></div>`
	case "uploaded":
		statusBlock = `<div class="dashboard-alert-item is-success"><strong>Изображения добавлены</strong><p>Обработано файлов: ` + template.HTMLEscapeString(c.Query("count")) + `. Напоминание администраторам о перезапуске сайта поставлено на отправку.</p></div>`
	case "deleted":
		statusBlock = `<div class="dashboard-alert-item is-success"><strong>Изображение удалено</strong><p>Напоминание администраторам о перезапуске сайта поставлено на отправку.</p></div>`
	}
	if errorMessage := strings.TrimSpace(c.Query("error")); errorMessage != "" {
		statusBlock += `<div class="dashboard-alert-item is-warning"><strong>Операция не выполнена</strong><p>` + template.HTMLEscapeString(errorMessage) + `</p></div>`
	}
	if skipped := strings.TrimSpace(c.Query("skipped")); skipped != "" {
		statusBlock += `<div class="dashboard-alert-item is-warning"><strong>Часть файлов пропущена</strong><p>` + template.HTMLEscapeString(skipped) + `</p></div>`
	}

	var galleryHTML strings.Builder
	for _, item := range images {
		dimensions := ""
		if item.Width > 0 && item.Height > 0 {
			dimensions = fmt.Sprintf("%d × %d · ", item.Width, item.Height)
		}
		galleryHTML.WriteString(`<article class="gallery-item"><a class="gallery-image-link" href="/gallery/image/` + item.URLName + `" target="_blank" rel="noreferrer"><img src="/gallery/image/` + item.URLName + `" alt="` + template.HTMLEscapeString(item.Name) + `" loading="lazy"></a><div class="gallery-item-body"><div><strong title="` + template.HTMLEscapeString(item.Name) + `">` + template.HTMLEscapeString(item.Name) + `</strong><p>` + template.HTMLEscapeString(dimensions+galleryFileSize(item.Size)) + `</p></div><form method="POST" action="/gallery/delete" onsubmit="return confirm('Удалить изображение из галереи?')">` + CSRFHiddenInput(c) + `<input type="hidden" name="filename" value="` + template.HTMLEscapeString(item.Name) + `"><button type="submit" class="btn btn-danger btn-compact">Удалить</button></form></div></article>`)
	}
	if galleryHTML.Len() == 0 && directoryErr == nil {
		galleryHTML.WriteString(`<div class="empty-state-inline"><strong>Папка пока пустая</strong><p>Загрузите первые изображения через форму выше.</p></div>`)
	}

	gallerySection := ""
	if directoryErr != nil {
		gallerySection = `<div class="dashboard-alert-item is-warning gallery-path-warning"><strong>Галерея недоступна</strong><p>` + template.HTMLEscapeString(directoryErr.Error()) + `</p></div>`
	} else {
		gallerySection = `<section class="gallery-upload-panel"><div class="section-heading"><div><h2>Добавить изображения</h2><p>JPEG или PNG, до 12 файлов за одну загрузку.</p></div><span class="status-badge">` + galleryFilesLabel(len(images)) + `</span></div><form method="POST" action="/gallery/upload" enctype="multipart/form-data" class="gallery-upload-form">` + CSRFHiddenInput(c) + `<label class="gallery-file-picker" for="gallery_images"><span>Выбрать изображения</span><input id="gallery_images" type="file" name="images" accept="image/jpeg,image/png" multiple required></label><button type="submit" class="btn btn-primary">Загрузить и сжать</button></form></section><section class="gallery-list-section"><div class="section-heading"><div><h2>Изображения</h2><p>` + template.HTMLEscapeString(directory) + `</p></div></div><div class="gallery-grid">` + galleryHTML.String() + `</div></section>`
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover"><title>Галерея</title><link rel="stylesheet" href="/static/css/style.css?v=10"></head><body>
{{SIDEBAR_HTML}}<main class="main-content gallery-page"><div class="page-header"><div><h1>Галерея</h1><p>Изображения для внешнего сайта из отдельной папки на сервере.</p></div></div>{{STATUS_BLOCK}}
<section class="card gallery-settings-panel"><div class="section-heading"><div><h2>Папка галереи</h2><p>Абсолютный путь к существующей папке на этом сервере.</p></div></div><form method="POST" action="/gallery/settings" class="gallery-path-form">{{CSRF_FIELD}}<input type="text" name="gallery_directory" value="{{GALLERY_DIRECTORY}}" placeholder="/var/www/site/gallery" required><button type="submit" class="btn btn-secondary">Сохранить путь</button></form></section>
{{GALLERY_SECTION}}</main></body></html>`
	page = strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "gallery"), 1)
	page = strings.Replace(page, "{{STATUS_BLOCK}}", statusBlock, 1)
	page = strings.Replace(page, "{{CSRF_FIELD}}", CSRFHiddenInput(c), 1)
	page = strings.Replace(page, "{{GALLERY_DIRECTORY}}", template.HTMLEscapeString(settings.GalleryDirectory), 1)
	page = strings.Replace(page, "{{GALLERY_SECTION}}", gallerySection, 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
}

func SaveGallerySettings(c *gin.Context) {
	directory := strings.TrimSpace(c.PostForm("gallery_directory"))
	if !filepath.IsAbs(directory) {
		c.Redirect(http.StatusFound, "/gallery?error="+url.QueryEscape("Путь должен быть абсолютным"))
		return
	}
	directory = filepath.Clean(directory)
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		c.Redirect(http.StatusFound, "/gallery?error="+url.QueryEscape("Указанная папка не существует или недоступна"))
		return
	}
	if _, err := os.ReadDir(directory); err != nil {
		c.Redirect(http.StatusFound, "/gallery?error="+url.QueryEscape("Нет доступа к чтению указанной папки"))
		return
	}
	if err := storage.UpdateGalleryDirectory(directory); err != nil {
		c.Redirect(http.StatusFound, "/gallery?error="+url.QueryEscape("Не удалось сохранить путь: "+err.Error()))
		return
	}
	security.LogEvent("gallery_path_updated", fmt.Sprintf("user=%s path=%s", c.GetString("userName"), directory))
	c.Redirect(http.StatusFound, "/gallery?ok=path")
}

func UploadGalleryImages(c *gin.Context) {
	directory, err := galleryDirectory()
	if err != nil {
		c.Redirect(http.StatusFound, "/gallery?error="+url.QueryEscape(err.Error()))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, galleryMaxRequestSize)
	if err := c.Request.ParseMultipartForm(8 << 20); err != nil {
		c.Redirect(http.StatusFound, "/gallery?error="+url.QueryEscape("Загрузка превышает 64 МБ или повреждена"))
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	files := c.Request.MultipartForm.File["images"]
	if len(files) == 0 {
		c.Redirect(http.StatusFound, "/gallery?error="+url.QueryEscape("Выберите хотя бы одно изображение"))
		return
	}
	if len(files) > galleryMaxFiles {
		c.Redirect(http.StatusFound, "/gallery?error="+url.QueryEscape("За один раз можно загрузить не более 12 изображений"))
		return
	}

	added := make([]string, 0, len(files))
	skipped := make([]string, 0)
	for _, header := range files {
		if err := validateGalleryUpload(header); err != nil {
			skipped = append(skipped, filepath.Base(header.Filename)+": "+err.Error())
			continue
		}
		file, err := header.Open()
		if err != nil {
			skipped = append(skipped, filepath.Base(header.Filename)+": не удалось прочитать файл")
			continue
		}
		name, saveErr := saveOptimizedGalleryImage(file, directory, header.Filename)
		_ = file.Close()
		if saveErr != nil {
			skipped = append(skipped, filepath.Base(header.Filename)+": "+saveErr.Error())
			continue
		}
		added = append(added, name)
	}
	if len(added) == 0 {
		c.Redirect(http.StatusFound, "/gallery?error="+url.QueryEscape(strings.Join(skipped, "; ")))
		return
	}
	security.LogEvent("gallery_images_uploaded", fmt.Sprintf("user=%s files=%s", c.GetString("userName"), strings.Join(added, ",")))
	notifyGalleryAdmins(c.GetString("userName"), "добавлены изображения", added)
	redirect := "/gallery?ok=uploaded&count=" + strconv.Itoa(len(added))
	if len(skipped) > 0 {
		redirect += "&skipped=" + url.QueryEscape(strings.Join(skipped, "; "))
	}
	c.Redirect(http.StatusFound, redirect)
}

func DeleteGalleryImage(c *gin.Context) {
	directory, err := galleryDirectory()
	if err != nil {
		c.Redirect(http.StatusFound, "/gallery?error="+url.QueryEscape(err.Error()))
		return
	}
	filename := strings.TrimSpace(c.PostForm("filename"))
	path, _, err := regularGalleryFile(directory, filename)
	if err != nil {
		c.Redirect(http.StatusFound, "/gallery?error="+url.QueryEscape("Изображение не найдено или недоступно"))
		return
	}
	if err := os.Remove(path); err != nil {
		c.Redirect(http.StatusFound, "/gallery?error="+url.QueryEscape("Не удалось удалить изображение: "+err.Error()))
		return
	}
	security.LogEvent("gallery_image_deleted", fmt.Sprintf("user=%s file=%s", c.GetString("userName"), filename))
	notifyGalleryAdmins(c.GetString("userName"), "удалено изображение", []string{filename})
	c.Redirect(http.StatusFound, "/gallery?ok=deleted")
}

func GalleryImage(c *gin.Context) {
	directory, err := galleryDirectory()
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	path, info, err := regularGalleryFile(directory, c.Param("name"))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "private, max-age=300")
	c.Header("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": info.Name()}))
	http.ServeFile(c.Writer, c.Request, path)
}
