# 🤖 SubTrends Bot: Your AI-Powered Reddit Time Machine

Ever wished you could get the TL;DR of any subreddit without drowning in endless scrolling? Say hello to SubTrends Bot! 🎉

## 🌟 What's This Sorcery?

SubTrends Bot is your personal Reddit trend analyzer that combines the power of:
- Reddit's top posts and discussions
- Claude 3 Haiku AI's summarization magic
- Telegram's smooth interface
- Your curiosity about what's trending!

## 🚀 Features

- 🎯 Get instant summaries of any subreddit's hottest discussions from the past day
- 🧠 AI-powered analysis of community trends, sentiments, and hot takes
- 🔒 Single-user mode for your personal use
- 🔗 Links to top posts for easy browsing
- 📊 Organized summaries with trending topics and community pulse
- 🚀 Fast responses with Claude 3 Haiku model

## 🛠️ Setup

### Prerequisites

- Docker installed on your system
- API Keys:
  - Telegram Bot Token
  - Anthropic API Key (for Claude)
  - Reddit API Credentials
  - Your Telegram User ID

### 🐳 Docker Quick Start

1. Clone this repository:
```bash
git clone https://github.com/yourusername/subtrends-bot
cd subtrends-bot
```

2. Create a `.env` file:
```env
TELEGRAM_TOKEN=your_telegram_bot_token
ANTHROPIC_API_KEY=your_anthropic_api_key
REDDIT_CLIENT_ID=your_reddit_client_id
REDDIT_CLIENT_SECRET=your_reddit_client_secret
REDDIT_USER_AGENT=your_app_name/1.0
AUTHORIZED_USER_ID=your_telegram_user_id
# Optional settings
ANTHROPIC_MODEL=claude-3-haiku-20240307
DEBUG=false
SHUTDOWN_TIMEOUT_SECONDS=5
```

3. Build and run with Docker:
```bash
docker build -t subtrends-bot .
docker run -d --env-file .env --name subtrends-bot subtrends-bot
```

## 🎮 Usage

1. Start a chat with your bot on Telegram
2. Send any subreddit name (with or without r/), for example: `programming`
3. The bot will:
   - Connect to Reddit and fetch top posts
   - Analyze the content using Claude AI
   - Return a nicely formatted summary with trending topics and community sentiment
   - Include links to the top posts for easy access

## 💡 Commands

- `/start` - Get a welcome message and basic instructions
- `/help` - View available commands and usage tips
- Just send any subreddit name to get a summary!

## 🔧 Technical Details

- Built with Go 1.23+
- Uses Claude 3 Haiku model for fast, efficient summarization
- Implements Reddit API rate limiting (1 request/second)
- Anthropic API rate limiting (10 requests/minute)
- Secure single-user authentication
- Docker-ready with Alpine Linux base

## ⚠️ Limitations

- Single user per bot instance
- Reddit API rate limits apply
- Summarizes posts from the past day only
- Default limit of 7 top posts analyzed

## 📜 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🎭 Credits

Built with ❤️ using:
- Go-Telegram-Bot-API
- Claude 3 Haiku by Anthropic
- Reddit API
- And lots of coffee ☕

Remember: With great power comes great responsibility. Use this bot wisely, and happy trending! 🚀
