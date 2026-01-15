"""Discord bot with slash commands, session management, and message handling."""

import asyncio
import logging
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any

import discord
from discord import app_commands
from discord.ext import commands

from config import Config, get_config
from openai_client import OpenAIClient
from reddit_client import RedditClient, RedditPost
from utils import read_json_file, write_json_file

logger = logging.getLogger(__name__)


@dataclass
class ModelInfo:
    """Information about an available AI model."""
    codename: str
    name: str
    description: str


# Available models for selection
AVAILABLE_MODELS = [
    ModelInfo(
        codename="gpt5nano",
        name="gpt-5-nano",
        description="Fast and efficient model (default)",
    ),
    ModelInfo(
        codename="gpt52",
        name="gpt-5.2",
        description="Most capable model for complex tasks",
    ),
]

DEFAULT_MODEL_NAME = AVAILABLE_MODELS[0].name
DEFAULT_REASONING_EFFORT = "minimal"

VALID_REASONING_EFFORTS = {"minimal", "medium", "high"}


def is_valid_model_name(name: str) -> bool:
    """Check if a model name is valid."""
    return any(m.name == name for m in AVAILABLE_MODELS)


def is_valid_reasoning_effort(level: str) -> bool:
    """Check if a reasoning effort level is valid."""
    return level in VALID_REASONING_EFFORTS


