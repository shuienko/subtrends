# SubTrends

> Your personal Reddit trend analyst, right in your Discord server.

SubTrends is a Discord bot that provides AI-powered summaries of trending topics in any subreddit. It leverages the Anthropic Claude API to deliver insightful and engaging analysis of the latest posts and discussions, helping you stay ahead of the curve.

## 🚀 Features

-   **Subreddit Analysis**: Get a comprehensive summary of any subreddit's recent hot topics.
-   **AI-Powered Summaries**: Uses state-of-the-art language models from Anthropic (Claude 3 Haiku, Sonnet) to generate summaries.
-   **Top Post Links**: Includes links to the top posts analyzed for quick access to the source material.
-   **Model Selection**: Choose the AI model that best fits your needs for speed or analytical depth.
-   **Usage History**: Keep track of the subreddits you've analyzed.
-   **Simple Slash Commands**: Easy-to-use Discord slash commands.
-   **Configurable**: Easily configure the bot using environment variables.
-   **Docker Support**: Comes with a `Dockerfile` for easy deployment.

## ⚙️ How It Works

The bot follows a simple workflow:

1.  A user invokes the `/trend` command in a Discord server.
2.  The bot fetches the top posts and comments from the specified subreddit using the Reddit API.
3.  The collected data is sent to the Anthropic API for analysis and summarization.
4.  The AI-generated summary is formatted and sent back to the Discord channel.

```mermaid
graph TD
    A[User on Discord] -- /trend subreddit --> B(SubTrends Bot);
    B -- Fetch top posts/comments --> C(Reddit API);
    C -- Returns data --> B;
    B -- Send data for summary --> D(Anthropic API);
    D -- Returns summary --> B;
    B -- Post summary --> A;
```

## 🤖 Discord Commands

Use these slash commands to interact with the bot:

-   `/trend <subreddit>`: Analyzes a subreddit and provides a trend summary.
    -   `subreddit`: The name of the subreddit (e.g., `golang` not `r/golang`).
-   `/model <model>`: Changes the AI model used for analysis. Available models:
    -   `haiku3`: Claude 3 Haiku (Fast and efficient)
    -   `haiku35`: Claude 3.5 Haiku (Balanced performance)
    -   `sonnet4`: Claude 3 Sonnet (Most capable for complex tasks)
-   `/history`: Displays your last 25 analyzed subreddits.
-   `/clear`: Clears your analysis history.

## 🛠️ Setup and Installation

### Prerequisites

-   [Go](https://golang.org/doc/install) (version 1.18 or newer)
-   [Docker](https://www.docker.com/get-started) (optional, for containerized deployment)
-   A Reddit App (for API credentials)
-   A Discord Bot Application (for bot token)
-   An Anthropic API Key

### 1. Clone the Repository

```bash
git clone https://github.com/your-username/subtrends.git
cd subtrends
```

### 2. Configure Environment Variables

Create a `.env` file in the root of the project and populate it with your credentials and custom settings. You can use the example below as a template.

```dotenv
# .env file
# Get these from your Discord Developer Portal application
DISCORD_BOT_TOKEN=your_discord_bot_token

# Get these from your Reddit App preferences (https://www.reddit.com/prefs/apps)
REDDIT_CLIENT_ID=your_reddit_client_id
REDDIT_CLIENT_SECRET=your_reddit_client_secret

# Get this from your Anthropic account dashboard
ANTHROPIC_API_KEY=your_anthropic_api_key

# --- Optional Settings ---
# You can override the default values from config.go
REDDIT_POST_LIMIT=7
REDDIT_COMMENT_LIMIT=7
REDDIT_TIMEFRAME=day # (day, week, month, year, all)
```

The bot can be configured further using the variables listed in the [Configuration](#-configuration) section.

### 3. Run the Application

#### Using Go

```bash
# Install dependencies
go mod tidy

# Run the bot
go run .
```

#### Using Docker

Build and run the Docker container:

```bash
# Build the Docker image
docker build -t subtrends .

# Run the container with the environment variables
docker run --env-file .env --name subtrends-bot -d subtrends
```

## 🔩 Configuration

The application is configured via environment variables. The following variables are available:

| Variable                       | Description                                               | Default Value                        |
| ------------------------------ | --------------------------------------------------------- | ------------------------------------ |
| **`DISCORD_BOT_TOKEN`**        | **Required.** Your Discord bot token.                     | -                                    |
| **`REDDIT_CLIENT_ID`**         | **Required.** Your Reddit application client ID.          | -                                    |
| **`REDDIT_CLIENT_SECRET`**     | **Required.** Your Reddit application client secret.      | -                                    |
| **`ANTHROPIC_API_KEY`**        | **Required.** Your Anthropic API key.                     | -                                    |
| `ANTHROPIC_API_ENDPOINT`       | Anthropic API endpoint URL.                               | `https://api.anthropic.com/v1/messages` |
| `ANTHROPIC_MAX_TOKENS`         | Max tokens for the AI-generated summary.                  | `1500`                               |
| `REDDIT_POST_LIMIT`            | Number of top posts to fetch from a subreddit.            | `7`                                  |
| `REDDIT_COMMENT_LIMIT`         | Number of top comments to fetch from each post.           | `7`                                  |
| `REDDIT_TIMEFRAME`             | Timeframe for fetching top posts (`day`, `week`, `all`).  | `day`                                |
| `SESSION_FILE_PATH`            | Path to store user session data.                          | `data/sessions.json`                 |
| `SHUTDOWN_TIMEOUT`             | Graceful shutdown timeout.                                | `5s`                                 |

## 💻 Technology Stack

-   **Backend**: Go
-   **Discord API Wrapper**: [discordgo](https://github.com/bwmarrin/discordgo)
-   **AI**: Anthropic Claude API
-   **Data Source**: Reddit API
-   **Containerization**: Docker 