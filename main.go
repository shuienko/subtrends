package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.Println("Starting SubTrends Discord Bot...")

	// Load configuration from environment variables
	LoadConfig()
	InitializeAnthropicRateLimiter()
	InitializeRedditRateLimiter()

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
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), AppConfig.ShutdownTimeout)
	defer shutdownCancel()

	// Stop the bot
	if err := bot.Stop(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Bot has been gracefully stopped")
}
