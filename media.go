package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const maxVideoBytes int64 = 45_000_000
const maxDownloadBytes int64 = 100_000_000
const maxWorkBytes int64 = 250_000_000

type mediaProcessor struct {
	cfg                                config
	tempRoot                           string
	downloadTimeout, conversionTimeout time.Duration
	run                                func(context.Context, string, string, ...string) ([]byte, error)
}

func newMediaProcessor(cfg config) *mediaProcessor {
	return &mediaProcessor{cfg: cfg, downloadTimeout: 2 * time.Minute, conversionTimeout: 3 * time.Minute, run: runMediaCommand}
}

// Keep command diagnostics bounded, including when an external tool misbehaves.
type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if remaining := 64*1024 - b.Len(); remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.Buffer.Write(p)
	}
	return n, nil
}

func runMediaCommand(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	// yt-dlp can launch FFmpeg; kill the entire process group on timeout.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	var output limitedBuffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	var limitErr error
	for {
		select {
		case err := <-done:
			if ctx.Err() != nil {
				return output.Bytes(), ctx.Err()
			}
			if limitErr != nil {
				return output.Bytes(), limitErr
			}
			if err != nil {
				return output.Bytes(), fmt.Errorf("%w: %s", err, strings.TrimSpace(output.String()))
			}
			return output.Bytes(), nil
		case <-ticker.C:
			// --max-filesize cannot always bound fragmented/unknown-size sources.
			// Also bound working disk usage while downloading, merging and converting.
			var size int64
			err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.Type().IsRegular() {
					info, err := d.Info()
					if err != nil {
						return err
					}
					size += info.Size()
				}
				return nil
			})
			if err == nil && size > maxWorkBytes && limitErr == nil {
				limitErr = errors.New("working files exceed disk budget")
				_ = cmd.Cancel()
			}
		}
	}
}

func (p *mediaProcessor) download(ctx context.Context, url, dir string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, p.downloadTimeout)
	defer cancel()
	manifest := filepath.Join(dir, "completed-path.txt")
	args := []string{
		"--ignore-config", "--no-playlist", "--playlist-items", "1",
		"--no-progress", "--no-simulate", "--no-cache-dir",
		"--max-filesize", strconv.FormatInt(maxDownloadBytes, 10),
		"--socket-timeout", "20", "--retries", "1", "--fragment-retries", "1",
		"--abort-on-unavailable-fragments", "--match-filters", "!is_live",
		"--merge-output-format", "mp4",
		"-f", "bv[ext=mp4][vcodec^=avc1][height<=1080]+ba[ext=m4a][acodec^=mp4a]/b[ext=mp4][vcodec^=avc1][acodec^=mp4a][height<=1080]/bv*+ba/b",
		"-o", filepath.Join(dir, "source.%(ext)s"),
		"--print-to-file", "after_move:filepath", manifest,
		"--", url,
	}
	if _, err := p.run(ctx, dir, p.cfg.YTDLPPath, args...); err != nil {
		return "", err
	}
	data, err := os.ReadFile(manifest)
	if err != nil {
		return "", fmt.Errorf("completed output: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 || lines[0] == "" {
		return "", errors.New("expected exactly one completed output path")
	}
	path := filepath.Clean(lines[0])
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	// Do not accept a partial file, symlink, or a path outside this job.
	if filepath.Dir(path) != dir || !strings.HasPrefix(filepath.Base(path), "source.") || strings.HasSuffix(path, ".part") {
		return "", errors.New("invalid completed output path")
	}
	if err := checkMediaSize(path, maxDownloadBytes); err != nil {
		return "", err
	}
	return path, nil
}

func checkMediaSize(path string, limit int64) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return errors.New("missing or invalid media file")
	}
	if info.Size() > limit {
		return fmt.Errorf("media exceeds %d byte size limit", limit)
	}
	return nil
}

type mediaStream struct {
	CodecType   string `json:"codec_type"`
	CodecName   string `json:"codec_name"`
	PixelFormat string `json:"pix_fmt"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

func (p *mediaProcessor) prepare(ctx context.Context, input, dir string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, p.conversionTimeout)
	defer cancel()
	data, err := p.run(ctx, dir, "ffprobe", "-v", "error", "-show_streams", "-of", "json", input)
	if err != nil {
		return "", fmt.Errorf("probe: %w", err)
	}
	var info struct {
		Streams []mediaStream `json:"streams"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return "", fmt.Errorf("probe: %w", err)
	}
	var video, audio *mediaStream
	for i := range info.Streams {
		s := &info.Streams[i]
		if s.CodecType == "video" && video == nil {
			video = s
		}
		if s.CodecType == "audio" && audio == nil {
			audio = s
		}
	}
	if video == nil || video.Width <= 0 || video.Height <= 0 {
		return "", errors.New("probe: no valid video stream")
	}
	compatible := video.CodecName == "h264" && video.PixelFormat == "yuv420p" && (audio == nil || audio.CodecName == "aac")
	output := filepath.Join(dir, "telegram.mp4")
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y", "-threads", "2", "-i", input, "-map", "0:v:0", "-map", "0:a:0?", "-map_metadata", "-1", "-map_chapters", "-1"}
	if compatible {
		args = append(args, "-c", "copy")
	} else {
		args = append(args, "-c:v", "libx264", "-preset", "fast", "-crf", "23", "-threads", "2", "-filter_threads", "2",
			"-vf", "scale=w='min(720,iw)':h='min(720,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2,setsar=1",
			"-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", "128k", "-ac", "2")
	}
	// Stop FFmpeg once the size limit is crossed, then reject the partial result.
	args = append(args, "-movflags", "+faststart", "-fs", strconv.FormatInt(maxVideoBytes+1, 10), output)
	if _, err := p.run(ctx, dir, "ffmpeg", args...); err != nil {
		return "", fmt.Errorf("conversion: %w", err)
	}
	if err := checkMediaSize(output, maxVideoBytes); err != nil {
		return "", fmt.Errorf("size: %w", err)
	}
	return output, nil
}

func (p *mediaProcessor) send(ctx context.Context, bot *tgbotapi.BotAPI, message *tgbotapi.Message, url string) error {
	dir, err := os.MkdirTemp(p.tempRoot, "dielinke-instagram-*")
	if err != nil {
		return fmt.Errorf("temp: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("media stage=cleanup: %v", err)
		}
	}()
	input, err := p.download(ctx, url, dir)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	path, err := p.prepare(ctx, input, dir)
	if err != nil {
		return err
	}
	// Own the handle so it is closed even if the Telegram library's multipart
	// writer exits early after a network failure.
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("upload: %w", err)
	}
	defer file.Close()
	video := tgbotapi.NewVideo(message.Chat.ID, tgbotapi.FileReader{Name: "reel.mp4", Reader: file})
	video.ReplyToMessageID = message.MessageID
	video.SupportsStreaming = true
	video.Caption = "Instagram reel"
	if _, err := bot.Send(video); err != nil {
		return fmt.Errorf("upload: %w", err)
	}
	return nil
}
