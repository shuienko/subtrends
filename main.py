"""Entry point for SubTrends Discord Bot."""

import asyncio
import logging
import os
import signal
import sys

from dotenv import load_dotenv

from bot import SubTrendsBot
from config import get_config
from utils import get_required_env_var

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    handlers=[logging.StreamHandler(sys.stdout)],
)
logger = logging.getLogger(__name__)


async def main() -> None:
    """Main entry point for the bot."""
    logger.info("Starting SubTrends Discord Bot...")
    
    # Load environment variables from .env file if present
    load_dotenv()
    
    # Load configuration
    config = get_config()
    
    # Get Discord token
    try:
        token = get_required_env_var("DISCORD_BOT_TOKEN")
    except Exception as e:
        logger.fatal(f"Failed to get Discord bot token: {e}")
        sys.exit(1)
    
    # Create bot instance
    bot = SubTrendsBot(config)
    
    # Set up signal handlers for graceful shutdown
    shutdown_event = asyncio.Event()
    
    def signal_handler(sig: signal.Signals) -> None:
        logger.info(f"Received signal {sig.name}, initiating shutdown...")
        shutdown_event.set()
    
    # Register signal handlers
    loop = asyncio.get_running_loop()
    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, lambda s=sig: signal_handler(s))
    
    # Start bot
    async def run_bot() -> None:
        try:
            await bot.start(token)
        except Exception as e:
            logger.error(f"Bot stopped with error: {e}")
            shutdown_event.set()
    
    bot_task = asyncio.create_task(run_bot())
    
    logger.info("SubTrends Discord Bot is now running. Press CTRL-C to exit.")
    
    # Wait for shutdown signal
    await shutdown_event.wait()
    
    logger.info("Shutdown signal received, stopping bot...")
    
    # Graceful shutdown with timeout
    try:
        await asyncio.wait_for(bot.close(), timeout=config.shutdown_timeout)
    except asyncio.TimeoutError:
        logger.warning("Shutdown timed out, forcing close...")
    except Exception as e:
        logger.error(f"Error during shutdown: {e}")
    
    # Cancel bot task if still running
    if not bot_task.done():
        bot_task.cancel()
        try:
            await bot_task
        except asyncio.CancelledError:
            pass
    
    logger.info("Bot has been gracefully stopped")


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        logger.info("Interrupted by user")
    except Exception as e:
        logger.fatal(f"Fatal error: {e}")
        sys.exit(1)
