package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// UserSession represents user session data for Discord users
type UserSession struct {
	UserID    string
	History   []string
	Model     string
	CreatedAt time.Time
}

// DiscordBot represents the Discord bot with its configuration and state
type DiscordBot struct {
	session      *discordgo.Session
	userSessions map[string]*UserSession
	sessionMutex sync.RWMutex
	stopChan     chan struct{}
}

// Available models for selection
var availableModels = []ModelInfo{
	{
		Codename:    "haiku3",
		Name:        "claude-3-haiku-20240307",
		Description: "Fast and efficient model (default)",
	},
	{
		Codename:    "haiku35",
		Name:        "claude-3-5-haiku-latest",
		Description: "Balanced performance and capabilities",
	},
	{
		Codename:    "sonnet4",
		Name:        "claude-sonnet-4-0",
		Description: "Most capable model for complex tasks",
	},
}

// ModelInfo represents information about an available model
type ModelInfo struct {
	Codename    string
	Name        string
	Description string
}

// NewDiscordBot creates a new Discord bot instance
func NewDiscordBot() (*DiscordBot, error) {
	// Get Discord bot token from environment
	token, err := GetRequiredEnvVar("DISCORD_BOT_TOKEN")
	if err != nil {
		return nil, err
	}

	// Create Discord session
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("error creating Discord session: %w", err)
	}

	// Set intents
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent

	bot := &DiscordBot{
		session:      dg,
		userSessions: make(map[string]*UserSession),
		stopChan:     make(chan struct{}),
	}

	// Load existing sessions
	bot.loadSessions()

	// Register handlers
	dg.AddHandler(bot.messageCreate)
	dg.AddHandler(bot.ready)
	dg.AddHandler(bot.interactionCreate)

	return bot, nil
}

// Start starts the Discord bot
func (bot *DiscordBot) Start(ctx context.Context) error {
	log.Println("Opening Discord connection...")

	err := bot.session.Open()
	if err != nil {
		return fmt.Errorf("error opening Discord connection: %w", err)
	}

	// Register slash commands
	if err := bot.registerCommands(); err != nil {
		log.Printf("Error registering commands: %v", err)
	}

	// Wait for context cancellation or stop signal
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-bot.stopChan:
		return nil
	}
}

// Stop gracefully stops the Discord bot
func (bot *DiscordBot) Stop(ctx context.Context) error {
	log.Println("Stopping Discord bot...")

	// Signal the bot to stop
	close(bot.stopChan)

	// Close Discord session
	if err := bot.session.Close(); err != nil {
		return fmt.Errorf("error closing Discord session: %w", err)
	}

	log.Println("Discord bot stopped successfully")
	return nil
}

// ready handler is called when the bot is ready
func (bot *DiscordBot) ready(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
}

// registerCommands registers slash commands with Discord
func (bot *DiscordBot) registerCommands() error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "trend",
			Description: "Analyze trends in a subreddit",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "subreddit",
					Description: "The subreddit to analyze (without r/)",
					Required:    true,
				},
			},
		},
		{
			Name:        "model",
			Description: "Change the AI model used for analysis",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "model",
					Description: "Choose AI model",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Haiku 3 (Fast)",
							Value: "haiku3",
						},
						{
							Name:  "Haiku 3.5 (Balanced)",
							Value: "haiku35",
						},
						{
							Name:  "Sonnet 4 (Most Capable)",
							Value: "sonnet4",
						},
					},
				},
			},
		},
		{
			Name:        "history",
			Description: "View your subreddit analysis history",
		},
		{
			Name:        "clear",
			Description: "Clear your analysis history",
		},
	}

	log.Println("Registering slash commands...")
	for _, cmd := range commands {
		_, err := bot.session.ApplicationCommandCreate(bot.session.State.User.ID, "", cmd)
		if err != nil {
			return fmt.Errorf("cannot create '%v' command: %w", cmd.Name, err)
		}
	}
	log.Println("Slash commands registered successfully")

	return nil
}

// getUserSession retrieves or creates a user session
func (bot *DiscordBot) getUserSession(userID string) *UserSession {
	bot.sessionMutex.Lock()
	defer bot.sessionMutex.Unlock()

	session, exists := bot.userSessions[userID]
	if !exists {
		session = &UserSession{
			UserID:    userID,
			History:   make([]string, 0, 50),
			Model:     availableModels[0].Name, // Default to first model
			CreatedAt: time.Now(),
		}
		bot.userSessions[userID] = session
	}

	return session
}

// saveUserSession saves a user session and persists to file
func (bot *DiscordBot) saveUserSession(userID string, session *UserSession) {
	bot.sessionMutex.Lock()
	defer bot.sessionMutex.Unlock()
	bot.userSessions[userID] = session

	// Persist sessions to file
	go bot.saveSessions()
}

