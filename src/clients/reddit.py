"""Reddit API client with OAuth and rate limiting."""

import asyncio
import logging
from datetime import UTC, datetime, timedelta
from typing import Any

import httpx

from src.clients.token_cache import TokenCache
from src.models.reddit_types import Comment, Post
from src.services.rate_limiter import RateLimiter

logger = logging.getLogger(__name__)

REDDIT_AUTH_URL = "https://www.reddit.com/api/v1/access_token"
REDDIT_API_BASE = "https://oauth.reddit.com"
DEFAULT_TOKEN_EXPIRY = 86400  # 24 hours


class RedditClientError(Exception):
    """Base exception for Reddit client errors."""

    pass


class RateLimitExceededError(RedditClientError):
    """Raised when rate limit retries are exhausted."""

    pass


class RedditClient:
    """Async Reddit API client with OAuth token caching and rate limiting."""

    def __init__(
        self,
        client_id: str,
        client_secret: str,
        user_agent: str,
        rate_limiter: RateLimiter | None = None,
        token_cache: TokenCache | None = None,
    ):
        self.client_id = client_id
        self.client_secret = client_secret
        self.user_agent = user_agent
        self.rate_limiter = rate_limiter or RateLimiter()
        self.token_cache = token_cache or TokenCache()
        self._client: httpx.AsyncClient | None = None
        self._access_token: str | None = None
        self._token_lock = asyncio.Lock()

    async def __aenter__(self) -> "RedditClient":
        """Initialize HTTP client on context entry."""
        self._client = httpx.AsyncClient(
            headers={"User-Agent": self.user_agent},
            timeout=30.0,
        )
        return self

    async def __aexit__(self, _exc_type: Any, _exc_val: Any, _exc_tb: Any) -> None:
        """Close HTTP client on context exit."""
        if self._client:
            await self._client.aclose()
            self._client = None

    async def _ensure_client(self) -> httpx.AsyncClient:
        """Ensure HTTP client is initialized."""
        if self._client is None:
            self._client = httpx.AsyncClient(
                headers={"User-Agent": self.user_agent},
                timeout=30.0,
            )
        return self._client

    async def _ensure_token(self) -> str:
        """Ensure we have a valid OAuth token."""
        # Fast path: check in-memory token first (no lock needed)
        if self._access_token:
            cached = await self.token_cache.get()
            if cached and cached.is_valid():
                return self._access_token

        # Slow path: need to refresh token (use lock to prevent race)
        async with self._token_lock:
            # Double-check after acquiring lock
            cached = await self.token_cache.get()
            if cached and cached.is_valid():
                self._access_token = cached.access_token
                return self._access_token

            # Get new token
            logger.info("Obtaining new Reddit OAuth token")
            client = await self._ensure_client()

            response = await client.post(
                REDDIT_AUTH_URL,
                auth=(self.client_id, self.client_secret),
                data={"grant_type": "client_credentials"},
                headers={"User-Agent": self.user_agent},
            )
            response.raise_for_status()

            data = response.json()
            self._access_token = data["access_token"]
            expires_in = data.get("expires_in", DEFAULT_TOKEN_EXPIRY)

            # Cache the token
            await self.token_cache.set(self._access_token, expires_in)

            return self._access_token

    async def _request(
        self,
        method: str,
        endpoint: str,
        max_retries: int = 3,
        **kwargs: Any,
    ) -> Any:
        """Make an authenticated API request with rate limiting and retries."""
        client = await self._ensure_client()
        token = await self._ensure_token()

        url = f"{REDDIT_API_BASE}{endpoint}"
        headers = {
            "Authorization": f"Bearer {token}",
            "User-Agent": self.user_agent,
        }

        for attempt in range(max_retries):
            async with self.rate_limiter.acquire():
                try:
                    response = await client.request(
                        method,
                        url,
                        headers=headers,
                        **kwargs,
                    )

                    # Handle rate limiting
                    if response.status_code == 429:
                        retry_after = int(response.headers.get("Retry-After", 5))
                        logger.warning(
                            f"Rate limited by Reddit, waiting {retry_after}s "
                            f"(attempt {attempt + 1}/{max_retries})"
                        )
                        await asyncio.sleep(retry_after)
                        continue

                    # Handle token expiry
                    if response.status_code == 401:
                        logger.warning("Token expired, refreshing")
                        await self.token_cache.clear()
                        token = await self._ensure_token()
                        headers["Authorization"] = f"Bearer {token}"
                        continue

                    response.raise_for_status()
                    return response.json()

                except httpx.HTTPStatusError as e:
                    if attempt == max_retries - 1:
                        raise RedditClientError(f"Request failed: {e}") from e
                    logger.warning(f"Request error (attempt {attempt + 1}): {e}")
                    await asyncio.sleep(2**attempt)  # Exponential backoff

        raise RateLimitExceededError("Max retries exceeded for Reddit API request")

    async def fetch_top_posts(
        self,
        subreddit: str,
        limit: int = 7,
        time_filter: str = "day",
    ) -> list[Post]:
        """
        Fetch top posts from a subreddit for the specified time period.

        Args:
            subreddit: Name of the subreddit (without r/ prefix)
            limit: Maximum number of posts to fetch
            time_filter: Time filter (hour, day, week, month, year, all)

        Returns:
            List of Post objects
        """
        try:
            data = await self._request(
                "GET",
                f"/r/{subreddit}/top",
                params={"t": time_filter, "limit": limit},
            )
        except RedditClientError as e:
            logger.error(f"Failed to fetch posts from r/{subreddit}: {e}")
            return []

        posts: list[Post] = []
        for child in data.get("data", {}).get("children", []):
            post_data = child.get("data", {})

            # Filter posts from last 24 hours
            created_utc = post_data.get("created_utc", 0)
            post_time = datetime.fromtimestamp(created_utc, tz=UTC)
            cutoff_time = datetime.now(tz=UTC) - timedelta(hours=24)

            if post_time < cutoff_time:
                continue

            post = Post(
                title=post_data.get("title", ""),
                url=post_data.get("url", ""),
                score=post_data.get("score", 0),
                subreddit=post_data.get("subreddit", subreddit),
                author=post_data.get("author", "[deleted]"),
                selftext=post_data.get("selftext", ""),
                num_comments=post_data.get("num_comments", 0),
                permalink=post_data.get("permalink", ""),
            )
            posts.append(post)

        logger.debug(f"Fetched {len(posts)} posts from r/{subreddit}")
        return posts

    async def fetch_post_comments(
        self,
        post: Post,
        limit: int = 7,
    ) -> list[Comment]:
        """
        Fetch top comments for a post.

        Args:
            post: The Post object to fetch comments for
            limit: Maximum number of comments to fetch

        Returns:
            List of Comment objects
        """
        if not post.permalink:
            return []

        try:
            # Remove leading slash for endpoint
            endpoint = post.permalink.rstrip("/") + ".json"
            if endpoint.startswith("/"):
                endpoint = endpoint[1:]

            data = await self._request(
                "GET",
                f"/{endpoint}",
                params={"limit": limit, "sort": "top"},
            )
        except RedditClientError as e:
            logger.error(f"Failed to fetch comments for '{post.title[:50]}': {e}")
            return []

        comments: list[Comment] = []

        # Comments are in the second listing
        if len(data) > 1:
            comment_listing = data[1].get("data", {}).get("children", [])

            for child in comment_listing[:limit]:
                if child.get("kind") != "t1":  # Skip non-comment items
                    continue

                comment_data = child.get("data", {})
                body = comment_data.get("body", "")

                # Skip deleted/removed comments
                if body in ("[deleted]", "[removed]", ""):
                    continue

                comment = Comment(
                    body=body,
                    score=comment_data.get("score", 0),
                    author=comment_data.get("author", "[deleted]"),
                )
                comments.append(comment)

        logger.debug(f"Fetched {len(comments)} comments for '{post.title[:30]}...'")
        return comments

    async def fetch_posts_with_comments(
        self,
        subreddit: str,
        num_posts: int = 7,
        num_comments: int = 7,
    ) -> list[Post]:
        """
        Fetch top posts and their comments from a subreddit.

        Args:
            subreddit: Name of the subreddit
            num_posts: Number of posts to fetch
            num_comments: Number of comments per post

        Returns:
            List of Post objects with comments populated
        """
        posts = await self.fetch_top_posts(subreddit, limit=num_posts)

        # Fetch comments for all posts in parallel
        async def fetch_comments_for_post(post: Post) -> Post:
            post.comments = await self.fetch_post_comments(post, limit=num_comments)
            return post

        tasks = [fetch_comments_for_post(post) for post in posts]
        await asyncio.gather(*tasks)

        return posts

    async def close(self) -> None:
        """Close the HTTP client."""
        if self._client:
            await self._client.aclose()
            self._client = None
