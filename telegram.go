package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

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

// UserRequest represents a user request to the bot
type UserRequest struct {
	UserID    int64
	Username  string
	Text      string
	Timestamp time.Time
}

// ModelInfo represents information about an available model
type ModelInfo struct {
	Codename    string
	Name        string
	Description string
}

// Bot represents a Telegram bot with its API client and configuration
type Bot struct {
	api             *tgbotapi.BotAPI
	logger          *log.Logger
	config          *Config
	stopChan        chan struct{}
	wg              sync.WaitGroup
	historyFilePath string

	// History of user requests (unique subreddit names)
	historyMutex sync.RWMutex
	history      []string

	// Model selection
	modelMutex sync.RWMutex
	model      string
}

// Available models for selection
var availableModels = []ModelInfo{
	{
		Codename:    "simple",
		Name:        "claude-3-haiku-20240307",
		Description: "Fast and efficient model (default)",
	},
	{
		Codename:    "balanced",
		Name:        "claude-3-sonnet-20240229",
		Description: "Balanced performance and capabilities",
	},
	{
		Codename:    "advanced",
		Name:        "claude-3-opus-20240229",
		Description: "Most capable model for complex tasks",
	},
}

// NewBot creates a new Bot instance with the provided configuration
func NewBot(config *Config) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(config.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Telegram bot API client: %w", err)
	}

	api.Debug = config.Debug
	logger := log.New(os.Stdout, "TelegramBot: ", log.LstdFlags)

	bot := &Bot{
		api:             api,
		logger:          logger,
		config:          config,
		stopChan:        make(chan struct{}),
		history:         make([]string, 0, 50), // Initialize history with capacity for 50 items
		model:           config.AnthropicModel, // Initialize model from config
		historyFilePath: config.HistoryFilePath,
	}

	// Load history from file if it exists
	if err := bot.loadHistoryFromFile(); err != nil {
		logger.Printf("Failed to load history from file: %v. Starting with empty history.", err)
	}

	return bot, nil
}

// loadHistoryFromFile loads the subreddit history from a file
func (b *Bot) loadHistoryFromFile() error {
	// Check if file exists
	if _, err := os.Stat(b.historyFilePath); os.IsNotExist(err) {
		// File doesn't exist, which is fine for a new instance
		return nil
	}

	// Read the file
	data, err := os.ReadFile(b.historyFilePath)
	if err != nil {
		return fmt.Errorf("failed to read history file: %w", err)
	}

	// Split by lines and filter empty lines
	lines := strings.Split(string(data), "\n")
	var subreddits []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			subreddits = append(subreddits, line)
		}
	}

	// Update history
	b.historyMutex.Lock()
	defer b.historyMutex.Unlock()
	b.history = subreddits

	b.logger.Printf("Loaded %d subreddits from history file", len(subreddits))
	return nil
}

// saveHistoryToFile saves the subreddit history to a file
func (b *Bot) saveHistoryToFile() error {
	b.historyMutex.RLock()
	defer b.historyMutex.RUnlock()

	// Create the file content
	content := strings.Join(b.history, "\n")

	// Write to file
	err := os.WriteFile(b.historyFilePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	b.logger.Printf("Saved %d subreddits to history file", len(b.history))
	return nil
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

	// Save history to file before stopping
	if err := b.saveHistoryToFile(); err != nil {
		b.logger.Printf("Error saving history to file: %v", err)
	}

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

	// Save the request to history
	b.saveToHistory(message)

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
/history - Show your saved subreddit history
/clearhistory - Clear your saved subreddit history
/model - Show or change the current AI model

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

	case "history":
		return b.handleHistoryCommand(message)

	case "clearhistory":
		return b.handleClearHistoryCommand(message)

	case "model":
		return b.handleModelCommand(message)

	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, "Unknown command. Try /help to see available commands.")
		_, err := b.api.Send(msg)
		return err
	}
}

