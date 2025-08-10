package main

import (
	"strings"
	"testing"
)

func TestModelValidationHelpers(t *testing.T) {
	if def := getDefaultModelName(); def == "" {
		t.Fatal("default model should not be empty")
	}
	if !isValidModelName(getDefaultModelName()) {
		t.Fatal("default model should be valid")
	}
	if isValidModelName("") {
		t.Fatal("empty model should be invalid")
	}
	if isValidModelName("unknown-model") {
		t.Fatal("unknown model should be invalid")
	}
}

func TestFormatAnalysisResponse(t *testing.T) {
	withEnv(t, "REDDIT_TIMEFRAME", "week")
	withEnv(t, "REDDIT_PUBLIC_URL", "https://reddit.example")
	LoadConfig()

	bot := &DiscordBot{userSessions: map[string]*UserSession{}}
	posts := []RedditPost{
		{Title: "T1", Ups: 10, Selftext: "", Permalink: "/r/test/comments/abc/slug"},
		{Title: "T2", Ups: 2, Selftext: "Body", Permalink: "/r/test/comments/def/slug"},
	}
	out := bot.formatAnalysisResponse("test", "SUMMARY", posts, 5)

	if !containsAll(out, []string{
		"## 📈 **r/test Trends**",
		"Key stats",
		"2 posts analyzed",
		"timeframe: week",
		"5 comments",
		"[T1](<https://reddit.example/r/test/comments/abc/slug>)",
		"[T2](<https://reddit.example/r/test/comments/def/slug>)",
	}) {
		t.Fatalf("formatted response missing expected parts:\n%s", out)
	}
}

func TestSessionPersistenceAndMigration(t *testing.T) {
	// Store invalid model and ensure it gets migrated to default on load
	tmpPath := t.TempDir() + "/sessions.json"
	withEnv(t, "SESSION_FILE_PATH", tmpPath)
	LoadConfig()

	bot1 := &DiscordBot{userSessions: map[string]*UserSession{}}
	bot1.userSessions["u1"] = &UserSession{UserID: "u1", History: []string{"golang"}, Model: "invalid"}
	bot1.saveSessions()

	bot2 := &DiscordBot{userSessions: map[string]*UserSession{}}
	bot2.loadSessions()

	s := bot2.userSessions["u1"]
	if s == nil {
		t.Fatalf("expected session to load")
	}
	if s.Model != getDefaultModelName() {
		t.Fatalf("model not migrated to default: %s", s.Model)
	}
	if len(s.History) != 1 || s.History[0] != "golang" {
		t.Fatalf("unexpected history: %#v", s.History)
	}
}

func TestGetAndSaveUserSession(t *testing.T) {
	withEnv(t, "SESSION_FILE_PATH", t.TempDir()+"/sessions.json")
	LoadConfig()

	bot := &DiscordBot{userSessions: map[string]*UserSession{}}
	// get creates default if missing and normalizes invalid model later
	s1 := bot.getUserSession("u1")
	if s1 == nil || s1.Model != getDefaultModelName() {
		t.Fatalf("unexpected default session: %#v", s1)
	}

	// saveUserSession persists changes
	s1.History = append(s1.History, "golang")
	bot.saveUserSession("u1", s1)
	// force synchronous persist
	bot.saveSessions()

	// new bot instance loads sessions
	bot2 := &DiscordBot{userSessions: map[string]*UserSession{}}
	bot2.loadSessions()
	if got := bot2.userSessions["u1"]; got == nil || len(got.History) != 1 {
		t.Fatalf("session not loaded/persisted: %#v", got)
	}
}

// small helper
func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