// saveSessions persists all sessions to data/sessions.json
func (bot *DiscordBot) saveSessions() {
	bot.sessionMutex.RLock()
	sessions := make(map[string]*UserSession)
	for k, v := range bot.userSessions {
		sessions[k] = v
	}
	bot.sessionMutex.RUnlock()

	sessionFile := filepath.Join("data", "sessions.json")
	if err := WriteJSONFile(sessionFile, sessions); err != nil {
		log.Printf("Error writing sessions file: %v", err)
	}
}

// loadSessions loads sessions from data/sessions.json
func (bot *DiscordBot) loadSessions() {
	sessionFile := filepath.Join("data", "sessions.json")
	var sessions map[string]*UserSession

	if err := ReadJSONFile(sessionFile, &sessions); err != nil {
		log.Printf("Error reading or parsing sessions file: %v", err)
		return
	}

	if sessions == nil {
		log.Println("No existing sessions found or file is empty.")
		return
	}

	bot.sessionMutex.Lock()
	defer bot.sessionMutex.Unlock()

	for userID, session := range sessions {
		bot.userSessions[userID] = session
	}

	log.Printf("Loaded %d user sessions", len(sessions))
}

// messageCreate handler for regular messages (for backward compatibility)
func (bot *DiscordBot) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Handle simple text commands for backward compatibility
	if strings.HasPrefix(m.Content, "!trend ") {
		subreddit := strings.TrimSpace(strings.TrimPrefix(m.Content, "!trend "))
		if subreddit != "" {
			bot.handleTrendCommand(s, m.ChannelID, m.Author.ID, subreddit)
		}
	}
}

// interactionCreate handler for slash commands
func (bot *DiscordBot) interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.ApplicationCommandData().Name == "trend" {
		bot.handleTrendSlashCommand(s, i)
	} else if i.ApplicationCommandData().Name == "model" {
		bot.handleModelSlashCommand(s, i)
	} else if i.ApplicationCommandData().Name == "history" {
		bot.handleHistorySlashCommand(s, i)
	} else if i.ApplicationCommandData().Name == "clear" {
		bot.handleClearSlashCommand(s, i)
	}
}

// handleTrendSlashCommand handles the /trend slash command
func (bot *DiscordBot) handleTrendSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		bot.respondError(s, i, "Please provide a subreddit name")
		return
	}

	subreddit := options[0].StringValue()
	userID := i.Member.User.ID

	// Respond immediately to acknowledge the command
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("🔍 Analyzing r/%s... This may take a moment.", subreddit),
		},
	})
	if err != nil {
		log.Printf("Error responding to interaction: %v", err)
		return
	}

	// Handle the analysis in a goroutine
	go bot.handleTrendAnalysis(s, i.ChannelID, userID, subreddit)
}

// handleTrendCommand handles the trend command (both slash and text)
func (bot *DiscordBot) handleTrendCommand(s *discordgo.Session, channelID, userID, subreddit string) {
	// Send initial message
	msg, err := s.ChannelMessageSend(channelID, fmt.Sprintf("🔍 Analyzing r/%s... This may take a moment.", subreddit))
	if err != nil {
		log.Printf("Error sending initial message: %v", err)
		return
	}

	bot.handleTrendAnalysis(s, channelID, userID, subreddit)

	// Delete the initial message
	s.ChannelMessageDelete(channelID, msg.ID)
}

// handleTrendAnalysis performs the actual subreddit analysis
func (bot *DiscordBot) handleTrendAnalysis(s *discordgo.Session, channelID, userID, subreddit string) {
	// Clean subreddit name
	subreddit = strings.TrimPrefix(subreddit, "r/")

	session := bot.getUserSession(userID)

	// Add to history if not already present
	found := false
	for _, existing := range session.History {
		if strings.EqualFold(existing, subreddit) {
			found = true
			break
		}
	}
	if !found {
		session.History = append(session.History, subreddit)
		bot.saveUserSession(userID, session)
		log.Printf("Added %s to history for user %s", subreddit, userID)
	}

	// Get Reddit data
	token, err := getRedditAccessToken()
	if err != nil {
		log.Printf("Failed to get access token: %v", err)
		bot.sendMessage(s, channelID, "❌ Failed to connect to Reddit API")
		return
	}

	data, err := subredditData(subreddit, token)
	if err != nil {
		log.Printf("Failed to get subreddit data: %v", err)
		bot.sendMessage(s, channelID, fmt.Sprintf("❌ Failed to analyze r/%s: %v", subreddit, err))
		return
	}

	// Generate summary
	summary, err := summarizePosts(data, session.Model)
	if err != nil {
		log.Printf("Failed to generate summary: %v", err)
		bot.sendMessage(s, channelID, "❌ Failed to generate AI summary")
		return
	}

	// Get post links
	posts, err := fetchTopPosts(subreddit, token)
	if err != nil {
		log.Printf("Failed to fetch posts for links: %v", err)
		posts = []RedditPost{} // Ensure posts is never nil
	}

	// Format and send response
	response := bot.formatAnalysisResponse(subreddit, summary, posts)
	bot.sendLongMessage(s, channelID, response)
}

