package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ProgressMessage represents a progress update sent to the client
type ProgressMessage struct {
	Type           string `json:"type"`
	Stage          string `json:"stage"`
	Progress       int    `json:"progress"`
	Message        string `json:"message"`
	EstimatedTime  int    `json:"estimated_time,omitempty"`
	Error          string `json:"error,omitempty"`
	Data           interface{} `json:"data,omitempty"`
}

// AnalysisStage represents different stages of the analysis process
type AnalysisStage string

const (
	StageConnecting   AnalysisStage = "connecting"
	StageFetchingPosts AnalysisStage = "fetching_posts"
	StageFetchingComments AnalysisStage = "fetching_comments"
	StageGeneratingSummary AnalysisStage = "generating_summary"
	StageComplete     AnalysisStage = "complete"
	StageError        AnalysisStage = "error"
)

// ProgressTracker manages progress updates for analysis
type ProgressTracker struct {
	conn         *websocket.Conn
	mutex        sync.Mutex
	currentStage AnalysisStage
	progress     int
	startTime    time.Time
	estimatedTotal time.Duration
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from same origin
		return true
	},
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(conn *websocket.Conn) *ProgressTracker {
	return &ProgressTracker{
		conn:      conn,
		startTime: time.Now(),
		estimatedTotal: 30 * time.Second, // Default estimate
	}
}

// SendProgress sends a progress update to the client
func (pt *ProgressTracker) SendProgress(stage AnalysisStage, progress int, message string) error {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	pt.currentStage = stage
	pt.progress = progress

	// Reset write deadline before sending
	pt.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))

	elapsed := time.Since(pt.startTime)
	var estimatedTime int
	if progress > 0 && progress < 100 {
		totalEstimated := time.Duration(float64(elapsed) * 100.0 / float64(progress))
		remaining := totalEstimated - elapsed
		estimatedTime = int(remaining.Seconds())
	}

	progressMsg := ProgressMessage{
		Type:          "progress",
		Stage:         string(stage),
		Progress:      progress,
		Message:       message,
		EstimatedTime: estimatedTime,
	}

	log.Printf("Sending progress: %d%% - %s", progress, message)
	return pt.conn.WriteJSON(progressMsg)
}

// SendError sends an error message to the client
func (pt *ProgressTracker) SendError(err error) error {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	progressMsg := ProgressMessage{
		Type:  "error",
		Stage: string(StageError),
		Error: err.Error(),
	}

	return pt.conn.WriteJSON(progressMsg)
}

// SendComplete sends completion message with results
func (pt *ProgressTracker) SendComplete(data interface{}) error {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	// Reset write deadline before sending
	pt.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))

	progressMsg := ProgressMessage{
		Type:     "complete",
		Stage:    string(StageComplete),
		Progress: 100,
		Message:  "Analysis complete!",
		Data:     data,
	}

	log.Printf("Sending completion message with data")
	return pt.conn.WriteJSON(progressMsg)
}

// Close closes the WebSocket connection
func (pt *ProgressTracker) Close() error {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()
	
	return pt.conn.Close()
}

// handleWebSocket handles WebSocket connections for real-time analysis
func (ws *WebServer) handleWebSocket(c *gin.Context) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade to WebSocket: %v", err)
		return
	}
	defer conn.Close()

	// Set connection timeouts - longer timeouts for analysis
	conn.SetReadDeadline(time.Now().Add(300 * time.Second))  // 5 minutes for read
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))   // 30 seconds for write

	// Create progress tracker
	tracker := NewProgressTracker(conn)
	defer tracker.Close()

	// Wait for analysis request
	var request struct {
		Subreddit string `json:"subreddit"`
	}

	if err := conn.ReadJSON(&request); err != nil {
		log.Printf("Failed to read WebSocket message: %v", err)
		tracker.SendError(err)
		return
	}

	// Validate subreddit name
	subreddit := strings.TrimPrefix(strings.TrimSpace(request.Subreddit), "r/")
	if subreddit == "" {
		tracker.SendError(fmt.Errorf("subreddit name is required"))
		return
	}

	// Create simplified session data for WebSocket (we can't access HTTP sessions in WebSocket)
	sessionData := &Session{
		UserID:    generateUserID(),
		History:   make([]string, 0),
		Model:     availableModels[0].Name, // Use default model
		CreatedAt: time.Now(),
	}

	// Start analysis with progress tracking
	if err := ws.analyzeWithProgress(subreddit, sessionData, tracker); err != nil {
		log.Printf("Analysis failed: %v", err)
		tracker.SendError(err)
		return
	}
}

