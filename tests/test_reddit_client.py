"""Tests for Reddit API client."""

import re
from datetime import UTC, datetime
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import httpx
import pytest
from pytest_httpx import HTTPXMock

from src.clients.reddit import (
    REDDIT_API_BASE,
    REDDIT_AUTH_URL,
    RedditClient,
)
from src.clients.token_cache import TokenCache
from src.models.reddit_types import Post
from src.services.rate_limiter import RateLimiter


@pytest.fixture
def mock_rate_limiter() -> MagicMock:
    limiter = MagicMock(spec=RateLimiter)
    # Create an async context manager mock
    limiter.acquire.return_value.__aenter__ = AsyncMock()
    limiter.acquire.return_value.__aexit__ = AsyncMock()
    return limiter


@pytest.fixture
def mock_token_cache(tmp_path: Path) -> TokenCache:
    return TokenCache(path=str(tmp_path / ".token_cache.json"))


@pytest.fixture
def client(mock_rate_limiter: MagicMock, mock_token_cache: TokenCache) -> RedditClient:
    return RedditClient(
        client_id="test_client_id",
        client_secret="test_client_secret",
        user_agent="test-agent:v1.0",
        rate_limiter=mock_rate_limiter,
        token_cache=mock_token_cache,
    )


def make_oauth_response() -> dict:
    """Create a mock OAuth token response."""
    return {
        "access_token": "test_access_token",
        "token_type": "bearer",
        "expires_in": 86400,
    }


def make_post_listing(posts: list[dict]) -> dict:
    """Create a mock Reddit listing response."""
    return {
        "kind": "Listing",
        "data": {
            "children": [{"kind": "t3", "data": post} for post in posts],
        },
    }


def make_post_data(
    title: str = "Test Post",
    subreddit: str = "test",
    score: int = 100,
    url: str = "https://example.com",
    created_utc: float | None = None,
) -> dict:
    """Create mock post data."""
    if created_utc is None:
        created_utc = datetime.now(tz=UTC).timestamp()
    return {
        "title": title,
        "subreddit": subreddit,
        "score": score,
        "url": url,
        "author": "testuser",
        "selftext": "",
        "num_comments": 5,
        "permalink": f"/r/{subreddit}/comments/abc123/{title.lower().replace(' ', '_')}/",
        "created_utc": created_utc,
    }


def make_comments_response(post_data: dict, comments: list[dict]) -> list:
    """Create mock comments response (includes post + comments)."""
    return [
        {"kind": "Listing", "data": {"children": [{"kind": "t3", "data": post_data}]}},
        {
            "kind": "Listing",
            "data": {"children": [{"kind": "t1", "data": comment} for comment in comments]},
        },
    ]


def make_comment_data(body: str = "Test comment", score: int = 10) -> dict:
    """Create mock comment data."""
    return {
        "body": body,
        "score": score,
        "author": "commenter",
    }


class TestRedditClientInit:
    """Tests for RedditClient initialization."""

    def test_initialization(self) -> None:
        client = RedditClient(
            client_id="my_id",
            client_secret="my_secret",
            user_agent="my-agent:v1.0",
        )

        assert client.client_id == "my_id"
        assert client.client_secret == "my_secret"
        assert client.user_agent == "my-agent:v1.0"

    def test_creates_default_rate_limiter(self) -> None:
        client = RedditClient(
            client_id="id",
            client_secret="secret",
            user_agent="agent",
        )

        assert client.rate_limiter is not None

    def test_creates_default_token_cache(self) -> None:
        client = RedditClient(
            client_id="id",
            client_secret="secret",
            user_agent="agent",
        )

        assert client.token_cache is not None


class TestRedditClientContextManager:
    """Tests for async context manager."""

    async def test_aenter_creates_client(self, client: RedditClient) -> None:
        async with client:
            assert client._client is not None

    async def test_aexit_closes_client(self, client: RedditClient) -> None:
        async with client:
            pass
        assert client._client is None


