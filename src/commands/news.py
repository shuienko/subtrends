"""Discord slash commands for news fetching."""

import io
import logging
from datetime import date
from typing import TYPE_CHECKING

import discord
from discord import app_commands
from discord.ext import commands

from src.services.news_fetcher import NewsFetcher
from src.services.summarizer import Summarizer

if TYPE_CHECKING:
    from discord import Interaction

logger = logging.getLogger(__name__)

DEFAULT_MODEL = "claude-haiku-4-5"


class NewsCog(commands.Cog):
    """Discord cog for news-related commands."""

    def __init__(
        self,
        bot: commands.Bot,
        fetcher: NewsFetcher,
        summarizer: Summarizer,
    ):
        self.bot = bot
        self.fetcher = fetcher
        self.summarizer = summarizer
        # Guild-specific model settings (in-memory)
        self.model_settings: dict[int, str] = {}

    def _get_model(self, guild_id: int | None) -> str:
        """Get the model setting for a guild."""
        if guild_id is None:
            return DEFAULT_MODEL
        return self.model_settings.get(guild_id, DEFAULT_MODEL)

    @app_commands.command(name="news", description="Get summarized Reddit news in Ukrainian")
    @app_commands.describe(group="News group to fetch (e.g., world, spain). Empty for all.")
    async def news(self, interaction: "Interaction", group: str | None = None) -> None:
        """Fetch and summarize Reddit news."""
        # Defer response for long operation (gives 15 minutes instead of 3 seconds)
        await interaction.response.defer()

        try:
            available_groups = self.fetcher.get_available_groups()

            if not available_groups:
                await interaction.followup.send(
                    "No subreddit groups configured. Please set SUB_* environment variables."
                )
                return

            # Determine which groups to fetch
            if group:
                group_lower = group.lower()
                if group_lower not in available_groups:
                    available = ", ".join(f"`{g}`" for g in available_groups.keys())
                    await interaction.followup.send(
                        f"Unknown group `{group}`. Available groups: {available}"
                    )
                    return
                target_groups = [group_lower]
            else:
                target_groups = list(available_groups.keys())

            guild_id = interaction.guild_id
            model = self._get_model(guild_id)

            await interaction.followup.send(
                f"Fetching news for group(s): {', '.join(target_groups)}..."
            )

            # Process each group
            for grp in target_groups:
                try:
                    # Fetch posts
                    subreddit_group = await self.fetcher.fetch_group(grp)

                    if not subreddit_group.posts:
                        await interaction.followup.send(
                            f"**{grp.upper()}**: No posts found in the last 24 hours."
                        )
                        continue

                    await interaction.followup.send(
                        f"Found {len(subreddit_group.posts)} posts for **{grp.upper()}**. "
                        f"Generating summary..."
                    )

                    # Summarize and translate
                    summary = await self.summarizer.summarize_and_translate(
                        group_name=grp,
                        posts=subreddit_group.posts,
                        model=model,
                    )

                    # Create preview (first 500 chars)
                    preview = summary[:500] + "..." if len(summary) > 500 else summary

                    # Create text file with full summary
                    header = f"{grp.upper()} - NEWS SUMMARY"
                    header_line = "=" * len(header)
                    file_content = f"{header}\n{header_line}\n\n{summary}"
                    file = discord.File(
                        fp=io.BytesIO(file_content.encode("utf-8")),
                        filename=f"{grp}_news_{date.today().isoformat()}.txt",
                    )

                    await interaction.followup.send(
                        content=f"**{grp.upper()} - News Summary**\n\n{preview}",
                        file=file,
                    )

                except Exception as e:
                    logger.exception(f"Error processing group '{grp}'")
                    await interaction.followup.send(f"Error fetching news for **{grp}**: {e}")

        except Exception as e:
            logger.exception("Error in /news command")
            await interaction.followup.send(f"An error occurred: {e}")

    @app_commands.command(name="setmodel", description="Set the AI model for news summaries")
    @app_commands.describe(model="Anthropic model name (e.g., claude-sonnet-4-20250514)")
    async def setmodel(self, interaction: "Interaction", model: str) -> None:
        """Set the Anthropic model for this server."""
        guild_id = interaction.guild_id

        if guild_id is None:
            await interaction.response.send_message(
                "This command can only be used in a server.",
                ephemeral=True,
            )
            return

        # Basic validation
        valid_prefixes = ("claude-", "claude-3", "claude-sonnet", "claude-opus", "claude-haiku")
        if not any(model.startswith(prefix) for prefix in valid_prefixes):
            await interaction.response.send_message(
                f"Invalid model name: `{model}`. Model should start with 'claude-'.",
                ephemeral=True,
            )
            return

        self.model_settings[guild_id] = model
        logger.info(f"Model set to '{model}' for guild {guild_id}")

        await interaction.response.send_message(
            f"Model set to: `{model}`\n"
            f"This setting will be used for all `/news` commands in this server."
        )

    @app_commands.command(name="getmodel", description="Show the current AI model setting")
    async def getmodel(self, interaction: "Interaction") -> None:
        """Show the current model setting."""
        model = self._get_model(interaction.guild_id)
        await interaction.response.send_message(f"Current model: `{model}`")

    @app_commands.command(name="groups", description="List available news groups")
    async def groups(self, interaction: "Interaction") -> None:
        """List available subreddit groups."""
        available_groups = self.fetcher.get_available_groups()

        if not available_groups:
            await interaction.response.send_message("No subreddit groups configured.")
            return

        lines = ["**Available News Groups:**\n"]
        for name, subreddits in available_groups.items():
            subs = ", ".join(f"r/{s}" for s in subreddits)
            lines.append(f"- **{name.upper()}**: {subs}")

        await interaction.response.send_message("\n".join(lines))


async def setup(bot: commands.Bot) -> None:
    """Setup function for loading the cog."""
    # This is called when using bot.load_extension()
    # We don't use it directly since we need to pass fetcher and summarizer
    pass
