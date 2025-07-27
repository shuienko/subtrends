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
ARG TARGETPLATFORM
RUN GOARCH=$(echo $TARGETPLATFORM | cut -d'/' -f2) GOARM=$(echo $TARGETPLATFORM | cut -d'/' -f3 | sed 's/v//') go build -o subtrends-bot .

# Final stage
FROM alpine:3.19

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/subtrends-bot .

# Create data directory for token cache
RUN mkdir -p data

# Command to run the Discord bot
CMD ["./subtrends-bot"]