"""Reddit API client with OAuth, token caching, and rate limiting."""

import asyncio
import logging
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any

import aiohttp
from aiolimiter import AsyncLimiter

from config import Config, get_config
from utils import read_json_file, write_json_file

logger = logging.getLogger(__name__)


@dataclass
class RedditPost:
    """Represents a Reddit post with essential fields."""
    title: str
    ups: int
    selftext: str
    permalink: str


@dataclass
class TokenData:
    """Cached token data structure."""
    access_token: str
    expires_at: datetime


class RedditClient:
    """Async Reddit API client with OAuth and rate limiting."""

    def __init__(self, config: Config | None = None):
        self.config = config or get_config()
        self._token_data: TokenData | None = None
        self._token_lock = asyncio.Lock()
        
        # Rate limiter: requests per second with burst
        self._limiter = AsyncLimiter(
            self.config.reddit_requests_per_second,
            1.0  # per second
        )

    async def _make_request(
        self,
        session: aiohttp.ClientSession,
        method: str,
        url: str,
        **kwargs: Any,
    ) -> dict[str, Any]:
        """Make an HTTP request with rate limiting and error handling."""
        async with self._limiter:
            logger.info(f"Sending request: {method} {url}")
            
            timeout = aiohttp.ClientTimeout(total=self.config.reddit_request_timeout)
            
            async with session.request(method, url, timeout=timeout, **kwargs) as resp:
                logger.info(f"Received response: {method} {url} - Status: {resp.status}")
                
                if resp.status < 200 or resp.status >= 300:
                    body = await resp.text()
                    logger.error(f"Unexpected status code: {method} {url} - Status: {resp.status} - Body: {body}")
                    raise aiohttp.ClientResponseError(
                        resp.request_info,
                        resp.history,
                        status=resp.status,
                        message=body,
                    )
                
                return await resp.json()

    def _load_token_from_file(self) -> TokenData | None:
        """Load token from file if valid."""
        data = read_json_file(self.config.reddit_token_file_path)
        if not data:
            return None
        
        try:
            access_token = data.get("access_token", "")
            expires_at_str = data.get("expires_at", "")
            
            if not access_token or not expires_at_str:
                return None
            
            # Parse ISO format datetime
            expires_at = datetime.fromisoformat(expires_at_str.replace("Z", "+00:00"))
            
            # Check if token is expired or about to expire
            now = datetime.now(timezone.utc)
            buffer = self.config.reddit_token_expiry_buffer
            if (expires_at.timestamp() - now.timestamp()) <= buffer:
                return None
            
            logger.info(f"Token loaded from file, expires at {expires_at}")
            return TokenData(access_token=access_token, expires_at=expires_at)
        except (ValueError, KeyError) as e:
            logger.warning(f"Failed to parse token from file: {e}")
            return None

    def _save_token_to_file(self, token: str, expires_in: int) -> None:
        """Save token to file."""
        expires_at = datetime.now(timezone.utc).timestamp() + expires_in
        expires_at_dt = datetime.fromtimestamp(expires_at, tz=timezone.utc)
        
        data = {
            "access_token": token,
            "expires_at": expires_at_dt.isoformat(),
        }
        
        if write_json_file(self.config.reddit_token_file_path, data):
            logger.info(f"Token saved to file, expires at {expires_at_dt}")
        else:
            logger.warning("Failed to save token to file")

    async def get_access_token(self) -> str:
        """Get a valid OAuth access token, using cache or requesting new one."""
        # Try to load from file first
        file_token = self._load_token_from_file()
        if file_token:
            self._token_data = file_token
            return file_token.access_token
        
        # Check cached token
        if self._token_data:
            now = datetime.now(timezone.utc)
            buffer = self.config.reddit_token_expiry_buffer
            if (self._token_data.expires_at.timestamp() - now.timestamp()) > buffer:
                logger.info(f"Using cached Reddit access token, expires at {self._token_data.expires_at}")
                return self._token_data.access_token
        
        # Need to acquire new token
        async with self._token_lock:
            # Double-check after acquiring lock
            if self._token_data:
                now = datetime.now(timezone.utc)
                buffer = self.config.reddit_token_expiry_buffer
                if (self._token_data.expires_at.timestamp() - now.timestamp()) > buffer:
                    return self._token_data.access_token
            
            logger.info("Requesting new Reddit access token")
            
            if not self.config.reddit_client_id or not self.config.reddit_client_secret:
                raise ValueError("Reddit client ID or secret is not configured")
            
            auth = aiohttp.BasicAuth(
                self.config.reddit_client_id,
                self.config.reddit_client_secret,
            )
            
            headers = {
                "User-Agent": self.config.reddit_user_agent,
                "Content-Type": "application/x-www-form-urlencoded",
            }
            
            data = "grant_type=client_credentials"
            
            async with aiohttp.ClientSession() as session:
                async with self._limiter:
                    timeout = aiohttp.ClientTimeout(total=self.config.reddit_request_timeout)
                    async with session.post(
                        self.config.reddit_auth_url,
                        auth=auth,
                        headers=headers,
                        data=data,
                        timeout=timeout,
                    ) as resp:
                        if resp.status != 200:
                            body = await resp.text()
                            raise aiohttp.ClientResponseError(
                                resp.request_info,
                                resp.history,
                                status=resp.status,
                                message=f"Token request failed: {body}",
                            )
                        
                        token_resp = await resp.json()
            
            access_token = token_resp.get("access_token", "")
            expires_in = token_resp.get("expires_in", 3600)
            
            if not access_token:
                raise ValueError("Empty access token received")
            
            # Cache token
            expires_at = datetime.fromtimestamp(
                datetime.now(timezone.utc).timestamp() + expires_in,
                tz=timezone.utc,
            )
            self._token_data = TokenData(access_token=access_token, expires_at=expires_at)
            
            # Save to file
            self._save_token_to_file(access_token, expires_in)
            
            logger.info(f"New Reddit token acquired, expires in {expires_in}s")
            return access_token

    async def fetch_top_posts(self, subreddit: str) -> list[RedditPost]:
        """Fetch top posts from a subreddit."""
        if not subreddit:
            raise ValueError("Subreddit name is required")
        
        # Clean subreddit name
        subreddit = subreddit.removeprefix("r/")
        
        logger.info(
            f"Fetching top {self.config.reddit_post_limit} posts from r/{subreddit} "
            f"for time frame: {self.config.reddit_timeframe}"
        )
        
        token = await self.get_access_token()
        
        url = (
            f"{self.config.reddit_base_url}/r/{subreddit}/top"
            f"?t={self.config.reddit_timeframe}&limit={self.config.reddit_post_limit}"
        )
        
        headers = {
            "Authorization": f"Bearer {token}",
            "User-Agent": self.config.reddit_user_agent,
        }
        
        async with aiohttp.ClientSession() as session:
            data = await self._make_request(session, "GET", url, headers=headers)
        
        posts: list[RedditPost] = []
        for child in data.get("data", {}).get("children", []):
            post_data = child.get("data", {})
            posts.append(RedditPost(
                title=post_data.get("title", ""),
                ups=post_data.get("ups", 0),
                selftext=post_data.get("selftext", ""),
                permalink=post_data.get("permalink", ""),
            ))
        
        if not posts:
            raise ValueError(f"No posts found in r/{subreddit}")
        
        logger.info(f"Successfully fetched {len(posts)} posts from r/{subreddit}")
        return posts

    async def fetch_top_comments(self, permalink: str) -> list[str]:
        """Fetch top comments for a post."""
        if not permalink:
            raise ValueError("Permalink is required")
        
        # Ensure permalink starts with /
        if not permalink.startswith("/"):
            permalink = "/" + permalink
        
        # Remove trailing slash
        permalink = permalink.rstrip("/")
        
        logger.info(f"Fetching top {self.config.reddit_comment_limit} comments for post: {permalink}")
        
        token = await self.get_access_token()
        
        url = f"{self.config.reddit_base_url}{permalink}.json?limit={self.config.reddit_comment_limit}"
        
        headers = {
            "Authorization": f"Bearer {token}",
            "User-Agent": self.config.reddit_user_agent,
        }
        
        async with aiohttp.ClientSession() as session:
            data = await self._make_request(session, "GET", url, headers=headers)
        
        if not isinstance(data, list) or len(data) < 2:
            raise ValueError("Unexpected comment data format")
        
        comments_raw = data[1]
        if not isinstance(comments_raw, dict):
            raise ValueError("Invalid comment data format")
        
        children = comments_raw.get("data", {}).get("children", [])
        
        comments: list[str] = []
        for child in children:
            if not isinstance(child, dict):
                continue
            body = child.get("data", {}).get("body", "")
            if body:
                comments.append(body)
        
        logger.info(f"Successfully fetched {len(comments)} comments for post: {permalink}")
        return comments

    async def get_subreddit_data(self, subreddit: str) -> tuple[str, list[RedditPost], int]:
        """Fetch data from a subreddit and format it for summarization.
        
        Returns:
            Tuple of (formatted_text, posts, total_comments)
        """
        subreddit = subreddit.removeprefix("r/")
        logger.info(f"Starting data collection for subreddit: r/{subreddit}")
        
        posts = await self.fetch_top_posts(subreddit)
        
        # Fetch comments concurrently with semaphore
        semaphore = asyncio.Semaphore(self.config.reddit_concurrent_requests)
        posts_with_comments: dict[int, list[str]] = {}
        errors: list[str] = []
        
        async def fetch_comments_for_post(idx: int, post: RedditPost) -> None:
            async with semaphore:
                logger.info(f"Processing post {idx + 1}: {post.title}")
                try:
                    comments = await self.fetch_top_comments(post.permalink)
                    posts_with_comments[idx] = comments
                    logger.info(f"Completed processing post {idx + 1} with {len(comments)} comments")
                except Exception as e:
                    errors.append(f"Failed to fetch comments for post {idx}: {e}")
        
        logger.info(
            f"Fetching comments for {len(posts)} posts with max concurrency of "
            f"{self.config.reddit_concurrent_requests}"
        )
        
        await asyncio.gather(*[
            fetch_comments_for_post(i, post) for i, post in enumerate(posts)
        ])
        
        # Log errors
        for error in errors:
            logger.warning(error)
        
        logger.info(f"Formatting data for {len(posts)} posts from r/{subreddit}")
        
        # Compute total comments
        total_comments = sum(len(comments) for comments in posts_with_comments.values())
        
        # Format posts and comments
        lines: list[str] = [f"# Top posts from r/{subreddit}\n"]
        
        for i, post in enumerate(posts):
            lines.append(f"## Post {i + 1}: {post.title}")
            lines.append(f"Upvotes: {post.ups}\n")
            
            if post.selftext:
                lines.append(f"Content:\n{post.selftext}\n")
            
            # Add comments if available
            comments = posts_with_comments.get(i, [])
            if comments:
                lines.append("Top Comments:")
                for comment in comments[:self.config.reddit_comment_limit]:
                    lines.append(f"- {comment}")
                lines.append("")
            
            # Add separator between posts
            if i < len(posts) - 1:
                lines.append("----------------------------\n")
        
        logger.info(f"Completed data collection for r/{subreddit} with {len(posts)} posts")
        return "\n".join(lines), posts, total_comments
