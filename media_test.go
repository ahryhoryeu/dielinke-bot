package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type telegramCall struct {
	method, text, reply, streaming string
	size                           int
}
type telegramMock struct {
	mu    sync.Mutex
	calls []telegramCall
}

func (m *telegramMock) snapshot() []telegramCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]telegramCall(nil), m.calls...)
}
func mockTelegram(t *testing.T, scenario string) (*tgbotapi.BotAPI, *telegramMock) {
	t.Helper()
	mock := &telegramMock{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := filepath.Base(r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if method == "getMe" {
			fmt.Fprint(w, `{"ok":true,"result":{"id":1,"is_bot":true,"username":"test"}}`)
			return
		}
		var size int
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
			if err := r.ParseMultipartForm(1024); err != nil {
				t.Errorf("multipart: %v", err)
				return
			}
			defer r.MultipartForm.RemoveAll()
			file, _, err := r.FormFile("video")
			if err != nil {
				t.Errorf("video: %v", err)
				return
			}
			defer file.Close()
			data, err := io.ReadAll(file)
			if err != nil {
				t.Errorf("file: %v", err)
			}
			size = len(data)
		} else {
			_ = r.ParseForm()
		}
		mock.mu.Lock()
		mock.calls = append(mock.calls, telegramCall{method, r.FormValue("text"), r.FormValue("reply_to_message_id"), r.FormValue("supports_streaming"), size})
		mock.mu.Unlock()
		if method == "sendVideo" && scenario == "upload-timeout" {
			time.Sleep(100 * time.Millisecond)
		}
		if (method == "sendVideo" && scenario == "upload-error") || (method == "sendMessage" && scenario == "fallback-error") {
			fmt.Fprint(w, `{"ok":false,"error_code":400,"description":"test failure"}`)
			return
		}
		fmt.Fprint(w, `{"ok":true,"result":{"message_id":2}}`)
	}))
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: time.Second}
	if scenario == "upload-timeout" {
		client.Timeout = 30 * time.Millisecond
	}
	bot, err := tgbotapi.NewBotAPIWithClient("test-token", server.URL+"/bot%s/%s", client)
	if err != nil {
		t.Fatal(err)
	}
	return bot, mock
}

