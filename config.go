package main

import (
	"os"
	"strconv"
	"time"
)

// Config stores all configuration for the application.
// Values are read from environment variables.
type Config struct {
	// Anthropic API settings
	AnthropicAPIEndpoint       string
	AnthropicAPIVersion        string
	AnthropicMaxTokens         int
	AnthropicTemperature       float64
	AnthropicRequestTimeout    time.Duration
	AnthropicRequestsPerMinute int
	AnthropicBurstSize         int
	SummaryHeader              string
	AnthropicAPIKey            string

	// Reddit API settings
	RedditBaseURL            string
	RedditAuthURL            string
	RedditPostLimit          int
	RedditCommentLimit       int
	RedditTimeFrame          string
	RedditRequestsPerSecond  int
	RedditBurstSize          int
	RedditTokenExpiryBuffer  time.Duration
	RedditTokenFilePath      string
	RedditRequestTimeout     time.Duration
	RedditConcurrentRequests int
	RedditUserAgent          string
	RedditPublicURL          string
	RedditClientID           string
	RedditClientSecret       string

	// Discord Bot settings
	DiscordMessageSplitLength int
	LegacyCommandPrefix       string
	SessionFilePath           string
	HistoryInitCapacity       int
	HistoryDisplayLimit       int

	// Application settings
	ShutdownTimeout time.Duration
}

// AppConfig holds the application's loaded configuration.
var AppConfig *Config

// LoadConfig loads configuration from environment variables and populates AppConfig.
func LoadConfig() {
	AppConfig = &Config{
		// Anthropic
		AnthropicAPIEndpoint:       getEnv("ANTHROPIC_API_ENDPOINT", "https://api.anthropic.com/v1/messages"),
		AnthropicAPIVersion:        getEnv("ANTHROPIC_API_VERSION", "2023-06-01"),
		AnthropicMaxTokens:         getEnvAsInt("ANTHROPIC_MAX_TOKENS", 1500),
		AnthropicTemperature:       getEnvAsFloat64("ANTHROPIC_TEMPERATURE", 0.7),
		AnthropicRequestTimeout:    getEnvAsDuration("ANTHROPIC_REQUEST_TIMEOUT", 45*time.Second),
		AnthropicRequestsPerMinute: getEnvAsInt("ANTHROPIC_REQUESTS_PER_MINUTE", 10),
		AnthropicBurstSize:         getEnvAsInt("ANTHROPIC_BURST_SIZE", 3),
		SummaryHeader:              getEnv("SUMMARY_HEADER", "📱 *REDDIT PULSE* 📱\n\n"),
		AnthropicAPIKey:            getEnv("ANTHROPIC_API_KEY", ""),

		// Reddit
		RedditBaseURL:            getEnv("REDDIT_BASE_URL", "https://oauth.reddit.com"),
		RedditAuthURL:            getEnv("REDDIT_AUTH_URL", "https://www.reddit.com/api/v1/access_token"),
		RedditPostLimit:          getEnvAsInt("REDDIT_POST_LIMIT", 7),
		RedditCommentLimit:       getEnvAsInt("REDDIT_COMMENT_LIMIT", 7),
		RedditTimeFrame:          getEnv("REDDIT_TIMEFRAME", "day"),
		RedditRequestsPerSecond:  getEnvAsInt("REDDIT_REQUESTS_PER_SECOND", 1),
		RedditBurstSize:          getEnvAsInt("REDDIT_BURST_SIZE", 5),
		RedditTokenExpiryBuffer:  getEnvAsDuration("REDDIT_TOKEN_EXPIRY_BUFFER", 5*time.Minute),
		RedditTokenFilePath:      getEnv("REDDIT_TOKEN_FILE_PATH", "data/reddit_token.json"),
		RedditRequestTimeout:     getEnvAsDuration("REDDIT_REQUEST_TIMEOUT", 10*time.Second),
		RedditConcurrentRequests: getEnvAsInt("REDDIT_CONCURRENT_REQUESTS", 3),
		RedditUserAgent:          getEnv("REDDIT_USER_AGENT", "SubTrends/1.0"),
		RedditPublicURL:          getEnv("REDDIT_PUBLIC_URL", "https://reddit.com"),
		RedditClientID:           getEnv("REDDIT_CLIENT_ID", ""),
		RedditClientSecret:       getEnv("REDDIT_CLIENT_SECRET", ""),

		// Discord Bot
		SessionFilePath:           getEnv("SESSION_FILE_PATH", "data/sessions.json"),
		HistoryInitCapacity:       getEnvAsInt("HISTORY_INIT_CAPACITY", 50),
		HistoryDisplayLimit:       getEnvAsInt("HISTORY_DISPLAY_LIMIT", 25),
		DiscordMessageSplitLength: getEnvAsInt("DISCORD_MESSAGE_SPLIT_LENGTH", 1900),
		LegacyCommandPrefix:       getEnv("LEGACY_COMMAND_PREFIX", "!trend "),

		// Application
		ShutdownTimeout: getEnvAsDuration("SHUTDOWN_TIMEOUT", 5*time.Second),
	}
}

// Helper functions to read and parse environment variables

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return fallback
}

func getEnvAsFloat64(key string, fallback float64) float64 {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
		return value
	}
	return fallback
}

func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	valueStr := getEnv(key, "")
	if value, err := time.ParseDuration(valueStr); err == nil {
		return value
	}
	return fallback
}
