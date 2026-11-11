# DieLinke Bot

A Telegram polling bot that replies to social media links. Public Instagram Reels can be downloaded and sent as Telegram videos, with a kkclip link when retrieval or upload fails. Twitter, X and Bluesky links use fxtwitter, fixupx and fxbsky respectively.

## Setup

1. Create a bot with [@BotFather](https://t.me/BotFather) and add it to your chat. For groups, configure its privacy setting so it can receive ordinary link messages.
2. Install Go, FFmpeg (including ffprobe), and yt-dlp if running download mode locally.
3. Configure and run:

   ```sh
   export TELEGRAM_BOT_TOKEN="your_bot_token_here"
   export INSTAGRAM_REELS_MODE="download"
   export YTDLP_PATH="yt-dlp"
   go mod download
   go run .
   ```

Only Instagram `/reel/<shortcode>/` URLs on `instagram.com` or `www.instagram.com` are supported. Tracking parameters are removed. Instagram posts and private/restricted content are outside the supported download scope.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `TELEGRAM_BOT_TOKEN` | required | Telegram bot token |
| `INSTAGRAM_REELS_MODE` | `link` | `download` sends videos; `link` replies with kkclip URLs |
| `YTDLP_PATH` | `yt-dlp` | Downloader executable path |

Link mode and all media failures use `https://kkclip.com/reel/<shortcode>/`. Setting `INSTAGRAM_REELS_MODE=link` is the rollback option. The deployment configuration enables `download`.

Downloads are anonymous: no Instagram cookies, paid download APIs or proxy subscriptions are required. Anonymous Instagram access and kkclip availability are not guaranteed. yt-dlp ignores ambient configuration files.

## Media processing and limits

- One media job runs at a time, with ten waiting jobs. A full queue triggers an immediate link reply attempt.
- Downloading has a two-minute deadline and a 100 MB source-file limit. Working files are monitored against a 250 MB disk budget (checked every 200 ms, so brief overshoot is possible).
- H.264/AAC MP4 up to 1080p is preferred, with other available formats as a fallback. Only the explicitly reported completed download is processed.
- ffprobe inspects the video. Compatible H.264/yuv420p and AAC streams are remuxed to MP4 with fast-start metadata. Other formats are converted to H.264/AAC with FFmpeg's fast preset and two threads; converted dimensions fit inside 720 × 720 without upscaling. Silent sources remain silent.
- Inspection and conversion share a three-minute deadline. Outputs over 45 MB are rejected. Each Telegram request has a two-minute timeout.
- Successful uploads reply to the original message and enable streaming. Download, inspection, conversion, size and upload failures each lead to one kkclip reply attempt. A failed fallback is logged without retrying.
- Temporary job files are removed after success or failure. Stage failures are logged; verbose Telegram payload logging is disabled. Persistent caching and webhooks are deferred.

## Docker and downloader updates

The runtime keeps yt-dlp in `/opt/yt-dlp`, an isolated Python environment, pinned to `2026.10.26`. FFmpeg and ffprobe come from Alpine. The build checks the exact downloader version and both media tools.

```sh
docker build -t dielinke-bot .
docker run --rm dielinke-bot yt-dlp --version
docker run --rm dielinke-bot ffmpeg -version
docker run --rm dielinke-bot ffprobe -version
docker run --rm -p 8080:8080 \
  -e TELEGRAM_BOT_TOKEN -e INSTAGRAM_REELS_MODE=download dielinke-bot
```

To update the pin, change `ARG YTDLP_VERSION` in `Dockerfile`, rebuild, test public Reels, then deploy the new image. To try a different release before changing the default:

```sh
docker build --build-arg YTDLP_VERSION=2026.10.26 -t dielinke-bot .
```

See [yt-dlp's options and output-path documentation](https://github.com/yt-dlp/yt-dlp#output-template) for the completed-output reporting used by this bot.

## Existing Render deployment

Keep the existing paid Render service and Telegram polling. Build/deploy the updated repository on that service and set `INSTAGRAM_REELS_MODE=download` in its environment. Do not run another polling instance with the same token during validation.

**The checked-in `render.yaml` still contains the historical `plan: free` value. Do not apply that tier to the live paid service or use a Blueprint sync to change its hosting plan.** This change does not alter the actual service settings. Use the service's existing deployment workflow and retain its paid tier. HTTP health checks remain at `/health` on port 8080.

## Validation

```sh
go test ./...
CGO_ENABLED=1 go test -race ./...
go vet ./...
```

Tests use fake media subprocesses and a mock Telegram endpoint to cover canonical URLs, lookalike hosts, successful uploads, remux/conversion selection, missing output, failures at each stage, size limits, timeouts, queue saturation, fallback failure, link mode and temporary-file cleanup. If FFmpeg is installed, real local fixtures also verify H.264/AAC output and fast-start metadata.

To exercise the download/conversion pipeline locally without starting Telegram polling:

```sh
INSTAGRAM_SMOKE_URL='https://www.instagram.com/reel/DfjQSqxRrQG/' \
  go test -run TestInstagramSmoke -v
```

To use the exact Docker runtime, compile the test binary for Linux and stream it into a disposable container:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -o /tmp/dielinke-smoke.test
docker run --rm -i \
  -e INSTAGRAM_SMOKE_URL='https://www.instagram.com/reel/DfjQSqxRrQG/' \
  dielinke-bot sh -c 'cat > /tmp/smoke.test && chmod +x /tmp/smoke.test && /tmp/smoke.test -test.run TestInstagramSmoke -test.v' \
  < /tmp/dielinke-smoke.test
```

Use `GOARCH=arm64` when the image runs on ARM64. The smoke test reports codecs, dimensions, duration and file size, then removes its temporary files.

Before accepting a live deployment:

1. Download a known public Reel inside the built Docker image without authentication, using the pipeline's options. Inspect the result with ffprobe for video/audio codecs and size.
2. Send several public Reel links to the existing bot on Render. Confirm that the replies play with audio in Telegram (for sources with audio).
3. Send concurrent messages to verify serialized media processing and queue fallback behavior.
4. Force a failure with an unavailable Reel; confirm exactly one kkclip reply to the original message. Check that temporary job directories are removed.
5. If needed, set `INSTAGRAM_REELS_MODE=link` to roll back while keeping polling and the paid service unchanged.

These live checks require a Reel URL, the existing Render service access and an authorized Telegram test chat. Automated mocks and codec inspection do not establish actual Telegram client playback or Instagram reachability from Render.

## Dependencies

- [go-telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api)
- [yt-dlp](https://github.com/yt-dlp/yt-dlp)
- [FFmpeg](https://ffmpeg.org/)

## License

MIT
