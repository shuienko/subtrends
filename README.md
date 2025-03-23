# 🤖 SubTrends Bot: Your AI-Powered Reddit Time Machine

Ever wished you could get the TL;DR of any subreddit without drowning in endless scrolling? Say hello to SubTrends Bot! 🎉

## 🌟 What's This Sorcery?

SubTrends Bot is your personal Reddit trend analyzer that combines the power of:
- Reddit's top posts and discussions
- Claude 3 Haiku AI's summarization capabilities
- Telegram and/or Discord integration for easy interaction
- Your curiosity about what's trending!

## 🚀 Features

- 🎯 Get instant summaries of any subreddit's hottest discussions from the past day
- 🧠 AI-powered analysis of community trends, sentiments, and hot takes
- 🔒 Authorization options for both Telegram and Discord
- 🔗 Links to top posts for easy browsing
- 📊 Organized summaries with trending topics and community pulse
- 🚀 Fast responses with Claude 3 Haiku model by default
- 📜 History tracking of previously requested subreddits
- ⚙️ Model selection capability for different Claude models
- 🛑 Proper rate limiting for Reddit and Anthropic APIs
- 💼 Token caching for efficient Reddit API usage
- 🤖 Multi-platform support: run on Telegram, Discord, or both simultaneously

## 🛠️ Setup

### Prerequisites

- Docker installed on your system
- API Keys:
  - Telegram Bot Token (if using Telegram)
  - Discord Bot Token (if using Discord)
  - Anthropic API Key (for Claude)
  - Reddit API Credentials
  - Your Telegram User ID (if using Telegram)
  - Discord Role ID for authorization (optional, if using Discord)

### 🐳 Docker Quick Start

1. Clone this repository:
```bash
git clone https://github.com/yourusername/subtrends-bot
cd subtrends-bot
```

2. Create a `.env` file:
```env
# Bot platform selection: "telegram", "discord", or "both"
BOT_PLATFORM=both

# Telegram configuration (required if using Telegram)
TELEGRAM_TOKEN=your_telegram_bot_token
AUTHORIZED_USER_ID=your_telegram_user_id

# Discord configuration (required if using Discord)
DISCORD_TOKEN=your_discord_bot_token
# Optional Discord settings
DISCORD_AUTHORIZED_ROLE_ID=role_id_for_authorized_users
DISCORD_GUILD_ID=specific_guild_id_if_restricted

# API credentials (always required)
ANTHROPIC_API_KEY=your_anthropic_api_key
REDDIT_CLIENT_ID=your_reddit_client_id
REDDIT_CLIENT_SECRET=your_reddit_client_secret
REDDIT_USER_AGENT=SubTrends/1.0

# Optional settings
ANTHROPIC_MODEL=claude-3-haiku-20240307
DEBUG=false
SHUTDOWN_TIMEOUT_SECONDS=5
HISTORY_FILE_PATH=data/subreddit_history.txt
```

3. Build and run with Docker:
```bash
docker build -t subtrends-bot .
docker run -d --env-file .env --name subtrends-bot subtrends-bot
```

## 🎮 Usage

### Telegram

1. Start a chat with your bot on Telegram
2. Send any subreddit name (with or without r/), for example: `programming`

### Discord

1. Invite the bot to your Discord server with appropriate permissions
2. Send any subreddit name in a channel where the bot can read messages
3. Use command prefix `!` for commands, for example: `!help`

For both platforms, the bot will:
- Connect to Reddit and fetch top posts (default: top 7 posts from the past day)
- Retrieve top comments for each post
- Analyze the content using Claude AI
- Return a nicely formatted summary with trending topics and community sentiment
- Include links to the top posts for easy access

## 💡 Commands

### Telegram Commands

- `/start` - Get a welcome message and basic instructions
- `/help` - View available commands and usage tips
- `/history` - View your saved subreddit history
- `/clearhistory` - Clear your subreddit history
- `/model` - View or change the AI model used for summaries
- Just send any subreddit name to get a summary!

### Discord Commands

- `!help` - View available commands and usage tips
- `!history` - View your saved subreddit history
- `!clear` - Clear your subreddit history
- `!model` - View or change the AI model used for summaries
- Just send any subreddit name to get a summary!

## 🧩 Technical Implementation

- **Go**: Built with Go (requires Go 1.23.4)
- **Telegram**: Uses go-telegram-bot-api/v5 for Telegram integration
- **Discord**: Uses bwmarrin/discordgo for Discord integration
- **Rate Limiting**: Implements request limiting for both Reddit (1 req/sec) and Anthropic APIs (10 req/min)
- **Concurrency**: Proper mutex handling for thread safety
- **Graceful Shutdown**: Signal handling for clean application termination
- **Error Handling**: Custom error types for better debugging
- **Token Management**: Reddit token caching to reduce authentication requests
- **History Persistence**: Saves subreddit history to local file for persistence across restarts

## ⚠️ Limitations

- For Telegram: Single user per bot instance (authenticated via Telegram User ID)
- For Discord: Role-based authentication or open to all users
- Reddit API rate limits apply (1 request per second)
- Anthropic API rate limits (10 requests per minute)
- Summarizes posts from the past day only
- Default limit of 7 top posts analyzed per request

## 📜 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🎭 Credits

Built with ❤️ using:
- Go-Telegram-Bot-API
- DiscordGo
- Claude 3 Haiku by Anthropic
- Reddit API
- And lots of coffee ☕

Remember: With great power comes great responsibility. Use this bot wisely, and happy trending! 🚀
