# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o bot

# Final stage
FROM alpine:3.21

# Install runtime dependencies
ARG YTDLP_VERSION=2026.10.26
ENV PATH="/opt/yt-dlp/bin:${PATH}"
RUN apk --no-cache add ca-certificates ffmpeg python3 py3-pip \
    && python3 -m venv /opt/yt-dlp \
    && /opt/yt-dlp/bin/pip install --no-cache-dir "yt-dlp==${YTDLP_VERSION}" \
    && test "$(yt-dlp --version)" = "${YTDLP_VERSION}" \
    && ffmpeg -version && ffprobe -version

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/bot .

# Run the bot
CMD ["./bot"]
