package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Custom error types
type EnvVarError struct {
	VarName string
	Err     error
}

func (e EnvVarError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("environment variable %s: %v", e.VarName, e.Err)
	}
	return fmt.Sprintf("environment variable %s is not set", e.VarName)
}

// ErrMissingEnvVar creates an error for a missing environment variable
func ErrMissingEnvVar(varName string) error {
	return EnvVarError{VarName: varName}
}

// ErrInvalidEnvVar creates an error for an invalid environment variable
func ErrInvalidEnvVar(varName string, err error) error {
	return EnvVarError{VarName: varName, Err: err}
}

// Bot represents a Telegram bot with its API client and configuration
type Bot struct {
	api      *tgbotapi.BotAPI
	logger   *log.Logger
	config   *Config
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewBot creates a new Bot instance with the provided configuration
func NewBot(config *Config) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(config.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Telegram bot API client: %w", err)
	}

	api.Debug = config.Debug
	logger := log.New(os.Stdout, "TelegramBot: ", log.LstdFlags)

	return &Bot{
		api:      api,
		logger:   logger,
		config:   config,
		stopChan: make(chan struct{}),
	}, nil
}

// Start begins the bot's update processing loop
func (b *Bot) Start(ctx context.Context) error {
	b.logger.Println("Bot starting...")

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := b.api.GetUpdatesChan(updateConfig)

	b.logger.Println("Bot started successfully")

	b.wg.Add(1)
	defer b.wg.Done()

	for {
		select {
		case update, ok := <-updates:
			if !ok {
				return fmt.Errorf("update channel closed")
			}

			if update.Message != nil {
				if err := b.handleMessage(update.Message); err != nil {
					b.logger.Printf("Error handling message: %v", err)
				}
			} else if update.CallbackQuery != nil {
				if err := b.handleCallback(update.CallbackQuery); err != nil {
					b.logger.Printf("Error handling callback: %v", err)
				}
			}
		case <-ctx.Done():
			b.logger.Println("Context canceled, stopping bot...")
			return ctx.Err()
		case <-b.stopChan:
			b.logger.Println("Stop signal received, stopping bot...")
			return nil
		}
	}
}

// Stop gracefully stops the bot
func (b *Bot) Stop(ctx context.Context) error {
	b.logger.Println("Stopping bot...")

	// Signal the bot to stop
	close(b.stopChan)

	// Wait for the bot to stop with a timeout
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		b.logger.Println("Bot stopped successfully")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for bot to stop: %w", ctx.Err())
	}
}

func (b *Bot) handleMessage(message *tgbotapi.Message) error {
	// Check if user is authorized
	if message.From.ID != b.config.AuthorizedUserID {
		reply := tgbotapi.NewMessage(message.Chat.ID, "Unauthorized user")
		_, err := b.api.Send(reply)
		return err
	}

	// Handle regular message
	token, err := getRedditAccessToken()
	if err != nil {
		b.logger.Printf("Failed to get access token: %v", err)
		reply := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Error: %v", err))
		_, _ = b.api.Send(reply)
		return err
	}

	data, err := subredditData(message.Text, token)
	if err != nil {
		reply := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Error: %v", err))
		_, _ = b.api.Send(reply)
		return err
	}

	summary, err := summarizePosts(data)
	if err != nil {
		reply := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Error: %v", err))
		_, _ = b.api.Send(reply)
		return err
	}

	reply := tgbotapi.NewMessage(message.Chat.ID, summary)
	_, err = b.api.Send(reply)
	return err
}

func (b *Bot) handleCallback(callback *tgbotapi.CallbackQuery) error {
	// Get Reddit data using the callback data (subreddit name)
	token, err := getRedditAccessToken()
	if err != nil {
		b.logger.Printf("Failed to get access token: %v", err)
		reply := tgbotapi.NewMessage(callback.Message.Chat.ID, fmt.Sprintf("Error: %v", err))
		_, _ = b.api.Send(reply)
		return err
	}

	data, err := subredditData(callback.Data, token)
	if err != nil {
		reply := tgbotapi.NewMessage(callback.Message.Chat.ID, fmt.Sprintf("Error: %v", err))
		_, _ = b.api.Send(reply)
		return err
	}

	summary, err := summarizePosts(data)
	if err != nil {
		reply := tgbotapi.NewMessage(callback.Message.Chat.ID, fmt.Sprintf("Error: %v", err))
		_, _ = b.api.Send(reply)
		return err
	}

	// Send the summary to the user
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, summary)
	_, err = b.api.Send(msg)

	// Answer the callback query to remove the loading indicator
	callback_response := tgbotapi.NewCallback(callback.ID, "")
	_, _ = b.api.Request(callback_response)

	return err
}
