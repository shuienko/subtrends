# Build stage
FROM golang:1.23-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application (static, portable)
ARG TARGETPLATFORM
ENV CGO_ENABLED=0
RUN GOOS=linux GOARCH=$(echo $TARGETPLATFORM | cut -d'/' -f2) GOARM=$(echo $TARGETPLATFORM | cut -d'/' -f3 | sed 's/v//') go build -ldflags='-s -w' -o subtrends-bot .

# Final stage
FROM alpine:3.19

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user and group
RUN addgroup -S app && adduser -S app -G app

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/subtrends-bot .

# Create data directory for token/session cache; ensure ownership
RUN mkdir -p /app/data && chown -R app:app /app

# Switch to non-root user
USER app

# Expose a volume for persistence
VOLUME ["/app/data"]

# Add a basic healthcheck ensuring the bot process is running
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD sh -c 'grep -qa subtrends-bot /proc/1/cmdline || exit 1'

# Command to run the Discord bot
CMD ["./subtrends-bot"]