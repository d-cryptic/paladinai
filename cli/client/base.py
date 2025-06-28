"""
Base HTTP client for Paladin AI CLI
Contains the core HTTP client functionality and session management.
"""

import os
import asyncio
from typing import Any, Optional

import aiohttp
from dotenv import load_dotenv

load_dotenv()


class BaseHTTPClient:
    """Base HTTP client with session management and retry logic."""

    def __init__(self) -> None:
        self.server_url = os.getenv("SERVER_URL", "http://127.0.0.1:8000")
        self.session: Optional[aiohttp.ClientSession] = None
        self.timeout = int(os.getenv("TIMEOUT", "60"))  # Increased default timeout
        self.max_retries = int(os.getenv("MAX_RETRIES", "3"))
        self.retry_delay = float(os.getenv("RETRY_DELAY", "1.0"))

    async def __aenter__(self) -> "BaseHTTPClient":
        # Configure session with better settings for stability
        connector = aiohttp.TCPConnector(
            limit=10,  # Connection pool limit
            limit_per_host=5,  # Per-host connection limit
            ttl_dns_cache=300,  # DNS cache TTL
            use_dns_cache=True,
            keepalive_timeout=30,  # Keep connections alive
            enable_cleanup_closed=True
        )

        timeout = aiohttp.ClientTimeout(
            total=self.timeout,
            connect=10,  # Connection timeout
            sock_read=30   # Socket read timeout
        )

        self.session = aiohttp.ClientSession(
            connector=connector,
            timeout=timeout,
            headers={"Connection": "keep-alive"}
        )
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
        return await self._request_with_retry("GET", endpoint)

    async def _post_json(self, endpoint: str, payload: dict) -> tuple[bool, dict]:
        """
        Make a POST request and return success status and JSON data.

        Args:
            endpoint: API endpoint to call
            payload: JSON payload to send

        Returns:
            Tuple of (success, data)
        """
        return await self._request_with_retry("POST", endpoint, payload)

    async def _request_with_retry(self, method: str, endpoint: str, payload: Optional[dict] = None) -> tuple[bool, dict]:
        """
        Make HTTP request with retry logic for connection issues.

        Args:
            method: HTTP method (GET, POST)
            endpoint: API endpoint to call
            payload: Optional JSON payload for POST requests

        Returns:
            Tuple of (success, data)
        """
        self._ensure_session()

        for attempt in range(self.max_retries + 1):
            try:
                if method == "GET":
                    async with self.session.get(f"{self.server_url}{endpoint}") as response:
                        return await self._handle_response(response)
                elif method == "POST":
                    async with self.session.post(
                        f"{self.server_url}{endpoint}",
                        json=payload
                    ) as response:
                        return await self._handle_response(response)
                else:
                    return False, {"error": f"Unsupported HTTP method: {method}"}

            except (aiohttp.ClientError, asyncio.TimeoutError, ConnectionResetError, OSError) as e:
                error_msg = str(e)

                # Check if this is the last attempt
                if attempt == self.max_retries:
                    return False, {
                        "error": f"Connection failed after {self.max_retries + 1} attempts: {error_msg}",
                        "error_type": "connection_error",
                        "attempts": attempt + 1
                    }

                # Log retry attempt (in a real app, use proper logging)
                print(f"⚠️  Connection attempt {attempt + 1} failed: {error_msg}")
                print(f"🔄 Retrying in {self.retry_delay * (2 ** attempt)} seconds...")

                # Exponential backoff
                await asyncio.sleep(self.retry_delay * (2 ** attempt))

            except Exception as e:
                # For non-connection errors, don't retry
                return False, {"error": f"Request error: {str(e)}"}

        return False, {"error": "Unexpected error in retry logic"}

    async def _handle_response(self, response: aiohttp.ClientResponse) -> tuple[bool, dict]:
        """
        Handle HTTP response and extract JSON data.

        Args:
            response: aiohttp response object

        Returns:
            Tuple of (success, data)
        """
        if response.status == 200:
            try:
                data = await response.json()
                return True, data
            except Exception as e:
                return False, {"error": f"Failed to parse JSON response: {str(e)}"}
        else:
            try:
                error_data = await response.json()
                return False, {"error": f"HTTP {response.status}", "details": error_data}
            except:
                return False, {"error": f"HTTP {response.status}"}
