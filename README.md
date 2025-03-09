# 🤖 SubTrends Bot: Your AI-Powered Reddit Time Machine

Ever wished you could get the TL;DR of any subreddit without drowning in endless scrolling? Say hello to SubTrends Bot! 🎉

## 🌟 What's This Sorcery?

SubTrends Bot is your personal Reddit trend analyzer that combines the power of:
- Reddit's top posts and discussions
- Claude AI's summarization magic
- Telegram's smooth interface
- Your curiosity about what's trending!

## 🚀 Features

- 🎯 Get instant summaries of any subreddit's hottest discussions
- 🧠 AI-powered analysis of community trends and sentiments
- 🔒 Single-user mode for your personal use
- 🤹 Handles text posts, discussions, and top comments

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
```

3. Build and run with Docker:
```bash
docker build -t subtrends-bot .
docker run -d --env-file .env --name subtrends-bot subtrends-bot
```

## 🎮 Usage

1. Start a chat with your bot on Telegram
2. Send any subreddit name (without r/), for example: `programming`
3. Wait for the magic to happen! ✨

## 💡 Pro Tips

- Summaries include popular opinions and community sentiment
- Each summary is crafted by Claude AI for human-like understanding

## 🔧 Technical Details

- Built with Go 1.21+
- Implements Reddit API rate limiting
- Secure single-user authentication
- Docker-ready with Alpine Linux base

## 🤝 Contributing

Found a bug? Want to add a feature? PRs are welcome! Just:
1. Fork the repo
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Open a Pull Request

## ⚠️ Limitations

- Single user per bot instance
- Reddit API rate limits apply
- Claude API token required

## 📜 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🎭 Credits

Built with ❤️ using:
- Go-Telegram-Bot-API
- Claude AI by Anthropic
- Reddit API
- And lots of coffee ☕

Remember: With great power comes great responsibility. Use this bot wisely, and happy trending! 🚀

## Recent Improvements

- Added graceful shutdown with context cancellation
- Implemented proper error handling with custom error types
- Added configuration via environment variables with defaults
- Improved logging with more context
- Added request timeouts for all API calls
- Implemented better caching for Reddit tokens with mutex protection
- Added rate limiting for API requests to prevent throttling
- Improved code organization with better separation of concerns
- Enhanced error messages for better debugging
- Added concurrent processing of Reddit posts with semaphore limiting