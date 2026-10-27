package cloudvideo

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"time"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"
)

// Thumbnail jobs have their own short, bounded queue and cache, so opening a
// grid never requests a full video conversion or blocks video playback jobs.
func NewThumbnails(binary string) *Engine {
	e := New(binary)
	e.thumbnails = true
	e.run = thumbnail
	return e
}

func thumbnail(ctx context.Context, bin, source, output string) error {
	f, err := os.Open(source)
	if err != nil {
		return err
	}
	defer f.Close()
	cfg, _, decodeErr := image.DecodeConfig(f)
	if decodeErr == nil {
		if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 32_000_000/cfg.Height {
			return errors.New("thumbnail image too large")
		}
		if _, err = f.Seek(0, 0); err != nil {
			return err
		}
		img, err := imaging.Decode(f, imaging.AutoOrientation(true))
		if err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		fitted := imaging.Fit(img, 480, 320, imaging.Lanczos)
		canvas := image.NewRGBA(fitted.Bounds())
		draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		draw.Draw(canvas, canvas.Bounds(), fitted, image.Point{}, draw.Over)
		out, err := os.Create(output)
		if err != nil {
			return err
		}
		defer out.Close()
		return jpeg.Encode(out, canvas, &jpeg.Options{Quality: 82})
	}
	if bin == "" {
		return errors.New("ffmpeg not installed")
	}
	cmd := exec.CommandContext(ctx, bin, "-hide_banner", "-loglevel", "error", "-nostdin", "-y", "-protocol_whitelist", "file,pipe", "-threads", "1", "-i", source, "-map", "0:v:0", "-frames:v", "1", "-an", "-map_metadata", "-1", "-vf", "scale=w='min(480,iw)':h='min(320,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2", "-filter_threads", "1", "-threads", "1", "-q:v", "3", output)
	cmd.WaitDelay = 3 * time.Second
	return cmd.Run()
}
