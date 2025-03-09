package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
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
		reply := tgbotapi.NewMessage(message.Chat.ID, "⛔ Sorry, you're not authorized to use this bot.")
		_, err := b.api.Send(reply)
		return err
	}

	// Handle commands
	if message.IsCommand() {
		return b.handleCommand(message)
	}

	// Handle regular message (subreddit name)
	subredditName := message.Text

	// Send typing action to show the bot is processing
	typingAction := tgbotapi.NewChatAction(message.Chat.ID, tgbotapi.ChatTyping)
	_, _ = b.api.Send(typingAction)

	// Send initial processing message
	processingMsg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("🔍 Analyzing r/%s...\nThis might take a moment to fetch and process the data.", strings.TrimPrefix(subredditName, "r/")))
	sentMsg, _ := b.api.Send(processingMsg)

	// Get Reddit data
	token, err := getRedditAccessToken()
	if err != nil {
		b.logger.Printf("Failed to get access token: %v", err)
		errorMsg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("❌ Error: Failed to connect to Reddit. Please try again later.\n\nTechnical details: %v", err))
		_, _ = b.api.Send(errorMsg)
		return err
	}

	// Update processing message
	editMsg := tgbotapi.NewEditMessageText(message.Chat.ID, sentMsg.MessageID, fmt.Sprintf("🔍 Connected to Reddit! Fetching posts from r/%s...", strings.TrimPrefix(subredditName, "r/")))
	_, _ = b.api.Send(editMsg)

	data, err := subredditData(subredditName, token)
	if err != nil {
		errorMsg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
		_, _ = b.api.Send(errorMsg)
		return err
	}

	// Update processing message
	editMsg = tgbotapi.NewEditMessageText(message.Chat.ID, sentMsg.MessageID, "🧠 Analyzing Reddit posts and generating summary...")
	_, _ = b.api.Send(editMsg)

	summary, err := summarizePosts(data)
	if err != nil {
		errorMsg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("❌ Error: Failed to generate summary.\n\nTechnical details: %v", err))
		_, _ = b.api.Send(errorMsg)
		return err
	}

	// Delete the processing message
	deleteMsg := tgbotapi.NewDeleteMessage(message.Chat.ID, sentMsg.MessageID)
	_, _ = b.api.Send(deleteMsg)

	// Fetch posts again to get the links
	posts, err := fetchTopPosts(subredditName, token)
	if err != nil {
		b.logger.Printf("Failed to fetch posts for links: %v", err)
	} else {
		// Append links to the summary
		summary += "\n\n🔗 Top Posts\n"
		// Define emoji numbers for better visual appeal
		emojiNumbers := []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"}
		for i, post := range posts {
			if i >= defaultPostLimit {
				break
			}
			// Use standard web URL instead of reddit:// protocol
			webLink := fmt.Sprintf("https://www.reddit.com%s", post.Permalink)
			// Use emoji instead of number
			emojiIndex := i
			if emojiIndex >= len(emojiNumbers) {
				emojiIndex = len(emojiNumbers) - 1
			}
			summary += fmt.Sprintf("%s [%s](%s)\n", emojiNumbers[emojiIndex], post.Title, webLink)
		}
	}

	// Send the summary
	reply := tgbotapi.NewMessage(message.Chat.ID, summary)
	reply.ParseMode = "Markdown"
	_, err = b.api.Send(reply)

	return err
}

// handleCommand processes bot commands
func (b *Bot) handleCommand(message *tgbotapi.Message) error {
	switch message.Command() {
	case "start":
		welcomeText := `👋 *Welcome to SubTrends Bot!*

I help you stay updated on what's trending in your favorite subreddits.

*How to use me:*
Simply send me a subreddit name (with or without "r/") and I'll analyze the top posts and comments to give you a concise summary of what's happening there.

For example, try sending: 
- r/technology
- science
- askreddit

Let's get started!`

		msg := tgbotapi.NewMessage(message.Chat.ID, welcomeText)
		msg.ParseMode = "Markdown"
		_, err := b.api.Send(msg)
		return err

	case "help":
		helpText := `*SubTrends Bot Help*

*Basic Commands:*
/start - Start the bot and see welcome message
/help - Show this help message

*How to use:*
Just send any subreddit name (with or without "r/") to get a summary of what's trending there.

*Examples:*
- r/worldnews
- datascience
- askhistorians

The bot will analyze the top posts and comments from the past day and provide you with a concise, organized summary.`

		msg := tgbotapi.NewMessage(message.Chat.ID, helpText)
		msg.ParseMode = "Markdown"
		_, err := b.api.Send(msg)
		return err

	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, "Unknown command. Try /help to see available commands.")
		_, err := b.api.Send(msg)
		return err
	}
}