// analyzeWithProgress performs subreddit analysis with real-time progress updates
func (ws *WebServer) analyzeWithProgress(subreddit string, sessionData *Session, tracker *ProgressTracker) error {
	// Stage 1: Connecting to Reddit
	tracker.SendProgress(StageConnecting, 5, "Connecting to Reddit...")
	
	token, err := getRedditAccessToken()
	if err != nil {
		return fmt.Errorf("failed to connect to Reddit: %w", err)
	}

	// Stage 2: Fetching posts
	tracker.SendProgress(StageFetchingPosts, 20, "Fetching top posts...")
	
	posts, err := fetchTopPosts(subreddit, token)
	if err != nil {
		return fmt.Errorf("failed to fetch posts: %w", err)
	}

	// Stage 3: Fetching comments
	tracker.SendProgress(StageFetchingComments, 40, fmt.Sprintf("Loading comments for %d posts...", len(posts)))
	
	data, err := subredditDataWithProgress(subreddit, token, tracker)
	if err != nil {
		return fmt.Errorf("failed to fetch comments: %w", err)
	}

	// Stage 4: Generating summary
	tracker.SendProgress(StageGeneratingSummary, 80, "Sending data to AI model...")
	
	summary, err := summarizePostsWithProgress(data, sessionData.Model, tracker)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	// Add to history
	ws.historyMutex.Lock()
	found := false
	for _, existing := range sessionData.History {
		if strings.EqualFold(existing, subreddit) {
			found = true
			break
		}
	}
	if !found {
		sessionData.History = append(sessionData.History, subreddit)
	}
	ws.historyMutex.Unlock()

	// Note: Session saving will be handled in the HTTP fallback
	// WebSocket connections don't have direct access to Gin context for session saving

	// Stage 5: Complete
	log.Printf("Preparing completion response for subreddit: %s", subreddit)
	response := gin.H{
		"summary":   summary,
		"posts":     posts,
		"subreddit": subreddit,
	}

	log.Printf("Sending completion message via WebSocket")
	if err := tracker.SendComplete(response); err != nil {
		log.Printf("Failed to send completion message: %v", err)
		return err
	}
	
	log.Printf("WebSocket analysis completed successfully")
	return nil
}

// subredditDataWithProgress collects subreddit data with progress updates
func subredditDataWithProgress(subreddit, token string, tracker *ProgressTracker) (string, error) {
	log.Printf("INFO: Starting data collection for subreddit: r/%s", strings.TrimPrefix(subreddit, "r/"))

	posts, err := fetchTopPosts(subreddit, token)
	if err != nil {
		return "", fmt.Errorf("failed to fetch posts: %w", err)
	}

	var builder strings.Builder
	cleanSubredditName := strings.TrimPrefix(subreddit, "r/")
	builder.WriteString(fmt.Sprintf("# Top posts from r/%s\n\n", cleanSubredditName))

	// Calculate progress increments
	progressPerPost := 40 / len(posts) // 40% progress for comment fetching
	currentProgress := 40

	// Process each post with progress updates
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

			// Update progress
			tracker.SendProgress(StageFetchingComments, currentProgress+i*progressPerPost, 
				fmt.Sprintf("Processing post %d of %d: %s", i+1, len(posts), post.Title[:min(50, len(post.Title))]+"..."))

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
		if err != nil {
			return "", err
		}
	}

	// Build the final data string
	for i, post := range posts {
		builder.WriteString(fmt.Sprintf("## Post %d: %s\n", i+1, post.Title))
		builder.WriteString(fmt.Sprintf("Score: %d\n", post.Ups))
		
		if post.Selftext != "" {
			builder.WriteString(fmt.Sprintf("Content: %s\n", post.Selftext))
		}
		
		builder.WriteString("\n### Top Comments:\n")
		
		if comments, exists := postsWithComments[i]; exists {
			for j, comment := range comments {
				builder.WriteString(fmt.Sprintf("%d. %s\n", j+1, comment))
			}
		}
		builder.WriteString("\n---\n\n")
	}

	return builder.String(), nil
}

// summarizePostsWithProgress wraps the summarizePosts function with progress tracking
func summarizePostsWithProgress(text string, model string, tracker *ProgressTracker) (string, error) {
	log.Printf("Starting AI summarization with model: %s", model)
	tracker.SendProgress(StageGeneratingSummary, 85, "Processing with AI model...")
	
	// Call the original function
	summary, err := summarizePosts(text, model)
	if err != nil {
		log.Printf("AI summarization failed: %v", err)
		return "", err
	}
	
	log.Printf("AI summarization completed successfully")
	tracker.SendProgress(StageGeneratingSummary, 95, "Finalizing summary...")
	return summary, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}