@dataclass
class UserSession:
    """User session data for Discord users."""
    user_id: str
    history: list[str] = field(default_factory=list)
    model: str = DEFAULT_MODEL_NAME
    reasoning_effort: str = DEFAULT_REASONING_EFFORT
    created_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))

    def to_dict(self) -> dict[str, Any]:
        """Convert session to dictionary for JSON serialization."""
        return {
            "user_id": self.user_id,
            "history": self.history,
            "model": self.model,
            "reasoning_effort": self.reasoning_effort,
            "created_at": self.created_at.isoformat(),
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "UserSession":
        """Create session from dictionary."""
        created_at_str = data.get("created_at", "")
        if created_at_str:
            try:
                created_at = datetime.fromisoformat(created_at_str.replace("Z", "+00:00"))
            except ValueError:
                created_at = datetime.now(timezone.utc)
        else:
            created_at = datetime.now(timezone.utc)
        
        return cls(
            user_id=data.get("user_id", ""),
            history=data.get("history", []),
            model=data.get("model", DEFAULT_MODEL_NAME),
            reasoning_effort=data.get("reasoning_effort", DEFAULT_REASONING_EFFORT),
            created_at=created_at,
        )


class SubTrendsBot(commands.Bot):
    """Discord bot for analyzing subreddit trends."""

    def __init__(self, config: Config | None = None):
        self.config = config or get_config()
        
        intents = discord.Intents.default()
        intents.message_content = True
        intents.guilds = True
        
        super().__init__(
            command_prefix=self.config.legacy_command_prefix,
            intents=intents,
        )
        
        self._user_sessions: dict[str, UserSession] = {}
        self._session_lock: asyncio.Lock | None = None
        self._reddit_client = RedditClient(self.config)
        self._openai_client = OpenAIClient(self.config)
        
        # Load existing sessions
        self._load_sessions()

    def _load_sessions(self) -> None:
        """Load sessions from file."""
        data = read_json_file(self.config.session_file_path)
        if not data:
            logger.info("No existing sessions found or file is empty.")
            return
        
        for user_id, session_data in data.items():
            session = UserSession.from_dict(session_data)
            
            # Normalize legacy/invalid values
            if not is_valid_model_name(session.model):
                session.model = DEFAULT_MODEL_NAME
            if not is_valid_reasoning_effort(session.reasoning_effort):
                session.reasoning_effort = DEFAULT_REASONING_EFFORT
            
            self._user_sessions[user_id] = session
        
        logger.info(f"Loaded {len(self._user_sessions)} user sessions")

    def _get_session_lock(self) -> asyncio.Lock:
        """Create the session lock lazily when an event loop is running."""
        if self._session_lock is None:
            self._session_lock = asyncio.Lock()
        return self._session_lock

    async def _save_sessions(self) -> None:
        """Save sessions to file."""
        session_lock = self._get_session_lock()
        async with session_lock:
            data = {
                user_id: session.to_dict()
                for user_id, session in self._user_sessions.items()
            }
        
        if not write_json_file(self.config.session_file_path, data):
            logger.error("Error writing sessions file")

    def _get_user_session(self, user_id: str) -> UserSession:
        """Get or create a user session."""
        if user_id not in self._user_sessions:
            self._user_sessions[user_id] = UserSession(user_id=user_id)
        
        session = self._user_sessions[user_id]
        
        # Migrate invalid values
        needs_save = False
        if not is_valid_model_name(session.model):
            session.model = DEFAULT_MODEL_NAME
            needs_save = True
        if not is_valid_reasoning_effort(session.reasoning_effort):
            session.reasoning_effort = DEFAULT_REASONING_EFFORT
            needs_save = True
        
        if needs_save:
            asyncio.create_task(self._save_sessions())
        
        return session

    async def _save_user_session(self, user_id: str, session: UserSession) -> None:
        """Save a user session."""
        self._user_sessions[user_id] = session
        await self._save_sessions()

    async def setup_hook(self) -> None:
        """Set up the bot's slash commands."""
        # Build model choices dynamically
        model_choices = [
            app_commands.Choice(
                name=f"{m.name} ({m.description})",
                value=m.codename,
            )
            for m in AVAILABLE_MODELS
        ]
        
        reasoning_choices = [
            app_commands.Choice(name="Minimal (fastest, cheapest)", value="minimal"),
            app_commands.Choice(name="Medium (balanced)", value="medium"),
            app_commands.Choice(name="High (most thorough, slowest)", value="high"),
        ]

        @self.tree.command(name="trend", description="Analyze trends in a subreddit")
        @app_commands.describe(subreddit="The subreddit to analyze (without r/)")
        async def trend_command(interaction: discord.Interaction, subreddit: str) -> None:
            await self._handle_trend_slash_command(interaction, subreddit)

        @trend_command.autocomplete("subreddit")
        async def trend_autocomplete(
            interaction: discord.Interaction,
            current: str,
        ) -> list[app_commands.Choice[str]]:
            return await self._handle_trend_autocomplete(interaction, current)

        @self.tree.command(name="model", description="Change the AI model used for analysis")
        @app_commands.describe(model="Choose AI model")
        @app_commands.choices(model=model_choices)
        async def model_command(
            interaction: discord.Interaction,
            model: app_commands.Choice[str],
        ) -> None:
            await self._handle_model_command(interaction, model)

        @self.tree.command(
            name="reasoning",
            description="Change the reasoning effort level used for analysis",
        )
        @app_commands.describe(level="Choose reasoning effort level")
        @app_commands.choices(level=reasoning_choices)
        async def reasoning_command(
            interaction: discord.Interaction,
            level: app_commands.Choice[str],
        ) -> None:
            await self._handle_reasoning_command(interaction, level)

        @self.tree.command(name="history", description="View your subreddit analysis history")
        async def history_command(interaction: discord.Interaction) -> None:
            await self._handle_history_command(interaction)

        @self.tree.command(name="clear", description="Clear your analysis history")
        async def clear_command(interaction: discord.Interaction) -> None:
            await self._handle_clear_command(interaction)

        # Sync commands globally
        logger.info("Syncing slash commands...")
        await self.tree.sync()
        logger.info("Slash commands synced successfully")

    async def on_ready(self) -> None:
        """Called when the bot is ready."""
        if self.user:
            logger.info(f"Logged in as: {self.user.name}#{self.user.discriminator}")

    async def on_message(self, message: discord.Message) -> None:
        """Handle incoming messages for legacy command support."""
        # Ignore messages from the bot itself
        if message.author == self.user:
            return
        
        # Handle legacy text command
        if message.content.startswith(self.config.legacy_command_prefix):
            subreddit = message.content[len(self.config.legacy_command_prefix):].strip()
            if subreddit:
                await self._handle_trend_text_command(message, subreddit)
        
        await self.process_commands(message)

    async def _handle_trend_slash_command(
        self,
        interaction: discord.Interaction,
        subreddit: str,
    ) -> None:
        """Handle the /trend slash command."""
        user_id = str(interaction.user.id)
        
        # Respond immediately
        await interaction.response.send_message(
            f"🔍 Analyzing r/{subreddit}... This may take a moment."
        )
        
        # Handle analysis in background
        asyncio.create_task(
            self._handle_trend_analysis(interaction.channel, user_id, subreddit)
        )

    async def _handle_trend_autocomplete(
        self,
        interaction: discord.Interaction,
        current: str,
    ) -> list[app_commands.Choice[str]]:
        """Provide autocomplete suggestions from history."""
        user_id = str(interaction.user.id)
        session = self._get_user_session(user_id)
        
        max_suggestions = 25
        current_lower = current.lower().strip()
        
        if not current_lower:
            # Return most recent history items
            start = max(0, len(session.history) - max_suggestions)
            return [
                app_commands.Choice(name=sub, value=sub)
                for sub in reversed(session.history[start:])
            ][:max_suggestions]
        
        # Filter by typed value, deduplicate
        seen: set[str] = set()
        choices: list[app_commands.Choice[str]] = []
        
        for sub in reversed(session.history):
            key = sub.lower()
            if key in seen:
                continue
            if current_lower in key:
                choices.append(app_commands.Choice(name=sub, value=sub))
                seen.add(key)
                if len(choices) >= max_suggestions:
                    break
        
        return choices

    async def _handle_trend_text_command(
        self,
        message: discord.Message,
        subreddit: str,
    ) -> None:
        """Handle the legacy !trend text command."""
        user_id = str(message.author.id)
        
        # Send initial message
        initial_msg = await message.channel.send(
            f"🔍 Analyzing r/{subreddit}... This may take a moment."
        )
        
        await self._handle_trend_analysis(message.channel, user_id, subreddit)
        
        # Delete initial message
        try:
            await initial_msg.delete()
        except discord.errors.NotFound:
            pass

    async def _handle_trend_analysis(
        self,
        channel: discord.abc.Messageable,
        user_id: str,
        subreddit: str,
    ) -> None:
        """Perform the actual subreddit analysis."""
        # Clean subreddit name
        subreddit = subreddit.removeprefix("r/")
        
        session = self._get_user_session(user_id)
        
        # Add to history if not already present (case-insensitive)
        if not any(s.lower() == subreddit.lower() for s in session.history):
            session.history.append(subreddit)
            await self._save_user_session(user_id, session)
            logger.info(f"Added {subreddit} to history for user {user_id}")
        
        try:
            # Get Reddit data
            data, posts, total_comments = await self._reddit_client.get_subreddit_data(subreddit)
            
            # Generate summary
            summary = await self._openai_client.summarize_posts(
                subreddit=subreddit,
                text=data,
                model=session.model,
                reasoning_effort=session.reasoning_effort,
            )
            
            # Format and send response
            response = self._format_analysis_response(subreddit, summary, posts, total_comments)
            await self._send_long_message(channel, response)
            
        except Exception as e:
            logger.error(f"Failed to analyze r/{subreddit}: {e}")
            await channel.send(f"❌ Failed to analyze r/{subreddit}: {e}")

    def _format_analysis_response(
        self,
        subreddit: str,
        summary: str,
        posts: list[RedditPost],
        total_comments: int,
    ) -> str:
        """Format the analysis response for Discord."""
        lines = [
            f"## 📈 **r/{subreddit} Trends**\n",
            f"**Key stats**: {len(posts)} posts analyzed • timeframe: {self.config.reddit_timeframe} • {total_comments} comments\n",
            summary,
            "",
        ]
        
        if posts:
            lines.append("### 🔗 **Top Posts**")
            for post in posts:
                lines.append(f"• [{post.title}](<{self.config.reddit_public_url}{post.permalink}>)")
        
        return "\n".join(lines)

    async def _send_long_message(
        self,
        channel: discord.abc.Messageable,
        content: str,
    ) -> None:
        """Send a message, splitting if it exceeds Discord's character limit."""
        max_length = self.config.discord_message_split_length
        
        if len(content) <= max_length:
            await channel.send(content)
            return
        
        # Split into chunks by lines
        lines = content.split("\n")
        current_chunk: list[str] = []
        current_length = 0
        
        for line in lines:
            line_length = len(line) + 1  # +1 for newline
            
            # If adding this line would exceed limit, send current chunk
            if current_length + line_length > max_length and current_chunk:
                await channel.send("\n".join(current_chunk))
                current_chunk = []
                current_length = 0
            
            # If single line is too long, truncate it
            if len(line) > max_length:
                line = line[:max_length - 3] + "..."
                line_length = len(line) + 1
            
            current_chunk.append(line)
            current_length += line_length
        
        # Send remaining content
        if current_chunk:
            await channel.send("\n".join(current_chunk))

    async def _handle_model_command(
        self,
        interaction: discord.Interaction,
        model: app_commands.Choice[str],
    ) -> None:
        """Handle the /model slash command."""
        user_id = str(interaction.user.id)
        
        # Find the selected model
        selected_model = next(
            (m for m in AVAILABLE_MODELS if m.codename == model.value),
            None,
        )
        
        if not selected_model:
            await interaction.response.send_message("❌ Invalid model selection")
            return
        
        # Update session
        session = self._get_user_session(user_id)
        session.model = selected_model.name
        await self._save_user_session(user_id, session)
        
        await interaction.response.send_message(
            f"✅ Model changed to **{selected_model.name}** ({selected_model.description})"
        )

    async def _handle_reasoning_command(
        self,
        interaction: discord.Interaction,
        level: app_commands.Choice[str],
    ) -> None:
        """Handle the /reasoning slash command."""
        user_id = str(interaction.user.id)
        level_value = level.value.lower()
        
        if not is_valid_reasoning_effort(level_value):
            await interaction.response.send_message("❌ Invalid reasoning effort level")
            return
        
        # Update session
        session = self._get_user_session(user_id)
        session.reasoning_effort = level_value
        await self._save_user_session(user_id, session)
        
        labels = {
            "minimal": "Minimal (fastest, cheapest)",
            "medium": "Medium (balanced)",
            "high": "High (most thorough, slowest)",
        }
        
        await interaction.response.send_message(
            f"✅ Reasoning effort set to **{labels.get(level_value, level_value)}**.\n\n"
            "All future analyses will use this level until you change it again."
        )

    async def _handle_history_command(self, interaction: discord.Interaction) -> None:
        """Handle the /history slash command."""
        user_id = str(interaction.user.id)
        session = self._get_user_session(user_id)
        
        if not session.history:
            await interaction.response.send_message(
                "📝 Your analysis history is empty. Use `/trend <subreddit>` to start analyzing!"
            )
            return
        
        lines = ["📝 **Your Analysis History**\n"]
        
        # Show recent history
        limit = self.config.history_display_limit
        start = max(0, len(session.history) - limit)
        
        for sub in session.history[start:]:
            lines.append(f"• r/{sub}")
        
        if len(session.history) > limit:
            lines.append(f"\n*Showing last {limit} of {len(session.history)} total analyses*")
        
        await interaction.response.send_message("\n".join(lines))

    async def _handle_clear_command(self, interaction: discord.Interaction) -> None:
        """Handle the /clear slash command."""
        user_id = str(interaction.user.id)
        session = self._get_user_session(user_id)
        
        session.history = []
        await self._save_user_session(user_id, session)
        
        await interaction.response.send_message("🗑️ **History cleared** successfully!")
