package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// UserSession represents user session data for Discord users
type UserSession struct {
	UserID          string
	History         []string
	Model           string
	ReasoningEffort string
	CreatedAt       time.Time
}

// DiscordBot represents the Discord bot with its configuration and state
type DiscordBot struct {
	session      *discordgo.Session
	userSessions map[string]*UserSession
	sessionMutex sync.RWMutex
	stopChan     chan struct{}
}

// Available models for selection (OpenAI)
var availableModels = []ModelInfo{
	{
		Codename:    "gpt5nano",
		Name:        "gpt-5-nano",
		Description: "Fast and efficient model (default)",
	},
	{
		Codename:    "gpt52",
		Name:        "gpt-5.2",
		Description: "Most capable model for complex tasks",
	},
}

func getDefaultModelName() string {
	return availableModels[0].Name
}

func getDefaultReasoningEffort() string {
	return defaultReasoningEffort
}

func isValidReasoningEffort(level string) bool {
	switch level {
	case "minimal", "medium", "high":
		return true
	default:
		return false
	}
}

func isValidModelName(name string) bool {
	if name == "" {
		return false
	}
	for _, m := range availableModels {
		if name == m.Name {
			return true
		}
	}
	return false
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
	// Dynamically create model choices from availableModels
	modelChoices := make([]*discordgo.ApplicationCommandOptionChoice, len(availableModels))
	for i, model := range availableModels {
		modelChoices[i] = &discordgo.ApplicationCommandOptionChoice{
			Name:  fmt.Sprintf("%s (%s)", model.Name, model.Description),
			Value: model.Codename,
		}
	}

	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "trend",
			Description: "Analyze trends in a subreddit",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "subreddit",
					Description:  "The subreddit to analyze (without r/)",
					Required:     true,
					Autocomplete: true,
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
					Choices:     modelChoices,
				},
			},
		},
		{
			Name:        "reasoning",
			Description: "Change the reasoning effort level used for analysis",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "level",
					Description: "Choose reasoning effort level",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Minimal (fastest, cheapest)",
							Value: "minimal",
						},
						{
							Name:  "Medium (balanced)",
							Value: "medium",
						},
						{
							Name:  "High (most thorough, slowest)",
							Value: "high",
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

	log.Println("Upserting slash commands via bulk overwrite...")

	// Use global bulk overwrite to create/update commands atomically
	_, err := bot.session.ApplicationCommandBulkOverwrite(bot.session.State.User.ID, "", commands)
	if err != nil {
		return fmt.Errorf("failed to bulk overwrite application commands: %w", err)
	}

	log.Println("Slash commands upserted successfully")

	return nil
}

// getUserSession retrieves or creates a user session
func (bot *DiscordBot) getUserSession(userID string) *UserSession {
	bot.sessionMutex.Lock()
	defer bot.sessionMutex.Unlock()

	session, exists := bot.userSessions[userID]
	if !exists {
		session = &UserSession{
			UserID:          userID,
			History:         make([]string, 0, AppConfig.HistoryInitCapacity),
			Model:           getDefaultModelName(),
			ReasoningEffort: getDefaultReasoningEffort(),
			CreatedAt:       time.Now(),
		}
		bot.userSessions[userID] = session
	} else {
		// Migrate invalid/legacy model names to default
		needsSave := false

		if !isValidModelName(session.Model) {
			session.Model = getDefaultModelName()
			needsSave = true
		}

		if !isValidReasoningEffort(session.ReasoningEffort) {
			session.ReasoningEffort = getDefaultReasoningEffort()
			needsSave = true
		}

		if needsSave {
			bot.userSessions[userID] = session
			go bot.saveSessions()
		}
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

	if err := WriteJSONFile(AppConfig.SessionFilePath, sessions); err != nil {
		log.Printf("Error writing sessions file: %v", err)
	}
}

// loadSessions loads sessions from data/sessions.json
func (bot *DiscordBot) loadSessions() {
	var sessions map[string]*UserSession

	if err := ReadJSONFile(AppConfig.SessionFilePath, &sessions); err != nil {
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
		// Normalize legacy/invalid models
		if !isValidModelName(session.Model) {
			session.Model = getDefaultModelName()
		}

		// Normalize legacy/invalid reasoning effort
		if !isValidReasoningEffort(session.ReasoningEffort) {
			session.ReasoningEffort = getDefaultReasoningEffort()
		}
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
	if strings.HasPrefix(m.Content, AppConfig.LegacyCommandPrefix) {
		subreddit := strings.TrimSpace(strings.TrimPrefix(m.Content, AppConfig.LegacyCommandPrefix))
		if subreddit != "" {
			bot.handleTrendCommand(s, m.ChannelID, m.Author.ID, subreddit)
		}
	}
}

// interactionCreate handler for slash commands
func (bot *DiscordBot) interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Handle autocomplete interactions separately
	switch i.Type {
	case discordgo.InteractionApplicationCommandAutocomplete:
		if i.ApplicationCommandData().Name == "trend" {
			bot.handleTrendAutocomplete(s, i)
		}
		return
	case discordgo.InteractionApplicationCommand:
		// Regular slash command invocations
		if i.ApplicationCommandData().Name == "trend" {
			bot.handleTrendSlashCommand(s, i)
		} else if i.ApplicationCommandData().Name == "model" {
			bot.handleModelSlashCommand(s, i)
		} else if i.ApplicationCommandData().Name == "reasoning" {
			bot.handleReasoningSlashCommand(s, i)
		} else if i.ApplicationCommandData().Name == "history" {
			bot.handleHistorySlashCommand(s, i)
		} else if i.ApplicationCommandData().Name == "clear" {
			bot.handleClearSlashCommand(s, i)
		}
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

// handleTrendAutocomplete provides history-based suggestions for the subreddit option
func (bot *DiscordBot) handleTrendAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Determine user ID (DM vs Guild)
	var userID string
	if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
	} else if i.User != nil {
		userID = i.User.ID
	}

	// Current typed value for the focused option
	var typed string
	for _, opt := range i.ApplicationCommandData().Options {
		if opt.Focused {
			typed = strings.TrimSpace(opt.StringValue())
			break
		}
	}

	session := bot.getUserSession(userID)

	// Build suggestions from history
	const maxSuggestions = 25
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, maxSuggestions)

	// If nothing typed, prioritize most recent history (from end)
	if typed == "" {
		start := 0
		if len(session.History) > maxSuggestions {
			start = len(session.History) - maxSuggestions
		}
		for i := len(session.History) - 1; i >= start; i-- {
			sub := session.History[i]
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: sub, Value: sub})
			if len(choices) >= maxSuggestions {
				break
			}
		}
	} else {
		lowerTyped := strings.ToLower(typed)
		// De-duplicate while preserving order from most recent
		seen := make(map[string]struct{})
		for i := len(session.History) - 1; i >= 0; i-- {
			sub := session.History[i]
			key := strings.ToLower(sub)
			if _, ok := seen[key]; ok {
				continue
			}
			if strings.Contains(key, lowerTyped) {
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: sub, Value: sub})
				seen[key] = struct{}{}
				if len(choices) >= maxSuggestions {
					break
				}
			}
		}
	}

	// Respond with autocomplete choices
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
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

	data, posts, totalComments, err := subredditData(subreddit, token)
	if err != nil {
		log.Printf("Failed to get subreddit data: %v", err)
		bot.sendMessage(s, channelID, fmt.Sprintf("❌ Failed to analyze r/%s: %v", subreddit, err))
		return
	}

	// Generate summary
	summary, err := summarizePosts(subreddit, data, session.Model, session.ReasoningEffort)
	if err != nil {
		log.Printf("Failed to generate summary: %v", err)
		bot.sendMessage(s, channelID, "❌ Failed to generate AI summary")
		return
	}

	// Format and send response
	response := bot.formatAnalysisResponse(subreddit, summary, posts, totalComments)
	bot.sendLongMessage(s, channelID, response)
}

