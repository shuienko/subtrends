package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// Config holds the application configuration
type Config struct {
	TelegramToken    string
	AuthorizedUserID int64
	Debug            bool
	ShutdownTimeout  time.Duration
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		return nil, ErrMissingEnvVar("TELEGRAM_TOKEN")
	}

	authorizedUserIDStr := os.Getenv("AUTHORIZED_USER_ID")
	if authorizedUserIDStr == "" {
		return nil, ErrMissingEnvVar("AUTHORIZED_USER_ID")
	}

	authorizedUserID, err := strconv.ParseInt(authorizedUserIDStr, 10, 64)
	if err != nil {
		return nil, ErrInvalidEnvVar("AUTHORIZED_USER_ID", err)
	}

	// Optional debug mode
	debugStr := os.Getenv("DEBUG")
	debug := debugStr == "true" || debugStr == "1"

	// Shutdown timeout with default
	shutdownTimeoutStr := getEnvOrDefault("SHUTDOWN_TIMEOUT_SECONDS", "5")
	shutdownTimeout, err := strconv.Atoi(shutdownTimeoutStr)
	if err != nil {
		shutdownTimeout = 5 // Default to 5 seconds
	}

	return &Config{
		TelegramToken:    token,
		AuthorizedUserID: authorizedUserID,
		Debug:            debug,
		ShutdownTimeout:  time.Duration(shutdownTimeout) * time.Second,
	}, nil
}

func main() {
	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create bot instance
	bot, err := NewBot(config)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	// Create a context that will be canceled on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Start bot in a goroutine
	go func() {
		if err := bot.Start(ctx); err != nil {
			log.Printf("Bot stopped with error: %v", err)
			cancel()
		}
	}()

	// Wait for termination signal
	<-signalChan
	log.Println("Shutdown signal received, stopping bot...")

	// Create a context with timeout for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer shutdownCancel()

	// Stop the bot
	if err := bot.Stop(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Bot has been gracefully stopped")
}
