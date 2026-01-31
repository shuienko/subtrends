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


class NewsCog(commands.Cog):
    """Discord cog for news-related commands."""

    def __init__(
        self,
        bot: commands.Bot,
        fetcher: NewsFetcher,
        summarizer: Summarizer,
        default_model: str,
    ):
        self.bot = bot
        self.fetcher = fetcher
        self.summarizer = summarizer
        self.default_model = default_model
        # Guild-specific model settings (in-memory)
        self.model_settings: dict[int, str] = {}

        # Create news command group with dynamic subcommands
        news_group = app_commands.Group(
            name="news", description="Get summarized Reddit news in Ukrainian"
        )

        # Add subcommand for each available group
        for group_name in sorted(fetcher.get_available_groups().keys()):
            self._add_group_subcommand(news_group, group_name)

        # Add "all" subcommand to fetch all groups
        @news_group.command(name="all", description="Fetch news from all groups")
        async def news_all(interaction: "Interaction") -> None:
            await self._fetch_news(interaction, target_groups=None)

        # Register command group with cog
        self.__cog_app_commands__.append(news_group)

    def _add_group_subcommand(self, news_group: app_commands.Group, group_name: str) -> None:
        """Add a subcommand for a specific news group."""

        @news_group.command(name=group_name, description=f"Fetch {group_name} news")
        async def group_command(interaction: "Interaction") -> None:
            await self._fetch_news(interaction, target_groups=[group_name])

    def _get_model(self, guild_id: int | None) -> str:
        """Get the model setting for a guild."""
        if guild_id is None:
            return self.default_model
        return self.model_settings.get(guild_id, self.default_model)

    async def _fetch_news(
        self, interaction: "Interaction", target_groups: list[str] | None
    ) -> None:
        """Fetch and summarize Reddit news for specified groups."""
        await interaction.response.defer()

        try:
            available_groups = self.fetcher.get_available_groups()

            if not available_groups:
                await interaction.followup.send(
                    "Групи сабредітів не налаштовані. "
                    "Будь ласка, встановіть змінні середовища SUB_*."
                )
                return

            # Use all groups if none specified
            if target_groups is None:
                target_groups = list(available_groups.keys())

            guild_id = interaction.guild_id
            model = self._get_model(guild_id)

            await interaction.followup.send(
                f"Завантаження новин для груп(и): {', '.join(target_groups)}..."
            )

            for grp in target_groups:
                try:
                    subreddit_group = await self.fetcher.fetch_group(grp)

                    if not subreddit_group.posts:
                        await interaction.followup.send(
                            f"**{grp.upper()}**: Не знайдено постів за останні 24 години."
                        )
                        continue

                    await interaction.followup.send(
                        f"Знайдено {len(subreddit_group.posts)} постів для **{grp.upper()}**. "
                        f"Генерую підсумок..."
                    )

                    summary = await self.summarizer.summarize_and_translate(
                        group_name=grp,
                        posts=subreddit_group.posts,
                        model=model,
                    )

                    header = f"{grp.upper()} - NEWS SUMMARY"
                    header_line = "=" * len(header)
                    file_content = f"{header}\n{header_line}\n\n{summary}"
                    file = discord.File(
                        fp=io.BytesIO(file_content.encode("utf-8")),
                        filename=f"{grp}_news_{date.today().isoformat()}.txt",
                    )

                    await interaction.followup.send(
                        content=f"**{grp.upper()}**",
                        file=file,
                    )

                except Exception as e:
                    logger.exception(f"Error processing group '{grp}'")
                    await interaction.followup.send(
                        f"Помилка завантаження новин для **{grp}**: {e}"
                    )

        except Exception as e:
            logger.exception("Error in /news command")
            await interaction.followup.send(f"Сталася помилка: {e}")

    @app_commands.command(name="setmodel", description="Set the AI model for news summaries")
    @app_commands.describe(model="Anthropic model name (e.g., claude-sonnet-4-20250514)")
    async def setmodel(self, interaction: "Interaction", model: str) -> None:
        """Set the Anthropic model for this server."""
        guild_id = interaction.guild_id

        if guild_id is None:
            await interaction.response.send_message(
                "Цю команду можна використовувати лише на сервері.",
                ephemeral=True,
            )
            return

        # Basic validation
        valid_prefixes = ("claude-", "claude-3", "claude-sonnet", "claude-opus", "claude-haiku")
        if not any(model.startswith(prefix) for prefix in valid_prefixes):
            await interaction.response.send_message(
                f"Недійсна назва моделі: `{model}`. Назва моделі повинна починатися з 'claude-'.",
                ephemeral=True,
            )
            return

        self.model_settings[guild_id] = model
        logger.info(f"Model set to '{model}' for guild {guild_id}")

        await interaction.response.send_message(
            f"Модель встановлено: `{model}`\n"
            f"Це налаштування буде використовуватися для всіх команд `/news` на цьому сервері."
        )

    @app_commands.command(name="getmodel", description="Show the current AI model setting")
    async def getmodel(self, interaction: "Interaction") -> None:
        """Show the current model setting."""
        model = self._get_model(interaction.guild_id)
        await interaction.response.send_message(f"Поточна модель: `{model}`")

    @app_commands.command(name="groups", description="List available news groups")
    async def groups(self, interaction: "Interaction") -> None:
        """List available subreddit groups."""
        available_groups = self.fetcher.get_available_groups()

        if not available_groups:
            await interaction.response.send_message("Групи сабредітів не налаштовані.")
            return

        lines = ["**Доступні групи новин:**\n"]
        for name, subreddits in available_groups.items():
            subs = ", ".join(f"r/{s}" for s in subreddits)
            lines.append(f"- **{name.upper()}**: {subs}")

        await interaction.response.send_message("\n".join(lines))
