# SubTrends - Reddit Analysis Web Application

A modern web application that analyzes Reddit subreddits and provides AI-powered summaries of trending topics and discussions.

## Features

- **Subreddit Analysis**: Analyze any subreddit to get AI-generated summaries of trending topics
- **Multiple AI Models**: Choose from different Claude AI models for analysis
- **User History**: Track your analysis history with session management
- **Real-time Processing**: Get instant results with progress indicators
- **Responsive Design**: Modern, mobile-friendly interface
- **Rate Limiting**: Built-in protection against API abuse

## Technology Stack

- **Backend**: Go with Gin web framework
- **Frontend**: HTML5, CSS3, JavaScript with Bootstrap 5
- **AI**: Anthropic Claude API for intelligent summarization
- **Data**: Reddit API for subreddit data
- **Session Management**: Gorilla Sessions for user state

## Quick Start

### Prerequisites

- Go 1.23 or higher
- Reddit API credentials
- Anthropic API key

### Environment Variables

```bash
# Required
REDDIT_CLIENT_ID=your_reddit_client_id
REDDIT_CLIENT_SECRET=your_reddit_client_secret
ANTHROPIC_API_KEY=your_anthropic_api_key

# Optional (with defaults)
PORT=8080
SESSION_SECRET=your-secret-key-change-in-production
STATIC_FILES_PATH=./static
TEMPLATE_PATH=./templates
HISTORY_FILE_PATH=data/subreddit_history.txt
SHUTDOWN_TIMEOUT_SECONDS=5
```

### Running Locally

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd subtrends
   ```

2. **Set environment variables**
   ```bash
   export REDDIT_CLIENT_ID=your_reddit_client_id
   export REDDIT_CLIENT_SECRET=your_reddit_client_secret
   export ANTHROPIC_API_KEY=your_anthropic_api_key
   ```

3. **Install dependencies**
   ```bash
   go mod tidy
   ```

4. **Run the application**
   ```bash
   go run .
   ```

5. **Open your browser**
   Navigate to `http://localhost:8080`

### Using Docker

1. **Build the image**
   ```bash
   docker build -t subtrends .
   ```

2. **Run the container**
   ```bash
   docker run -p 8080:8080 \
     -e REDDIT_CLIENT_ID=your_reddit_client_id \
     -e REDDIT_CLIENT_SECRET=your_reddit_client_secret \
     -e ANTHROPIC_API_KEY=your_anthropic_api_key \
     subtrends
   ```

## Usage

### Analyzing Subreddits

1. Enter a subreddit name (with or without "r/") in the search box
2. Click "Analyze" to start the analysis
3. Wait for the AI to process the data and generate a summary
4. View the results with trending topics, community pulse, and hot takes
5. Click on post links to view the original Reddit posts

### Managing History

- View your analysis history on the History page
- Click on any previous subreddit to analyze it again
- Clear your history with the "Clear History" button

### Changing AI Models

- Visit the Model page to see available AI models
- Select a different model from the dropdown
- The new model will be used for future analyses

## API Endpoints

- `GET /` - Main application page
- `POST /analyze` - Analyze a subreddit
- `GET /history` - View analysis history
- `POST /clear-history` - Clear analysis history
- `GET /model` - View model selection page
- `POST /model` - Change AI model
- `GET /health` - Health check endpoint

## Architecture

### Core Components

- **Web Server** (`web.go`): HTTP server with Gin framework
- **Reddit Integration** (`reddit.go`): Reddit API client with rate limiting
- **AI Integration** (`anthropic.go`): Anthropic Claude API client
- **Configuration** (`main.go`): Environment-based configuration
- **Templates**: HTML templates for the web interface
- **Static Assets**: CSS and JavaScript for the frontend

### Data Flow

1. User submits subreddit name via web form
2. Server validates input and creates user session
3. Reddit API fetches top posts and comments
4. Anthropic API generates AI summary
5. Results are formatted and returned to user
6. Analysis is saved to user's history

## Configuration

### Available AI Models

- **haiku3**: Fast and efficient model (default)
- **haiku35**: Balanced performance and capabilities
- **sonnet4**: Most capable model for complex tasks

### Rate Limiting

- Reddit API: 1 request/second with burst of 5
- Anthropic API: 10 requests/minute with burst of 3

## Development

### Project Structure

```
subtrends/
├── main.go              # Application entry point
├── web.go               # Web server and routes
├── reddit.go            # Reddit API integration
├── anthropic.go         # Anthropic AI integration
├── utils.go             # Utility functions
├── templates/           # HTML templates
│   ├── layout.html
│   ├── index.html
│   ├── history.html
│   └── model.html
├── static/              # Static assets
│   ├── css/style.css
│   └── js/app.js
├── go.mod               # Go dependencies
├── Dockerfile           # Docker configuration
└── README.md            # This file
```

### Building

```bash
# Build for current platform
go build -o web .

# Build for specific platform
GOOS=linux GOARCH=amd64 go build -o web .
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For issues and questions, please open an issue on the GitHub repository.
