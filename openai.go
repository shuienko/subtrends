// Package main provides functionality to interact with the OpenAI API
// for summarizing Reddit posts using GPT models.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
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
	// Rate limiter for OpenAI API
	openaiLimiter *rate.Limiter
)

func InitializeOpenAIRateLimiter() {
	// Initialize the rate limiter from config
	openaiLimiter = rate.NewLimiter(rate.Every(time.Minute/time.Duration(AppConfig.OpenAIRequestsPerMinute)), AppConfig.OpenAIBurstSize)
}

// OpenAIMessage represents a single message in the conversation
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest represents the structure of a request to the OpenAI API
type ChatCompletionRequest struct {
	Model    string          `json:"model"`
	Messages []OpenAIMessage `json:"messages"`
}

// ChatCompletionResponse represents the structure of a response from the OpenAI API
type ChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message,omitempty"`
	} `json:"error,omitempty"`
}

// summarizePosts takes a string of Reddit posts and returns a summarized version using the OpenAI API
func summarizePosts(subreddit, text string, model string) (string, error) {
	log.Printf("INFO: Making OpenAI API call with model: %s", model)

	if AppConfig.OpenAIAPIKey == "" {
		return "", fmt.Errorf("OpenAI API key is not configured")
	}

	// Prepare the API request
	request := createOpenAIRequest(model, text, subreddit)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), AppConfig.OpenAIRequestTimeout)
	defer cancel()

	// Make the API call
	response, err := makeOpenAIAPICall(ctx, request, AppConfig.OpenAIAPIKey)
	if err != nil {
		return "", fmt.Errorf("API call failed: %w", err)
	}

	// Format and return the response
	return formatResponse(response)
}

// createOpenAIRequest creates a request structure for the OpenAI API
func createOpenAIRequest(model, text, subredditName string) ChatCompletionRequest {
	// Format the prompt with the Reddit data and subreddit name
	prompt := fmt.Sprintf(promptTemplate, subredditName, text)

	// Create the request structure (keep minimal; rely on server defaults)
	return ChatCompletionRequest{
		Model: model,
		Messages: []OpenAIMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}
}

// makeOpenAIAPICall sends a request to the OpenAI API and returns the response
func makeOpenAIAPICall(ctx context.Context, request ChatCompletionRequest, apiKey string) (*ChatCompletionResponse, error) {
	// Apply rate limiting
	if err := openaiLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// Marshal the request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", AppConfig.OpenAIAPIEndpoint, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: AppConfig.OpenAIRequestTimeout,
	}

	// Send the request
	startTime := time.Now()
	resp, err := client.Do(req)
	requestDuration := time.Since(startTime)

	log.Printf("INFO: OpenAI API request completed in %v", requestDuration)

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
	var response ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	// Check for API errors
	if response.Error != nil && response.Error.Message != "" {
		return nil, fmt.Errorf("API error: %s", response.Error.Message)
	}

	return &response, nil
}

// formatResponse extracts and formats the text from the OpenAI API response
func formatResponse(response *ChatCompletionResponse) (string, error) {
	if response == nil {
		return "", fmt.Errorf("nil response received")
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("empty content in response")
	}

	// Extract the text from the response
	text := response.Choices[0].Message.Content
	if text == "" {
		return "", fmt.Errorf("empty text in response content")
	}

	// Ensure proper Markdown formatting (preserve simple behavior)
	if !strings.Contains(text, "*") {
		text = strings.ReplaceAll(text, "TRENDING TOPICS", "*TRENDING TOPICS*")
		text = strings.ReplaceAll(text, "COMMUNITY PULSE", "*COMMUNITY PULSE*")
		text = strings.ReplaceAll(text, "HOT TAKES", "*HOT TAKES*")
	}

	// Format the response with a header
	return AppConfig.SummaryHeader + text, nil
}