func fakeMediaTools(t *testing.T, scenario string) (*mediaProcessor, string) {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	jobs := filepath.Join(root, "jobs")
	for _, dir := range []string{bin, jobs} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("MEDIA_SCENARIO", scenario)
	t.Setenv("MEDIA_ARGUMENTS", filepath.Join(root, "arguments"))
	script := `#!/bin/sh
set -eu
name=$(basename "$0")
printf '%s\n' "$name" "$@" >> "$MEDIA_ARGUMENTS"
case "$name" in
 yt-dlp)
  case "$MEDIA_SCENARIO" in
   download-error|fallback-error) exit 1;;
   download-timeout) sleep 30 & wait;;
  esac
  while [ "$#" -gt 0 ]; do
   if [ "$1" = "--print-to-file" ]; then shift; shift; manifest=$1; fi
   shift
  done
  printf 'source media' > source.mp4
  case "$MEDIA_SCENARIO" in
   missing-output) exit 0;;
   missing-file) rm source.mp4;;
   partial) printf '%s/source.mp4.part\n' "$PWD" > "$manifest"; exit 0;;
   escape) printf '/etc/passwd\n' > "$manifest"; exit 0;;
   multiple) printf '%s/source.mp4\n%s/source.webm\n' "$PWD" "$PWD" > "$manifest"; exit 0;;
   source-oversize) truncate -s 100000001 source.mp4;;
  esac
  printf '%s/source.mp4\n' "$PWD" > "$manifest"
  ;;
 ffprobe)
  case "$MEDIA_SCENARIO" in
   probe-error) exit 1;;
   probe-timeout) sleep 30 & wait;;
   bad-json) printf 'invalid'; exit 0;;
   no-video) printf '{"streams":[]}'; exit 0;;
   transcode) printf '{"streams":[{"codec_type":"video","codec_name":"vp9","pix_fmt":"yuv420p","width":1920,"height":1080},{"codec_type":"audio","codec_name":"opus"}]}'; exit 0;;
   silent) printf '{"streams":[{"codec_type":"video","codec_name":"h264","pix_fmt":"yuv420p","width":720,"height":1280}]}'; exit 0;;
  esac
  printf '{"streams":[{"codec_type":"video","codec_name":"h264","pix_fmt":"yuv420p","width":720,"height":1280},{"codec_type":"audio","codec_name":"aac"}]}'
  ;;
 ffmpeg)
  case "$MEDIA_SCENARIO" in
   conversion-error) exit 1;;
   conversion-timeout) sleep 30 & wait;;
  esac
  for output do :; done
  printf 'telegram video' > "$output"
  case "$MEDIA_SCENARIO" in
   output-oversize) truncate -s 45000001 "$output";;
   empty-output) truncate -s 0 "$output";;
  esac
  ;;
esac
`
	for _, tool := range []string{"yt-dlp", "ffprobe", "ffmpeg"} {
		if err := os.WriteFile(filepath.Join(bin, tool), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	p := newMediaProcessor(config{YTDLPPath: filepath.Join(bin, "yt-dlp"), InstagramReelsMode: "download"})
	p.tempRoot = jobs
	p.downloadTimeout = time.Second
	p.conversionTimeout = time.Second
	if scenario == "download-timeout" {
		p.downloadTimeout = 30 * time.Millisecond
	}
	if scenario == "conversion-timeout" || scenario == "probe-timeout" {
		p.conversionTimeout = 30 * time.Millisecond
	}
	return p, root
}

func reelMessage(id int) *tgbotapi.Message {
	return &tgbotapi.Message{MessageID: id, Chat: &tgbotapi.Chat{ID: 123}, Text: "https://instagram.com/reel/Abc_123/?igsh=secret"}
}

func TestMediaRepliesAndCleanup(t *testing.T) {
	for _, scenario := range []string{"success", "transcode", "silent", "download-error", "missing-output", "missing-file", "partial", "escape", "multiple", "source-oversize", "probe-error", "probe-timeout", "bad-json", "no-video", "conversion-error", "output-oversize", "empty-output", "download-timeout", "conversion-timeout", "upload-error", "upload-timeout", "fallback-error"} {
		t.Run(scenario, func(t *testing.T) {
			p, root := fakeMediaTools(t, scenario)
			bot, mock := mockTelegram(t, scenario)
			w := newMediaWorker(bot, p)
			handleMessage(bot, reelMessage(42), p.cfg, w)
			w.close()
			calls := mock.snapshot()
			var methods []string
			for _, call := range calls {
				methods = append(methods, call.method)
				if call.reply != "42" {
					t.Errorf("reply target: %+v", call)
				}
				if call.method == "sendVideo" && (call.size != len("telegram video") || call.streaming != "true") {
					t.Errorf("upload: %+v", call)
				}
				if call.method == "sendMessage" && !strings.HasSuffix(call.text, "https://kkclip.com/reel/Abc_123/") {
					t.Errorf("fallback: %+v", call)
				}
			}
			expected := []string{"sendMessage"}
			if scenario == "success" || scenario == "transcode" || scenario == "silent" {
				expected = []string{"sendVideo"}
			}
			if scenario == "upload-error" || scenario == "upload-timeout" {
				expected = []string{"sendVideo", "sendMessage"}
			}
			if !reflect.DeepEqual(methods, expected) {
				t.Errorf("calls %v, want %v", methods, expected)
			}
			entries, err := os.ReadDir(p.tempRoot)
			if err != nil || len(entries) != 0 {
				t.Errorf("temporary files remain: %v, %v", entries, err)
			}
			data, err := os.ReadFile(filepath.Join(root, "arguments"))
			if err != nil {
				t.Fatal(err)
			}
			args := string(data)
			for _, want := range []string{"--ignore-config\n", "--no-playlist\n", "--max-filesize\n100000000\n", "after_move:filepath\n", "https://www.instagram.com/reel/Abc_123/\n"} {
				if !strings.Contains(args, want) {
					t.Errorf("missing downloader option %q", want)
				}
			}
			if strings.Contains(args, "secret") || strings.Contains(args, "--cookies") {
				t.Error("unexpected tracking/auth arguments")
			}
			if scenario == "success" && !strings.Contains(args, "-c\ncopy\n") {
				t.Error("compatible video should remux")
			}
			if scenario == "transcode" {
				for _, want := range []string{"libx264\n", "-preset\nfast\n", "-threads\n2\n", "min(720,iw)", "-c:a\naac\n", "+faststart\n"} {
					if !strings.Contains(args, want) {
						t.Errorf("missing conversion option %q", want)
					}
				}
			}
		})
	}
}

func TestQueueSaturation(t *testing.T) {
	bot, mock := mockTelegram(t, "")
	p := newMediaProcessor(config{InstagramReelsMode: "download", YTDLPPath: "unused"})
	p.tempRoot = t.TempDir()
	started, release := make(chan struct{}, 11), make(chan struct{})
	p.run = func(context.Context, string, string, ...string) ([]byte, error) {
		started <- struct{}{}
		<-release
		return nil, errors.New("forced failure")
	}
	w := newMediaWorker(bot, p)
	handleMessage(bot, reelMessage(1), p.cfg, w)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	for i := 2; i <= 11; i++ {
		handleMessage(bot, reelMessage(i), p.cfg, w)
	}
	if len(w.jobs) != 10 {
		t.Fatalf("queued %d, want 10", len(w.jobs))
	}
	handleMessage(bot, reelMessage(12), p.cfg, w)
	calls := mock.snapshot()
	if len(calls) != 1 || calls[0].reply != "12" || calls[0].method != "sendMessage" {
		t.Errorf("saturated queue must immediately fall back: %+v", calls)
	}
	select {
	case <-started:
		t.Error("second media job started while first was active")
	default:
	}
	close(release)
	w.close()
	if len(mock.snapshot()) != 12 {
		t.Errorf("expected one fallback per job: %+v", mock.snapshot())
	}
	entries, _ := os.ReadDir(p.tempRoot)
	if len(entries) != 0 {
		t.Error("temporary files remain")
	}
}

func TestLinkMode(t *testing.T) {
	bot, mock := mockTelegram(t, "")
	handleMessage(bot, reelMessage(42), config{InstagramReelsMode: "link"}, nil)
	calls := mock.snapshot()
	if len(calls) != 1 || calls[0].method != "sendMessage" || !strings.Contains(calls[0].text, "https://kkclip.com/reel/Abc_123/") {
		t.Fatalf("link mode: %+v", calls)
	}
}

func TestCommandTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := runMediaCommand(ctx, t.TempDir(), "sh", "-c", "sleep 30 & wait")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("child process survived timeout")
	}
}

