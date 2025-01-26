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

type RedditTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type RedditPost struct {
	Title     string `json:"title"`
	Ups       int    `json:"ups"`
	Selftext  string `json:"selftext"`
	Permalink string `json:"permalink"`
}

type RedditResponse struct {
	Data struct {
		Children []struct {
			Data RedditPost `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type RedditComment struct {
	Data struct {
		Children []struct {
			Data struct {
				Body string `json:"body"`
				Ups  int    `json:"ups"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

type AnthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func summarizePosts(text string) (string, error) {
	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = "claude-3-haiku-20240307" // default model if not specified
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY environment variable is not set")
	}

	request := AnthropicRequest{
		Model: model,
		Messages: []Message{
			{
				Role: "user",
				Content: fmt.Sprintf(
					`Please provide a concise summary of these Reddit posts and discussions. 
Focus on:
- Main themes and topics
- Key points from popular comments
- Notable trends or patterns
- Overall community sentiment

Posts to analyze:\n\n%s`,
					text,
				),
			},
		},
		MaxTokens:   1000,
		Temperature: 0.7,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	var response AnthropicResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("error unmarshaling response: %v", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf("API error: %s", response.Error.Message)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	output := "=== Claude's Summary ===\n"
	output += strings.TrimSpace(response.Content[0].Text)
	return output, nil
}

func getRedditAccessToken() string {
	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		log.Fatal("Missing REDDIT_CLIENT_ID or REDDIT_CLIENT_SECRET environment variables")
	}

	data := strings.NewReader("grant_type=client_credentials")
	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", data)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to get access token: %v", err)
	}
	defer resp.Body.Close()

	var tokenResponse RedditTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		log.Fatalf("Failed to parse access token response: %v", err)
	}

	return tokenResponse.AccessToken
}

func fetchTopPosts(subreddit, token string) []RedditPost {
	url := fmt.Sprintf("https://oauth.reddit.com/r/%s/top?limit=7&t=day", subreddit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "golang:reddit_top_posts:v1.0 (by /u/yourusername)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to fetch top posts: %v", err)
	}
	defer resp.Body.Close()

	var redditResponse RedditResponse
	if err := json.NewDecoder(resp.Body).Decode(&redditResponse); err != nil {
		log.Fatalf("Failed to parse top posts response: %v", err)
	}

	var posts []RedditPost
	for _, child := range redditResponse.Data.Children {
		posts = append(posts, child.Data)
	}

	return posts
}

func fetchTopComments(permalink, token string) []string {
	url := fmt.Sprintf("https://oauth.reddit.com%s.json?limit=100", permalink)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "golang:reddit_top_posts:v1.0 (by /u/yourusername)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to fetch comments: %v", err)
	}
	defer resp.Body.Close()

	var comments []RedditComment
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		log.Fatalf("Failed to parse comments response: %v", err)
	}

	var topComments []string
	if len(comments) > 1 {
		var allComments []struct {
			Body string
			Ups  int
		}
		for _, child := range comments[1].Data.Children {
			if child.Data.Body != "" {
				allComments = append(allComments, struct {
					Body string
					Ups  int
				}{
					Body: child.Data.Body,
					Ups:  child.Data.Ups,
				})
			}
		}

		// Sort comments by upvotes in descending order
		for i := 0; i < len(allComments)-1; i++ {
			for j := 0; j < len(allComments)-i-1; j++ {
				if allComments[j].Ups < allComments[j+1].Ups {
					allComments[j], allComments[j+1] = allComments[j+1], allComments[j]
				}
			}
		}

		for i := 0; i < len(allComments) && i < 7; i++ {
			topComments = append(topComments, allComments[i].Body)
		}
	}

	return topComments
}

func subredditData(subreddit, token string) string {
	output := ""
	posts := fetchTopPosts(subreddit, token)

	for i, post := range posts {
		output += fmt.Sprintf("Post %d: %s\n", i+1, post.Title)
		output += fmt.Sprintf("Upvotes: %d\n", post.Ups)
		if post.Selftext != "" {
			output += fmt.Sprintf("Content: %s\n", post.Selftext)
		}
		topComments := fetchTopComments(post.Permalink, token)
		output += fmt.Sprintln("Top Comments:")
		for j, comment := range topComments {
			output += fmt.Sprintf("\t%d. %s\n", j+1, comment)
		}
	}
	return output
}

func main() {
	fmt.Print("Enter subreddit name: ")
	var subreddit string
	fmt.Scanln(&subreddit)

	token := getRedditAccessToken()
	data := subredditData(subreddit, token)
	summary, err := summarizePosts(data)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(summary)
	}
}
