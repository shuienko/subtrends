package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// DiscordBot represents a Discord bot with configuration
type DiscordBot struct {
	session         *discordgo.Session
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

	// Discord specific configurations
	authorizedRoleID string
	guildID          string
}

// NewDiscordBot creates a new DiscordBot instance with the provided configuration
func NewDiscordBot(config *Config) (*DiscordBot, error) {
	// Create Discord session with bot token
	// Note: Discord tokens need "Bot " prefix unlike Telegram
	session, err := discordgo.New("Bot " + config.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	// Configure session
	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages

	logger := log.New(os.Stdout, "DiscordBot: ", log.LstdFlags)

	bot := &DiscordBot{
		session:          session,
		logger:           logger,
		config:           config,
		stopChan:         make(chan struct{}),
		history:          make([]string, 0, 50),
		model:            config.AnthropicModel,
		historyFilePath:  config.HistoryFilePath,
		authorizedRoleID: config.DiscordAuthorizedRoleID,
		guildID:          config.DiscordGuildID,
	}

	// Load history from file if it exists (reusing the same method as Telegram bot)
	if err := bot.loadHistoryFromFile(); err != nil {
		logger.Printf("Failed to load history from file: %v. Starting with empty history.", err)
	}

	return bot, nil
}

// loadHistoryFromFile loads the subreddit history from a file
func (b *DiscordBot) loadHistoryFromFile() error {
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
func (b *DiscordBot) saveHistoryToFile() error {
	b.historyMutex.RLock()
	defer b.historyMutex.RUnlock()

	// Create the file content
	content := strings.Join(b.history, "\n")

	// Ensure the directory exists
	dir := filepath.Dir(b.historyFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for history file: %w", err)
	}

	// Write to file
	err := os.WriteFile(b.historyFilePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	b.logger.Printf("Saved %d subreddits to history file", len(b.history))
	return nil
}

// Start begins the bot's update processing loop
func (b *DiscordBot) Start(ctx context.Context) error {
	b.logger.Println("Discord bot starting...")

	// Register the messageCreate handler
	b.session.AddHandler(b.messageHandler)

	// Open a websocket connection to Discord
	err := b.session.Open()
	if err != nil {
		return fmt.Errorf("error opening connection to Discord: %w", err)
	}

	b.logger.Println("Discord bot started successfully")

	b.wg.Add(1)
	defer b.wg.Done()

	// Keep running until context is cancelled or stop signal received
	select {
	case <-ctx.Done():
		b.logger.Println("Context canceled, stopping bot...")
		return ctx.Err()
	case <-b.stopChan:
		b.logger.Println("Stop signal received, stopping bot...")
		return nil
	}
}

// Stop gracefully stops the bot
func (b *DiscordBot) Stop(ctx context.Context) error {
	b.logger.Println("Stopping Discord bot...")

	// Save history to file before stopping
	if err := b.saveHistoryToFile(); err != nil {
		b.logger.Printf("Error saving history to file: %v", err)
	}

	// Signal the bot to stop
	close(b.stopChan)

	// Close Discord session
	err := b.session.Close()
	if err != nil {
		b.logger.Printf("Error closing Discord session: %v", err)
	}

	// Wait for the bot to stop with a timeout
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		b.logger.Println("Discord bot stopped successfully")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for bot to stop: %w", ctx.Err())
	}
}

// messageHandler handles incoming Discord messages
func (b *DiscordBot) messageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Check if user is authorized (if in a guild)
	if m.GuildID != "" {
		isAuthorized, err := b.isUserAuthorized(s, m.GuildID, m.Author.ID)
		if err != nil {
			b.logger.Printf("Error checking user authorization: %v", err)
			return
		}

		if !isAuthorized {
			s.ChannelMessageSend(m.ChannelID, "⛔ Sorry, you're not authorized to use this bot.")
			return
		}
	}

	// Save the request to history
	b.saveToHistory(m)

	// Handle commands (Discord uses / for slash commands, but we'll check for ! prefix)
	if strings.HasPrefix(m.Content, "!") {
		command := strings.TrimPrefix(m.Content, "!")
		parts := strings.Fields(command)
		if len(parts) > 0 {
			commandName := parts[0]
			args := parts[1:]

			switch commandName {
			case "help":
				b.handleHelpCommand(s, m)
			case "history":
				b.handleHistoryCommand(s, m)
			case "clear":
				b.handleClearHistoryCommand(s, m)
			case "model":
				b.handleModelCommand(s, m, args)
			default:
				// Treat as subreddit lookup
				b.handleSubredditRequest(s, m, m.Content)
			}
		}
	} else {
		// Treat non-command messages as subreddit requests
		b.handleSubredditRequest(s, m, m.Content)
	}
}

// isUserAuthorized checks if a user is authorized based on their roles
func (b *DiscordBot) isUserAuthorized(s *discordgo.Session, guildID, userID string) (bool, error) {
	// If no authorized role is specified, default to true
	if b.authorizedRoleID == "" {
		return true, nil
	}

	// For DMs, authorize the user if their ID matches the authorized user from config
	if guildID == "" && fmt.Sprintf("%d", b.config.AuthorizedUserID) == userID {
		return true, nil
	}

	// Get member info
	member, err := s.GuildMember(guildID, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get guild member: %w", err)
	}

	// Check if user has the authorized role
	for _, roleID := range member.Roles {
		if roleID == b.authorizedRoleID {
			return true, nil
		}
	}

	return false, nil
}

// saveToHistory adds a subreddit to the history if it's not already there
func (b *DiscordBot) saveToHistory(m *discordgo.MessageCreate) {
	// Extract subreddit from message
	subreddit := strings.TrimSpace(m.Content)
	if strings.HasPrefix(subreddit, "!") || strings.HasPrefix(subreddit, "/") || subreddit == "" {
		return // Skip commands and empty messages
	}

	// Add to history if not already present
	b.historyMutex.Lock()
	defer b.historyMutex.Unlock()

	for _, item := range b.history {
		if item == subreddit {
			return // Already in history
		}
	}

	// Add to history
	b.history = append(b.history, subreddit)
	b.logger.Printf("Added '%s' to history", subreddit)
}

// handleHelpCommand sends help information
func (b *DiscordBot) handleHelpCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	helpText := `**SubTrends Bot Commands**:
!help - Show this help message
!history - Show your recently searched subreddits
!clear - Clear your subreddit history
!model [simple/balanced/advanced] - Select Anthropic model for AI analysis

Send a subreddit name to get an analysis of its top posts and comments.`

	s.ChannelMessageSend(m.ChannelID, helpText)
}

