# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make setup       # Create venv and install dependencies (first time)
make run         # Run the bot locally (requires .env)
make lint        # Run ruff check, ruff format --check, and mypy
make format      # Auto-format code with ruff
make test        # Run pytest
make test-cov    # Run pytest with coverage

# Run single test
.venv/bin/pytest tests/test_file.py::test_name -v
```

## Architecture

Discord bot that fetches Reddit posts, summarizes them via Anthropic API, and translates to Ukrainian.

**Request flow:**
1. User invokes `/news [group]` slash command
2. `NewsCog` (commands/news.py) defers response, calls `NewsFetcher`
3. `NewsFetcher` fetches posts from all subreddits in the group via `RedditClient` (parallel fetch)
4. `Summarizer` performs two-step AI processing: summarize → translate to Ukrainian
5. Response sent as `.txt` file attachment (bypasses Discord's 2000 char limit)

**Key patterns:**
- All I/O is async (httpx, aiofiles, discord.py, anthropic SDK)
- Reddit requests go through `RateLimiter` (semaphore + requests/min tracking)
- Reddit OAuth token cached to `.token_cache.json` with file locks
- Subreddit groups defined via `SUB_<NAME>` env vars (parsed by `Config`)
- Dependency injection: `main.py` wires all components and passes them to `NewsCog`
- Per-guild model settings stored in-memory (`NewsCog.model_settings` dict)

**Entry point:** `src/main.py`

## Configuration

Subreddit groups use pattern `SUB_<GROUPNAME>=sub1,sub2,sub3` in `.env`. The group name becomes lowercase (e.g., `SUB_WORLD` → group "world").

## Testing

Tests use `pytest-asyncio` with `asyncio_mode = "auto"` (no need to mark async tests). HTTP mocking via `pytest-httpx`.