// formatAnalysisResponse formats the analysis response for Discord
func (bot *DiscordBot) formatAnalysisResponse(subreddit, summary string, posts []RedditPost, totalComments int) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("## 📈 **r/%s Trends**\n\n", subreddit))
	// Key stats line
	builder.WriteString(fmt.Sprintf("**Key stats**: %d posts analyzed • timeframe: %s • %d comments\n\n", len(posts), AppConfig.RedditTimeFrame, totalComments))
	builder.WriteString(summary)
	builder.WriteString("\n\n")

	if len(posts) > 0 {
		builder.WriteString("### 🔗 **Top Posts**\n")
		for _, post := range posts {
			builder.WriteString(fmt.Sprintf("• [%s](<%s%s>)\n", post.Title, AppConfig.RedditPublicURL, post.Permalink))
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
			Content: fmt.Sprintf("✅ Model changed to **%s** (%s)", selectedModel.Name, selectedModel.Description),
		},
	})
	if err != nil {
		log.Printf("Error responding to model interaction: %v", err)
	}
}

// handleReasoningSlashCommand handles the /reasoning slash command
func (bot *DiscordBot) handleReasoningSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		bot.respondError(s, i, "Please select a reasoning effort level")
		return
	}

	level := strings.ToLower(options[0].StringValue())
	if !isValidReasoningEffort(level) {
		bot.respondError(s, i, "Invalid reasoning effort level")
		return
	}

	userID := i.Member.User.ID
	session := bot.getUserSession(userID)
	session.ReasoningEffort = level
	bot.saveUserSession(userID, session)

	var humanLabel string
	switch level {
	case "minimal":
		humanLabel = "Minimal (fastest, cheapest)"
	case "medium":
		humanLabel = "Medium (balanced)"
	case "high":
		humanLabel = "High (most thorough, slowest)"
	default:
		humanLabel = level
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("✅ Reasoning effort set to **%s**.\n\nAll future analyses will use this level until you change it again.", humanLabel),
		},
	})
	if err != nil {
		log.Printf("Error responding to reasoning interaction: %v", err)
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
		if len(session.History) > AppConfig.HistoryDisplayLimit {
			start = len(session.History) - AppConfig.HistoryDisplayLimit
		}

		for i := start; i < len(session.History); i++ {
			builder.WriteString(fmt.Sprintf("• r/%s\n", session.History[i]))
		}

		if len(session.History) > AppConfig.HistoryDisplayLimit {
			builder.WriteString(fmt.Sprintf("\n*Showing last %d of %d total analyses*", AppConfig.HistoryDisplayLimit, len(session.History)))
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

	session.History = make([]string, 0, AppConfig.HistoryInitCapacity)
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
	maxLength := AppConfig.DiscordMessageSplitLength // Leave some buffer under 2000 limit

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