// formatAnalysisResponse formats the analysis response for Discord
func (bot *DiscordBot) formatAnalysisResponse(subreddit, summary string, posts []RedditPost) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("## 📈 **r/%s Trends**\n\n", subreddit))
	builder.WriteString(summary)
	builder.WriteString("\n\n")

	if len(posts) > 0 {
		builder.WriteString("### 🔗 **Top Posts**\n")
		for _, post := range posts {
			builder.WriteString(fmt.Sprintf("• [%s](<https://reddit.com%s>)\n", post.Title, post.Permalink))
		}
	}

	return builder.String()
}

// handleModelSlashCommand handles the /model slash command
func (bot *DiscordBot) handleModelSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		bot.respondError(s, i, "Please select a model")
		return
	}

	modelCodename := options[0].StringValue()
	userID := i.Member.User.ID

	// Validate model
	var selectedModel ModelInfo
	validModel := false
	for _, model := range availableModels {
		if modelCodename == model.Codename {
			validModel = true
			selectedModel = model
			break
		}
	}

	if !validModel {
		bot.respondError(s, i, "Invalid model selection")
		return
	}

	// Update user session
	session := bot.getUserSession(userID)
	session.Model = selectedModel.Name
	bot.saveUserSession(userID, session)

	// Respond
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("✅ Model changed to **%s** (%s)", selectedModel.Description, selectedModel.Codename),
		},
	})
	if err != nil {
		log.Printf("Error responding to model interaction: %v", err)
	}
}

// handleHistorySlashCommand handles the /history slash command
func (bot *DiscordBot) handleHistorySlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := i.Member.User.ID
	session := bot.getUserSession(userID)

	var response string
	if len(session.History) == 0 {
		response = "📝 Your analysis history is empty. Use `/trend <subreddit>` to start analyzing!"
	} else {
		var builder strings.Builder
		builder.WriteString("📝 **Your Analysis History**\n\n")

		// Show recent history (last 25)
		start := 0
		if len(session.History) > 25 {
			start = len(session.History) - 25
		}

		for i := start; i < len(session.History); i++ {
			builder.WriteString(fmt.Sprintf("• r/%s\n", session.History[i]))
		}

		if len(session.History) > 25 {
			builder.WriteString(fmt.Sprintf("\n*Showing last 25 of %d total analyses*", len(session.History)))
		}

		response = builder.String()
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response,
		},
	})
	if err != nil {
		log.Printf("Error responding to history interaction: %v", err)
	}
}

// handleClearSlashCommand handles the /clear slash command
func (bot *DiscordBot) handleClearSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := i.Member.User.ID
	session := bot.getUserSession(userID)

	session.History = make([]string, 0, 50)
	bot.saveUserSession(userID, session)

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "🗑️ **History cleared** successfully!",
		},
	})
	if err != nil {
		log.Printf("Error responding to clear interaction: %v", err)
	}
}

// Helper functions
func (bot *DiscordBot) sendMessage(s *discordgo.Session, channelID, content string) {
	_, err := s.ChannelMessageSend(channelID, content)
	if err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

// sendLongMessage splits long messages into chunks under Discord's 2000 character limit
func (bot *DiscordBot) sendLongMessage(s *discordgo.Session, channelID, content string) {
	const maxLength = 1900 // Leave some buffer under 2000 limit

	if len(content) <= maxLength {
		bot.sendMessage(s, channelID, content)
		return
	}

	// Split into chunks
	lines := strings.Split(content, "\n")
	var currentChunk strings.Builder

	for _, line := range lines {
		// If adding this line would exceed the limit, send current chunk
		if currentChunk.Len()+len(line)+1 > maxLength {
			if currentChunk.Len() > 0 {
				bot.sendMessage(s, channelID, currentChunk.String())
				currentChunk.Reset()
			}
		}

		// If a single line is too long, truncate it
		if len(line) > maxLength {
			line = line[:maxLength-3] + "..."
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n")
		}
		currentChunk.WriteString(line)
	}

	// Send remaining content
	if currentChunk.Len() > 0 {
		bot.sendMessage(s, channelID, currentChunk.String())
	}
}

func (bot *DiscordBot) respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "❌ " + message,
		},
	})
	if err != nil {
		log.Printf("Error responding with error: %v", err)
	}
}
