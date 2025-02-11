package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Constants for rate limiting and API endpoints
const (
	// API URLs
	redditBaseURL = "https://oauth.reddit.com"
	redditAuthURL = "https://www.reddit.com/api/v1/access_token"

	// Default parameters
	defaultPostLimit    = 7
	defaultCommentLimit = 7
	defaultTimeFrame    = "day"
)

var (
	cachedToken     string
	tokenExpiration time.Time
)

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

// makeRequest handles HTTP requests with rate limiting and common error handling
func makeRequest(req *http.Request) (*http.Response, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// getRedditAccessToken obtains an OAuth token for Reddit API access, with caching
func getRedditAccessToken() (string, error) {
	// Check if cached token is still valid
	if time.Now().Before(tokenExpiration) && cachedToken != "" {
		log.Printf("INFO: Using cached Reddit access token, expires in %v", time.Until(tokenExpiration))
		return cachedToken, nil
	}

	log.Printf("INFO: Requesting new Reddit access token")

	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("missing REDDIT_CLIENT_ID or REDDIT_CLIENT_SECRET environment variables")
	}

	data := strings.NewReader("grant_type=client_credentials")
	req, err := http.NewRequest("POST", redditAuthURL, data)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := makeRequest(req)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %v", err)
	}
	defer resp.Body.Close()

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return "", fmt.Errorf("failed to parse access token response: %v", err)
	}

	// Cache the token with expiration time
	cachedToken = tokenResponse.AccessToken
	tokenExpiration = time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)

	log.Printf("INFO: Reddit access token obtained successfully, expires in %d seconds", tokenResponse.ExpiresIn)
	return cachedToken, nil
}

// fetchTopPosts retrieves the top posts from a specified subreddit
func fetchTopPosts(subreddit, token string) ([]RedditPost, error) {
	log.Printf("INFO: Fetching top posts for subreddit: %s", subreddit)

	agent := os.Getenv("REDDIT_USER_AGENT")
	if agent == "" {
		return nil, fmt.Errorf("REDDIT_USER_AGENT environment variable is not set")
	}

	url := fmt.Sprintf("%s/r/%s/top?limit=%d&t=%s", redditBaseURL, subreddit, defaultPostLimit, defaultTimeFrame)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", agent)

	resp, err := makeRequest(req)
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
func fetchTopComments(permalink, token string) ([]string, error) {
	log.Printf("INFO: Fetching top comments for post: %s", permalink)

	agent := os.Getenv("REDDIT_USER_AGENT")
	url := fmt.Sprintf("%s%s.json?limit=%d", redditBaseURL, permalink, 100)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", agent)

	resp, err := makeRequest(req)
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

		// Take top comments based on defaultCommentLimit
		for i := 0; i < len(allComments) && i < defaultCommentLimit; i++ {
			topComments = append(topComments, allComments[i].Body)
		}
	}

	log.Printf("INFO: Successfully fetched %d top comments: %s", len(topComments), permalink)
	return topComments, nil
}

// subredditData aggregates data from a subreddit including posts and their top comments
func subredditData(subreddit, token string) (string, error) {

	output := ""
	posts, err := fetchTopPosts(subreddit, token)
	if err != nil {
		return "", fmt.Errorf("failed to fetch posts: %v", err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, post := range posts {
		mu.Lock()
		output += fmt.Sprintf("Post %d: %s\n", i+1, post.Title)
		output += fmt.Sprintf("Upvotes: %d\n", post.Ups)
		if post.Selftext != "" {
			output += fmt.Sprintf("Content: %s\n", post.Selftext)
		}
		output += fmt.Sprintln("Top Comments:")
		mu.Unlock()

		wg.Add(1)
		go func(post RedditPost, index int) {
			defer wg.Done()

			topComments, err := fetchTopComments(post.Permalink, token)
			if err != nil {
				log.Printf("WARNING: Failed to fetch comments for post %d: %v", index+1, err)
				return
			}

			mu.Lock()
			for j, comment := range topComments {
				output += fmt.Sprintf("\t%d. %s\n", j+1, comment)
			}
			mu.Unlock()
		}(post, i)
	}

	wg.Wait()
	return output, nil
}
