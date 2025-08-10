package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveReadTokenFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "token.json")

	// Point config to temp file and small expiry buffer
	withEnv(t, "REDDIT_TOKEN_FILE_PATH", file)
	withEnv(t, "REDDIT_TOKEN_EXPIRY_BUFFER", "0s")
	LoadConfig()

	if err := saveTokenToFile("token123", 2); err != nil {
		t.Fatalf("saveTokenToFile failed: %v", err)
	}

	token, err := readTokenFromFile()
	if err != nil {
		t.Fatalf("readTokenFromFile failed: %v", err)
	}
	if token != "token123" {
		t.Fatalf("unexpected token: %s", token)
	}

	// After expiry buffer passes, token should be considered invalid
	// Overwrite with an expiring token
	if err := saveTokenToFile("short", 0); err != nil {
		t.Fatalf("saveTokenToFile (short) failed: %v", err)
	}
	// Wait a tick to ensure time comparison passes
	time.Sleep(2 * time.Millisecond)
	token, err = readTokenFromFile()
	if err != nil {
		t.Fatalf("readTokenFromFile short failed: %v", err)
	}
	if token != "" {
		t.Fatalf("expected empty token due to expiry, got: %s", token)
	}

	// ensure restrictive perms on file
	info, err := os.Stat(file)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("unexpected perms: %v", perm)
	}
}

// Exercising makeRequest, fetchTopPosts, fetchTopComments via a local server
func TestRedditHTTPFlow(t *testing.T) {
	// minimal in-memory reddit-like API
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/access_token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		// quick fake token
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 1})
	})
	mux.HandleFunc("/r/test/top", func(w http.ResponseWriter, r *http.Request) {
		// return 2 posts
		resp := RedditResponse{}
		resp.Data.Children = []struct {
			Data RedditPost "json:\"data\""
		}{
			{Data: RedditPost{Title: "P1", Ups: 10, Selftext: "", Permalink: "/r/test/comments/aa/slug"}},
			{Data: RedditPost{Title: "P2", Ups: 5, Selftext: "Body", Permalink: "/r/test/comments/bb/slug"}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/r/test/comments/aa/slug.json", func(w http.ResponseWriter, r *http.Request) {
		// Reddit comments shape: an array where second element holds comments
		payload := []any{
			map[string]any{},
			map[string]any{"data": map[string]any{"children": []any{
				map[string]any{"data": map[string]any{"body": "c1"}},
				map[string]any{"data": map[string]any{"body": "c2"}},
			}}},
		}
		_ = json.NewEncoder(w).Encode(payload)
	})
	mux.HandleFunc("/r/test/comments/bb/slug.json", func(w http.ResponseWriter, r *http.Request) {
		payload := []any{
			map[string]any{},
			map[string]any{"data": map[string]any{"children": []any{
				map[string]any{"data": map[string]any{"body": "x"}},
			}}},
		}
		_ = json.NewEncoder(w).Encode(payload)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// point config to server
	withEnv(t, "REDDIT_AUTH_URL", srv.URL+"/api/v1/access_token")
	withEnv(t, "REDDIT_BASE_URL", srv.URL)
	withEnv(t, "REDDIT_PUBLIC_URL", srv.URL)
	withEnv(t, "REDDIT_USER_AGENT", "ua")
	withEnv(t, "REDDIT_POST_LIMIT", "2")
	withEnv(t, "REDDIT_COMMENT_LIMIT", "2")
	withEnv(t, "REDDIT_TIMEFRAME", "day")
	withEnv(t, "REDDIT_REQUESTS_PER_SECOND", "100")
	withEnv(t, "REDDIT_BURST_SIZE", "100")
	withEnv(t, "REDDIT_REQUEST_TIMEOUT", "2s")
	withEnv(t, "REDDIT_CONCURRENT_REQUESTS", "2")
	withEnv(t, "REDDIT_TOKEN_EXPIRY_BUFFER", "0s")
	withEnv(t, "REDDIT_CLIENT_ID", "id")
	withEnv(t, "REDDIT_CLIENT_SECRET", "secret")
	LoadConfig()
	InitializeRedditRateLimiter()

	// First, get token via flow to ensure code path executes
	tok, err := getRedditAccessToken()
	if err != nil || tok == "" {
		t.Fatalf("expected token, got err=%v tok=%q", err, tok)
	}

	// Now fetch posts and comments and aggregate
	data, posts, totalComments, err := subredditData("test", tok)
	if err != nil {
		t.Fatalf("subredditData error: %v", err)
	}
	if len(posts) != 2 || totalComments != 3 {
		t.Fatalf("unexpected posts/comments: %d %d", len(posts), totalComments)
	}
	if data == "" {
		t.Fatalf("expected formatted markdown data")
	}

	// let token expire and ensure new one is fetched
	time.Sleep(2 * time.Second)
	tok2, err := getRedditAccessToken()
	if err != nil || tok2 == "" {
		t.Fatalf("expected renewed token: err=%v tok2=%q", err, tok2)
	}
}
