# Use official Go image as builder
FROM golang:1.21-alpine AS builder

# Install git and build dependencies
RUN apk add --no-cache git gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o bot .

# Create final image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite

# Copy binary from builder
COPY --from=builder /app/bot /bot

# Create volume for SQLite database
VOLUME /data

# Set working directory
WORKDIR /

# Environment variables will be provided at runtime
ENV TELEGRAM_TOKEN=""
ENV ANTHROPIC_API_KEY=""
ENV REDDIT_CLIENT_ID=""
ENV REDDIT_CLIENT_SECRET=""
ENV REDDIT_USER_AGENT=""
ENV AUTHORIZED_USER_ID=""
ENV ANTHROPIC_MODEL=""

# Run the bot
CMD ["/bot"]