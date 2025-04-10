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
FROM scratch

# Copy the binary from builder
COPY --from=builder /app/bot /bot

# Run the bot
CMD ["/bot"]