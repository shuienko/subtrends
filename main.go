package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// Config holds the application configuration
type Config struct {
	// Bot platform selection
	BotPlatform string // "telegram", "discord", or "both"

	// Telegram configuration
	TelegramToken    string
	AuthorizedUserID int64

	// Discord configuration
	DiscordToken            string
	DiscordAuthorizedRoleID string
	DiscordGuildID          string

	// Common configuration
	Debug           bool
	ShutdownTimeout time.Duration
	AnthropicModel  string
	HistoryFilePath string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Bot platform with default (telegram for backward compatibility)
	botPlatform := getEnvOrDefault("BOT_PLATFORM", "telegram")
	if botPlatform != "telegram" && botPlatform != "discord" && botPlatform != "both" {
		return nil, fmt.Errorf("invalid BOT_PLATFORM value: must be 'telegram', 'discord', or 'both'")
	}

	config := &Config{
		BotPlatform: botPlatform,
	}

	// Telegram configuration (required if telegram or both)
	if botPlatform == "telegram" || botPlatform == "both" {
		token := os.Getenv("TELEGRAM_TOKEN")
		if token == "" {
			return nil, ErrMissingEnvVar("TELEGRAM_TOKEN")
		}
		config.TelegramToken = token

		authorizedUserIDStr := os.Getenv("AUTHORIZED_USER_ID")
		if authorizedUserIDStr == "" {
			return nil, ErrMissingEnvVar("AUTHORIZED_USER_ID")
		}

		authorizedUserID, err := strconv.ParseInt(authorizedUserIDStr, 10, 64)
		if err != nil {
			return nil, ErrInvalidEnvVar("AUTHORIZED_USER_ID", err)
		}
		config.AuthorizedUserID = authorizedUserID
	}

	// Discord configuration (required if discord or both)
	if botPlatform == "discord" || botPlatform == "both" {
		discordToken := os.Getenv("DISCORD_TOKEN")
		if discordToken == "" {
			return nil, ErrMissingEnvVar("DISCORD_TOKEN")
		}
		config.DiscordToken = discordToken

		// Optional: Role-based authorization for Discord
		config.DiscordAuthorizedRoleID = os.Getenv("DISCORD_AUTHORIZED_ROLE_ID")

		// Optional: Guild ID restriction
		config.DiscordGuildID = os.Getenv("DISCORD_GUILD_ID")
	}

	// Optional debug mode
	debugStr := os.Getenv("DEBUG")
	config.Debug = debugStr == "true" || debugStr == "1"

	// Shutdown timeout with default
	shutdownTimeoutStr := getEnvOrDefault("SHUTDOWN_TIMEOUT_SECONDS", "5")
	shutdownTimeout, err := strconv.Atoi(shutdownTimeoutStr)
	if err != nil {
		shutdownTimeout = 5 // Default to 5 seconds
	}
	config.ShutdownTimeout = time.Duration(shutdownTimeout) * time.Second

	// Anthropic model with default
	config.AnthropicModel = getEnvOrDefault("ANTHROPIC_MODEL", "claude-3-haiku-20240307")

	// History file path with default
	config.HistoryFilePath = getEnvOrDefault("HISTORY_FILE_PATH", "data/subreddit_history.txt")

	return config, nil
}

// ensureDataDirectory creates the data directory if it doesn't exist
func ensureDataDirectory() error {
	// Use the tokenFilePath constant from reddit.go
	dataDir := "data"

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

	// Initialize bots based on configuration
	var telegramBot *Bot
	var discordBot *DiscordBot

	// Create a context that will be canceled on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Telegram bot if needed
	if config.BotPlatform == "telegram" || config.BotPlatform == "both" {
		telegramBot, err = NewBot(config)
		if err != nil {
			log.Fatalf("Failed to create Telegram bot: %v", err)
		}

		// Start Telegram bot in a goroutine
		go func() {
			if err := telegramBot.Start(ctx); err != nil {
				log.Printf("Telegram bot stopped with error: %v", err)
				cancel()
			}
		}()

		log.Println("Telegram bot initialized and started")
	}

	// Initialize Discord bot if needed
	if config.BotPlatform == "discord" || config.BotPlatform == "both" {
		discordBot, err = NewDiscordBot(config)
		if err != nil {
			log.Fatalf("Failed to create Discord bot: %v", err)
		}

		// Start Discord bot in a goroutine
		go func() {
			if err := discordBot.Start(ctx); err != nil {
				log.Printf("Discord bot stopped with error: %v", err)
				cancel()
			}
		}()

		log.Println("Discord bot initialized and started")
	}

	// Set up signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Wait for termination signal
	<-signalChan
	log.Println("Shutdown signal received, stopping bots...")

	// Create a context with timeout for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer shutdownCancel()

	// Stop the bots
	if telegramBot != nil {
		if err := telegramBot.Stop(shutdownCtx); err != nil {
			log.Printf("Error during Telegram bot shutdown: %v", err)
		}
	}

	if discordBot != nil {
		if err := discordBot.Stop(shutdownCtx); err != nil {
			log.Printf("Error during Discord bot shutdown: %v", err)
		}
	}

	log.Println("Bots have been gracefully stopped")
}