class TestFetchTopPosts:
    """Tests for fetch_top_posts method."""

    @pytest.mark.httpx_mock(can_send_already_matched_responses=True)
    async def test_fetch_top_posts_success(
        self,
        client: RedditClient,
        mock_token_cache: TokenCache,
        httpx_mock: HTTPXMock,
    ) -> None:
        # Setup token
        await mock_token_cache.set("test_token", 3600)

        # Mock posts request (use url pattern to match query params)
        post_data = make_post_data(title="Test Post", score=100)
        httpx_mock.add_response(
            url=re.compile(rf"{re.escape(REDDIT_API_BASE)}/r/test/top.*"),
            method="GET",
            json=make_post_listing([post_data]),
        )

        async with client:
            posts = await client.fetch_top_posts("test", limit=10)

        assert len(posts) == 1
        assert posts[0].title == "Test Post"
        assert posts[0].score == 100

    @pytest.mark.httpx_mock(can_send_already_matched_responses=True)
    async def test_fetch_top_posts_filters_old_posts(
        self,
        client: RedditClient,
        mock_token_cache: TokenCache,
        httpx_mock: HTTPXMock,
    ) -> None:
        await mock_token_cache.set("test_token", 3600)

        # One recent post, one old post
        recent_post = make_post_data(
            title="Recent",
            created_utc=datetime.now(tz=UTC).timestamp(),
        )
        old_post = make_post_data(
            title="Old",
            created_utc=datetime.now(tz=UTC).timestamp() - 100000,  # More than 24h ago
        )

        httpx_mock.add_response(
            url=re.compile(rf"{re.escape(REDDIT_API_BASE)}/r/test/top.*"),
            method="GET",
            json=make_post_listing([recent_post, old_post]),
        )

        async with client:
            posts = await client.fetch_top_posts("test")

        assert len(posts) == 1
        assert posts[0].title == "Recent"

    @pytest.mark.httpx_mock(can_send_already_matched_responses=True)
    async def test_fetch_top_posts_handles_error(
        self,
        client: RedditClient,
        mock_token_cache: TokenCache,
        httpx_mock: HTTPXMock,
    ) -> None:
        await mock_token_cache.set("test_token", 3600)

        # Return error for posts (match any method since retries might happen)
        httpx_mock.add_response(
            url=re.compile(rf"{re.escape(REDDIT_API_BASE)}/r/private/top.*"),
            method="GET",
            status_code=403,
        )

        async with client:
            posts = await client.fetch_top_posts("private")

        # Should return empty list on error
        assert posts == []


class TestFetchPostComments:
    """Tests for fetch_post_comments method."""

    @pytest.mark.httpx_mock(can_send_already_matched_responses=True)
    async def test_fetch_comments_success(
        self,
        client: RedditClient,
        mock_token_cache: TokenCache,
        httpx_mock: HTTPXMock,
    ) -> None:
        await mock_token_cache.set("test_token", 3600)

        post_data = make_post_data()
        comments = [
            make_comment_data(body="First comment", score=50),
            make_comment_data(body="Second comment", score=30),
        ]

        httpx_mock.add_response(
            url=re.compile(rf"{re.escape(REDDIT_API_BASE)}/r/test/comments/.*"),
            method="GET",
            json=make_comments_response(post_data, comments),
        )

        post = Post(
            title="Test",
            url="https://example.com",
            score=100,
            subreddit="test",
            author="user",
            permalink="/r/test/comments/abc123/test/",
        )

        async with client:
            fetched_comments = await client.fetch_post_comments(post, limit=5)

        assert len(fetched_comments) == 2
        assert fetched_comments[0].body == "First comment"
        assert fetched_comments[1].body == "Second comment"

    @pytest.mark.httpx_mock(can_send_already_matched_responses=True)
    async def test_fetch_comments_skips_deleted(
        self,
        client: RedditClient,
        mock_token_cache: TokenCache,
        httpx_mock: HTTPXMock,
    ) -> None:
        await mock_token_cache.set("test_token", 3600)

        post_data = make_post_data()
        comments = [
            make_comment_data(body="Valid comment"),
            {"body": "[deleted]", "score": 10, "author": "[deleted]"},
            {"body": "[removed]", "score": 5, "author": "[deleted]"},
            {"body": "", "score": 0, "author": "user"},
        ]

        httpx_mock.add_response(
            url=re.compile(rf"{re.escape(REDDIT_API_BASE)}/r/test/comments/.*"),
            method="GET",
            json=make_comments_response(post_data, comments),
        )

        post = Post(
            title="Test",
            url="",
            score=100,
            subreddit="test",
            author="user",
            permalink="/r/test/comments/abc/test/",
        )

        async with client:
            fetched_comments = await client.fetch_post_comments(post)

        assert len(fetched_comments) == 1
        assert fetched_comments[0].body == "Valid comment"

    async def test_fetch_comments_no_permalink(
        self,
        client: RedditClient,
    ) -> None:
        post = Post(
            title="Test",
            url="",
            score=100,
            subreddit="test",
            author="user",
            permalink="",  # No permalink
        )

        async with client:
            comments = await client.fetch_post_comments(post)

        assert comments == []


