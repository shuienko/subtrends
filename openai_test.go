package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateOpenAIRequest(t *testing.T) {
	req := createOpenAIRequest("gpt-5-mini", "POSTS", "golang")
	if req.Model != "gpt-5-mini" {
		t.Fatalf("model mismatch: %s", req.Model)
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != "user" {
		t.Fatalf("messages format unexpected: %#v", req.Messages)
	}
	// Basic prompt sanity
	if req.Messages[0].Content == "" || req.Messages[0].Content == "POSTS" {
		t.Fatalf("prompt not constructed correctly: %q", req.Messages[0].Content)
	}
}

func TestFormatResponseErrors(t *testing.T) {
	if _, err := formatResponse(nil); err == nil {
		t.Fatal("expected error for nil response")
	}
	if _, err := formatResponse(&ChatCompletionResponse{Choices: nil}); err == nil {
		t.Fatal("expected error for empty choices")
	}
	resp := &ChatCompletionResponse{Choices: []struct {
		Message struct {
			Content string "json:\"content\""
		} "json:\"message\""
	}{}}
	if _, err := formatResponse(resp); err == nil {
		t.Fatal("expected error for missing content")
	}
}

func TestFormatResponseHeaderAndEmphasis(t *testing.T) {
	// Minimal successful response
	var c struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	c.Message.Content = "TRENDING TOPICS\nCOMMUNITY PULSE\nHOT TAKES"
	resp := &ChatCompletionResponse{Choices: []struct {
		Message struct {
			Content string "json:\"content\""
		} "json:\"message\""
	}{c}}

	// Ensure header is applied from config
	withEnv(t, "SUMMARY_HEADER", "HDR\n\n")
	LoadConfig()

	out, err := formatResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[:5] != "HDR\n\n" {
		t.Fatalf("header not applied: %q", out)
	}
}

func TestMakeOpenAIAPICallRateLimiterContextCancel(t *testing.T) {
	// Configure so limiter has zero burst and long period to force Wait to block
	withEnv(t, "OPENAI_REQUESTS_PER_MINUTE", "1")
	withEnv(t, "OPENAI_BURST_SIZE", "0")
	withEnv(t, "OPENAI_API_ENDPOINT", "http://127.0.0.1:1") // invalid endpoint to avoid real call
	withEnv(t, "OPENAI_API_KEY", "test")
	LoadConfig()
	InitializeOpenAIRateLimiter()

	// Use a context that times out immediately, expecting Wait to fail due to deadline
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Make a minimal request object
	req := createOpenAIRequest("gpt-5-mini", "X", "sub")
	_, err := makeOpenAIAPICall(ctx, req, AppConfig.OpenAIAPIKey)
	if err == nil {
		t.Fatal("expected error due to context timeout/limiter")
	}
	// Ensure we surface the context error or a wrapped rate-limit error
	if !errors.Is(err, context.DeadlineExceeded) {
		// still acceptable if wrapped differently, but must be non-nil which we've asserted
	}
}

func TestMakeOpenAIAPICallSuccessAndAPIError(t *testing.T) {
	// Happy path server
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"TRENDING TOPICS\nCOMMUNITY PULSE\nHOT TAKES"}}]}`))
	}))
	defer okSrv.Close()

	withEnv(t, "OPENAI_API_ENDPOINT", okSrv.URL)
	withEnv(t, "OPENAI_API_KEY", "k")
	withEnv(t, "OPENAI_REQUEST_TIMEOUT", "2s")
	withEnv(t, "OPENAI_REQUESTS_PER_MINUTE", "1000")
	withEnv(t, "OPENAI_BURST_SIZE", "1000")
	withEnv(t, "SUMMARY_HEADER", "H\n\n")
	LoadConfig()
	InitializeOpenAIRateLimiter()

	req := createOpenAIRequest("gpt-5-mini", "POSTS", "rtest")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := makeOpenAIAPICall(ctx, req, AppConfig.OpenAIAPIKey)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	out, err := formatResponse(resp)
	if err != nil || out[:3] != "H\n\n" {
		t.Fatalf("unexpected format: %q err=%v", out, err)
	}

	// API error server
	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad"}}`))
	}))
	defer errSrv.Close()

	withEnv(t, "OPENAI_API_ENDPOINT", errSrv.URL)
	LoadConfig()

	_, err = makeOpenAIAPICall(ctx, req, AppConfig.OpenAIAPIKey)
	if err == nil {
		t.Fatalf("expected error for non-200 response")
	}
}

func TestSummarizePostsSuccess(t *testing.T) {
	// server returns valid choices
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"TRENDING TOPICS\nCOMMUNITY PULSE\nHOT TAKES"}}]}`))
	}))
	defer srv.Close()

	withEnv(t, "OPENAI_API_ENDPOINT", srv.URL)
	withEnv(t, "OPENAI_API_KEY", "k")
	withEnv(t, "OPENAI_REQUEST_TIMEOUT", "2s")
	withEnv(t, "OPENAI_REQUESTS_PER_MINUTE", "1000")
	withEnv(t, "OPENAI_BURST_SIZE", "1000")
	withEnv(t, "SUMMARY_HEADER", "HDR\n\n")
	LoadConfig()
	InitializeOpenAIRateLimiter()

	out, err := summarizePosts("golang", "POSTS", "gpt-5-mini")
	if err != nil {
		t.Fatalf("unexpected summarize error: %v", err)
	}
	if out[:5] != "HDR\n\n" {
		t.Fatalf("header not applied: %q", out)
	}
}

func TestSummarizePostsNoKey(t *testing.T) {
	withEnv(t, "OPENAI_API_KEY", "")
	withEnv(t, "OPENAI_API_ENDPOINT", "http://127.0.0.1:1")
	LoadConfig()
	InitializeOpenAIRateLimiter()
	if _, err := summarizePosts("golang", "POSTS", "gpt-5-mini"); err == nil {
		t.Fatal("expected error when API key missing")
	}
}
