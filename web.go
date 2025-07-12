package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

// Session represents user session data
type Session struct {
	UserID    string
	Username  string
	History   []string
	Model     string
	CreatedAt time.Time
}

// WebServer represents the web server with its configuration and session store
type WebServer struct {
	router       *gin.Engine
	config       *Config
	store        *sessions.CookieStore
	server       *http.Server
	stopChan     chan struct{}
	wg           sync.WaitGroup
	historyMutex sync.RWMutex
}

// Available models for selection
var availableModels = []ModelInfo{
	{
		Codename:    "haiku3",
		Name:        "claude-3-haiku-20240307",
		Description: "Fast and efficient model (default)",
	},
	{
		Codename:    "haiku35",
		Name:        "claude-3-5-haiku-latest",
		Description: "Balanced performance and capabilities",
	},
	{
		Codename:    "sonnet4",
		Name:        "claude-sonnet-4-0",
		Description: "Most capable model for complex tasks",
	},
}

// ModelInfo represents information about an available model
type ModelInfo struct {
	Codename    string
	Name        string
	Description string
}

// NewWebServer creates a new WebServer instance with the provided configuration
func NewWebServer(config *Config) (*WebServer, error) {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Create router
	router := gin.Default()

	// Create session store
	store := sessions.NewCookieStore([]byte(config.SessionSecret))

	server := &WebServer{
		router:   router,
		config:   config,
		store:    store,
		stopChan: make(chan struct{}),
	}

	// Setup routes
	server.setupRoutes()

	// Create HTTP server
	server.server = &http.Server{
		Addr:    ":" + config.Port,
		Handler: router,
	}

	return server, nil
}

// setupRoutes configures all the web routes
func (ws *WebServer) setupRoutes() {
	// Serve static files
	ws.router.Static("/static", ws.config.StaticFilesPath)

	// Load HTML templates
	ws.router.LoadHTMLGlob(filepath.Join(ws.config.TemplatePath, "*.html"))

	// Routes
	ws.router.GET("/", ws.handleHome)
	ws.router.POST("/analyze", ws.handleAnalyze)
	ws.router.GET("/history", ws.handleHistory)
	ws.router.POST("/clear-history", ws.handleClearHistory)
	ws.router.GET("/model", ws.handleModelGet)
	ws.router.POST("/model", ws.handleModelChange)
	ws.router.GET("/health", ws.handleHealth)
}

// Start begins the web server
func (ws *WebServer) Start(ctx context.Context) error {
	ws.wg.Add(1)
	defer ws.wg.Done()

	log.Printf("Starting web server on port %s", ws.config.Port)

	// Start server in a goroutine
	go func() {
		if err := ws.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for context cancellation or stop signal
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ws.stopChan:
		return nil
	}
}

// Stop gracefully stops the web server
func (ws *WebServer) Stop(ctx context.Context) error {
	log.Println("Stopping web server...")

	// Signal the server to stop
	close(ws.stopChan)

	// Shutdown the HTTP server
	if err := ws.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	// Wait for the server to stop
	done := make(chan struct{})
	go func() {
		ws.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Web server stopped successfully")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for server to stop: %w", ctx.Err())
	}
}

// getSession retrieves or creates a user session
func (ws *WebServer) getSession(c *gin.Context) *Session {
	session, _ := ws.store.Get(c.Request, "subtrends-session")

	// Get or create session data
	var sessionData Session
	if data, ok := session.Values["data"]; ok {
		if sd, ok := data.(Session); ok {
			sessionData = sd
		}
	}

	// Initialize if empty
	if sessionData.UserID == "" {
		sessionData = Session{
			UserID:    generateUserID(),
			History:   make([]string, 0, 50),
			Model:     ws.config.AnthropicModel,
			CreatedAt: time.Now(),
		}
	}

	return &sessionData
}

// saveSession saves the session data
func (ws *WebServer) saveSession(c *gin.Context, sessionData *Session) error {
	session, _ := ws.store.Get(c.Request, "subtrends-session")
	session.Values["data"] = *sessionData
	return session.Save(c.Request, c.Writer)
}

// generateUserID generates a unique user ID
func generateUserID() string {
	return fmt.Sprintf("user_%d", time.Now().UnixNano())
}

// handleHome serves the main page
func (ws *WebServer) handleHome(c *gin.Context) {
	sessionData := ws.getSession(c)

	c.HTML(http.StatusOK, "index.html", gin.H{
		"Title":   "SubTrends - Reddit Analysis",
		"UserID":  sessionData.UserID,
		"History": sessionData.History,
		"Model":   sessionData.Model,
		"Models":  availableModels,
	})
}

// handleAnalyze processes subreddit analysis requests
func (ws *WebServer) handleAnalyze(c *gin.Context) {
	subreddit := c.PostForm("subreddit")
	if subreddit == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Subreddit name is required"})
		return
	}

	// Clean subreddit name
	subreddit = strings.TrimPrefix(subreddit, "r/")

	sessionData := ws.getSession(c)

	// Add to history if not already present
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

	// Save session
	ws.saveSession(c, sessionData)

	// Get Reddit data
	token, err := getRedditAccessToken()
	if err != nil {
		log.Printf("Failed to get access token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to Reddit"})
		return
	}

	data, err := subredditData(subreddit, token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Generate summary
	summary, err := summarizePosts(data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate summary"})
		return
	}

	// Get post links
	posts, err := fetchTopPosts(subreddit, token)
	if err != nil {
		log.Printf("Failed to fetch posts for links: %v", err)
		posts = []RedditPost{} // Ensure posts is never nil
	}

	// Format response
	response := gin.H{
		"summary":   summary,
		"posts":     posts,
		"subreddit": subreddit,
	}

	c.JSON(http.StatusOK, response)
}

// handleHistory serves the user's history
func (ws *WebServer) handleHistory(c *gin.Context) {
	sessionData := ws.getSession(c)

	c.HTML(http.StatusOK, "history.html", gin.H{
		"Title":   "SubTrends - History",
		"History": sessionData.History,
	})
}

// handleClearHistory clears the user's history
func (ws *WebServer) handleClearHistory(c *gin.Context) {
	sessionData := ws.getSession(c)

	ws.historyMutex.Lock()
	sessionData.History = make([]string, 0, 50)
	ws.historyMutex.Unlock()

	ws.saveSession(c, sessionData)

	c.JSON(http.StatusOK, gin.H{"message": "History cleared successfully"})
}

// handleModelGet shows the current model
func (ws *WebServer) handleModelGet(c *gin.Context) {
	sessionData := ws.getSession(c)

	c.HTML(http.StatusOK, "model.html", gin.H{
		"Title":        "SubTrends - Model Selection",
		"CurrentModel": sessionData.Model,
		"Models":       availableModels,
	})
}

// handleModelChange changes the AI model
func (ws *WebServer) handleModelChange(c *gin.Context) {
	modelCodename := c.PostForm("model")
	if modelCodename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model codename is required"})
		return
	}

	// Validate model codename
	var selectedModel ModelInfo
	validModel := false
	for _, model := range availableModels {
		if modelCodename == model.Codename {
			validModel = true
			selectedModel = model
			break
		}
	}

	if !validModel {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid model codename"})
		return
	}

	sessionData := ws.getSession(c)
	sessionData.Model = selectedModel.Name
	ws.saveSession(c, sessionData)

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Model changed to: %s", selectedModel.Codename),
		"model":   selectedModel,
	})
}

// handleHealth serves health check endpoint
func (ws *WebServer) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}
