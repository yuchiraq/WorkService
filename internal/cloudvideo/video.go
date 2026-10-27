// Package cloudvideo creates bounded, disposable browser previews. Originals
// stay untouched; HTTP handlers must authorize every prepare and stream request.
package cloudvideo

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const maxOutput = 256 << 20

type Result struct {
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
}
type job struct {
	path, state string
	at          time.Time
	seen        time.Time
}
type Engine struct {
	mu                sync.Mutex
	jobs              map[string]*job
	directory, binary string
	working           bool
	thumbnails        bool
	run               func(context.Context, string, string, string) error
}

func New(binary string) *Engine {
	return &Engine{binary: binary, jobs: map[string]*job{}, run: convert}
}

func (e *Engine) Prepare(key, source string) Result {
	e.mu.Lock()
	defer e.mu.Unlock()
	limit, outputLimit, extension, timeout, prefix := 4, int64(maxOutput), ".mp4", 10*time.Minute, "workservice-video-*"
	if e.thumbnails {
		limit, outputLimit, extension, timeout, prefix = 256, 1<<20, ".jpg", 30*time.Second, "workservice-thumbnails-*"
	}
	for k, j := range e.jobs {
		age := 6 * time.Hour
		if j.state == "error" {
			age = time.Minute
		}
		if j.state != "processing" && time.Since(j.at) > age {
			if err := os.Remove(j.path); err == nil || os.IsNotExist(err) {
				delete(e.jobs, k)
			}
		}
	}
	if j := e.jobs[key]; j != nil {
		j.seen = time.Now()
		if j.state == "ready" {
			if _, err := os.Stat(j.path); err != nil {
				delete(e.jobs, key)
			} else {
				return Result{State: "ready"}
			}
		} else {
			if j.state == "error" {
				return Result{State: "error", Message: "Не удалось подготовить видео. Возможно, файл повреждён или превышены лимиты обработки. Оригинал можно скачать"}
			}
			return Result{State: j.state}
		}
	}
	if e.working {
		return Result{State: "busy", Message: "Сервер обрабатывает другое видео"}
	}
	bin := e.binary
	if bin == "" {
		bin = "ffmpeg"
	}
	resolved, err := exec.LookPath(bin)
	if err != nil && !e.thumbnails {
		return Result{State: "unavailable", Message: "Для просмотра этого видео на сервере требуется FFmpeg. Оригинал можно скачать"}
	}
	info, err := os.Stat(source)
	if err != nil || info.Size() > 512<<20 {
		return Result{State: "error", Message: "Видео недоступно или больше 512 МБ. Скачайте оригинал"}
	}
	if len(e.jobs) >= limit {
		oldest := ""
		for k, j := range e.jobs {
			if oldest == "" || j.at.Before(e.jobs[oldest].at) {
				oldest = k
			}
		}
		if j := e.jobs[oldest]; j != nil {
			if err := os.Remove(j.path); err != nil && !os.IsNotExist(err) {
				return Result{State: "busy"}
			}
			delete(e.jobs, oldest)
		}
	}
	if e.directory == "" {
		e.directory, err = os.MkdirTemp("", prefix)
		if err != nil {
			return Result{State: "error", Message: "Недостаточно места для предпросмотра"}
		}
	}
	f, err := os.CreateTemp(e.directory, "*"+extension)
	if err != nil {
		return Result{State: "error", Message: "Не удалось создать предпросмотр"}
	}
	out := f.Name()
	f.Close()
	j := &job{path: out, state: "processing", at: time.Now(), seen: time.Now()}
	e.jobs[key] = j
	e.working = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		stop := make(chan struct{})
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					e.mu.Lock()
					abandoned := time.Since(j.seen) > time.Minute
					e.mu.Unlock()
					if abandoned {
						cancel()
						return
					}
					if info, err := os.Stat(out); err == nil && info.Size() > outputLimit {
						cancel()
						return
					}
				}
			}
		}()
		err := e.run(ctx, resolved, source, out)
		close(stop)
		info, statErr := os.Stat(out)
		e.mu.Lock()
		defer e.mu.Unlock()
		e.working = false
		j.at = time.Now()
		if err != nil || ctx.Err() != nil || statErr != nil || info.Size() == 0 || info.Size() > outputLimit {
			j.state = "error"
			os.Remove(out)
		} else {
			j.state = "ready"
		}
	}()
	return Result{State: "processing"}
}

func (e *Engine) Open(key string) (*os.File, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	j := e.jobs[key]
	if j == nil || j.state != "ready" {
		return nil, os.ErrNotExist
	}
	return os.Open(j.path)
}

func convert(ctx context.Context, bin, source, output string) error {
	// No shell, remote protocols, original names, or user-supplied FFmpeg flags.
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y", "-protocol_whitelist", "file,pipe", "-threads", "2", "-i", filepath.Clean(source), "-map", "0:v:0", "-map", "0:a:0?", "-map_metadata", "-1", "-map_chapters", "-1", "-vf", "scale=w='min(1280,iw)':h='min(720,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2,setsar=1", "-r", "30", "-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p", "-threads", "2", "-filter_threads", "1", "-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart", output}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.WaitDelay = 3 * time.Second
	if err := cmd.Run(); err != nil {
		return errors.New("video conversion failed")
	}
	return nil
}
