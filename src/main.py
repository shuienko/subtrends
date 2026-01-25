"""Main entry point for the SubTrends Discord bot."""

import asyncio
import logging
import sys

import discord
from discord.ext import commands

from src.clients.anthropic_client import AnthropicClient
from src.clients.reddit import RedditClient
from src.commands.news import NewsCog
from src.config import Config
from src.services.news_fetcher import NewsFetcher
from src.services.rate_limiter import RateLimiter
from src.services.summarizer import Summarizer

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s | %(levelname)-8s | %(name)s | %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)

# Reduce noise from third-party libraries
logging.getLogger("httpx").setLevel(logging.WARNING)
logging.getLogger("discord").setLevel(logging.WARNING)
logging.getLogger("anthropic").setLevel(logging.WARNING)

logger = logging.getLogger(__name__)


async def main() -> None:
    """Initialize and run the Discord bot."""
    logger.info("Starting SubTrends bot...")

    # Load configuration
    try:
        config = Config.from_env()
    except ValueError as e:
        logger.error(f"Configuration error: {e}")
        sys.exit(1)

    logger.info(f"Loaded {len(config.subreddit_groups)} subreddit groups")
    for name, subs in config.subreddit_groups.items():
        logger.info(f"  - {name}: {', '.join(subs)}")

    # Initialize rate limiter
    rate_limiter = RateLimiter(
        max_concurrent=config.max_concurrent_requests,
        requests_per_minute=config.requests_per_minute,
    )

    # Initialize Reddit client
    reddit_client = RedditClient(
        client_id=config.reddit_client_id,
        client_secret=config.reddit_client_secret,
        user_agent=config.reddit_user_agent,
        rate_limiter=rate_limiter,
    )

    # Initialize Anthropic client
    anthropic_client = AnthropicClient(api_key=config.anthropic_api_key)

    # Initialize services
    fetcher = NewsFetcher(
        reddit_client=reddit_client,
        subreddit_groups=config.subreddit_groups,
        num_posts=config.num_posts,
        num_comments=config.num_comments,
    )
    summarizer = Summarizer(client=anthropic_client)

    # Initialize Discord bot
    intents = discord.Intents.default()
    bot = commands.Bot(command_prefix="!", intents=intents)

    @bot.event
    async def on_ready() -> None:
        """Called when the bot is ready."""
        if bot.user:
            logger.info(f"Logged in as {bot.user} (ID: {bot.user.id})")

        # Sync slash commands
        try:
            synced = await bot.tree.sync()
            logger.info(f"Synced {len(synced)} slash commands")
        except Exception as e:
            logger.error(f"Failed to sync commands: {e}")

    @bot.event
    async def on_connect() -> None:
        """Called when the bot connects to Discord."""
        logger.info("Connected to Discord")

    @bot.event
    async def on_disconnect() -> None:
        """Called when the bot disconnects from Discord."""
        logger.warning("Disconnected from Discord")

    # Add the news cog
    news_cog = NewsCog(bot=bot, fetcher=fetcher, summarizer=summarizer)
    await bot.add_cog(news_cog)
    logger.info("Loaded NewsCog with commands: /news, /setmodel, /getmodel, /groups")

    # Run the bot
    async with reddit_client:
        try:
            await bot.start(config.discord_token)
        except KeyboardInterrupt:
            logger.info("Received keyboard interrupt, shutting down...")
        finally:
            await bot.close()
            await anthropic_client.close()
            logger.info("Bot shut down cleanly")


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        pass
