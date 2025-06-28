"""
OpenAI client methods for Paladin AI CLI
Contains methods for OpenAI chat completion functionality.
"""

from typing import Optional, Dict, Any

from .base import BaseHTTPClient


class OpenAIMixin:
    """Mixin class for OpenAI functionality."""

    async def chat_with_openai(self, message: str, context: Optional[Dict[str, Any]] = None, interactive: bool = False) -> bool:
        """
        Send a message to OpenAI via server endpoint.

        Args:
            message: The message to send to OpenAI
            context: Optional context dictionary
            interactive: If True, shows loading screen and simplified output format

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
            if data.get("success"):
                if interactive:
                    print("\n🤖 Paladin AI:")
                    print(f"   {data.get('content', 'No content')}")

                    if data.get("usage"):
                        usage = data["usage"]
                        print(f"\n📊 Tokens: {usage['total_tokens']} total")
                else:
                    print("🤖 OpenAI Response:")
                    print(f"   Content: {data.get('content', 'No content')}")
                    print(f"   Model: {data.get('model', 'Unknown')}")

                    if data.get("usage"):
                        usage = data["usage"]
                        print(f"\n📊 Token Usage: {usage['total_tokens']} total "
                              f"({usage['prompt_tokens']} prompt + {usage['completion_tokens']} completion)")
                return True
            else:
                error_prefix = "\n❌" if interactive else "❌"
                print(f"{error_prefix} OpenAI Error: {data.get('error')}")
                return False
        else:
            error_prefix = "\n❌" if interactive else "❌"
            error_msg = data.get('error', 'Unknown error')
            error_type = data.get('error_type', 'unknown')
            attempts = data.get('attempts', 1)

            # Provide specific guidance based on error type
            if error_type == 'connection_error':
                print(f"{error_prefix} Connection Error: {error_msg}")
                if attempts > 1:
                    print(f"   🔄 Tried {attempts} times with exponential backoff")
                print("   💡 Suggestions:")
                print("      • Check if the server is running: make run-server")
                print("      • Verify server URL in cli/.env")
                print("      • Check network connectivity")
            elif error_type == 'timeout_error':
                print(f"{error_prefix} Timeout Error: {error_msg}")
                print("   💡 The request took too long to complete")
                print("      • Try a simpler query")
                print("      • Check server logs for issues")
            else:
                print(f"{error_prefix} Error communicating with server: {error_msg}")
                if error_type != 'unknown':
                    print(f"   Error type: {error_type}")

            return False

    async def chat_with_openai_interactive(self, message: str, context: Optional[Dict[str, Any]] = None) -> bool:
        """
        Send a message to OpenAI via server endpoint with loading screen for interactive mode.

        This method is kept for backward compatibility but delegates to chat_with_openai.
        """
        return await self.chat_with_openai(message, context, interactive=True)
