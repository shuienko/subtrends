# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make setup       # Create venv and install dependencies (first time)
make run         # Run the bot locally (requires .env)
make lint        # Run ruff check, ruff format --check, and mypy
make format      # Auto-format code with ruff
make test        # Run pytest
```

## Architecture

Discord bot that fetches Reddit posts, summarizes them via Anthropic API, and translates to Ukrainian.

**Request flow:**
1. User invokes `/news [group]` slash command
2. `NewsCog` (commands/news.py) defers response, calls `NewsFetcher`
3. `NewsFetcher` fetches posts from all subreddits in the group via `RedditClient`
4. `Summarizer` sends posts to Anthropic API for summary, then translation
5. Response is chunked (2000 char Discord limit) and sent back

**Key patterns:**
- All I/O is async (httpx, aiofiles, discord.py, anthropic SDK)
- Reddit requests go through `RateLimiter` (semaphore + requests/min tracking)
- Reddit OAuth token cached to `.token_cache.json` with file locks
- Subreddit groups defined via `SUB_<NAME>` env vars (parsed by `Config`)

**Entry point:** `src/main.py` wires all components and starts the bot.

## Configuration

Subreddit groups use pattern `SUB_<GROUPNAME>=sub1,sub2,sub3` in `.env`. The group name becomes lowercase (e.g., `SUB_WORLD` → group "world").
