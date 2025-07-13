# 📊 SubTrends - Your Reddit Crystal Ball 🔮

> *Discover what's really happening on Reddit with AI-powered insights*

SubTrends transforms the chaos of Reddit into crystal-clear insights! 🚀 Our smart web app dives deep into any subreddit and emerges with bite-sized summaries that actually make sense. No more endless scrolling through comment threads – just pure, distilled Reddit wisdom.

---

## ✨ What Makes SubTrends Awesome

🎯 **Subreddit X-Ray Vision** - Point us at any subreddit and watch the magic happen  
🧠 **AI-Powered Insights** - Multiple Claude models to match your analysis needs  
📚 **Personal History Hub** - Never lose track of your discoveries  
⚡ **Lightning Fast** - Real-time processing with smooth progress indicators  
📱 **Mobile-First Design** - Looks gorgeous on any device  
🛡️ **Smart Rate Limiting** - Plays nice with APIs (and won't get you banned!)

---

## 🛠️ Tech Stack That Powers the Magic

| Component | Technology | Why It's Awesome |
|-----------|------------|------------------|
| 🚀 **Backend** | Go + Gin | Blazing fast, rock solid |
| 🎨 **Frontend** | HTML5 + CSS3 + Bootstrap 5 | Clean, responsive, beautiful |
| 🤖 **AI Brain** | Anthropic Claude API | The smartest summarization on the planet |
| 📡 **Data Source** | Reddit API | Fresh content, straight from the source |
| 🍪 **Sessions** | Gorilla Sessions | Secure, stateful user experience |

## 🚀 Quick Start Guide

### 📋 What You'll Need

- 🐹 **Go 1.23+** - The language that makes everything fast
- 🔑 **Reddit API credentials** - Your ticket to the Reddit universe
- 🤖 **Anthropic API key** - The brain behind the magic

### 🔧 Environment Setup

Create these environment variables to unlock the full power:

```bash
# 🔴 Required (The Holy Trinity)
REDDIT_CLIENT_ID=your_reddit_client_id
REDDIT_CLIENT_SECRET=your_reddit_client_secret
ANTHROPIC_API_KEY=your_anthropic_api_key

# 🟡 Optional (But Nice to Have)
PORT=8080                                    # Where the magic happens
SESSION_SECRET=your-secret-key-change-me     # Keep your sessions secure
STATIC_FILES_PATH=./static                   # Where the pretty stuff lives
TEMPLATE_PATH=./templates                    # HTML template location
HISTORY_FILE_PATH=data/subreddit_history.txt # Your analysis archive
SHUTDOWN_TIMEOUT_SECONDS=5                   # Graceful goodbye time
```

### 💻 Local Development (The Classic Way)

```bash
# 1️⃣ Grab the code
git clone <repository-url>
cd subtrends

# 2️⃣ Feed it your secrets
export REDDIT_CLIENT_ID=your_reddit_client_id
export REDDIT_CLIENT_SECRET=your_reddit_client_secret
export ANTHROPIC_API_KEY=your_anthropic_api_key

# 3️⃣ Get the dependencies
go mod tidy

# 4️⃣ Fire it up! 🔥
go run .

# 5️⃣ Open the magic portal
# Navigate to http://localhost:8080
```

### 🐳 Docker Deployment (The Cool Way)

```bash
# Build your container empire
docker build -t subtrends .

# Launch into orbit! 🚀
docker run -p 8080:8080 \
  -e REDDIT_CLIENT_ID=your_reddit_client_id \
  -e REDDIT_CLIENT_SECRET=your_reddit_client_secret \
  -e ANTHROPIC_API_KEY=your_anthropic_api_key \
  subtrends
```

## 🎮 How to Use Your New Superpower

### 🔍 Analyzing Subreddits (The Main Event)

1. 📝 **Type a subreddit name** - Works with or without "r/" (we're flexible like that)
2. 🚀 **Hit "Analyze"** - Watch the loading animation do its thing
3. ⏳ **Grab some coffee** - Our AI is working hard behind the scenes
4. 📊 **Feast your eyes** - Get trending topics, community pulse, and spicy hot takes
5. 🔗 **Dive deeper** - Click post links to see the original Reddit chaos

### 📚 Managing Your Discovery History

- 👀 **Browse past analyses** - Never lose a great discovery
- 🔄 **Re-analyze favorites** - One click to refresh any subreddit
- 🧹 **Clean slate** - Clear your history when you need a fresh start

### 🧠 Switching AI Models

Choose your fighter! Each model has its own personality:
- 🏃‍♂️ **Haiku 3** - Lightning fast, perfect for quick insights
- ⚖️ **Haiku 3.5** - The goldilocks option (just right)
- 🧙‍♂️ **Sonnet 4** - The wise sage for complex communities

## 🛣️ API Routes (For the Curious)

| Route | Method | What It Does |
|-------|--------|--------------|
| `/` | GET | 🏠 The main stage where magic happens |
| `/analyze` | POST | 🔬 The brain surgery endpoint |
| `/history` | GET | 📖 Your personal analysis library |
| `/clear-history` | POST | 🗑️ The reset button |
| `/model` | GET | 🎭 Model selection theater |
| `/model` | POST | 🔄 Switch your AI companion |
| `/health` | GET | 💊 System pulse check |

---

## 🏗️ Under the Hood

### 🧩 The Core Squad

| Component | File | Superpower |
|-----------|------|------------|
| 🌐 **Web Server** | `web.go` | Gin-powered HTTP magic |
| 🔗 **Reddit Connector** | `reddit.go` | API wizardry with smart rate limiting |
| 🤖 **AI Brain** | `anthropic.go` | Claude integration that just works |
| ⚙️ **Mission Control** | `main.go` | The conductor of this orchestra |
| 🎨 **Pretty Pages** | `templates/` | HTML that doesn't hurt your eyes |
| ✨ **Style & Flair** | `static/` | CSS and JS that sparks joy |

### 🌊 The Data Journey

```
User Input → Validation → Reddit API → AI Processing → Pretty Results → Happy User! 🎉
     ↓
Session Magic → History Storage → Future Reference
```

### ⚡ Performance & Limits

We play nice with everyone:

- 🐌 **Reddit API**: 1 request/second (burst of 5) - Steady and respectful
- 🧠 **Anthropic API**: 10 requests/minute (burst of 3) - Quality over quantity

## 🛠️ Development Zone

### 📁 Project Map

```
subtrends/
├── 🚀 main.go              # The launchpad
├── 🌐 web.go               # HTTP server & route magic
├── 🔗 reddit.go            # Reddit API whisperer
├── 🤖 anthropic.go         # AI conversation master
├── 🔧 utils.go             # The Swiss Army knife
├── 🎨 templates/           # Beautiful HTML homes
│   ├── layout.html         #   The foundation
│   ├── index.html          #   The main stage
│   ├── history.html        #   Memory lane
│   └── model.html          #   AI selection center
├── ✨ static/              # Style & interaction
│   ├── css/style.css       #   The makeup artist
│   └── js/app.js           #   The interaction maestro
├── 📦 go.mod               # Dependency manifest
├── 🐳 Dockerfile           # Container blueprint
└── 📖 README.md            # You are here!
```

### 🔨 Building Your Empire

```bash
# 🏠 Build for your machine
go build -o web .

# 🌍 Build for the world (Linux)
GOOS=linux GOARCH=amd64 go build -o web .
```

---

## 🤝 Join the Fun!

Want to make SubTrends even more awesome? Here's how:

1. 🍴 **Fork it** - Make it your own
2. 🌿 **Branch it** - Create your feature branch  
3. ✨ **Code it** - Work your magic
4. 🧪 **Test it** - Make sure it doesn't break
5. 📤 **PR it** - Share your brilliance!

---

## 📜 Legal Stuff

This project rocks the **MIT License** - basically, you can do almost anything with it! Check the LICENSE file for the fine print.

## 🆘 Need Help?

Got questions? Found a bug? Have a brilliant idea? 

**Drop an issue on GitHub** - we're friendly and we don't bite! 🐕

---

<div align="center">

**Made with ❤️ and lots of ☕**

*Happy analyzing! 🎉*

</div>
