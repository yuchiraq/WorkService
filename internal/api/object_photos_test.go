package api

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestOptimizedObjectPhoto(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3000, 1500))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var source bytes.Buffer
	if err := png.Encode(&source, img); err != nil {
		t.Fatal(err)
	}
	full, thumb, w, h, err := optimizedObjectPhoto(&source)
	if err != nil || w != 2560 || h != 1280 {
		t.Fatalf("resize %dx%d: %v", w, h, err)
	}
	for _, data := range [][]byte{full, thumb} {
		decoded, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		r, g, b, _ := decoded.At(decoded.Bounds().Max.X-1, decoded.Bounds().Max.Y-1).RGBA()
		if r < 65000 || g < 65000 || b < 65000 {
			t.Fatal("transparent PNG not flattened on white")
		}
	}
	cfg, _, _ := image.DecodeConfig(bytes.NewReader(thumb))
	if cfg.Width != 480 || cfg.Height != 240 {
		t.Fatal("thumbnail dimensions", cfg)
	}
	if _, _, _, _, err = optimizedObjectPhoto(bytes.NewBufferString(`<svg onload="alert(1)"></svg>`)); err == nil {
		t.Fatal("SVG accepted")
	}
}

func TestPhotoEXIFOrientation(t *testing.T) {
	var source bytes.Buffer
	if err := jpeg.Encode(&source, image.NewRGBA(image.Rect(0, 0, 80, 40)), nil); err != nil {
		t.Fatal(err)
	}
	// JPEG APP1 with a little-endian TIFF Orientation=6 (90 degrees clockwise).
	exif := []byte{0xff, 0xe1, 0, 34, 'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	data := append(append(append([]byte{}, source.Bytes()[:2]...), exif...), source.Bytes()[2:]...)
	full, _, w, h, err := optimizedObjectPhoto(bytes.NewReader(data))
	if err != nil || w != 40 || h != 80 {
		t.Fatalf("orientation %dx%d: %v", w, h, err)
	}
	if bytes.Contains(full, []byte("Exif")) {
		t.Fatal("EXIF not removed")
	}
}
