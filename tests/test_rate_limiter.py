"""Tests for rate limiting."""

import asyncio
import time

from src.services.rate_limiter import RateLimiter


class TestRateLimiter:
    """Tests for RateLimiter."""

    async def test_default_initialization(self) -> None:
        limiter = RateLimiter()

        assert limiter.max_concurrent == 5
        assert limiter.requests_per_minute == 60

    async def test_custom_initialization(self) -> None:
        limiter = RateLimiter(max_concurrent=10, requests_per_minute=30)

        assert limiter.max_concurrent == 10
        assert limiter.requests_per_minute == 30

    async def test_acquire_single_request(self) -> None:
        limiter = RateLimiter(max_concurrent=5, requests_per_minute=60)

        async with limiter.acquire():
            # Should acquire without blocking
            pass

        assert len(limiter.request_times) == 1

    async def test_acquire_multiple_requests(self) -> None:
        limiter = RateLimiter(max_concurrent=5, requests_per_minute=60)

        for _ in range(5):
            async with limiter.acquire():
                pass

        assert len(limiter.request_times) == 5

    async def test_concurrent_limit_respected(self) -> None:
        limiter = RateLimiter(max_concurrent=2, requests_per_minute=100)
        concurrent_count = 0
        max_concurrent_observed = 0

        async def worker() -> None:
            nonlocal concurrent_count, max_concurrent_observed
            async with limiter.acquire():
                concurrent_count += 1
                max_concurrent_observed = max(max_concurrent_observed, concurrent_count)
                await asyncio.sleep(0.05)  # Simulate work
                concurrent_count -= 1

        # Run 10 workers concurrently
        await asyncio.gather(*[worker() for _ in range(10)])

        assert max_concurrent_observed <= 2

    async def test_request_times_tracked(self) -> None:
        limiter = RateLimiter(max_concurrent=5, requests_per_minute=60)

        for _ in range(3):
            async with limiter.acquire():
                pass

        assert len(limiter.request_times) == 3
        # Request times should be monotonically increasing
        times = list(limiter.request_times)
        assert times == sorted(times)

    async def test_request_times_deque_maxlen(self) -> None:
        limiter = RateLimiter(max_concurrent=5, requests_per_minute=3)

        for _ in range(5):
            async with limiter.acquire():
                pass

        # Deque should only keep last 3 (requests_per_minute)
        assert len(limiter.request_times) == 3

    async def test_rate_limit_waits_when_exceeded(self) -> None:
        # Use very small window for faster test
        limiter = RateLimiter(max_concurrent=10, requests_per_minute=3)

        # Fill up the rate limit quickly
        for _ in range(3):
            async with limiter.acquire():
                pass

        # Manually set old request times to simulate time passing
        # Set oldest request to 59 seconds ago (should trigger 1 second wait)
        old_time = time.monotonic() - 59
        limiter.request_times.clear()
        for _ in range(3):
            limiter.request_times.append(old_time)

        start = time.monotonic()
        async with limiter.acquire():
            pass
        elapsed = time.monotonic() - start

        # Should have waited approximately 1 second
        assert elapsed >= 0.9

    async def test_concurrent_acquire_thread_safe(self) -> None:
        limiter = RateLimiter(max_concurrent=3, requests_per_minute=100)
        results: list[int] = []

        async def worker(task_id: int) -> None:
            async with limiter.acquire():
                results.append(task_id)
                await asyncio.sleep(0.01)

        # Run many concurrent workers
        await asyncio.gather(*[worker(i) for i in range(20)])

        # All workers should complete
        assert len(results) == 20
        # Request times should be tracked (up to max)
        assert len(limiter.request_times) <= limiter.requests_per_minute
