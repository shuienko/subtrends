"""Tests for summarization service."""

from unittest.mock import AsyncMock, MagicMock

import pytest

from src.models.reddit_types import Comment, Post
from src.services.summarizer import MAX_CONTENT_LENGTH, Summarizer


@pytest.fixture
def mock_anthropic_client() -> MagicMock:
    client = MagicMock()
    client.generate = AsyncMock(return_value="Mock response")
    return client


@pytest.fixture
def summarizer(mock_anthropic_client: MagicMock) -> Summarizer:
    return Summarizer(client=mock_anthropic_client)


@pytest.fixture
def sample_posts() -> list[Post]:
    return [
        Post(
            title="Breaking News: Major Event",
            url="https://example.com/news1",
            score=1000,
            subreddit="news",
            author="reporter",
            selftext="This is the content of the news post.",
            permalink="/r/news/comments/abc123/breaking_news/",
            comments=[
                Comment(body="This is huge!", score=100, author="user1"),
                Comment(body="Can't believe it", score=50, author="user2"),
            ],
        ),
        Post(
            title="Technology Update",
            url="https://example.com/tech",
            score=500,
            subreddit="technology",
            author="techwriter",
            permalink="/r/technology/comments/xyz789/tech_update/",
            comments=[
                Comment(body="Interesting development", score=75, author="dev1"),
            ],
        ),
    ]


class TestSummarizer:
    """Tests for Summarizer class."""

    async def test_summarize_calls_client_with_prompt(
        self,
        summarizer: Summarizer,
        mock_anthropic_client: MagicMock,
        sample_posts: list[Post],
    ) -> None:
        await summarizer.summarize("world", sample_posts)

        mock_anthropic_client.generate.assert_called_once()
        call_kwargs = mock_anthropic_client.generate.call_args.kwargs
        assert "prompt" in call_kwargs
        assert "system" in call_kwargs
        assert "WORLD" in call_kwargs["prompt"]

    async def test_summarize_includes_post_content(
        self,
        summarizer: Summarizer,
        mock_anthropic_client: MagicMock,
        sample_posts: list[Post],
    ) -> None:
        await summarizer.summarize("world", sample_posts)

        call_kwargs = mock_anthropic_client.generate.call_args.kwargs
        prompt = call_kwargs["prompt"]

        assert "Breaking News: Major Event" in prompt
        assert "Technology Update" in prompt
        assert "r/news" in prompt
        assert "r/technology" in prompt

    async def test_summarize_includes_comments(
        self,
        summarizer: Summarizer,
        mock_anthropic_client: MagicMock,
        sample_posts: list[Post],
    ) -> None:
        await summarizer.summarize("world", sample_posts)

        call_kwargs = mock_anthropic_client.generate.call_args.kwargs
        prompt = call_kwargs["prompt"]

        assert "This is huge!" in prompt
        assert "Interesting development" in prompt

    async def test_summarize_includes_selftext(
        self,
        summarizer: Summarizer,
        mock_anthropic_client: MagicMock,
        sample_posts: list[Post],
    ) -> None:
        await summarizer.summarize("world", sample_posts)

        call_kwargs = mock_anthropic_client.generate.call_args.kwargs
        prompt = call_kwargs["prompt"]

        assert "This is the content of the news post." in prompt

    async def test_summarize_passes_model(
        self,
        summarizer: Summarizer,
        mock_anthropic_client: MagicMock,
        sample_posts: list[Post],
    ) -> None:
        await summarizer.summarize("world", sample_posts, model="claude-sonnet-4-20250514")

        call_kwargs = mock_anthropic_client.generate.call_args.kwargs
        assert call_kwargs["model"] == "claude-sonnet-4-20250514"

    async def test_summarize_returns_response(
        self,
        summarizer: Summarizer,
        mock_anthropic_client: MagicMock,
        sample_posts: list[Post],
    ) -> None:
        mock_anthropic_client.generate.return_value = "Summary of the news"

        result = await summarizer.summarize("world", sample_posts)

        assert result == "Summary of the news"

    async def test_summarize_empty_posts(
        self,
        summarizer: Summarizer,
        mock_anthropic_client: MagicMock,
    ) -> None:
        result = await summarizer.summarize("world", [])

        assert "Не знайдено постів" in result
        mock_anthropic_client.generate.assert_not_called()


