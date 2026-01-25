"""Configuration management for SubTrends."""

import logging
import os
import re
from dataclasses import dataclass, field

from dotenv import load_dotenv

logger = logging.getLogger(__name__)


@dataclass
class Config:
    """Application configuration loaded from environment variables."""

    # Discord
    discord_token: str

    # Reddit
    reddit_client_id: str
    reddit_client_secret: str
    reddit_user_agent: str

    # Anthropic
    anthropic_api_key: str

    # Subreddit groups (parsed from SUB_* env vars)
    subreddit_groups: dict[str, list[str]] = field(default_factory=dict)

    # Data fetching
    num_posts: int = 7
    num_comments: int = 7

    # Rate limiting
    max_concurrent_requests: int = 5
    requests_per_minute: int = 60

    @classmethod
    def from_env(cls) -> "Config":
        """Load configuration from environment variables."""
        load_dotenv()

        # Required variables
        discord_token = os.environ.get("DISCORD_TOKEN")
        if not discord_token:
            raise ValueError("DISCORD_TOKEN environment variable is required")

        reddit_client_id = os.environ.get("REDDIT_CLIENT_ID")
        if not reddit_client_id:
            raise ValueError("REDDIT_CLIENT_ID environment variable is required")

        reddit_client_secret = os.environ.get("REDDIT_CLIENT_SECRET")
        if not reddit_client_secret:
            raise ValueError("REDDIT_CLIENT_SECRET environment variable is required")

        reddit_user_agent = os.environ.get("REDDIT_USER_AGENT", "subtrends:v1.0")

        anthropic_api_key = os.environ.get("ANTHROPIC_API_KEY")
        if not anthropic_api_key:
            raise ValueError("ANTHROPIC_API_KEY environment variable is required")

        # Parse subreddit groups
        subreddit_groups = cls._parse_subreddit_groups()
        if not subreddit_groups:
            logger.warning("No subreddit groups found (SUB_* environment variables)")

        # Optional config
        num_posts = int(os.environ.get("NUM_POSTS", "7"))
        num_comments = int(os.environ.get("NUM_COMMENTS", "7"))
        max_concurrent = int(os.environ.get("MAX_CONCURRENT_REQUESTS", "5"))
        requests_per_min = int(os.environ.get("REQUESTS_PER_MINUTE", "60"))

        return cls(
            discord_token=discord_token,
            reddit_client_id=reddit_client_id,
            reddit_client_secret=reddit_client_secret,
            reddit_user_agent=reddit_user_agent,
            anthropic_api_key=anthropic_api_key,
            subreddit_groups=subreddit_groups,
            num_posts=num_posts,
            num_comments=num_comments,
            max_concurrent_requests=max_concurrent,
            requests_per_minute=requests_per_min,
        )

    @staticmethod
    def _parse_subreddit_groups() -> dict[str, list[str]]:
        """Parse SUB_<GROUPNAME>=sub1,sub2,sub3 from environment."""
        groups: dict[str, list[str]] = {}
        pattern = re.compile(r"^SUB_([A-Z0-9_]+)$")

        for key, value in os.environ.items():
            match = pattern.match(key)
            if match:
                group_name = match.group(1).lower()
                subreddits = [s.strip() for s in value.split(",") if s.strip()]
                if subreddits:
                    groups[group_name] = subreddits
                    logger.info(f"Found subreddit group '{group_name}': {subreddits}")

        return groups