// handleHistoryCommand displays the user's subreddit search history
func (b *DiscordBot) handleHistoryCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	b.historyMutex.RLock()
	defer b.historyMutex.RUnlock()

	if len(b.history) == 0 {
		s.ChannelMessageSend(m.ChannelID, "Your subreddit history is empty.")
		return
	}

	// Format history message
	historyMessage := "**Your recent subreddits**:\n"
	for i, subreddit := range b.history {
		historyMessage += fmt.Sprintf("%d. %s\n", i+1, subreddit)
	}

	s.ChannelMessageSend(m.ChannelID, historyMessage)
}

// handleClearHistoryCommand clears the user's subreddit search history
func (b *DiscordBot) handleClearHistoryCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	b.historyMutex.Lock()
	defer b.historyMutex.Unlock()

	b.history = []string{}
	b.logger.Println("History cleared")

	s.ChannelMessageSend(m.ChannelID, "Your subreddit history has been cleared.")
}

// handleModelCommand allows users to select the Anthropic model
func (b *DiscordBot) handleModelCommand(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	// If no arguments provided, show current model
	if len(args) == 0 {
		currentModel := b.getCurrentModel()
		modelInfo := "Unknown model"

		for _, model := range availableModels {
			if model.Name == currentModel {
				modelInfo = fmt.Sprintf("%s (%s): %s", model.Codename, model.Name, model.Description)
				break
			}
		}

		message := fmt.Sprintf("Current model: %s\n\nAvailable models:\n", modelInfo)
		for _, model := range availableModels {
			message += fmt.Sprintf("- %s (%s): %s\n", model.Codename, model.Name, model.Description)
		}

		s.ChannelMessageSend(m.ChannelID, message)
		return
	}

	// Set model based on argument
	modelCode := strings.ToLower(args[0])
	var selectedModel string
	var modelDescription string

	for _, model := range availableModels {
		if strings.EqualFold(model.Codename, modelCode) {
			selectedModel = model.Name
			modelDescription = model.Description
			break
		}
	}

	if selectedModel == "" {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Unknown model code '%s'. Use simple, balanced, or advanced.", modelCode))
		return
	}

	// Update the model
	b.setCurrentModel(selectedModel)
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Model updated to: %s - %s", modelCode, modelDescription))
}

// getCurrentModel gets the current model name
func (b *DiscordBot) getCurrentModel() string {
	b.modelMutex.RLock()
	defer b.modelMutex.RUnlock()
	return b.model
}

// setCurrentModel sets the current model name
func (b *DiscordBot) setCurrentModel(modelName string) {
	b.modelMutex.Lock()
	defer b.modelMutex.Unlock()
	b.model = modelName
}

// handleSubredditRequest processes a subreddit analysis request
func (b *DiscordBot) handleSubredditRequest(s *discordgo.Session, m *discordgo.MessageCreate, subreddit string) {
	// Send "typing" indicator
	s.ChannelTyping(m.ChannelID)

	// Send initial message
	statusMsg, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🔍 Analyzing r/%s...", subreddit))
	if err != nil {
		b.logger.Printf("Error sending initial status message: %v", err)
		return
	}

	// Get the access token for Reddit API
	token, err := getRedditAccessToken()
	if err != nil {
		errMsg := fmt.Sprintf("❌ Failed to authenticate with Reddit: %v", err)
		s.ChannelMessageEdit(m.ChannelID, statusMsg.ID, errMsg)
		return
	}

	// Update status message
	s.ChannelMessageEdit(m.ChannelID, statusMsg.ID, fmt.Sprintf("🔍 Fetching data for r/%s...", subreddit))

	// Fetch data from Reddit
	analysis, err := subredditData(subreddit, token)
	if err != nil {
		errMsg := fmt.Sprintf("❌ Error analyzing r/%s: %v", subreddit, err)
		s.ChannelMessageEdit(m.ChannelID, statusMsg.ID, errMsg)
		return
	}

	// Send the analysis, possibly in chunks if it's too long for Discord
	const maxDiscordMsgLength = 2000
	if len(analysis) <= maxDiscordMsgLength {
		// Edit the existing message with the full analysis
		s.ChannelMessageEdit(m.ChannelID, statusMsg.ID, analysis)
	} else {
		// Delete the status message
		s.ChannelMessageDelete(m.ChannelID, statusMsg.ID)

		// Split into chunks and send multiple messages
		for i := 0; i < len(analysis); i += maxDiscordMsgLength {
			end := i + maxDiscordMsgLength
			if end > len(analysis) {
				end = len(analysis)
			}
			chunk := analysis[i:end]
			s.ChannelMessageSend(m.ChannelID, chunk)
		}
	}
}
