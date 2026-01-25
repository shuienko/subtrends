"""Tests for Reddit data types."""

from src.models.reddit_types import Comment, Post, SubredditGroup


class TestComment:
    """Tests for Comment dataclass."""

    def test_comment_creation(self) -> None:
        comment = Comment(body="Test comment", score=42, author="testuser")

        assert comment.body == "Test comment"
        assert comment.score == 42
        assert comment.author == "testuser"

    def test_comment_str_truncates_body(self) -> None:
        long_body = "x" * 200
        comment = Comment(body=long_body, score=10, author="user")

        result = str(comment)

        assert len(result) < len(long_body)
        assert "[10]" in result
        assert "user" in result
        assert "..." in result


class TestPost:
    """Tests for Post dataclass."""

    def test_post_creation_minimal(self) -> None:
        post = Post(
            title="Test Post",
            url="https://example.com",
            score=100,
            subreddit="test",
            author="testuser",
        )

        assert post.title == "Test Post"
        assert post.url == "https://example.com"
        assert post.score == 100
        assert post.subreddit == "test"
        assert post.author == "testuser"
        assert post.selftext == ""
        assert post.num_comments == 0
        assert post.permalink == ""
        assert post.comments == []

    def test_post_creation_full(self) -> None:
        comment = Comment(body="Nice post!", score=5, author="commenter")
        post = Post(
            title="Full Post",
            url="https://example.com/article",
            score=500,
            subreddit="news",
            author="poster",
            selftext="This is the post content",
            num_comments=10,
            permalink="/r/news/comments/abc123/full_post/",
            comments=[comment],
        )

        assert post.selftext == "This is the post content"
        assert post.num_comments == 10
        assert post.permalink == "/r/news/comments/abc123/full_post/"
        assert len(post.comments) == 1
        assert post.comments[0].body == "Nice post!"

    def test_post_str(self) -> None:
        post = Post(
            title="My Title",
            url="https://example.com",
            score=123,
            subreddit="python",
            author="dev",
        )

        result = str(post)

        assert "[123]" in result
        assert "r/python" in result
        assert "My Title" in result

    def test_full_url_with_permalink(self) -> None:
        post = Post(
            title="Test",
            url="https://external.com/link",
            score=1,
            subreddit="test",
            author="user",
            permalink="/r/test/comments/xyz789/test/",
        )

        assert post.full_url == "https://reddit.com/r/test/comments/xyz789/test/"

    def test_full_url_without_permalink(self) -> None:
        post = Post(
            title="Test",
            url="https://external.com/link",
            score=1,
            subreddit="test",
            author="user",
        )

        assert post.full_url == "https://external.com/link"


class TestSubredditGroup:
    """Tests for SubredditGroup dataclass."""

    def test_subreddit_group_creation(self) -> None:
        group = SubredditGroup(
            name="world",
            subreddits=["news", "worldnews", "europe"],
        )

        assert group.name == "world"
        assert group.subreddits == ["news", "worldnews", "europe"]
        assert group.posts == []

    def test_subreddit_group_with_posts(self) -> None:
        post = Post(
            title="Breaking News",
            url="https://example.com",
            score=1000,
            subreddit="news",
            author="reporter",
        )
        group = SubredditGroup(
            name="world",
            subreddits=["news"],
            posts=[post],
        )

        assert len(group.posts) == 1
        assert group.posts[0].title == "Breaking News"

    def test_subreddit_group_str(self) -> None:
        posts = [
            Post(title=f"Post {i}", url="", score=i, subreddit="test", author="user")
            for i in range(5)
        ]
        group = SubredditGroup(
            name="tech",
            subreddits=["programming", "webdev"],
            posts=posts,
        )

        result = str(group)

        assert "tech" in result
        assert "5 posts" in result
        assert "2 subreddits" in result
