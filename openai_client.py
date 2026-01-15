"""OpenAI API client with rate limiting for summarizing Reddit posts."""

import logging
from typing import Any

import aiohttp
from aiolimiter import AsyncLimiter

from config import Config, get_config

logger = logging.getLogger(__name__)

# Prompt template for summarization
PROMPT_TEMPLATE = """Please provide an engaging and fun summary of these Reddit posts and discussions from r/{subreddit}. 

Focus on:
- Main themes and topics; group similar topics together
- Key points from popular comments with interesting insights
- Notable trends, patterns, or controversies
- Overall community sentiment and mood

Format your response with:
- 📊 TRENDING TOPICS: List the main themes with emoji indicators
- 💬 COMMUNITY PULSE: Describe the overall sentiment and notable discussions
- 🔥 HOT TAKES: Highlight the most interesting or controversial opinions

Rules:
- Be conversational and engaging
- Use appropriate emojis to make the summary more visually appealing
- Use 5-9 emojis total; max 1 per bullet; don't repeat the same emoji in a section
- Don't reply with the summary for each post individually
- Keep your tone friendly and humorous where appropriate
- Organize information in a clear, scannable format with bullet points and sections

Posts to analyze:

{text}"""

DEFAULT_REASONING_EFFORT = "minimal"


class OpenAIClient:
    """Async OpenAI API client with rate limiting."""

    def __init__(self, config: Config | None = None):
        self.config = config or get_config()
        
        # Rate limiter: requests per minute
        # Convert requests per minute to rate per second
        rate = self.config.openai_requests_per_minute / 60.0
        self._limiter = AsyncLimiter(rate, 1.0)

    async def summarize_posts(
        self,
        subreddit: str,
        text: str,
        model: str,
        reasoning_effort: str = "",
    ) -> str:
        """Summarize Reddit posts using OpenAI API.
        
        Args:
            subreddit: Name of the subreddit
            text: Formatted text containing posts and comments
            model: OpenAI model to use
            reasoning_effort: Reasoning effort level (minimal, medium, high)
            
        Returns:
            Formatted summary text
        """
        logger.info(f"Making OpenAI API call with model: {model}")
        
        if not self.config.openai_api_key:
            raise ValueError("OpenAI API key is not configured")
        
        # Prepare request
        request_body = self._create_request(
            model=model,
            text=text,
            subreddit=subreddit,
            reasoning_effort=reasoning_effort or DEFAULT_REASONING_EFFORT,
        )
        
        # Make API call
        response = await self._make_api_call(request_body)
        
        # Format response
        return self._format_response(response)

    def _create_request(
        self,
        model: str,
        text: str,
        subreddit: str,
        reasoning_effort: str,
    ) -> dict[str, Any]:
        """Create the request body for OpenAI API."""
        prompt = PROMPT_TEMPLATE.format(subreddit=subreddit, text=text)
        
        request: dict[str, Any] = {
            "model": model,
            "messages": [
                {
                    "role": "user",
                    "content": prompt,
                }
            ],
        }
        
        if reasoning_effort:
            request["reasoning_effort"] = reasoning_effort
        
        return request

    async def _make_api_call(self, request_body: dict[str, Any]) -> dict[str, Any]:
        """Make the API call to OpenAI with rate limiting."""
        async with self._limiter:
            headers = {
                "Content-Type": "application/json",
                "Authorization": f"Bearer {self.config.openai_api_key}",
            }
            
            timeout = aiohttp.ClientTimeout(total=self.config.openai_request_timeout)
            
            import time
            start_time = time.time()
            
            async with aiohttp.ClientSession() as session:
                async with session.post(
                    self.config.openai_api_endpoint,
                    json=request_body,
                    headers=headers,
                    timeout=timeout,
                ) as resp:
                    request_duration = time.time() - start_time
                    logger.info(f"OpenAI API request completed in {request_duration:.2f}s")
                    
                    if resp.status != 200:
                        body = await resp.text()
                        raise aiohttp.ClientResponseError(
                            resp.request_info,
                            resp.history,
                            status=resp.status,
                            message=f"API returned non-200 status code {resp.status}: {body}",
                        )
                    
                    response = await resp.json()
        
        # Check for API errors in response
        if response.get("error"):
            error_msg = response["error"].get("message", "Unknown error")
            raise ValueError(f"API error: {error_msg}")
        
        return response

    def _format_response(self, response: dict[str, Any]) -> str:
        """Format the API response into a readable summary."""
        if not response:
            raise ValueError("Nil response received")
        
        choices = response.get("choices", [])
        if not choices:
            raise ValueError("Empty content in response")
        
        text = choices[0].get("message", {}).get("content", "")
        if not text:
            raise ValueError("Empty text in response content")
        
        # Ensure proper Markdown formatting
        if "*" not in text:
            text = text.replace("TRENDING TOPICS", "*TRENDING TOPICS*")
            text = text.replace("COMMUNITY PULSE", "*COMMUNITY PULSE*")
            text = text.replace("HOT TAKES", "*HOT TAKES*")
        
        # Add header
        return self.config.summary_header + text
