"""
Health check client methods for Paladin AI CLI
Contains methods for testing server connectivity and health status.
"""

from .base import BaseHTTPClient


class HealthMixin:
    """Mixin class for health check functionality."""

    async def test_connection(self) -> bool:
        """Test connection to the server"""
        success, data = await self._get_json("/health")
        
        if success:
            print("✅ Server connection successful!")
            print(f"   Status: {data.get('status')}")
            print(f"   Service: {data.get('service')}")
            return True
        else:
            print(f"❌ Failed to connect to server: {data.get('error')}")
            return False

    async def get_hello(self) -> bool:
        """Get hello message from server"""
        success, data = await self._get_json("/")
        
        if success:
            print(f"📨 Server says: {data.get('message')}")
            return True
        else:
            print(f"❌ Failed to get hello message: {data.get('error')}")
            return False

    async def get_api_status(self) -> bool:
        """Get API status from server"""
        success, data = await self._get_json("/api/v1/status")
        
        if success:
            print(f"🔧 API Version: {data.get('api_version')}")
            print(f"🟢 Server Status: {data.get('server_status')}")
            print(f"💬 Message: {data.get('message')}")
            return True
        else:
            print(f"❌ Failed to get API status: {data.get('error')}")
            return False
