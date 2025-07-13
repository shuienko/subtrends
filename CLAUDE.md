# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

SubTrends is a Go web application that analyzes Reddit subreddits and provides AI-powered summaries using Anthropic's Claude API. It fetches top posts and comments from specified subreddits and generates engaging summaries with trending topics, community pulse, and hot takes.

## Development Commands

### Local Development
```bash
# Install dependencies
go mod tidy

# Run the application locally
go run .

# Build for current platform
go build -o web .

# Build for specific platform (e.g., Linux)
GOOS=linux GOARCH=amd64 go build -o web .
```

### Docker
```bash
# Build Docker image
docker build -t subtrends .

# Run with Docker
docker run -p 8080:8080 \
  -e REDDIT_CLIENT_ID=your_reddit_client_id \
  -e REDDIT_CLIENT_SECRET=your_reddit_client_secret \
  -e ANTHROPIC_API_KEY=your_anthropic_api_key \
  subtrends
```

## Required Environment Variables

For the application to function properly, these environment variables must be set:

- `REDDIT_CLIENT_ID` - Reddit API client ID
- `REDDIT_CLIENT_SECRET` - Reddit API client secret  
- `ANTHROPIC_API_KEY` - Anthropic Claude API key

Optional environment variables with defaults:
- `PORT=8080` - Server port
- `SESSION_SECRET=your-secret-key-change-in-production` - Session encryption key
- `STATIC_FILES_PATH=./static` - Path to static files
- `TEMPLATE_PATH=./templates` - Path to HTML templates
- `HISTORY_FILE_PATH=data/subreddit_history.txt` - History file location
- `SHUTDOWN_TIMEOUT_SECONDS=5` - Graceful shutdown timeout

## Architecture

The application follows a modular Go structure with clear separation of concerns:

### Core Components

- **main.go**: Application entry point with configuration loading and graceful shutdown
- **web.go**: HTTP server using Gin framework, session management, and route handlers
- **reddit.go**: Reddit API integration with OAuth token management and rate limiting
- **anthropic.go**: Anthropic Claude API client for generating AI summaries
- **utils.go**: Utility functions for environment variable handling

### Key Features

- **Session Management**: Uses Gorilla Sessions with cookie store for user state
- **Rate Limiting**: Built-in protection for both Reddit (1 req/sec, burst 5) and Anthropic APIs (10 req/min, burst 3)
- **Token Caching**: Reddit OAuth tokens are cached in memory and persisted to `data/reddit_token.json`
- **Concurrent Processing**: Posts and comments are fetched concurrently with semaphore limiting (max 3 concurrent requests)
- **Graceful Shutdown**: Proper context handling and timeout-based shutdown

### API Integration

**Reddit API Flow**:
1. Obtain OAuth token using client credentials
2. Fetch top posts from specified subreddit (default: 7 posts from "day" timeframe)
3. Fetch top comments for each post (default: 7 comments per post)
4. Aggregate data for AI summarization

**Anthropic API Flow**:
1. Format Reddit data with subreddit-specific prompt template
2. Send to Claude with configurable model (haiku3, haiku35, sonnet4)
3. Return formatted summary with trending topics, community pulse, and hot takes

### Available Models

The application supports three Claude models:
- `haiku3` (claude-3-haiku-20240307) - Fast and efficient, default model
- `haiku35` (claude-3-5-haiku-latest) - Balanced performance and capabilities  
- `sonnet4` (claude-sonnet-4-0) - Most capable for complex tasks

### Data Flow

1. User submits subreddit name via web form
2. Server validates input and manages user session
3. Reddit API fetches top posts and comments concurrently
4. Data is formatted and sent to Anthropic API for summarization
5. AI-generated summary is returned with post links
6. Analysis is saved to user's session history

### File Structure

- `/templates/` - HTML templates (index.html, history.html, model.html)
- `/static/css/` - Stylesheets
- `/static/js/` - JavaScript for frontend interactions
- `/data/` - Runtime data directory for token cache and history

## Error Handling

The application includes comprehensive error handling:
- Custom `EnvVarError` type for missing environment variables
- Graceful handling of API failures with user-friendly error messages
- Rate limiting with context-aware waiting
- Token refresh logic with file persistence fallback