class TestTranslateToUkrainian:
    """Tests for translate_to_ukrainian method."""

    async def test_translate_calls_client(
        self,
        summarizer: Summarizer,
        mock_anthropic_client: MagicMock,
    ) -> None:
        await summarizer.translate_to_ukrainian("Text to translate")

        mock_anthropic_client.generate.assert_called_once()
        call_kwargs = mock_anthropic_client.generate.call_args.kwargs
        assert "Text to translate" in call_kwargs["prompt"]
        assert "system" in call_kwargs

    async def test_translate_passes_model(
        self,
        summarizer: Summarizer,
        mock_anthropic_client: MagicMock,
    ) -> None:
        await summarizer.translate_to_ukrainian("Text", model="claude-sonnet-4-20250514")

        call_kwargs = mock_anthropic_client.generate.call_args.kwargs
        assert call_kwargs["model"] == "claude-sonnet-4-20250514"

    async def test_translate_empty_text(
        self,
        summarizer: Summarizer,
        mock_anthropic_client: MagicMock,
    ) -> None:
        result = await summarizer.translate_to_ukrainian("")

        assert result == ""
        mock_anthropic_client.generate.assert_not_called()

    async def test_translate_returns_response(
        self,
        summarizer: Summarizer,
        mock_anthropic_client: MagicMock,
    ) -> None:
        mock_anthropic_client.generate.return_value = "Переклад українською"

        result = await summarizer.translate_to_ukrainian("Translation to Ukrainian")

        assert result == "Переклад українською"


class TestSummarizeAndTranslate:
    """Tests for summarize_and_translate method."""

    async def test_calls_both_summarize_and_translate(
        self,
        summarizer: Summarizer,
        mock_anthropic_client: MagicMock,
        sample_posts: list[Post],
    ) -> None:
        mock_anthropic_client.generate.side_effect = ["Summary", "Переклад"]

        await summarizer.summarize_and_translate("world", sample_posts)

        assert mock_anthropic_client.generate.call_count == 2

    async def test_appends_source_urls(
        self,
        summarizer: Summarizer,
        mock_anthropic_client: MagicMock,
        sample_posts: list[Post],
    ) -> None:
        mock_anthropic_client.generate.side_effect = ["Summary", "Переклад"]

        result = await summarizer.summarize_and_translate("world", sample_posts)

        assert "SOURCES" in result or "ДЖЕРЕЛА" in result
        assert "https://reddit.com/r/news/comments/abc123/breaking_news/" in result
        assert "https://reddit.com/r/technology/comments/xyz789/tech_update/" in result

    async def test_truncates_long_titles_in_sources(
        self,
        summarizer: Summarizer,
        mock_anthropic_client: MagicMock,
    ) -> None:
        long_title = "A" * 100
        posts = [
            Post(
                title=long_title,
                url="https://example.com",
                score=100,
                subreddit="test",
                author="user",
                permalink="/r/test/comments/abc/long/",
            )
        ]
        mock_anthropic_client.generate.side_effect = ["Summary", "Translation"]

        result = await summarizer.summarize_and_translate("test", posts)

        # Title should be truncated with ...
        assert "..." in result
        assert long_title not in result

    async def test_passes_model_to_both_calls(
        self,
        summarizer: Summarizer,
        mock_anthropic_client: MagicMock,
        sample_posts: list[Post],
    ) -> None:
        mock_anthropic_client.generate.side_effect = ["Summary", "Translation"]

        await summarizer.summarize_and_translate("world", sample_posts, model="test-model")

        for call in mock_anthropic_client.generate.call_args_list:
            assert call.kwargs["model"] == "test-model"


class TestBuildSummaryPrompt:
    """Tests for _build_summary_prompt method."""

    def test_truncates_long_selftext(self, summarizer: Summarizer) -> None:
        long_selftext = "x" * 2000
        post = Post(
            title="Post",
            url="",
            score=100,
            subreddit="test",
            author="user",
            selftext=long_selftext,
        )

        prompt = summarizer._build_summary_prompt("test", [post])

        # Selftext should be truncated to 1000 chars
        assert long_selftext not in prompt
        assert "..." in prompt

    def test_truncates_long_comments(self, summarizer: Summarizer) -> None:
        long_comment = "y" * 1000
        post = Post(
            title="Post",
            url="",
            score=100,
            subreddit="test",
            author="user",
            comments=[Comment(body=long_comment, score=10, author="commenter")],
        )

        prompt = summarizer._build_summary_prompt("test", [post])

        # Comment should be truncated to 500 chars
        assert long_comment not in prompt

    def test_truncates_overall_content(self, summarizer: Summarizer) -> None:
        # Create many posts to exceed MAX_CONTENT_LENGTH
        posts = [
            Post(
                title=f"Post {i} with a longer title to take up space",
                url="",
                score=100,
                subreddit="test",
                author="user",
                selftext="Some content " * 100,
                comments=[
                    Comment(body="Comment " * 50, score=10, author=f"user{j}") for j in range(5)
                ],
            )
            for i in range(100)
        ]

        prompt = summarizer._build_summary_prompt("test", posts)

        assert len(prompt) <= MAX_CONTENT_LENGTH + 200  # Some buffer for truncation message
        assert "[Content truncated due to length]" in prompt

    def test_includes_scores(self, summarizer: Summarizer) -> None:
        post = Post(
            title="Post",
            url="",
            score=999,
            subreddit="test",
            author="user",
            comments=[Comment(body="Comment", score=42, author="commenter")],
        )

        prompt = summarizer._build_summary_prompt("test", [post])

        assert "999" in prompt  # Post score
        assert "42" in prompt  # Comment score
