package api

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestGalleryFilePathRejectsTraversal(t *testing.T) {
	directory := t.TempDir()
	valid, err := galleryFilePath(directory, "photo.jpg")
	if err != nil {
		t.Fatalf("galleryFilePath() valid error = %v", err)
	}
	if valid != filepath.Join(directory, "photo.jpg") {
		t.Fatalf("galleryFilePath() = %q", valid)
	}

	for _, name := range []string{"../photo.jpg", `..\photo.jpg`, "folder/photo.jpg", "photo.gif", ""} {
		if _, err := galleryFilePath(directory, name); err == nil {
			t.Fatalf("galleryFilePath() accepted %q", name)
		}
	}
}

func TestSaveOptimizedGalleryImageResizesJPEG(t *testing.T) {
	directory := t.TempDir()
	source := image.NewNRGBA(image.Rect(0, 0, 3000, 1200))
	for y := 0; y < source.Bounds().Dy(); y++ {
		for x := 0; x < source.Bounds().Dx(); x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: uint8(x % 255), G: uint8(y % 255), B: 120, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, source, &jpeg.Options{Quality: 96}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}

	name, err := saveOptimizedGalleryImage(bytes.NewReader(encoded.Bytes()), directory, "Моя фотография.jpeg")
	if err != nil {
		t.Fatalf("saveOptimizedGalleryImage() error = %v", err)
	}
	if filepath.Ext(name) != ".jpg" {
		t.Fatalf("saved extension = %q", filepath.Ext(name))
	}
	file, err := os.Open(filepath.Join(directory, name))
	if err != nil {
		t.Fatalf("open saved image: %v", err)
	}
	defer file.Close()
	config, format, err := image.DecodeConfig(file)
	if err != nil {
		t.Fatalf("DecodeConfig() error = %v", err)
	}
	if format != "jpeg" || config.Width != galleryMaxDimension || config.Height != 1024 {
		t.Fatalf("saved image = %s %dx%d", format, config.Width, config.Height)
	}
}

func TestSanitizeGalleryBaseName(t *testing.T) {
	if got := sanitizeGalleryBaseName("../Летний объект 2026.jpg"); got != "Летний-объект-2026" {
		t.Fatalf("sanitizeGalleryBaseName() = %q", got)
	}
}
