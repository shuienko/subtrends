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

# Build the application
RUN go build -o bot .

# Final stage
FROM alpine:3.19

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/bot .

# Command to run the bot
CMD ["./bot"]