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

	"golang.org/x/time/rate"
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

	// Rate limiting
	requestsPerSecond = 1
	burstSize         = 5

	// Token caching
	tokenExpiryBuffer = 5 * time.Minute
	tokenFilePath     = "reddit_token.json"
)

var (
	// Token caching
	tokenMutex      sync.RWMutex
	cachedToken     string
	tokenExpiration time.Time

	// Rate limiter
	redditLimiter = rate.NewLimiter(rate.Limit(requestsPerSecond), burstSize)

	// User agent for Reddit API requests
	redditUserAgent = getEnvOrDefault("REDDIT_USER_AGENT", "SubTrends/1.0")
)

// TokenData represents the structure of the token file
type TokenData struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// RedditTokenResponse represents the OAuth token response from Reddit
type RedditTokenResponse struct {
	AccessToken string        `json:"access_token"`
	ExpiresIn   time.Duration `json:"expires_in"`
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
	// Apply rate limiting
	ctx := req.Context()
	log.Printf("INFO: Waiting for rate limiter before making request to: %s %s", req.Method, req.URL.String())
	if err := redditLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// Set a timeout for the request
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	log.Printf("INFO: Sending request: %s %s", req.Method, req.URL.String())
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("ERROR: Request failed: %s %s - %v", req.Method, req.URL.String(), err)
		return nil, fmt.Errorf("request failed: %w", err)
	}

	log.Printf("INFO: Received response: %s %s - Status: %d", req.Method, req.URL.String(), resp.StatusCode)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		log.Printf("ERROR: Unexpected status code: %s %s - Status: %d - Body: %s", req.Method, req.URL.String(), resp.StatusCode, string(body))
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// saveTokenToFile saves the token and its expiration time to a file
func saveTokenToFile(token string, expiresIn time.Duration) error {
	tokenData := TokenData{
		AccessToken: token,
		ExpiresAt:   time.Now().Add(time.Second * expiresIn),
	}

	data, err := json.MarshalIndent(tokenData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %w", err)
	}

	if err := os.WriteFile(tokenFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	log.Printf("INFO: Token saved to file, expires at %v", tokenData.ExpiresAt)
	return nil
}

// readTokenFromFile attempts to read the token from the file
func readTokenFromFile() (string, error) {
	data, err := os.ReadFile(tokenFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read token file: %w", err)
	}

	var tokenData TokenData
	if err := json.Unmarshal(data, &tokenData); err != nil {
		return "", fmt.Errorf("failed to unmarshal token data: %w", err)
	}

	// Check if token is expired or about to expire
	if time.Now().Add(tokenExpiryBuffer).After(tokenData.ExpiresAt) {
		return "", nil
	}

	log.Printf("INFO: Token loaded from file, expires at %v", tokenData.ExpiresAt)
	return tokenData.AccessToken, nil
}

// getRedditAccessToken obtains an OAuth token for Reddit API access, with caching and file persistence
func getRedditAccessToken() (string, error) {
	// First try to read from file
	token, err := readTokenFromFile()
	if err != nil {
		log.Printf("WARNING: Failed to read token from file: %v", err)
	} else if token != "" {
		return token, nil
	}

	// Check if cached token is still valid (with buffer time)
	tokenMutex.RLock()
	if time.Now().Add(tokenExpiryBuffer).Before(tokenExpiration) && cachedToken != "" {
		token := cachedToken
		tokenMutex.RUnlock()
		log.Printf("INFO: Using cached Reddit access token, expires in %v", time.Until(tokenExpiration))
		return token, nil
	}
	tokenMutex.RUnlock()

	// Need to get a new token
	tokenMutex.Lock()
	defer tokenMutex.Unlock()

	// Double-check after acquiring write lock
	if time.Now().Add(tokenExpiryBuffer).Before(tokenExpiration) && cachedToken != "" {
		log.Printf("INFO: Using cached Reddit access token, expires in %v", time.Until(tokenExpiration))
		return cachedToken, nil
	}

	log.Printf("INFO: Requesting new Reddit access token")

	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return "", ErrMissingEnvVar("REDDIT_CLIENT_ID or REDDIT_CLIENT_SECRET")
	}

	data := strings.NewReader("grant_type=client_credentials")
	req, err := http.NewRequest("POST", redditAuthURL, data)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("User-Agent", redditUserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := makeRequest(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp RedditTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token received")
	}

	// Cache the token in memory
	cachedToken = tokenResp.AccessToken
	tokenExpiration = time.Now().Add(time.Second * tokenResp.ExpiresIn)

	// Save token to file
	if err := saveTokenToFile(tokenResp.AccessToken, tokenResp.ExpiresIn); err != nil {
		log.Printf("WARNING: Failed to save token to file: %v", err)
	}

	log.Printf("INFO: New Reddit token acquired, expires in %v", tokenResp.ExpiresIn*time.Second)
	return cachedToken, nil
}

// fetchTopPosts fetches top posts from a subreddit
func fetchTopPosts(subreddit, token string) ([]RedditPost, error) {
	if subreddit == "" {
		return nil, fmt.Errorf("subreddit name is required")
	}

	// Clean subreddit name (remove r/ prefix if present)
	subreddit = strings.TrimPrefix(subreddit, "r/")

	log.Printf("INFO: Fetching top %d posts from r/%s for time frame: %s", defaultPostLimit, subreddit, defaultTimeFrame)

	url := fmt.Sprintf("%s/r/%s/top?t=%s&limit=%d", redditBaseURL, subreddit, defaultTimeFrame, defaultPostLimit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", redditUserAgent)

	resp, err := makeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch posts: %w", err)
	}
	defer resp.Body.Close()

	var redditResp RedditResponse
	if err := json.NewDecoder(resp.Body).Decode(&redditResp); err != nil {
		return nil, fmt.Errorf("failed to decode posts response: %w", err)
	}

	posts := make([]RedditPost, 0, len(redditResp.Data.Children))
	for _, child := range redditResp.Data.Children {
		posts = append(posts, child.Data)
	}

	if len(posts) == 0 {
		return nil, fmt.Errorf("no posts found in r/%s", subreddit)
	}

	log.Printf("INFO: Successfully fetched %d posts from r/%s", len(posts), subreddit)
	return posts, nil
}

// fetchTopComments fetches top comments for a post
func fetchTopComments(permalink, token string) ([]string, error) {
	if permalink == "" {
		return nil, fmt.Errorf("permalink is required")
	}

	// Ensure permalink starts with /
	if !strings.HasPrefix(permalink, "/") {
		permalink = "/" + permalink
	}

	// Remove trailing slash if present
	permalink = strings.TrimSuffix(permalink, "/")

	log.Printf("INFO: Fetching top %d comments for post: %s", defaultCommentLimit, permalink)

	url := fmt.Sprintf("%s%s.json?limit=%d", redditBaseURL, permalink, defaultCommentLimit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", redditUserAgent)

	resp, err := makeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch comments: %w", err)
	}
	defer resp.Body.Close()

	var commentData []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&commentData); err != nil {
		return nil, fmt.Errorf("failed to decode comments response: %w", err)
	}

	if len(commentData) < 2 {
		return nil, fmt.Errorf("unexpected comment data format")
	}

	// Extract comments from the second element which contains the comments
	commentsRaw, ok := commentData[1].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid comment data format")
	}

	commentsData, ok := commentsRaw["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid comment data structure")
	}

	children, ok := commentsData["children"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid children data structure")
	}

	comments := make([]string, 0, len(children))
	for _, child := range children {
		childMap, ok := child.(map[string]interface{})
		if !ok {
			continue
		}

		childData, ok := childMap["data"].(map[string]interface{})
		if !ok {
			continue
		}

		body, ok := childData["body"].(string)
		if !ok || body == "" {
			continue
		}

		comments = append(comments, body)
	}

	log.Printf("INFO: Successfully fetched %d comments for post: %s", len(comments), permalink)
	return comments, nil
}

