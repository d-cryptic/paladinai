"""
OpenAI client methods for Paladin AI CLI
Contains methods for OpenAI chat completion functionality.
"""

from typing import Optional, Dict, Any

from .base import BaseHTTPClient


class OpenAIMixin:
    """Mixin class for OpenAI functionality."""

    async def chat_with_openai(self, message: str, context: Optional[Dict[str, Any]] = None) -> bool:
        """Send a message to OpenAI via server endpoint"""
        payload = {
            "message": message,
            "additional_context": context
        }

        success, data = await self._post_json("/api/v1/chat", payload)
        
        if success:
            if data.get("success"):
                print("🤖 OpenAI Response:")
                print(f"   Content: {data.get('content', 'No content')}")
                print(f"   Model: {data.get('model', 'Unknown')}")
                
                if data.get("usage"):
                    usage = data["usage"]
                    print(f"\n📊 Token Usage: {usage['total_tokens']} total "
                          f"({usage['prompt_tokens']} prompt + {usage['completion_tokens']} completion)")
                return True
            else:
                print(f"❌ OpenAI Error: {data.get('error')}")
                return False
        else:
            print(f"❌ Error communicating with server: {data.get('error')}")
            return False
