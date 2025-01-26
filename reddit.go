package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Constants for rate limiting
const (
	// Reddit API allows 60 requests per minute
	requestsPerMinute = 60
	// Calculate minimum time between requests
	minTimeBetweenRequests = time.Minute / requestsPerMinute
)

// RateLimiter handles API request timing
type RateLimiter struct {
	lastRequest time.Time
}

// RedditTokenResponse represents the OAuth token response from Reddit
type RedditTokenResponse struct {
	AccessToken string `json:"access_token"`
}

// RedditPost represents a Reddit post with essential fields
type RedditPost struct {
	Title     string `json:"title"`
	Ups       int    `json:"ups"`
	Selftext  string `json:"selftext"`
	Permalink string `json:"permalink"`
}

// RedditResponse represents the full response from Reddit's post listing API
type RedditResponse struct {
	Data struct {
		Children []struct {
			Data RedditPost `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

// RedditComment represents the comment response structure from Reddit
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

// waitForRateLimit ensures we don't exceed Reddit's rate limits
func (rl *RateLimiter) waitForRateLimit() {
	elapsed := time.Since(rl.lastRequest)
	if elapsed < minTimeBetweenRequests {
		time.Sleep(minTimeBetweenRequests - elapsed)
	}
	rl.lastRequest = time.Now()
}

// makeRequest handles HTTP requests with rate limiting and common error handling
func (rl *RateLimiter) makeRequest(req *http.Request) (*http.Response, error) {
	rl.waitForRateLimit()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}

	// Handle rate limiting response codes
	if resp.StatusCode == 429 {
		// If we hit the rate limit, wait for a minute and retry
		log.Printf("Rate limit hit, waiting for 1 minute before retry")
		time.Sleep(time.Minute)
		return rl.makeRequest(req)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// getRedditAccessToken obtains an OAuth token for Reddit API access
func getRedditAccessToken(rl *RateLimiter) (string, error) {
	log.Printf("INFO: Requesting Reddit access token")

	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("missing REDDIT_CLIENT_ID or REDDIT_CLIENT_SECRET environment variables")
	}

	data := strings.NewReader("grant_type=client_credentials")
	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", data)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := rl.makeRequest(req)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %v", err)
	}
	defer resp.Body.Close()

	var tokenResponse RedditTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return "", fmt.Errorf("failed to parse access token response: %v", err)
	}

	log.Printf("INFO: Reddit access token obtained successfully")
	return tokenResponse.AccessToken, nil
}

// fetchTopPosts retrieves the top posts from a specified subreddit
func fetchTopPosts(rl *RateLimiter, subreddit, token string) ([]RedditPost, error) {
	log.Printf("INFO: Fetching top posts for subreddit: %s", subreddit)

	agent := os.Getenv("REDDIT_USER_AGENT")
	if agent == "" {
		return nil, fmt.Errorf("REDDIT_USER_AGENT environment variable is not set")
	}

	url := fmt.Sprintf("https://oauth.reddit.com/r/%s/top?limit=7&t=day", subreddit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", agent)

	resp, err := rl.makeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch top posts: %v", err)
	}
	defer resp.Body.Close()

	var redditResponse RedditResponse
	if err := json.NewDecoder(resp.Body).Decode(&redditResponse); err != nil {
		return nil, fmt.Errorf("failed to parse top posts response: %v", err)
	}

	var posts []RedditPost
	for _, child := range redditResponse.Data.Children {
		posts = append(posts, child.Data)
	}

	log.Printf("INFO: Successfully fetched %d top posts", len(posts))
	return posts, nil
}

// fetchTopComments retrieves the top comments for a specific post
func fetchTopComments(rl *RateLimiter, permalink, token string) ([]string, error) {
	log.Printf("INFO: Fetching top comments for post: %s", permalink)

	agent := os.Getenv("REDDIT_USER_AGENT")
	url := fmt.Sprintf("https://oauth.reddit.com%s.json?limit=100", permalink)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", agent)

	resp, err := rl.makeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch comments: %v", err)
	}
	defer resp.Body.Close()

	var comments []RedditComment
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return nil, fmt.Errorf("failed to parse comments response: %v", err)
	}

	var topComments []string
	if len(comments) > 1 {
		var allComments []struct {
			Body string
			Ups  int
		}

		// Extract all non-empty comments
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

		// Take top 7 comments
		for i := 0; i < len(allComments) && i < 7; i++ {
			topComments = append(topComments, allComments[i].Body)
		}
	}

	log.Printf("INFO: Successfully fetched %d top comments", len(topComments))
	return topComments, nil
}

// subredditData aggregates data from a subreddit including posts and their top comments
func subredditData(subreddit, token string) (string, error) {
	rl := &RateLimiter{
		lastRequest: time.Now().Add(-minTimeBetweenRequests), // Initialize to allow immediate first request
	}

	output := ""
	posts, err := fetchTopPosts(rl, subreddit, token)
	if err != nil {
		return "", fmt.Errorf("failed to fetch posts: %v", err)
	}

	for i, post := range posts {
		output += fmt.Sprintf("Post %d: %s\n", i+1, post.Title)
		output += fmt.Sprintf("Upvotes: %d\n", post.Ups)
		if post.Selftext != "" {
			output += fmt.Sprintf("Content: %s\n", post.Selftext)
		}

		topComments, err := fetchTopComments(rl, post.Permalink, token)
		if err != nil {
			log.Printf("WARNING: Failed to fetch comments for post %d: %v", i+1, err)
			continue
		}

		output += fmt.Sprintln("Top Comments:")
		for j, comment := range topComments {
			output += fmt.Sprintf("\t%d. %s\n", j+1, comment)
		}
	}
	return output, nil
}