// subredditData fetches data from a subreddit and formats it for summarization
func subredditData(subreddit, token string) (string, error) {
	log.Printf("INFO: Starting data collection for subreddit: r/%s", strings.TrimPrefix(subreddit, "r/"))

	posts, err := fetchTopPosts(subreddit, token)
	if err != nil {
		return "", fmt.Errorf("failed to fetch posts: %w", err)
	}

	var builder strings.Builder
	cleanSubredditName := strings.TrimPrefix(subreddit, "r/")
	builder.WriteString(fmt.Sprintf("# Top posts from r/%s\n\n", cleanSubredditName))

	// Process each post with a limit on concurrent requests
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 3) // Limit concurrent requests
	commentsMutex := sync.Mutex{}
	postsWithComments := make(map[int][]string)
	errChan := make(chan error, len(posts))

	log.Printf("INFO: Fetching comments for %d posts with max concurrency of 3", len(posts))

	for i, post := range posts {
		wg.Add(1)
		go func(i int, post RedditPost) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			log.Printf("INFO: Processing post %d: %s", i+1, post.Title)
			comments, err := fetchTopComments(post.Permalink, token)
			if err != nil {
				errChan <- fmt.Errorf("failed to fetch comments for post %d: %w", i, err)
				return
			}

			commentsMutex.Lock()
			postsWithComments[i] = comments
			commentsMutex.Unlock()
			log.Printf("INFO: Completed processing post %d with %d comments", i+1, len(comments))
		}(i, post)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		log.Printf("WARNING: %v", err)
	}

	log.Printf("INFO: Formatting data for %d posts from r/%s", len(posts), cleanSubredditName)

	// Format posts and comments
	for i, post := range posts {
		builder.WriteString(fmt.Sprintf("## Post %d: %s\n", i+1, post.Title))
		builder.WriteString(fmt.Sprintf("Upvotes: %d\n\n", post.Ups))

		if post.Selftext != "" {
			// Include full post content without truncation
			builder.WriteString(fmt.Sprintf("Content:\n%s\n\n", post.Selftext))
		}

		// Add comments if available
		comments, ok := postsWithComments[i]
		if ok && len(comments) > 0 {
			builder.WriteString("Top Comments:\n")
			for j, comment := range comments {
				if j >= defaultCommentLimit {
					break
				}

				// Include full comment without truncation
				builder.WriteString(fmt.Sprintf("- %s\n", comment))
			}
			builder.WriteString("\n")
		}

		// Add a separator between posts for better readability
		if i < len(posts)-1 {
			builder.WriteString("----------------------------\n\n")
		}
	}

	log.Printf("INFO: Completed data collection for r/%s with %d posts", cleanSubredditName, len(posts))
	return builder.String(), nil
}
