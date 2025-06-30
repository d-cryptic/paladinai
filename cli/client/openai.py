"""
OpenAI client methods for Paladin AI CLI
Contains methods for OpenAI chat completion functionality.
"""

from typing import Optional, Dict, Any

from .base import BaseHTTPClient
from utils.formatter.formatter import formatter
from utils.formatter.markdown_formatter import markdown_formatter


class OpenAIMixin:
    """Mixin class for OpenAI functionality."""

    async def chat_with_openai(self, message: str, context: Optional[Dict[str, Any]] = None, interactive: bool = False, analysis_only: bool = False) -> bool:
        """
        Send a message to OpenAI via server endpoint.

        Args:
            message: The message to send to OpenAI
            context: Optional context dictionary
            interactive: If True, shows loading screen and simplified output format
            analysis_only: If True, shows only analysis section of the response

        Returns:
            bool: True if successful, False otherwise
        """
        async def _chat_request():
            payload = {
                "message": message,
                "additional_context": context
            }
            return await self._post_json("/api/v1/chat", payload)

        # Execute request with or without loading screen
        if interactive:
            # Import here to avoid circular imports
            import sys
            import os
            sys.path.append(os.path.dirname(os.path.dirname(__file__)))
            from utils.loading import with_loading
            success, data = await with_loading(_chat_request(), "🤖 Thinking")
        else:
            success, data = await _chat_request()

        # Handle response
        if success:
            # Check if we have markdown content to display
            # The server returns markdown in the 'content' field when formatted_markdown is available
            has_markdown = False
            markdown_content = None
            
            # Check for markdown in content field (from server restructuring)
            if "content" in data and isinstance(data["content"], str) and len(data["content"].strip()) > 50:
                # This is likely markdown content from the server (check it's not just empty whitespace)
                markdown_content = data["content"]
                has_markdown = True
            # Also check raw_result for formatted_markdown
            elif "raw_result" in data and "formatted_markdown" in data["raw_result"]:
                markdown_content = data["raw_result"]["formatted_markdown"]
                has_markdown = True
            
            # Use markdown formatter if available, otherwise fall back to standard formatter
            if has_markdown and markdown_content and not analysis_only:
                # Create a proper response structure for the markdown formatter
                markdown_response = {
                    "success": data.get("success", True),
                    "session_id": data.get("session_id"),
                    "formatted_markdown": markdown_content,
                    "metadata": data.get("metadata", {}),
                    "categorization": data.get("raw_result", {}).get("categorization", {})
                }
                # Use Rich markdown formatter for better display
                markdown_formatter.format_response(markdown_response)
            elif analysis_only:
                print("🤖 Paladin AI Response:")
                formatted_response = formatter.format_analysis_only(data)
                print(formatted_response)
            else:
                # Use the standard formatter to display workflow results
                if interactive:
                    print("\n🤖 Paladin AI:")
                    formatted_response = formatter.format_workflow_response(data, interactive=True)
                    print(formatted_response)
                else:
                    print("🤖 Paladin AI Response:")
                    formatted_response = formatter.format_workflow_response(data, interactive=False)
                    print(formatted_response)
                
                # Show session info if available (only if not analysis_only)
                if data.get("session_id"):
                    print(f"\n📋 Session: {data['session_id']}")

            return True
        else:
            error_msg = data.get('error', 'Unknown error')
            error_type = data.get('error_type', 'unknown')
            
            # Use markdown formatter for better error display
            markdown_formatter.format_error_response(error_msg, error_type)
            
            return False

    async def chat_with_openai_interactive(self, message: str, context: Optional[Dict[str, Any]] = None) -> bool:
        """
        Send a message to OpenAI via server endpoint with loading screen for interactive mode.

        This method is kept for backward compatibility but delegates to chat_with_openai.
        """
        return await self.chat_with_openai(message, context, interactive=True)
