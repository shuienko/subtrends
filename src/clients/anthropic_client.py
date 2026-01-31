"""Anthropic API client wrapper."""

import asyncio
import logging
from typing import Any

import anthropic

logger = logging.getLogger(__name__)

DEFAULT_MAX_TOKENS = 4096
MAX_RETRIES = 3
INITIAL_RETRY_DELAY = 2  # seconds


class AnthropicClient:
    """Async wrapper for the Anthropic API."""

    def __init__(self, api_key: str | None = None, default_model: str = "claude-opus-4-5"):
        """
        Initialize the Anthropic client.

        Args:
            api_key: Anthropic API key (uses ANTHROPIC_API_KEY env var if not provided)
            default_model: Default model to use for requests
        """
        self.client = anthropic.AsyncAnthropic(api_key=api_key)
        self.default_model = default_model

    async def generate(
        self,
        prompt: str,
        system: str | None = None,
        model: str | None = None,
        max_tokens: int = DEFAULT_MAX_TOKENS,
    ) -> str:
        """
        Generate a response from the Anthropic API.

        Args:
            prompt: The user prompt
            system: Optional system prompt
            model: Model to use (defaults to instance default)
            max_tokens: Maximum tokens in response

        Returns:
            The generated text response
        """
        model = model or self.default_model
        messages: list[dict[str, Any]] = [{"role": "user", "content": prompt}]

        kwargs: dict[str, Any] = {
            "model": model,
            "max_tokens": max_tokens,
            "messages": messages,
        }

        if system:
            kwargs["system"] = system

        logger.debug(f"Sending request to Anthropic API (model: {model})")

        last_error: Exception | None = None

        for attempt in range(MAX_RETRIES):
            try:
                response = await self.client.messages.create(**kwargs)

                # Extract text from response
                text_content = ""
                for block in response.content:
                    if block.type == "text":
                        text_content += block.text

                logger.debug(
                    f"Received response: {response.usage.input_tokens} input tokens, "
                    f"{response.usage.output_tokens} output tokens"
                )

                return text_content

            except anthropic.RateLimitError as e:
                last_error = e
                delay = INITIAL_RETRY_DELAY * (2**attempt)
                logger.warning(
                    f"Anthropic rate limit error (attempt {attempt + 1}/{MAX_RETRIES}), "
                    f"retrying in {delay}s: {e}"
                )
                await asyncio.sleep(delay)

            except anthropic.InternalServerError as e:
                # Handle 529 Overloaded and other 5xx errors
                last_error = e
                delay = INITIAL_RETRY_DELAY * (2**attempt)
                logger.warning(
                    f"Anthropic server error (attempt {attempt + 1}/{MAX_RETRIES}), "
                    f"retrying in {delay}s: {e}"
                )
                await asyncio.sleep(delay)

            except anthropic.BadRequestError as e:
                # Don't retry bad requests - they won't succeed
                logger.error(f"Anthropic bad request error: {e}")
                raise

            except anthropic.APIError as e:
                logger.error(f"Anthropic API error: {e}")
                raise

        # All retries exhausted
        logger.error(f"All {MAX_RETRIES} retries exhausted for Anthropic API")
        if last_error:
            raise last_error
        raise RuntimeError("Anthropic API request failed after retries")

    async def close(self) -> None:
        """Close the client."""
        await self.client.close()
