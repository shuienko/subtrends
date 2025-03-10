// Package main provides functionality to interact with the Anthropic API
// for summarizing Reddit posts using Claude AI models.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	// API related constants
	anthropicAPIEndpoint = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion  = "2023-06-01"
	defaultModel         = "claude-3-haiku-20240307"

	// Environment variable names
	envAnthropicAPIKey = "ANTHROPIC_API_KEY"
	envAnthropicModel  = "ANTHROPIC_MODEL"

	// Request parameters
	defaultMaxTokens   = 1500
	defaultTemperature = 0.7
	requestTimeout     = 45 * time.Second

	// Output formatting
	summaryHeader = "📱 *REDDIT PULSE* 📱\n\n"

	// Rate limiting
	anthropicRequestsPerMinute = 10
	anthropicBurstSize         = 3
)

// promptTemplate defines the template for the summarization request
const promptTemplate = `Please provide an engaging and fun summary of these Reddit posts and discussions from r/%s. 

Focus on:
- Main themes and topics; group similar topics together
- Key points from popular comments with interesting insights
- Notable trends, patterns, or controversies
- Overall community sentiment and mood

Format your response with:
- 📊 TRENDING TOPICS: List the main themes with emoji indicators
- 💬 COMMUNITY PULSE: Describe the overall sentiment and notable discussions
- 🔥 HOT TAKES: Highlight the most interesting or controversial opinions

Rules:
- Be conversational and engaging, like you're telling a friend about what's happening on Reddit
- Use appropriate emojis to make the summary more visually appealing
- Don't reply with the summary for each post individually
- Keep your tone friendly and slightly humorous where appropriate
- Organize information in a clear, scannable format with bullet points and sections

Posts to analyze:

%s`

var (
	// Rate limiter for Anthropic API
	anthropicLimiter = rate.NewLimiter(rate.Every(time.Minute/time.Duration(anthropicRequestsPerMinute)), anthropicBurstSize)
)

// Message represents a single message in the conversation with Claude
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnthropicRequest represents the structure of a request to the Anthropic API
type AnthropicRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

// AnthropicResponse represents the structure of a response from the Anthropic API
type AnthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"error,omitempty"`
	} `json:"error,omitempty"`
}

// summarizePosts takes a string of Reddit posts and returns a summarized version using the Anthropic API
func summarizePosts(text string) (string, error) {
	// Get model from environment or use default
	model := getEnvOrDefault(envAnthropicModel, defaultModel)
	log.Printf("INFO: Making Anthropic API call with model: %s", model)

	// Get API key from environment
	apiKey, err := getRequiredEnvVar(envAnthropicAPIKey)
	if err != nil {
		return "", err
	}

	// Prepare the API request
	request := createAnthropicRequest(model, text)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	// Make the API call
	response, err := makeAnthropicAPICall(ctx, request, apiKey)
	if err != nil {
		return "", fmt.Errorf("API call failed: %w", err)
	}

	// Format and return the response
	return formatResponse(response)
}

// getEnvOrDefault returns the value of an environment variable or a default value if not set

// getRequiredEnvVar returns the value of a required environment variable or an error if not set
func getRequiredEnvVar(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", ErrMissingEnvVar(key)
	}
	return value, nil
}

// createAnthropicRequest creates a request structure for the Anthropic API
func createAnthropicRequest(model, text string) AnthropicRequest {
	// Extract subreddit name from the text
	subredditName := "unknown"
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# Top posts from r/") {
			parts := strings.Split(line, "r/")
			if len(parts) > 1 {
				subredditName = strings.TrimSpace(parts[1])
				break
			}
		}
	}

	// Format the prompt with the Reddit data and subreddit name
	prompt := fmt.Sprintf(promptTemplate, subredditName, text)

	// Create the request structure
	return AnthropicRequest{
		Model: model,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   defaultMaxTokens,
		Temperature: defaultTemperature,
	}
}

// makeAnthropicAPICall sends a request to the Anthropic API and returns the response
func makeAnthropicAPICall(ctx context.Context, request AnthropicRequest, apiKey string) (*AnthropicResponse, error) {
	// Apply rate limiting
	if err := anthropicLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// Marshal the request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIEndpoint, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: requestTimeout,
	}

	// Send the request
	startTime := time.Now()
	resp, err := client.Do(req)
	requestDuration := time.Since(startTime)

	log.Printf("INFO: Anthropic API request completed in %v", requestDuration)

	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned non-200 status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse the response
	var response AnthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	// Check for API errors
	if response.Error != nil && response.Error.Message != "" {
		return nil, fmt.Errorf("API error: %s", response.Error.Message)
	}

	return &response, nil
}

// formatResponse extracts and formats the text from the Anthropic API response
func formatResponse(response *AnthropicResponse) (string, error) {
	if response == nil {
		return "", fmt.Errorf("nil response received")
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("empty content in response")
	}

	// Extract the text from the response
	text := response.Content[0].Text
	if text == "" {
		return "", fmt.Errorf("empty text in response content")
	}

	// Ensure proper Markdown formatting
	// Replace any instances of * that aren't part of Markdown formatting
	// This is a simple approach - a more robust solution would use regex
	if !strings.Contains(text, "*") {
		// If there are no asterisks, add some basic formatting
		text = strings.ReplaceAll(text, "TRENDING TOPICS", "*TRENDING TOPICS*")
		text = strings.ReplaceAll(text, "COMMUNITY PULSE", "*COMMUNITY PULSE*")
		text = strings.ReplaceAll(text, "HOT TAKES", "*HOT TAKES*")
	}

	// Format the response with a header
	return summaryHeader + text, nil
}