func TestRealFFmpegCompatibility(t *testing.T) {
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skip("FFmpeg tools not installed")
		}
	}
	for _, codec := range []string{"libx264", "mpeg4"} {
		t.Run(codec, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "source.mkv")
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			_, err := runMediaCommand(ctx, dir, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=1280x720:r=10", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100", "-t", "0.5", "-c:v", codec, "-threads", "2", "-pix_fmt", "yuv420p", "-c:a", "aac", input)
			if err != nil {
				t.Fatal(err)
			}
			p := newMediaProcessor(config{})
			output, err := p.prepare(ctx, input, dir)
			if err != nil {
				t.Fatal(err)
			}
			data, err := runMediaCommand(ctx, dir, "ffprobe", "-v", "error", "-show_streams", "-of", "json", output)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), `"codec_name": "h264"`) || !strings.Contains(string(data), `"codec_name": "aac"`) {
				t.Fatalf("unexpected streams: %s", data)
			}
			if codec == "mpeg4" && !strings.Contains(string(data), `"width": 720`) {
				t.Fatalf("not scaled: %s", data)
			}
			contents, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			moov, mdat := strings.Index(string(contents), "moov"), strings.Index(string(contents), "mdat")
			if moov < 0 || mdat < 0 || moov > mdat {
				t.Fatal("fast-start metadata missing")
			}
		})
	}
}

// Opt in explicitly: normal tests never contact Instagram or Telegram.
func TestInstagramSmoke(t *testing.T) {
	raw := os.Getenv("INSTAGRAM_SMOKE_URL")
	if raw == "" {
		t.Skip("set INSTAGRAM_SMOKE_URL to test a public Reel without cookies")
	}
	url, ok := FindInstagramReelURL(raw)
	if !ok {
		t.Fatal("INSTAGRAM_SMOKE_URL must be an Instagram Reel URL")
	}
	p := newMediaProcessor(loadConfig())
	dir := t.TempDir()
	input, err := p.download(context.Background(), url, dir)
	if err != nil {
		t.Fatal(err)
	}
	output, err := p.prepare(context.Background(), input, dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	data, err := runMediaCommand(ctx, dir, "ffprobe", "-v", "error", "-show_entries", "stream=codec_type,codec_name,width,height:format=size,duration", "-of", "json", output)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Prepared Telegram video: %s", data)
}
