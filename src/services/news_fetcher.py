"""Service for fetching news from Reddit subreddits."""

import asyncio
import logging

from src.clients.reddit import RedditClient
from src.models.reddit_types import Post, SubredditGroup

logger = logging.getLogger(__name__)


class NewsFetcher:
    """Orchestrates fetching Reddit posts and comments for subreddit groups."""

    def __init__(
        self,
        reddit_client: RedditClient,
        subreddit_groups: dict[str, list[str]],
        num_posts: int = 7,
        num_comments: int = 7,
    ):
        """
        Initialize the news fetcher.

        Args:
            reddit_client: RedditClient instance
            subreddit_groups: Dict mapping group names to subreddit lists
            num_posts: Number of posts to fetch per subreddit
            num_comments: Number of comments to fetch per post
        """
        self.reddit_client = reddit_client
        self.subreddit_groups = subreddit_groups
        self.num_posts = num_posts
        self.num_comments = num_comments

    def get_available_groups(self) -> dict[str, list[str]]:
        """Return the available subreddit groups."""
        return self.subreddit_groups.copy()

    async def fetch_group(self, group_name: str) -> SubredditGroup:
        """
        Fetch all posts and comments for a subreddit group.

        Args:
            group_name: Name of the group (case-insensitive)

        Returns:
            SubredditGroup with all fetched posts

        Raises:
            ValueError: If group name is not found
        """
        group_name_lower = group_name.lower()

        if group_name_lower not in self.subreddit_groups:
            available = ", ".join(self.subreddit_groups.keys())
            raise ValueError(f"Unknown group '{group_name}'. Available groups: {available}")

        subreddits = self.subreddit_groups[group_name_lower]
        logger.info(f"Fetching news for group '{group_name}' from {len(subreddits)} subreddits")

        # Fetch posts from all subreddits in parallel
        tasks = [self._fetch_subreddit(subreddit) for subreddit in subreddits]
        results = await asyncio.gather(*tasks, return_exceptions=True)

        # Collect successful results
        all_posts: list[Post] = []
        for subreddit, result in zip(subreddits, results):
            if isinstance(result, BaseException):
                logger.error(f"Failed to fetch r/{subreddit}: {result}")
                continue
            all_posts.extend(result)

        # Sort by score descending
        all_posts.sort(key=lambda p: p.score, reverse=True)

        logger.info(f"Fetched total of {len(all_posts)} posts for group '{group_name}'")

        return SubredditGroup(
            name=group_name_lower,
            subreddits=subreddits,
            posts=all_posts,
        )

    async def _fetch_subreddit(self, subreddit: str) -> list[Post]:
        """Fetch posts with comments from a single subreddit."""
        logger.debug(f"Fetching from r/{subreddit}")

        posts = await self.reddit_client.fetch_posts_with_comments(
            subreddit=subreddit,
            num_posts=self.num_posts,
            num_comments=self.num_comments,
        )

        logger.debug(f"Fetched {len(posts)} posts from r/{subreddit}")
        return posts
