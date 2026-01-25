"""Tests for news fetching service."""

from unittest.mock import AsyncMock, MagicMock

import pytest

from src.models.reddit_types import Comment, Post
from src.services.news_fetcher import NewsFetcher


@pytest.fixture
def mock_reddit_client() -> MagicMock:
    client = MagicMock()
    client.fetch_posts_with_comments = AsyncMock(return_value=[])
    return client


@pytest.fixture
def subreddit_groups() -> dict[str, list[str]]:
    return {
        "world": ["news", "worldnews", "europe"],
        "tech": ["programming", "webdev"],
    }


@pytest.fixture
def fetcher(mock_reddit_client: MagicMock, subreddit_groups: dict[str, list[str]]) -> NewsFetcher:
    return NewsFetcher(
        reddit_client=mock_reddit_client,
        subreddit_groups=subreddit_groups,
        num_posts=5,
        num_comments=3,
    )


class TestNewsFetcherInit:
    """Tests for NewsFetcher initialization."""

    def test_initialization(
        self, mock_reddit_client: MagicMock, subreddit_groups: dict[str, list[str]]
    ) -> None:
        fetcher = NewsFetcher(
            reddit_client=mock_reddit_client,
            subreddit_groups=subreddit_groups,
            num_posts=10,
            num_comments=5,
        )

        assert fetcher.reddit_client is mock_reddit_client
        assert fetcher.subreddit_groups == subreddit_groups
        assert fetcher.num_posts == 10
        assert fetcher.num_comments == 5


class TestGetAvailableGroups:
    """Tests for get_available_groups method."""

    def test_returns_copy_of_groups(self, fetcher: NewsFetcher) -> None:
        groups = fetcher.get_available_groups()

        assert groups == {
            "world": ["news", "worldnews", "europe"],
            "tech": ["programming", "webdev"],
        }
        # Should be a copy, not the original
        groups["world"] = ["modified"]
        assert fetcher.subreddit_groups["world"] == ["news", "worldnews", "europe"]

    def test_empty_groups(self, mock_reddit_client: MagicMock) -> None:
        fetcher = NewsFetcher(
            reddit_client=mock_reddit_client,
            subreddit_groups={},
        )

        groups = fetcher.get_available_groups()

        assert groups == {}


class TestFetchGroup:
    """Tests for fetch_group method."""

    async def test_fetch_group_returns_subreddit_group(
        self, fetcher: NewsFetcher, mock_reddit_client: MagicMock
    ) -> None:
        mock_reddit_client.fetch_posts_with_comments.return_value = []

        result = await fetcher.fetch_group("world")

        assert result.name == "world"
        assert result.subreddits == ["news", "worldnews", "europe"]

    async def test_fetch_group_case_insensitive(
        self, fetcher: NewsFetcher, mock_reddit_client: MagicMock
    ) -> None:
        mock_reddit_client.fetch_posts_with_comments.return_value = []

        result = await fetcher.fetch_group("WORLD")

        assert result.name == "world"

    async def test_fetch_group_unknown_raises(self, fetcher: NewsFetcher) -> None:
        with pytest.raises(ValueError, match="Unknown group"):
            await fetcher.fetch_group("nonexistent")

    async def test_fetch_group_calls_client_for_all_subreddits(
        self, fetcher: NewsFetcher, mock_reddit_client: MagicMock
    ) -> None:
        mock_reddit_client.fetch_posts_with_comments.return_value = []

        await fetcher.fetch_group("world")

        assert mock_reddit_client.fetch_posts_with_comments.call_count == 3
        call_args = [
            call.kwargs["subreddit"]
            for call in mock_reddit_client.fetch_posts_with_comments.call_args_list
        ]
        assert set(call_args) == {"news", "worldnews", "europe"}

    async def test_fetch_group_passes_num_posts_and_comments(
        self, fetcher: NewsFetcher, mock_reddit_client: MagicMock
    ) -> None:
        mock_reddit_client.fetch_posts_with_comments.return_value = []

        await fetcher.fetch_group("tech")

        call = mock_reddit_client.fetch_posts_with_comments.call_args_list[0]
        assert call.kwargs["num_posts"] == 5
        assert call.kwargs["num_comments"] == 3

    async def test_fetch_group_aggregates_posts(
        self, fetcher: NewsFetcher, mock_reddit_client: MagicMock
    ) -> None:
        # Return different posts for different subreddits
        def mock_fetch(subreddit: str, **kwargs: int) -> list[Post]:
            return [
                Post(
                    title=f"Post from {subreddit}",
                    url=f"https://{subreddit}.example.com",
                    score=100,
                    subreddit=subreddit,
                    author="user",
                )
            ]

        mock_reddit_client.fetch_posts_with_comments.side_effect = mock_fetch

        result = await fetcher.fetch_group("world")

        assert len(result.posts) == 3
        subreddits = {p.subreddit for p in result.posts}
        assert subreddits == {"news", "worldnews", "europe"}

    async def test_fetch_group_sorts_posts_by_score(
        self, fetcher: NewsFetcher, mock_reddit_client: MagicMock
    ) -> None:
        posts = [
            Post(title="Low", url="", score=10, subreddit="news", author="user"),
            Post(title="High", url="", score=1000, subreddit="news", author="user"),
            Post(title="Medium", url="", score=100, subreddit="news", author="user"),
        ]
        mock_reddit_client.fetch_posts_with_comments.return_value = posts

        result = await fetcher.fetch_group("tech")

        scores = [p.score for p in result.posts]
        assert scores == sorted(scores, reverse=True)

    async def test_fetch_group_handles_client_exception(
        self, fetcher: NewsFetcher, mock_reddit_client: MagicMock
    ) -> None:
        # One subreddit fails, others succeed
        call_count = 0

        async def mock_fetch(subreddit: str, **kwargs: int) -> list[Post]:
            nonlocal call_count
            call_count += 1
            if subreddit == "worldnews":
                raise Exception("Network error")
            return [
                Post(
                    title=f"Post from {subreddit}",
                    url="",
                    score=100,
                    subreddit=subreddit,
                    author="user",
                )
            ]

        mock_reddit_client.fetch_posts_with_comments.side_effect = mock_fetch

        result = await fetcher.fetch_group("world")

        # Should have posts from 2 successful subreddits
        assert len(result.posts) == 2
        subreddits = {p.subreddit for p in result.posts}
        assert "worldnews" not in subreddits

    async def test_fetch_group_includes_comments(
        self, fetcher: NewsFetcher, mock_reddit_client: MagicMock
    ) -> None:
        post_with_comments = Post(
            title="Post with comments",
            url="",
            score=100,
            subreddit="news",
            author="user",
            comments=[
                Comment(body="Great post!", score=50, author="commenter1"),
                Comment(body="Interesting", score=30, author="commenter2"),
            ],
        )
        mock_reddit_client.fetch_posts_with_comments.return_value = [post_with_comments]

        result = await fetcher.fetch_group("tech")

        assert len(result.posts[0].comments) == 2
