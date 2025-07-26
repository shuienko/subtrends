package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

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
	log.Println("Starting SubTrends Discord Bot...")

	// Ensure data directory exists
	if err := ensureDataDirectory(); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Create Discord bot instance
	bot, err := NewDiscordBot()
	if err != nil {
		log.Fatalf("Failed to create Discord bot: %v", err)
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

	log.Println("SubTrends Discord Bot is now running. Press CTRL-C to exit.")

	// Wait for termination signal
	<-signalChan
	log.Println("Shutdown signal received, stopping bot...")

	// Create a context with timeout for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Stop the bot
	if err := bot.Stop(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Bot has been gracefully stopped")
}
