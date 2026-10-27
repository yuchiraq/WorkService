package cloudvideo

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func waitReady(t *testing.T, e *Engine, key, source string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		r := e.Prepare(key, source)
		if r.State == "ready" {
			return
		}
		if r.State == "error" || r.State == "unavailable" {
			t.Fatal(r)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("preview timeout")
}

func TestCacheAndConcurrency(t *testing.T) {
	e := New(os.Args[0])
	e.directory = t.TempDir()
	source := filepath.Join(t.TempDir(), "source")
	os.WriteFile(source, []byte("original"), 0600)
	started, release := make(chan struct{}), make(chan struct{})
	e.run = func(_ context.Context, _ string, _ string, out string) error {
		close(started)
		<-release
		return os.WriteFile(out, []byte("preview"), 0600)
	}
	if r := e.Prepare("one", source); r.State != "processing" {
		t.Fatal(r)
	}
	<-started
	if r := e.Prepare("one", source); r.State != "processing" {
		t.Fatal(r)
	}
	if r := e.Prepare("two", source); r.State != "busy" {
		t.Fatal(r)
	}
	close(release)
	waitReady(t, e, "one", source)
	f, err := e.Open("one")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(f)
	f.Close()
	if string(b) != "preview" {
		t.Fatal(string(b))
	}
	if _, err = e.Open("unknown"); !os.IsNotExist(err) {
		t.Fatal("unknown key opened", err)
	}
	if b, _ = os.ReadFile(source); string(b) != "original" {
		t.Fatal("original changed")
	}
}

func TestMissingConverter(t *testing.T) {
	e := New(filepath.Join(t.TempDir(), "not-installed"))
	if r := e.Prepare("missing", "source"); r.State != "unavailable" {
		t.Fatal(r)
	}
}

func TestRealMOVConversion(t *testing.T) {
	bin := os.Getenv("WORKSERVICE_FFMPEG")
	if bin == "" {
		t.Skip("set WORKSERVICE_FFMPEG for codec smoke test")
	}
	source := filepath.Join(t.TempDir(), "sample.mov")
	cmd := exec.Command(bin, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=120x80:rate=5", "-t", "0.4", "-c:v", "prores_ks", "-threads", "1", source)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(b))
	}
	e := New(bin)
	e.directory = t.TempDir()
	waitReady(t, e, "real", source)
	f, err := e.Open("real")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b := make([]byte, 12)
	f.Read(b)
	if string(b[4:8]) != "ftyp" {
		t.Fatal("not MP4")
	}
	poster := NewThumbnails(bin)
	poster.directory = t.TempDir()
	waitReady(t, poster, "poster", source)
	p, err := poster.Open("poster")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	cfg, err := jpeg.DecodeConfig(p)
	if err != nil || cfg.Width > 480 || cfg.Height > 320 {
		t.Fatal("invalid video thumbnail", cfg, err)
	}
}

func TestImageThumbnailWithoutFFmpeg(t *testing.T) {
	var original bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 800, 600))
	if err := png.Encode(&original, img); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "image")
	os.WriteFile(source, original.Bytes(), 0600)
	e := NewThumbnails(filepath.Join(t.TempDir(), "ffmpeg-not-installed"))
	e.directory = t.TempDir()
	waitReady(t, e, "image", source)
	f, err := e.Open("image")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	decoded, err := jpeg.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() > 480 || decoded.Bounds().Dy() > 320 {
		t.Fatal("oversized thumbnail")
	}
	r, g, b, _ := decoded.At(0, 0).RGBA()
	if r < 65000 || g < 65000 || b < 65000 {
		t.Fatal("transparent image was not composed on white")
	}
	unchanged, _ := os.ReadFile(source)
	if !bytes.Equal(unchanged, original.Bytes()) {
		t.Fatal("original changed")
	}
}
