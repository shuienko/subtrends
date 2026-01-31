# SubTrends

A Discord bot that fetches Reddit news from configured subreddit groups, summarizes them using Claude AI, and translates the summaries to Ukrainian.

## Features

- `/news <group>` - Fetch and summarize Reddit news from a specific group (e.g., `/news world`, `/news tech`)
- `/news all` - Fetch news from all configured subreddit groups
- `/setmodel <model>` - Set the AI model for summaries (per-server)
- `/getmodel` - Show the current model setting
- `/groups` - List available news groups

All bot responses are in Ukrainian.

## Setup

### Prerequisites

- Python 3.11+
- Discord bot token ([Discord Developer Portal](https://discord.com/developers/applications))
- Reddit API credentials ([Reddit Apps](https://www.reddit.com/prefs/apps))
- Anthropic API key ([Anthropic Console](https://console.anthropic.com/))

### Local Development

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/subtrends.git
   cd subtrends
   ```

2. Copy the example environment file and fill in your credentials:
   ```bash
   cp .env.example .env
   # Edit .env with your API keys
   ```

3. Set up the virtual environment and install dependencies:
   ```bash
   make setup
   ```

4. Run the bot:
   ```bash
   make run
   ```

### Docker

1. Build the Docker image:
   ```bash
   make docker-build
   ```

2. Run the container:
   ```bash
   make docker-run
   ```

## Configuration

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DISCORD_TOKEN` | Yes | Discord bot token |
| `REDDIT_CLIENT_ID` | Yes | Reddit API client ID |
| `REDDIT_CLIENT_SECRET` | Yes | Reddit API client secret |
| `REDDIT_USER_AGENT` | No | User agent for Reddit API (default: `subtrends:v1.0`) |
| `ANTHROPIC_API_KEY` | Yes | Anthropic API key |
| `DEFAULT_MODEL` | No | Anthropic model for summaries (default: `claude-opus-4-5`) |
| `SUB_<NAME>` | Yes | Subreddit groups (comma-separated) |
| `NUM_POSTS` | No | Posts per subreddit (default: 7) |
| `NUM_COMMENTS` | No | Comments per post (default: 7) |
| `MAX_CONCURRENT_REQUESTS` | No | Maximum concurrent Reddit API requests (default: 5) |
| `REQUESTS_PER_MINUTE` | No | Rate limit for Reddit requests (default: 60) |

### Subreddit Groups

Define groups using environment variables with the `SUB_` prefix:

```bash
SUB_WORLD=news,geopolitics,europe,economics,technology
SUB_SPAIN=spain,es
SUB_TECH=programming,webdev,machinelearning
```

## Development

### Available Make Commands

```bash
make help          # Show all commands
make setup         # Create venv and install dependencies
make run           # Run the bot locally
make test          # Run tests
make test-cov      # Run tests with coverage
make lint          # Run linting (ruff + mypy)
make format        # Format code with ruff
make clean         # Remove venv and cache files
make docker-build  # Build Docker image
make docker-run    # Run in Docker
make docker-shell  # Open a shell in the Docker container
```

## CI/CD

The project uses GitHub Actions to:
1. Run linting on every push and PR
2. Build and push Docker image to Docker Hub on pushes to `main`

### Required GitHub Secrets

- `DOCKER_USERNAME` - Your Docker Hub username
- `DOCKER_PASSWORD` - Docker Hub password

## Architecture

```
src/
├── main.py              # Entry point
├── config.py            # Environment configuration
├── clients/
│   ├── reddit.py        # Reddit API client with OAuth
│   ├── anthropic_client.py  # Anthropic API wrapper
│   └── token_cache.py   # OAuth token caching
├── services/
│   ├── news_fetcher.py  # Reddit fetching orchestration
│   ├── summarizer.py    # Summary + translation
│   └── rate_limiter.py  # Request rate limiting
├── models/
│   └── reddit_types.py  # Data models
└── commands/
    └── news.py          # Discord slash commands
```

## License

MIT
