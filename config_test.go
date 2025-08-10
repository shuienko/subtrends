package main

import (
	"os"
	"testing"
	"time"
)

// helper to set and restore env vars within a test
func withEnv(t *testing.T, key, value string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	if value == "" {
		_ = os.Unsetenv(key)
	} else {
		_ = os.Setenv(key, value)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func TestLoadConfigDefaults(t *testing.T) {
	// Clear a subset of envs to ensure defaults apply
	withEnv(t, "OPENAI_API_ENDPOINT", "")
	withEnv(t, "OPENAI_REQUEST_TIMEOUT", "")
	withEnv(t, "OPENAI_REQUESTS_PER_MINUTE", "")
	withEnv(t, "OPENAI_BURST_SIZE", "")
	withEnv(t, "SUMMARY_HEADER", "")

	withEnv(t, "REDDIT_BASE_URL", "")
	withEnv(t, "REDDIT_POST_LIMIT", "")
	withEnv(t, "REDDIT_COMMENT_LIMIT", "")
	withEnv(t, "REDDIT_TIMEFRAME", "")
	withEnv(t, "REDDIT_REQUESTS_PER_SECOND", "")
	withEnv(t, "REDDIT_BURST_SIZE", "")
	withEnv(t, "REDDIT_TOKEN_EXPIRY_BUFFER", "")
	withEnv(t, "REDDIT_TOKEN_FILE_PATH", "")
	withEnv(t, "REDDIT_REQUEST_TIMEOUT", "")
	withEnv(t, "REDDIT_CONCURRENT_REQUESTS", "")
	withEnv(t, "REDDIT_USER_AGENT", "")
	withEnv(t, "REDDIT_PUBLIC_URL", "")

	withEnv(t, "SESSION_FILE_PATH", "")
	withEnv(t, "HISTORY_INIT_CAPACITY", "")
	withEnv(t, "HISTORY_DISPLAY_LIMIT", "")
	withEnv(t, "DISCORD_MESSAGE_SPLIT_LENGTH", "")
	withEnv(t, "LEGACY_COMMAND_PREFIX", "")

	withEnv(t, "SHUTDOWN_TIMEOUT", "")

	LoadConfig()
	if AppConfig == nil {
		t.Fatal("AppConfig is nil after LoadConfig")
	}

	if AppConfig.OpenAIAPIEndpoint != "https://api.openai.com/v1/chat/completions" {
		t.Errorf("unexpected OpenAIAPIEndpoint default: %s", AppConfig.OpenAIAPIEndpoint)
	}
	if AppConfig.OpenAIRequestTimeout != 45*time.Second {
		t.Errorf("unexpected OpenAIRequestTimeout default: %v", AppConfig.OpenAIRequestTimeout)
	}
	if AppConfig.OpenAIRequestsPerMinute != 10 {
		t.Errorf("unexpected OpenAIRequestsPerMinute default: %d", AppConfig.OpenAIRequestsPerMinute)
	}

	if AppConfig.RedditBaseURL != "https://oauth.reddit.com" {
		t.Errorf("unexpected RedditBaseURL default: %s", AppConfig.RedditBaseURL)
	}
	if AppConfig.RedditPostLimit != 7 {
		t.Errorf("unexpected RedditPostLimit default: %d", AppConfig.RedditPostLimit)
	}
	if AppConfig.RedditCommentLimit != 7 {
		t.Errorf("unexpected RedditCommentLimit default: %d", AppConfig.RedditCommentLimit)
	}
	if AppConfig.RedditTimeFrame != "day" {
		t.Errorf("unexpected RedditTimeFrame default: %s", AppConfig.RedditTimeFrame)
	}
	if AppConfig.RedditRequestsPerSecond != 1 {
		t.Errorf("unexpected RedditRequestsPerSecond default: %d", AppConfig.RedditRequestsPerSecond)
	}
	if AppConfig.RedditBurstSize != 5 {
		t.Errorf("unexpected RedditBurstSize default: %d", AppConfig.RedditBurstSize)
	}
	if AppConfig.RedditTokenExpiryBuffer != 5*time.Minute {
		t.Errorf("unexpected RedditTokenExpiryBuffer default: %v", AppConfig.RedditTokenExpiryBuffer)
	}
	if AppConfig.RedditTokenFilePath != "data/reddit_token.json" {
		t.Errorf("unexpected RedditTokenFilePath default: %s", AppConfig.RedditTokenFilePath)
	}
	if AppConfig.RedditRequestTimeout != 10*time.Second {
		t.Errorf("unexpected RedditRequestTimeout default: %v", AppConfig.RedditRequestTimeout)
	}
	if AppConfig.RedditConcurrentRequests != 3 {
		t.Errorf("unexpected RedditConcurrentRequests default: %d", AppConfig.RedditConcurrentRequests)
	}
	if AppConfig.RedditUserAgent != "SubTrends/1.0" {
		t.Errorf("unexpected RedditUserAgent default: %s", AppConfig.RedditUserAgent)
	}
	if AppConfig.RedditPublicURL != "https://reddit.com" {
		t.Errorf("unexpected RedditPublicURL default: %s", AppConfig.RedditPublicURL)
	}

	if AppConfig.SessionFilePath != "data/sessions.json" {
		t.Errorf("unexpected SessionFilePath default: %s", AppConfig.SessionFilePath)
	}
	if AppConfig.HistoryInitCapacity != 50 {
		t.Errorf("unexpected HistoryInitCapacity default: %d", AppConfig.HistoryInitCapacity)
	}
	if AppConfig.HistoryDisplayLimit != 25 {
		t.Errorf("unexpected HistoryDisplayLimit default: %d", AppConfig.HistoryDisplayLimit)
	}
	if AppConfig.DiscordMessageSplitLength != 1900 {
		t.Errorf("unexpected DiscordMessageSplitLength default: %d", AppConfig.DiscordMessageSplitLength)
	}
	if AppConfig.LegacyCommandPrefix != "!trend " {
		t.Errorf("unexpected LegacyCommandPrefix default: %s", AppConfig.LegacyCommandPrefix)
	}
	if AppConfig.ShutdownTimeout != 5*time.Second {
		t.Errorf("unexpected ShutdownTimeout default: %v", AppConfig.ShutdownTimeout)
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	withEnv(t, "OPENAI_API_ENDPOINT", "https://example.com/api")
	withEnv(t, "OPENAI_REQUEST_TIMEOUT", "2s")
	withEnv(t, "OPENAI_REQUESTS_PER_MINUTE", "99")
	withEnv(t, "OPENAI_BURST_SIZE", "7")
	withEnv(t, "SUMMARY_HEADER", "HEADER\n\n")

	withEnv(t, "REDDIT_BASE_URL", "https://oauth.example")
	withEnv(t, "REDDIT_POST_LIMIT", "3")
	withEnv(t, "REDDIT_COMMENT_LIMIT", "5")
	withEnv(t, "REDDIT_TIMEFRAME", "week")
	withEnv(t, "REDDIT_REQUESTS_PER_SECOND", "2")
	withEnv(t, "REDDIT_BURST_SIZE", "9")
	withEnv(t, "REDDIT_TOKEN_EXPIRY_BUFFER", "1m")
	withEnv(t, "REDDIT_TOKEN_FILE_PATH", "tmp/token.json")
	withEnv(t, "REDDIT_REQUEST_TIMEOUT", "3s")
	withEnv(t, "REDDIT_CONCURRENT_REQUESTS", "4")
	withEnv(t, "REDDIT_USER_AGENT", "UA/1.0")
	withEnv(t, "REDDIT_PUBLIC_URL", "https://r.example")

	withEnv(t, "SESSION_FILE_PATH", "tmp/sessions.json")
	withEnv(t, "HISTORY_INIT_CAPACITY", "11")
	withEnv(t, "HISTORY_DISPLAY_LIMIT", "22")
	withEnv(t, "DISCORD_MESSAGE_SPLIT_LENGTH", "1234")
	withEnv(t, "LEGACY_COMMAND_PREFIX", "#t ")

	withEnv(t, "SHUTDOWN_TIMEOUT", "1s")

	LoadConfig()

	if AppConfig.OpenAIAPIEndpoint != "https://example.com/api" {
		t.Fatalf("OpenAIAPIEndpoint override failed: %s", AppConfig.OpenAIAPIEndpoint)
	}
	if AppConfig.OpenAIRequestTimeout != 2*time.Second {
		t.Fatalf("OpenAIRequestTimeout override failed: %v", AppConfig.OpenAIRequestTimeout)
	}
	if AppConfig.OpenAIRequestsPerMinute != 99 {
		t.Fatalf("OpenAIRequestsPerMinute override failed: %d", AppConfig.OpenAIRequestsPerMinute)
	}
	if AppConfig.OpenAIBurstSize != 7 {
		t.Fatalf("OpenAIBurstSize override failed: %d", AppConfig.OpenAIBurstSize)
	}
	if AppConfig.SummaryHeader != "HEADER\n\n" {
		t.Fatalf("SummaryHeader override failed: %q", AppConfig.SummaryHeader)
	}

	if AppConfig.RedditBaseURL != "https://oauth.example" {
		t.Fatalf("RedditBaseURL override failed: %s", AppConfig.RedditBaseURL)
	}
	if AppConfig.RedditPostLimit != 3 {
		t.Fatalf("RedditPostLimit override failed: %d", AppConfig.RedditPostLimit)
	}
	if AppConfig.RedditCommentLimit != 5 {
		t.Fatalf("RedditCommentLimit override failed: %d", AppConfig.RedditCommentLimit)
	}
	if AppConfig.RedditTimeFrame != "week" {
		t.Fatalf("RedditTimeFrame override failed: %s", AppConfig.RedditTimeFrame)
	}
	if AppConfig.RedditRequestsPerSecond != 2 {
		t.Fatalf("RedditRequestsPerSecond override failed: %d", AppConfig.RedditRequestsPerSecond)
	}
	if AppConfig.RedditBurstSize != 9 {
		t.Fatalf("RedditBurstSize override failed: %d", AppConfig.RedditBurstSize)
	}
	if AppConfig.RedditTokenExpiryBuffer != 1*time.Minute {
		t.Fatalf("RedditTokenExpiryBuffer override failed: %v", AppConfig.RedditTokenExpiryBuffer)
	}
	if AppConfig.RedditTokenFilePath != "tmp/token.json" {
		t.Fatalf("RedditTokenFilePath override failed: %s", AppConfig.RedditTokenFilePath)
	}
	if AppConfig.RedditRequestTimeout != 3*time.Second {
		t.Fatalf("RedditRequestTimeout override failed: %v", AppConfig.RedditRequestTimeout)
	}
	if AppConfig.RedditConcurrentRequests != 4 {
		t.Fatalf("RedditConcurrentRequests override failed: %d", AppConfig.RedditConcurrentRequests)
	}
	if AppConfig.RedditUserAgent != "UA/1.0" {
		t.Fatalf("RedditUserAgent override failed: %s", AppConfig.RedditUserAgent)
	}
	if AppConfig.RedditPublicURL != "https://r.example" {
		t.Fatalf("RedditPublicURL override failed: %s", AppConfig.RedditPublicURL)
	}

	if AppConfig.SessionFilePath != "tmp/sessions.json" {
		t.Fatalf("SessionFilePath override failed: %s", AppConfig.SessionFilePath)
	}
	if AppConfig.HistoryInitCapacity != 11 {
		t.Fatalf("HistoryInitCapacity override failed: %d", AppConfig.HistoryInitCapacity)
	}
	if AppConfig.HistoryDisplayLimit != 22 {
		t.Fatalf("HistoryDisplayLimit override failed: %d", AppConfig.HistoryDisplayLimit)
	}
	if AppConfig.DiscordMessageSplitLength != 1234 {
		t.Fatalf("DiscordMessageSplitLength override failed: %d", AppConfig.DiscordMessageSplitLength)
	}
	if AppConfig.LegacyCommandPrefix != "#t " {
		t.Fatalf("LegacyCommandPrefix override failed: %s", AppConfig.LegacyCommandPrefix)
	}
	if AppConfig.ShutdownTimeout != 1*time.Second {
		t.Fatalf("ShutdownTimeout override failed: %v", AppConfig.ShutdownTimeout)
	}
}
