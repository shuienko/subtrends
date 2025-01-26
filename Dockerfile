# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies and SQLite
RUN apk add --no-cache \
    gcc \
    musl-dev \
    sqlite-dev

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application with static linking
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o bot .

# Final stage
FROM alpine:3.19

# Install only SQLite runtime libraries
RUN apk add --no-cache sqlite-libs

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/bot .

# Command to run the bot
CMD ["./bot"]