// handleHistoryCommand handles the /history command
func (b *Bot) handleHistoryCommand(message *tgbotapi.Message) error {
	b.historyMutex.RLock()
	defer b.historyMutex.RUnlock()

	if len(b.history) == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "📜 *Subreddit History*\n\nYou haven't visited any subreddits yet.")
		msg.ParseMode = "Markdown"
		_, err := b.api.Send(msg)
		return err
	}

	// Build the history message
	var historyText strings.Builder
	historyText.WriteString("📜 *Your Subreddit History*\n\n")

	// Display the subreddits in reverse order (assuming newest is at the end)
	for i := len(b.history) - 1; i >= 0; i-- {
		subreddit := b.history[i]
		historyText.WriteString(fmt.Sprintf("• `%s`\n", subreddit))
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, historyText.String())
	msg.ParseMode = "Markdown"
	_, err := b.api.Send(msg)
	return err
}

// handleClearHistoryCommand handles the /clearhistory command
func (b *Bot) handleClearHistoryCommand(message *tgbotapi.Message) error {
	b.historyMutex.Lock()

	// Clear the history
	b.history = make([]string, 0, 50)

	// Save the empty history to file
	err := b.saveHistoryToFile()

	b.historyMutex.Unlock()

	if err != nil {
		b.logger.Printf("Error saving empty history to file: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Error clearing history.")
		_, err := b.api.Send(msg)
		return err
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "✅ Subreddit history has been cleared.")
	_, err = b.api.Send(msg)
	return err
}

// handleModelCommand handles the /model command
func (b *Bot) handleModelCommand(message *tgbotapi.Message) error {
	args := message.CommandArguments()
	if args == "" {
		// Show current model
		b.modelMutex.RLock()
		currentModel := b.model
		b.modelMutex.RUnlock()

		var modelText strings.Builder
		modelText.WriteString("*Current AI Model*\n\n")

		// Find current model info
		var currentModelInfo ModelInfo
		for _, model := range availableModels {
			if model.Name == currentModel {
				currentModelInfo = model
				break
			}
		}
		modelText.WriteString(fmt.Sprintf("Currently using: `%s` (%s)\n", currentModelInfo.Codename, currentModelInfo.Description))
		modelText.WriteString("\n*Available Models:*\n")
		for _, model := range availableModels {
			modelText.WriteString(fmt.Sprintf("- `%s`: %s\n", model.Codename, model.Description))
		}
		modelText.WriteString("\nTo change the model, use:\n`/model <codename>`")

		msg := tgbotapi.NewMessage(message.Chat.ID, modelText.String())
		msg.ParseMode = "Markdown"
		_, err := b.api.Send(msg)
		return err
	}

	// Validate model codename
	var selectedModel ModelInfo
	validModel := false
	for _, model := range availableModels {
		if args == model.Codename {
			validModel = true
			selectedModel = model
			break
		}
	}

	if !validModel {
		var codenames strings.Builder
		codenames.WriteString("❌ Invalid model codename. Available models:\n")
		for _, model := range availableModels {
			codenames.WriteString(fmt.Sprintf("- `%s`: %s\n", model.Codename, model.Description))
		}
		msg := tgbotapi.NewMessage(message.Chat.ID, codenames.String())
		msg.ParseMode = "Markdown"
		_, err := b.api.Send(msg)
		return err
	}

	// Update model
	b.modelMutex.Lock()
	b.model = selectedModel.Name
	b.modelMutex.Unlock()

	// Update environment variable
	os.Setenv("ANTHROPIC_MODEL", selectedModel.Name)

	msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("✅ Model changed to: `%s` (%s)", selectedModel.Codename, selectedModel.Description))
	msg.ParseMode = "Markdown"
	_, err := b.api.Send(msg)
	return err
}

// saveToHistory saves a subreddit name to the history if it's not a command
func (b *Bot) saveToHistory(message *tgbotapi.Message) {
	// Skip commands
	if message.IsCommand() {
		return
	}

	// Clean the subreddit name (remove r/ prefix if present)
	subredditName := strings.TrimPrefix(message.Text, "r/")

	b.historyMutex.Lock()
	defer b.historyMutex.Unlock()

	// Check if this subreddit is already in history
	for _, existingSubreddit := range b.history {
		if strings.EqualFold(existingSubreddit, subredditName) {
			// Subreddit already in history, nothing to do
			return
		}
	}

	// Add new unique subreddit to history
	b.history = append(b.history, subredditName)

	// Save history to file after adding a new item
	go func() {
		if err := b.saveHistoryToFile(); err != nil {
			b.logger.Printf("Error saving history to file after adding new subreddit: %v", err)
		}
	}()
}
