package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// Config holds the application configuration
type Config struct {
	Port            string
	StaticFilesPath string
	SessionSecret   string
	TemplatePath    string
	ShutdownTimeout time.Duration
	HistoryFilePath string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Web server configuration
	port := getEnvOrDefault("PORT", "8080")
	staticFilesPath := getEnvOrDefault("STATIC_FILES_PATH", "./static")
	sessionSecret := getEnvOrDefault("SESSION_SECRET", "your-secret-key-change-in-production")
	templatePath := getEnvOrDefault("TEMPLATE_PATH", "./templates")

	// Shutdown timeout with default
	shutdownTimeoutStr := getEnvOrDefault("SHUTDOWN_TIMEOUT_SECONDS", "5")
	shutdownTimeout, err := strconv.Atoi(shutdownTimeoutStr)
	if err != nil {
		shutdownTimeout = 5 // Default to 5 seconds
	}

	// History file path with default
	historyFilePath := getEnvOrDefault("HISTORY_FILE_PATH", "data/subreddit_history.txt")

	return &Config{
		Port:            port,
		StaticFilesPath: staticFilesPath,
		SessionSecret:   sessionSecret,
		TemplatePath:    templatePath,
		ShutdownTimeout: time.Duration(shutdownTimeout) * time.Second,
		HistoryFilePath: historyFilePath,
	}, nil
}

// ensureDataDirectory creates the data directory if it doesn't exist
func ensureDataDirectory() error {
	// Get data directory path from token file path
	dataDir := filepath.Dir(tokenFilePath)

	// Check if directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		log.Printf("Creating data directory: %s", dataDir)
		// Create directory with permissions
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}
	}

	return nil
}

func main() {
	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Ensure data directory exists
	if err := ensureDataDirectory(); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Create web server instance
	server, err := NewWebServer(config)
	if err != nil {
		log.Fatalf("Failed to create web server: %v", err)
	}

	// Create a context that will be canceled on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		if err := server.Start(ctx); err != nil {
			log.Printf("Server stopped with error: %v", err)
			cancel()
		}
	}()

	// Wait for termination signal
	<-signalChan
	log.Println("Shutdown signal received, stopping server...")

	// Create a context with timeout for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer shutdownCancel()

	// Stop the server
	if err := server.Stop(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Server has been gracefully stopped")
}