class TestOAuthToken:
    """Tests for OAuth token handling."""

    @pytest.mark.httpx_mock(can_send_already_matched_responses=True)
    async def test_fetches_new_token_when_missing(
        self,
        client: RedditClient,
        httpx_mock: HTTPXMock,
    ) -> None:
        httpx_mock.add_response(
            url=REDDIT_AUTH_URL,
            method="POST",
            json=make_oauth_response(),
        )

        httpx_mock.add_response(
            url=re.compile(rf"{re.escape(REDDIT_API_BASE)}/r/test/top.*"),
            method="GET",
            json=make_post_listing([make_post_data()]),
        )

        async with client:
            await client.fetch_top_posts("test")

        # Should have made OAuth request
        requests = httpx_mock.get_requests()
        auth_requests = [r for r in requests if "access_token" in str(r.url)]
        assert len(auth_requests) == 1

    @pytest.mark.httpx_mock(can_send_already_matched_responses=True)
    async def test_reuses_cached_token(
        self,
        client: RedditClient,
        mock_token_cache: TokenCache,
        httpx_mock: HTTPXMock,
    ) -> None:
        # Pre-set a valid token
        await mock_token_cache.set("cached_token", 3600)

        httpx_mock.add_response(
            url=re.compile(rf"{re.escape(REDDIT_API_BASE)}/r/test/top.*"),
            method="GET",
            json=make_post_listing([make_post_data()]),
        )

        async with client:
            await client.fetch_top_posts("test")

        # Should NOT have made OAuth request
        requests = httpx_mock.get_requests()
        auth_requests = [r for r in requests if "access_token" in str(r.url)]
        assert len(auth_requests) == 0


class TestRateLimitHandling:
    """Tests for rate limit handling."""

    @pytest.mark.httpx_mock(can_send_already_matched_responses=True)
    async def test_retries_on_rate_limit(
        self,
        client: RedditClient,
        mock_token_cache: TokenCache,
        httpx_mock: HTTPXMock,
    ) -> None:
        await mock_token_cache.set("test_token", 3600)

        # First request returns 429, then succeeds
        call_count = [0]

        def response_callback(request: httpx.Request) -> httpx.Response:
            call_count[0] += 1
            if call_count[0] == 1:
                return httpx.Response(
                    status_code=429,
                    headers={"Retry-After": "0"},
                )
            return httpx.Response(
                status_code=200,
                json=make_post_listing([make_post_data()]),
            )

        httpx_mock.add_callback(
            response_callback,
            url=re.compile(rf"{re.escape(REDDIT_API_BASE)}/r/test/top.*"),
            method="GET",
        )

        async with client:
            posts = await client.fetch_top_posts("test")

        assert len(posts) == 1


class TestFetchPostsWithComments:
    """Tests for fetch_posts_with_comments method."""

    @pytest.mark.httpx_mock(can_send_already_matched_responses=True)
    async def test_fetches_posts_and_comments(
        self,
        client: RedditClient,
        mock_token_cache: TokenCache,
        httpx_mock: HTTPXMock,
    ) -> None:
        await mock_token_cache.set("test_token", 3600)

        post_data = make_post_data(title="Test Post")
        httpx_mock.add_response(
            url=re.compile(rf"{re.escape(REDDIT_API_BASE)}/r/test/top.*"),
            method="GET",
            json=make_post_listing([post_data]),
        )

        # Comments endpoint
        comments = [make_comment_data(body="A comment")]
        httpx_mock.add_response(
            url=re.compile(rf"{re.escape(REDDIT_API_BASE)}/r/test/comments/.*"),
            method="GET",
            json=make_comments_response(post_data, comments),
        )

        async with client:
            posts = await client.fetch_posts_with_comments("test", num_posts=5, num_comments=3)

        assert len(posts) == 1
        assert posts[0].title == "Test Post"
        assert len(posts[0].comments) == 1
        assert posts[0].comments[0].body == "A comment"
