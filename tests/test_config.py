"""Tests for configuration management."""

import os
from unittest.mock import patch

import pytest

from src.config import Config


# Mock load_dotenv to prevent loading .env file during tests
@pytest.fixture(autouse=True)
def mock_dotenv() -> None:
    with patch("src.config.load_dotenv"):
        yield


class TestConfigParseSubredditGroups:
    """Tests for subreddit group parsing from environment variables."""

    def test_parse_single_group(self) -> None:
        env = {"SUB_WORLD": "news,worldnews,europe"}

        with patch.dict(os.environ, env, clear=True):
            groups = Config._parse_subreddit_groups()

        assert "world" in groups
        assert groups["world"] == ["news", "worldnews", "europe"]

    def test_parse_multiple_groups(self) -> None:
        env = {
            "SUB_WORLD": "news,worldnews",
            "SUB_TECH": "programming,webdev",
            "SUB_SCIENCE": "science,askscience",
        }

        with patch.dict(os.environ, env, clear=True):
            groups = Config._parse_subreddit_groups()

        assert len(groups) == 3
        assert groups["world"] == ["news", "worldnews"]
        assert groups["tech"] == ["programming", "webdev"]
        assert groups["science"] == ["science", "askscience"]

    def test_group_name_lowercase(self) -> None:
        env = {"SUB_MYGROUP": "sub1,sub2"}

        with patch.dict(os.environ, env, clear=True):
            groups = Config._parse_subreddit_groups()

        assert "mygroup" in groups
        assert "MYGROUP" not in groups

    def test_strips_whitespace(self) -> None:
        env = {"SUB_TEST": "  news  ,  worldnews  ,  europe  "}

        with patch.dict(os.environ, env, clear=True):
            groups = Config._parse_subreddit_groups()

        assert groups["test"] == ["news", "worldnews", "europe"]

    def test_ignores_empty_subreddits(self) -> None:
        env = {"SUB_TEST": "news,,worldnews,,,europe,"}

        with patch.dict(os.environ, env, clear=True):
            groups = Config._parse_subreddit_groups()

        assert groups["test"] == ["news", "worldnews", "europe"]

    def test_ignores_non_sub_env_vars(self) -> None:
        env = {
            "SUB_WORLD": "news",
            "DISCORD_TOKEN": "secret",
            "OTHER_VAR": "value",
            "SUBMARINE": "not_a_group",
        }

        with patch.dict(os.environ, env, clear=True):
            groups = Config._parse_subreddit_groups()

        assert len(groups) == 1
        assert "world" in groups

    def test_empty_value_not_added(self) -> None:
        env = {"SUB_EMPTY": ""}

        with patch.dict(os.environ, env, clear=True):
            groups = Config._parse_subreddit_groups()

        assert "empty" not in groups

    def test_whitespace_only_value_not_added(self) -> None:
        env = {"SUB_WHITESPACE": "   ,   ,   "}

        with patch.dict(os.environ, env, clear=True):
            groups = Config._parse_subreddit_groups()

        assert "whitespace" not in groups

    def test_underscore_in_group_name(self) -> None:
        env = {"SUB_MY_GROUP_NAME": "sub1,sub2"}

        with patch.dict(os.environ, env, clear=True):
            groups = Config._parse_subreddit_groups()

        assert "my_group_name" in groups

    def test_numbers_in_group_name(self) -> None:
        env = {"SUB_GROUP123": "sub1,sub2"}

        with patch.dict(os.environ, env, clear=True):
            groups = Config._parse_subreddit_groups()

        assert "group123" in groups


class TestConfigFromEnv:
    """Tests for Config.from_env() method."""

    def test_missing_discord_token_raises(self) -> None:
        env = {
            "REDDIT_CLIENT_ID": "id",
            "REDDIT_CLIENT_SECRET": "secret",
            "ANTHROPIC_API_KEY": "key",
        }

        with patch.dict(os.environ, env, clear=True):
            with pytest.raises(ValueError, match="DISCORD_TOKEN"):
                Config.from_env()

    def test_missing_reddit_client_id_raises(self) -> None:
        env = {
            "DISCORD_TOKEN": "token",
            "REDDIT_CLIENT_SECRET": "secret",
            "ANTHROPIC_API_KEY": "key",
        }

        with patch.dict(os.environ, env, clear=True):
            with pytest.raises(ValueError, match="REDDIT_CLIENT_ID"):
                Config.from_env()

    def test_missing_reddit_client_secret_raises(self) -> None:
        env = {
            "DISCORD_TOKEN": "token",
            "REDDIT_CLIENT_ID": "id",
            "ANTHROPIC_API_KEY": "key",
        }

        with patch.dict(os.environ, env, clear=True):
            with pytest.raises(ValueError, match="REDDIT_CLIENT_SECRET"):
                Config.from_env()

    def test_missing_anthropic_api_key_raises(self) -> None:
        env = {
            "DISCORD_TOKEN": "token",
            "REDDIT_CLIENT_ID": "id",
            "REDDIT_CLIENT_SECRET": "secret",
        }

        with patch.dict(os.environ, env, clear=True):
            with pytest.raises(ValueError, match="ANTHROPIC_API_KEY"):
                Config.from_env()

    def test_loads_required_config(self) -> None:
        env = {
            "DISCORD_TOKEN": "my_discord_token",
            "REDDIT_CLIENT_ID": "my_client_id",
            "REDDIT_CLIENT_SECRET": "my_client_secret",
            "ANTHROPIC_API_KEY": "my_api_key",
            "SUB_TEST": "news,worldnews",
        }

        with patch.dict(os.environ, env, clear=True):
            config = Config.from_env()

        assert config.discord_token == "my_discord_token"
        assert config.reddit_client_id == "my_client_id"
        assert config.reddit_client_secret == "my_client_secret"
        assert config.anthropic_api_key == "my_api_key"
        assert "test" in config.subreddit_groups

    def test_default_values(self) -> None:
        env = {
            "DISCORD_TOKEN": "token",
            "REDDIT_CLIENT_ID": "id",
            "REDDIT_CLIENT_SECRET": "secret",
            "ANTHROPIC_API_KEY": "key",
        }

        with patch.dict(os.environ, env, clear=True):
            config = Config.from_env()

        # Check defaults are set (actual values may vary by .env.example)
        assert config.reddit_user_agent.startswith("subtrends:")
        assert config.num_posts == 7
        assert config.num_comments == 7
        assert config.max_concurrent_requests == 5
        assert config.requests_per_minute == 60
        assert config.default_model.startswith("claude-")

    def test_overrides_optional_config(self) -> None:
        env = {
            "DISCORD_TOKEN": "token",
            "REDDIT_CLIENT_ID": "id",
            "REDDIT_CLIENT_SECRET": "secret",
            "ANTHROPIC_API_KEY": "key",
            "REDDIT_USER_AGENT": "custom-agent:v2.0",
            "NUM_POSTS": "10",
            "NUM_COMMENTS": "15",
            "MAX_CONCURRENT_REQUESTS": "3",
            "REQUESTS_PER_MINUTE": "30",
            "DEFAULT_MODEL": "claude-sonnet-4-20250514",
        }

        with patch.dict(os.environ, env, clear=True):
            config = Config.from_env()

        assert config.reddit_user_agent == "custom-agent:v2.0"
        assert config.num_posts == 10
        assert config.num_comments == 15
        assert config.max_concurrent_requests == 3
        assert config.requests_per_minute == 30
        assert config.default_model == "claude-sonnet-4-20250514"
