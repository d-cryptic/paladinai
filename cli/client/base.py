"""
Base HTTP client for Paladin AI CLI
Contains the core HTTP client functionality and session management.
"""

import os
from typing import Any, Optional

import aiohttp
from dotenv import load_dotenv

load_dotenv()


class BaseHTTPClient:
    """Base HTTP client with session management."""
    
    def __init__(self) -> None:
        self.server_url = os.getenv("SERVER_URL", "http://127.0.0.1:8000")
        self.session: Optional[aiohttp.ClientSession] = None

    async def __aenter__(self) -> "BaseHTTPClient":
        self.session = aiohttp.ClientSession()
        return self

    async def __aexit__(
        self, exc_type: Any, exc_val: Any, exc_tb: Any
    ) -> None:
        if self.session:
            await self.session.close()

    def _ensure_session(self) -> None:
        """Ensure session is initialized."""
        if self.session is None:
            raise RuntimeError("Session not initialized")

    async def _get_json(self, endpoint: str) -> tuple[bool, dict]:
        """
        Make a GET request and return success status and JSON data.
        
        Args:
            endpoint: API endpoint to call
            
        Returns:
            Tuple of (success, data)
        """
        self._ensure_session()
        try:
            async with self.session.get(f"{self.server_url}{endpoint}") as response:
                if response.status == 200:
                    data = await response.json()
                    return True, data
                else:
                    return False, {"error": f"HTTP {response.status}"}
        except Exception as e:
            return False, {"error": str(e)}

    async def _post_json(self, endpoint: str, payload: dict) -> tuple[bool, dict]:
        """
        Make a POST request and return success status and JSON data.
        
        Args:
            endpoint: API endpoint to call
            payload: JSON payload to send
            
        Returns:
            Tuple of (success, data)
        """
        self._ensure_session()
        try:
            async with self.session.post(
                f"{self.server_url}{endpoint}",
                json=payload
            ) as response:
                if response.status == 200:
                    data = await response.json()
                    return True, data
                else:
                    return False, {"error": f"HTTP {response.status}"}
        except Exception as e:
            return False, {"error": str(e)}
