// Package main provides functionality to interact with the Anthropic API
// for summarizing Reddit posts using Claude AI models.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
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
	defaultMaxTokens   = 1000
	defaultTemperature = 0.7

	// Output formatting
	summaryHeader = "=== Claude's Summary ===\n"
)

// promptTemplate defines the template for the summarization request
const promptTemplate = `Please provide a concise summary of these Reddit posts and discussions. 
Focus on:
- Main themes and topics
- Key points from popular comments
- Notable trends or patterns
- Overall community sentiment

Rules:
- Don't reply with anything but summary.
- Don't reply with the summary for each post. You must cover themes, trends and key points.

Posts to analyze:

%s`

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

	// Make the API call
	response, err := makeAnthropicAPICall(request, apiKey)
	if err != nil {
		return "", fmt.Errorf("API call failed: %v", err)
	}

	// Format and return the response
	return formatResponse(response)
}

// getEnvOrDefault returns the value of an environment variable or a default value if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getRequiredEnvVar returns the value of a required environment variable or an error if not set
func getRequiredEnvVar(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s environment variable is not set", key)
	}
	return value, nil
}

// createAnthropicRequest creates a new request structure for the Anthropic API
func createAnthropicRequest(model, text string) AnthropicRequest {
	return AnthropicRequest{
		Model: model,
		Messages: []Message{
			{
				Role:    "user",
				Content: fmt.Sprintf(promptTemplate, text),
			},
		},
		MaxTokens:   defaultMaxTokens,
		Temperature: defaultTemperature,
	}
}

// makeAnthropicAPICall sends the request to the Anthropic API and returns the response
func makeAnthropicAPICall(request AnthropicRequest, apiKey string) (*AnthropicResponse, error) {
	// Marshal request to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", anthropicAPIEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERROR: Error in Anthropic API call: %v", err)
		return nil, fmt.Errorf("error reading response: %v", err)
	}
	log.Printf("INFO: Anthropic API call successful with status: %d", resp.StatusCode)

	// Parse response
	var response AnthropicResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %v", err)
	}

	// Check for API errors
	if response.Error != nil {
		return nil, fmt.Errorf("API error: %s", response.Error.Message)
	}

	return &response, nil
}

// formatResponse formats the API response into the desired output format
func formatResponse(response *AnthropicResponse) (string, error) {
	if len(response.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return summaryHeader + strings.TrimSpace(response.Content[0].Text), nil
}
