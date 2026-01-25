"""OAuth token caching for Reddit API."""

import asyncio
import json
import logging
import time
from dataclasses import asdict, dataclass
from pathlib import Path

import aiofiles

logger = logging.getLogger(__name__)

DEFAULT_CACHE_PATH = ".token_cache.json"
TOKEN_EXPIRY_BUFFER = 300  # Refresh 5 minutes before actual expiry


@dataclass
class CachedToken:
    """Cached OAuth token with expiry information."""

    access_token: str
    expires_at: float  # Unix timestamp

    def is_valid(self) -> bool:
        """Check if token is still valid (with buffer time)."""
        return time.time() < self.expires_at - TOKEN_EXPIRY_BUFFER


class TokenCache:
    """Async file-based OAuth token cache with lock to prevent race conditions."""

    def __init__(self, path: str = DEFAULT_CACHE_PATH):
        self.path = Path(path)
        self._lock = asyncio.Lock()

    async def get(self) -> CachedToken | None:
        """Load cached token if it exists and is valid."""
        async with self._lock:
            if not self.path.exists():
                logger.debug("Token cache file does not exist")
                return None

            try:
                async with aiofiles.open(self.path) as f:
                    data = json.loads(await f.read())
                    token = CachedToken(
                        access_token=data["access_token"],
                        expires_at=data["expires_at"],
                    )

                    if token.is_valid():
                        logger.debug(
                            "Using cached token (expires in %.0f seconds)",
                            token.expires_at - time.time(),
                        )
                        return token
                    else:
                        logger.debug("Cached token has expired")
                        return None

            except (json.JSONDecodeError, KeyError) as e:
                logger.warning(f"Failed to read token cache: {e}")
                return None

    async def set(self, access_token: str, expires_in: int) -> None:
        """Save a new token to the cache."""
        async with self._lock:
            cached = CachedToken(
                access_token=access_token,
                expires_at=time.time() + expires_in,
            )

            try:
                async with aiofiles.open(self.path, mode="w") as f:
                    await f.write(json.dumps(asdict(cached), indent=2))
                logger.info(f"Token cached successfully (expires in {expires_in}s)")
            except OSError as e:
                logger.error(f"Failed to write token cache: {e}")

    async def clear(self) -> None:
        """Remove the cached token."""
        if self.path.exists():
            self.path.unlink()
            logger.debug("Token cache cleared")
