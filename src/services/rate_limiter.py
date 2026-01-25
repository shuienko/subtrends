"""Rate limiting for API requests."""

import asyncio
import logging
import time
from collections import deque
from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager

logger = logging.getLogger(__name__)


class RateLimiter:
    """Async rate limiter with concurrent request limiting and requests-per-minute tracking."""

    def __init__(self, max_concurrent: int = 5, requests_per_minute: int = 60):
        """
        Initialize rate limiter.

        Args:
            max_concurrent: Maximum number of concurrent requests
            requests_per_minute: Maximum requests allowed per minute
        """
        self.max_concurrent = max_concurrent
        self.requests_per_minute = requests_per_minute
        self.semaphore = asyncio.Semaphore(max_concurrent)
        self.request_times: deque[float] = deque(maxlen=requests_per_minute)
        self._lock = asyncio.Lock()

    @asynccontextmanager
    async def acquire(self) -> AsyncGenerator[None, None]:
        """
        Acquire a rate-limited slot for making a request.

        This context manager ensures:
        1. No more than max_concurrent requests run simultaneously
        2. No more than requests_per_minute requests are made per minute
        """
        async with self.semaphore:
            await self._wait_for_rate_limit()
            yield

    async def _wait_for_rate_limit(self) -> None:
        """Wait if we've exceeded the requests-per-minute limit."""
        async with self._lock:
            now = time.monotonic()

            # Check if we need to wait for rate limit window
            if len(self.request_times) >= self.requests_per_minute:
                oldest = self.request_times[0]
                time_since_oldest = now - oldest

                if time_since_oldest < 60:
                    wait_time = 60 - time_since_oldest
                    logger.debug(f"Rate limit reached, waiting {wait_time:.2f}s")
                    await asyncio.sleep(wait_time)

            # Record this request
            self.request_times.append(time.monotonic())
