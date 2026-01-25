"""Tests for OAuth token caching."""

import json
import time
from pathlib import Path

import pytest

from src.clients.token_cache import TOKEN_EXPIRY_BUFFER, CachedToken, TokenCache


class TestCachedToken:
    """Tests for CachedToken dataclass."""

    def test_token_creation(self) -> None:
        token = CachedToken(access_token="test_token", expires_at=time.time() + 3600)

        assert token.access_token == "test_token"
        assert token.expires_at > time.time()

    def test_is_valid_with_future_expiry(self) -> None:
        # Token expires in 1 hour (well beyond the buffer)
        token = CachedToken(
            access_token="test_token",
            expires_at=time.time() + 3600,
        )

        assert token.is_valid() is True

    def test_is_valid_with_past_expiry(self) -> None:
        # Token already expired
        token = CachedToken(
            access_token="test_token",
            expires_at=time.time() - 100,
        )

        assert token.is_valid() is False

    def test_is_valid_within_buffer_is_invalid(self) -> None:
        # Token expires within buffer time (5 minutes)
        token = CachedToken(
            access_token="test_token",
            expires_at=time.time() + TOKEN_EXPIRY_BUFFER - 10,
        )

        assert token.is_valid() is False

    def test_is_valid_just_beyond_buffer(self) -> None:
        # Token expires just beyond the buffer
        token = CachedToken(
            access_token="test_token",
            expires_at=time.time() + TOKEN_EXPIRY_BUFFER + 10,
        )

        assert token.is_valid() is True


class TestTokenCache:
    """Tests for TokenCache."""

    @pytest.fixture
    def cache_path(self, tmp_path: Path) -> Path:
        return tmp_path / ".test_token_cache.json"

    @pytest.fixture
    def cache(self, cache_path: Path) -> TokenCache:
        return TokenCache(path=str(cache_path))

    async def test_get_returns_none_when_file_missing(
        self, cache: TokenCache, cache_path: Path
    ) -> None:
        assert not cache_path.exists()

        result = await cache.get()

        assert result is None

    async def test_set_creates_cache_file(self, cache: TokenCache, cache_path: Path) -> None:
        assert not cache_path.exists()

        await cache.set("my_token", expires_in=3600)

        assert cache_path.exists()
        data = json.loads(cache_path.read_text())
        assert data["access_token"] == "my_token"
        assert data["expires_at"] > time.time()

    async def test_get_returns_valid_token(self, cache: TokenCache, cache_path: Path) -> None:
        await cache.set("valid_token", expires_in=3600)

        result = await cache.get()

        assert result is not None
        assert result.access_token == "valid_token"
        assert result.is_valid()

    async def test_get_returns_none_for_expired_token(
        self, cache: TokenCache, cache_path: Path
    ) -> None:
        # Write an expired token directly
        expired_data = {
            "access_token": "expired_token",
            "expires_at": time.time() - 1000,
        }
        cache_path.write_text(json.dumps(expired_data))

        result = await cache.get()

        assert result is None

    async def test_get_returns_none_for_token_within_buffer(
        self, cache: TokenCache, cache_path: Path
    ) -> None:
        # Write a token that expires within the buffer period
        almost_expired_data = {
            "access_token": "almost_expired",
            "expires_at": time.time() + TOKEN_EXPIRY_BUFFER - 10,
        }
        cache_path.write_text(json.dumps(almost_expired_data))

        result = await cache.get()

        assert result is None

    async def test_get_handles_invalid_json(self, cache: TokenCache, cache_path: Path) -> None:
        cache_path.write_text("not valid json {{{")

        result = await cache.get()

        assert result is None

    async def test_get_handles_missing_fields(self, cache: TokenCache, cache_path: Path) -> None:
        cache_path.write_text('{"access_token": "token"}')  # Missing expires_at

        result = await cache.get()

        assert result is None

    async def test_clear_removes_file(self, cache: TokenCache, cache_path: Path) -> None:
        await cache.set("token_to_clear", expires_in=3600)
        assert cache_path.exists()

        await cache.clear()

        assert not cache_path.exists()

    async def test_clear_handles_missing_file(self, cache: TokenCache, cache_path: Path) -> None:
        assert not cache_path.exists()

        # Should not raise
        await cache.clear()

    async def test_set_overwrites_existing(self, cache: TokenCache, cache_path: Path) -> None:
        await cache.set("first_token", expires_in=3600)
        await cache.set("second_token", expires_in=7200)

        result = await cache.get()

        assert result is not None
        assert result.access_token == "second_token"

    async def test_concurrent_access(self, cache: TokenCache, cache_path: Path) -> None:
        """Test that concurrent reads and writes don't corrupt data."""
        import asyncio

        async def writer(token_id: int) -> None:
            await cache.set(f"token_{token_id}", expires_in=3600)

        async def reader() -> CachedToken | None:
            return await cache.get()

        # Run concurrent operations
        await asyncio.gather(
            writer(1),
            reader(),
            writer(2),
            reader(),
            writer(3),
        )

        # Final state should be valid
        result = await cache.get()
        assert result is not None
        assert result.access_token.startswith("token_")
