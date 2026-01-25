"""News summarization and translation service."""

import logging

from src.clients.anthropic_client import AnthropicClient
from src.models.reddit_types import Post

logger = logging.getLogger(__name__)

SUMMARIZER_SYSTEM = """You are a witty news summarizer with a dry sense of humor.
Given Reddit posts and their top comments, create a concise, informative summary that:

1. Highlights the most significant news stories
2. Incorporates insights from top comments when they add valuable context
3. Groups related stories together when appropriate
4. Uses clear, journalistic language with occasional subtle humor or witty observations
5. Maintains objectivity while noting community sentiment when relevant
6. Adds a touch of irony or clever commentary where appropriate
   (but don't overdo it - one or two witty remarks per story max)

Format: Use PLAIN TEXT with ASCII formatting (no markdown). For each major story:
- Use UPPERCASE for main headers, followed by a line of === underneath
- Use bullet points with • or - for lists
- Use *asterisks* for emphasis instead of bold
- Keep total length under 1500 words

Focus on the substance and key developments, not on Reddit-specific details.
Let your personality shine through occasionally."""

TRANSLATOR_SYSTEM = """You are a professional Ukrainian translator specializing in news content.
Translate the following text to Ukrainian:

1. Maintain journalistic tone and style
2. Use standard Ukrainian (literary language, not Surzhyk)
3. Preserve ASCII formatting exactly (UPPERCASE headers, === lines, • bullets, *emphasis*)
4. Transliterate proper nouns appropriately (use Ukrainian conventions)
5. Keep the same structure and emphasis as the original
6. For technical terms, use commonly accepted Ukrainian equivalents

Provide only the translation, no explanations or notes."""

MAX_CONTENT_LENGTH = 100000  # Max chars to send to API


class Summarizer:
    """Service for summarizing Reddit posts and translating to Ukrainian."""

    def __init__(self, client: AnthropicClient, default_model: str | None = None):
        """
        Initialize the summarizer.

        Args:
            client: AnthropicClient instance
            default_model: Default model to use (overrides client default)
        """
        self.client = client
        self.default_model = default_model

    async def summarize(
        self,
        group_name: str,
        posts: list[Post],
        model: str | None = None,
    ) -> str:
        """
        Summarize Reddit posts into a concise news summary.

        Args:
            group_name: Name of the subreddit group
            posts: List of posts with comments
            model: Model to use (optional)

        Returns:
            Summarized news content
        """
        if not posts:
            return f"No posts found for group '{group_name}' in the last 24 hours."

        prompt = self._build_summary_prompt(group_name, posts)
        model = model or self.default_model

        logger.info(f"Summarizing {len(posts)} posts for group '{group_name}'")

        summary = await self.client.generate(
            prompt=prompt,
            system=SUMMARIZER_SYSTEM,
            model=model,
        )

        return summary

    async def translate_to_ukrainian(
        self,
        text: str,
        model: str | None = None,
    ) -> str:
        """
        Translate text to Ukrainian.

        Args:
            text: Text to translate
            model: Model to use (optional)

        Returns:
            Translated text in Ukrainian
        """
        if not text:
            return ""

        model = model or self.default_model

        logger.info("Translating summary to Ukrainian")

        translation = await self.client.generate(
            prompt=f"Translate this news summary to Ukrainian:\n\n{text}",
            system=TRANSLATOR_SYSTEM,
            model=model,
        )

        return translation

    async def summarize_and_translate(
        self,
        group_name: str,
        posts: list[Post],
        model: str | None = None,
    ) -> str:
        """
        Summarize posts and translate the summary to Ukrainian.

        Args:
            group_name: Name of the subreddit group
            posts: List of posts with comments
            model: Model to use

        Returns:
            Summarized and translated news in Ukrainian
        """
        # Step 1: Summarize
        summary = await self.summarize(group_name, posts, model)

        # Step 2: Translate
        translation = await self.translate_to_ukrainian(summary, model)

        # Step 3: Append source URLs
        if posts:
            urls_section = "\n\n════════════════════════════════════════\n"
            urls_section += "SOURCES / ДЖЕРЕЛА\n"
            urls_section += "════════════════════════════════════════\n\n"
            for post in posts:
                title = post.title[:60] + "..." if len(post.title) > 60 else post.title
                urls_section += f"• {title}\n  {post.full_url}\n\n"
            translation += urls_section

        return translation

    def _build_summary_prompt(self, group_name: str, posts: list[Post]) -> str:
        """Build the prompt for summarization."""
        sections: list[str] = []

        for i, post in enumerate(posts, 1):
            section = f"## Post {i}: {post.title}\n"
            section += f"**Subreddit:** r/{post.subreddit} | **Score:** {post.score}\n\n"

            # Include selftext if available (for text posts)
            if post.selftext and post.selftext.strip():
                selftext = post.selftext[:1000]  # Limit selftext
                if len(post.selftext) > 1000:
                    selftext += "..."
                section += f"**Content:**\n{selftext}\n\n"

            # Include top comments
            if post.comments:
                section += "**Top Comments:**\n"
                for j, comment in enumerate(post.comments, 1):
                    # Limit comment length
                    body = comment.body[:500]
                    if len(comment.body) > 500:
                        body += "..."
                    section += f"{j}. [{comment.score} points] {body}\n"

            sections.append(section)

        content = "\n---\n\n".join(sections)

        # Truncate if too long
        if len(content) > MAX_CONTENT_LENGTH:
            content = content[:MAX_CONTENT_LENGTH] + "\n\n[Content truncated due to length]"

        prompt = (
            f"Summarize the following Reddit posts from the '{group_name.upper()}' news group. "
            f"These are the top posts and comments from the last 24 hours:\n\n"
            f"{content}"
        )

        return prompt
