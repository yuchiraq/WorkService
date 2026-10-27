package cloudvideo

import (
	"context"
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
}
