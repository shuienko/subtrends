"""Configuration management for SubTrends bot."""

import os
from dataclasses import dataclass


@dataclass
class Config:
    """Configuration loaded from environment variables."""

    # OpenAI API settings
    openai_api_endpoint: str
    openai_request_timeout: float  # seconds
    openai_requests_per_minute: int
    openai_burst_size: int
    summary_header: str
    openai_api_key: str

    # Reddit API settings
    reddit_base_url: str
    reddit_auth_url: str
    reddit_post_limit: int
    reddit_comment_limit: int
    reddit_timeframe: str
    reddit_requests_per_second: int
    reddit_burst_size: int
    reddit_token_expiry_buffer: float  # seconds
    reddit_token_file_path: str
    reddit_request_timeout: float  # seconds
    reddit_concurrent_requests: int
    reddit_user_agent: str
    reddit_public_url: str
    reddit_client_id: str
    reddit_client_secret: str

    # Discord Bot settings
    discord_message_split_length: int
    legacy_command_prefix: str
    session_file_path: str
    history_init_capacity: int
    history_display_limit: int

    # Application settings
    shutdown_timeout: float  # seconds


def _get_env(key: str, default: str = "") -> str:
    """Get environment variable with fallback."""
    return os.environ.get(key, default)


def _get_env_int(key: str, default: int) -> int:
    """Get environment variable as integer with fallback."""
    value = _get_env(key, "")
    if value:
        try:
            return int(value)
        except ValueError:
            pass
    return default


def _get_env_float(key: str, default: float) -> float:
    """Get environment variable as float with fallback."""
    value = _get_env(key, "")
    if value:
        try:
            return float(value)
        except ValueError:
            pass
    return default


def _parse_duration(value: str, default: float) -> float:
    """Parse duration string (e.g., '120s', '5m') to seconds."""
    if not value:
        return default
    
    value = value.strip().lower()
    try:
        if value.endswith("s"):
            return float(value[:-1])
        elif value.endswith("m"):
            return float(value[:-1]) * 60
        elif value.endswith("h"):
            return float(value[:-1]) * 3600
        else:
            return float(value)
    except ValueError:
        return default


def _get_env_duration(key: str, default: float) -> float:
    """Get environment variable as duration in seconds."""
    value = _get_env(key, "")
    return _parse_duration(value, default)


def load_config() -> Config:
    """Load configuration from environment variables."""
    return Config(
        # OpenAI
        openai_api_endpoint=_get_env("OPENAI_API_ENDPOINT", "https://api.openai.com/v1/chat/completions"),
        openai_request_timeout=_get_env_duration("OPENAI_REQUEST_TIMEOUT", 120.0),
        openai_requests_per_minute=_get_env_int("OPENAI_REQUESTS_PER_MINUTE", 10),
        openai_burst_size=_get_env_int("OPENAI_BURST_SIZE", 3),
        summary_header=_get_env("SUMMARY_HEADER", "📱 *REDDIT PULSE* 📱\n\n"),
        openai_api_key=_get_env("OPENAI_API_KEY", ""),

        # Reddit
        reddit_base_url=_get_env("REDDIT_BASE_URL", "https://oauth.reddit.com"),
        reddit_auth_url=_get_env("REDDIT_AUTH_URL", "https://www.reddit.com/api/v1/access_token"),
        reddit_post_limit=_get_env_int("REDDIT_POST_LIMIT", 7),
        reddit_comment_limit=_get_env_int("REDDIT_COMMENT_LIMIT", 7),
        reddit_timeframe=_get_env("REDDIT_TIMEFRAME", "day"),
        reddit_requests_per_second=_get_env_int("REDDIT_REQUESTS_PER_SECOND", 1),
        reddit_burst_size=_get_env_int("REDDIT_BURST_SIZE", 5),
        reddit_token_expiry_buffer=_get_env_duration("REDDIT_TOKEN_EXPIRY_BUFFER", 300.0),  # 5 minutes
        reddit_token_file_path=_get_env("REDDIT_TOKEN_FILE_PATH", "data/reddit_token.json"),
        reddit_request_timeout=_get_env_duration("REDDIT_REQUEST_TIMEOUT", 10.0),
        reddit_concurrent_requests=_get_env_int("REDDIT_CONCURRENT_REQUESTS", 3),
        reddit_user_agent=_get_env("REDDIT_USER_AGENT", "SubTrends/1.0"),
        reddit_public_url=_get_env("REDDIT_PUBLIC_URL", "https://reddit.com"),
        reddit_client_id=_get_env("REDDIT_CLIENT_ID", ""),
        reddit_client_secret=_get_env("REDDIT_CLIENT_SECRET", ""),

        # Discord Bot
        session_file_path=_get_env("SESSION_FILE_PATH", "data/sessions.json"),
        history_init_capacity=_get_env_int("HISTORY_INIT_CAPACITY", 50),
        history_display_limit=_get_env_int("HISTORY_DISPLAY_LIMIT", 25),
        discord_message_split_length=_get_env_int("DISCORD_MESSAGE_SPLIT_LENGTH", 1900),
        legacy_command_prefix=_get_env("LEGACY_COMMAND_PREFIX", "!trend "),

        # Application
        shutdown_timeout=_get_env_duration("SHUTDOWN_TIMEOUT", 5.0),
    )


# Global config instance
app_config: Config | None = None


def get_config() -> Config:
    """Get the global config instance, loading it if necessary."""
    global app_config
    if app_config is None:
        app_config = load_config()
    return app_config
