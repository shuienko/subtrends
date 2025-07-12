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
RUN go build -o web .

# Final stage
FROM alpine:3.19

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/web .

# Create directories for templates and static files
RUN mkdir -p templates static/css static/js

# Copy templates and static files
COPY templates/ templates/
COPY static/ static/

# Expose port
EXPOSE 8080

# Command to run the web server
CMD ["./web"]