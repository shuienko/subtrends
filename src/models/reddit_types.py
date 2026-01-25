"""Data types for Reddit content."""

from dataclasses import dataclass, field


@dataclass
class Comment:
    """A Reddit comment."""

    body: str
    score: int
    author: str

    def __str__(self) -> str:
        return f"[{self.score}] {self.author}: {self.body[:100]}..."


@dataclass
class Post:
    """A Reddit post with its top comments."""

    title: str
    url: str
    score: int
    subreddit: str
    author: str
    selftext: str = ""
    num_comments: int = 0
    permalink: str = ""
    comments: list[Comment] = field(default_factory=list)

    def __str__(self) -> str:
        return f"[{self.score}] r/{self.subreddit}: {self.title}"

    @property
    def full_url(self) -> str:
        """Get the full Reddit URL for this post."""
        if self.permalink:
            return f"https://reddit.com{self.permalink}"
        return self.url


@dataclass
class SubredditGroup:
    """A group of subreddits with their fetched posts."""

    name: str
    subreddits: list[str]
    posts: list[Post] = field(default_factory=list)

    def __str__(self) -> str:
        return f"{self.name}: {len(self.posts)} posts from {len(self.subreddits)} subreddits"
