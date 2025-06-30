"""
OpenAI API service for Paladin AI Server
Handles OpenAI API calls with system prompts and environment configuration.
"""

import os
from typing import Dict, List, Optional, Any
from openai import AsyncOpenAI
from dotenv import load_dotenv
from langfuse import observe

# Import system prompts
from prompts.system import SYSTEM_PROMPT

# Load environment variables
load_dotenv()


class OpenAIService:
    """Service class for handling OpenAI API calls with system prompts."""
    
    def __init__(self):
        """Initialize OpenAI client with environment configuration."""
        self.api_key = os.getenv("OPENAI_API_KEY")
        self.model = os.getenv("OPENAI_MODEL", "gpt-4o")
        self.base_url = os.getenv("OPENAI_BASE_URL")  # Optional for custom endpoints
        self.max_tokens = int(os.getenv("OPENAI_MAX_TOKENS", "4000"))
        self.temperature = float(os.getenv("OPENAI_TEMPERATURE", "0.1"))
        
        if not self.api_key:
            raise ValueError("OPENAI_API_KEY environment variable is required")
        
        # Initialize async OpenAI client
        client_kwargs = {"api_key": self.api_key}
        if self.base_url:
            client_kwargs["base_url"] = self.base_url
            
        self.client = AsyncOpenAI(**client_kwargs)
    
    @observe(name="openai_chat_completion")
    async def chat_completion(
        self,
        user_message: str,
        system_prompt: Optional[str] = None,
        model: Optional[str] = None,
        max_tokens: Optional[int] = None,
        temperature: Optional[float] = None,
        additional_context: Optional[Dict[str, Any]] = None
    ) -> Dict[str, Any]:
        """
        Make a chat completion request to OpenAI API.
        
        Args:
            user_message: The user's message/query
            system_prompt: Custom system prompt (defaults to SYSTEM_PROMPT)
            model: Model to use (defaults to configured model)
            max_tokens: Maximum tokens for response
            temperature: Temperature for response generation
            additional_context: Additional context to include in the prompt
            
        Returns:
            Dict containing the response and metadata
        """
        try:
            # Use provided parameters or fall back to defaults
            model = model or self.model
            max_tokens = max_tokens or self.max_tokens
            temperature = temperature or self.temperature
            system_prompt = system_prompt or SYSTEM_PROMPT
            
            # Build messages array
            messages = [
                {"role": "system", "content": system_prompt}
            ]
            
            # Add additional context if provided
            if additional_context:
                context_message = self._format_additional_context(additional_context)
                messages.append({"role": "system", "content": context_message})
            
            # Add user message
            messages.append({"role": "user", "content": user_message})
            
            # Make API call
            response = await self.client.chat.completions.create(
                model=model,
                messages=messages,
                max_tokens=max_tokens,
                temperature=temperature,
                response_format={"type": "json_object"}  # Force JSON response
            )
            
            # Extract response content
            content = response.choices[0].message.content
            
            return {
                "success": True,
                "content": content,
                "model": model,
                "usage": {
                    "prompt_tokens": response.usage.prompt_tokens,
                    "completion_tokens": response.usage.completion_tokens,
                    "total_tokens": response.usage.total_tokens
                },
                "finish_reason": response.choices[0].finish_reason
            }
            
        except Exception as e:
            return {
                "success": False,
                "error": str(e),
                "content": None
            }

    def _format_additional_context(self, context: Dict[str, Any]) -> str:
        """
        Format additional context into a readable string for the system prompt.

        Args:
            context: Dictionary containing additional context information

        Returns:
            Formatted context string
        """
        context_lines = ["Additional Context:"]
        for key, value in context.items():
            context_lines.append(f"- {key}: {value}")
        return "\n".join(context_lines)

    @observe(name="openai_markdown_formatting")
    async def markdown_formatting(
        self,
        user_message: str,
        system_prompt: Optional[str] = None,
        model: Optional[str] = None,
        max_tokens: Optional[int] = None,
        temperature: Optional[float] = None
    ) -> Dict[str, Any]:
        """
        Make a chat completion request to OpenAI API for markdown formatting.
        This method returns plain text/markdown instead of JSON.
        
        Args:
            user_message: The user's message/query
            system_prompt: Custom system prompt
            model: Model to use (defaults to configured model)
            max_tokens: Maximum tokens for response
            temperature: Temperature for response generation
            
        Returns:
            Dict containing the response and metadata
        """
        try:
            # Use provided parameters or fall back to defaults
            model = model or self.model
            max_tokens = max_tokens or self.max_tokens
            temperature = temperature or self.temperature
            system_prompt = system_prompt or "You are an expert technical writer."
            
            # Build messages array
            messages = [
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_message}
            ]
            
            # Make API call WITHOUT json format constraint
            response = await self.client.chat.completions.create(
                model=model,
                messages=messages,
                max_tokens=max_tokens,
                temperature=temperature
                # No response_format here - we want plain text/markdown
            )
            
            # Extract response content
            content = response.choices[0].message.content
            
            return {
                "success": True,
                "content": content,
                "model": model,
                "usage": {
                    "prompt_tokens": response.usage.prompt_tokens,
                    "completion_tokens": response.usage.completion_tokens,
                    "total_tokens": response.usage.total_tokens
                },
                "finish_reason": response.choices[0].finish_reason
            }
            
        except Exception as e:
            return {
                "success": False,
                "error": str(e),
                "content": None
            }

# Global service instance
openai = OpenAIService()