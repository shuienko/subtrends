# Use golang alpine as base image
FROM golang:1.23-alpine

# Install build dependencies and SQLite
RUN apk add --no-cache \
    gcc \
    musl-dev \
    sqlite \
    sqlite-dev \
    sqlite-libs

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o bot .

# Command to run the bot
CMD ["./bot"]