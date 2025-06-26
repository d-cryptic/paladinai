"""
Paladin AI CLI Client
Simple CLI client to connect and communicate with the FastAPI server.
"""

import argparse
import asyncio
import os
import sys

import aiohttp
from dotenv import load_dotenv

from banner import display_banner

load_dotenv()

class PaladinCLI:
    def __init__(self):
        self.server_url = os.getenv("SERVER_URL", "http://127.0.0.1:8000")
        self.session = None

    async def __aenter__(self):
        self.session = aiohttp.ClientSession()
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        if self.session:
            await self.session.close()

    async def test_connection(self):
        """Test connection to the server"""
        try:
            async with self.session.get(f"{self.server_url}/health") as response:
                if response.status == 200:
                    data = await response.json()
                    print(f"✅ Server connection successful!")
                    print(f"   Status: {data.get('status')}")
                    print(f"   Service: {data.get('service')}")
                    return True
                else:
                    print(f"❌ Server responded with status: {response.status}")
                    return False
        except Exception as e:
            print(f"❌ Failed to connect to server: {e}")
            return False

    async def get_hello(self):
        """Get hello message from server"""
        try:
            async with self.session.get(f"{self.server_url}/") as response:
                if response.status == 200:
                    data = await response.json()
                    print(f"📨 Server says: {data.get('message')}")
                    return True
                else:
                    print(f"❌ Failed to get hello message. Status: {response.status}")
                    return False
        except Exception as e:
            print(f"❌ Error getting hello message: {e}")
            return False

    async def get_api_status(self):
        """Get API status from server"""
        try:
            async with self.session.get(f"{self.server_url}/api/v1/status") as response:
                if response.status == 200:
                    data = await response.json()
                    print(f"🔧 API Version: {data.get('api_version')}")
                    print(f"🟢 Server Status: {data.get('server_status')}")
                    print(f"💬 Message: {data.get('message')}")
                    return True
                else:
                    print(f"❌ Failed to get API status. Status: {response.status}")
                    return False
        except Exception as e:
            print(f"❌ Error getting API status: {e}")
            return False


async def main():
    parser = argparse.ArgumentParser(description="Paladin AI CLI Client")
    parser.add_argument("--test", action="store_true", help="Test server connection")
    parser.add_argument(
        "--hello", action="store_true", help="Get hello message from server"
    )
    parser.add_argument(
        "--status", action="store_true", help="Get API status from server"
    )
    parser.add_argument("--all", action="store_true", help="Run all tests")

    args = parser.parse_args()

    # Display banner
    display_banner()

    async with PaladinCLI() as cli:
        if args.test or args.all:
            print("\n🔍 Testing server connection...")
            await cli.test_connection()

        if args.hello or args.all:
            print("\n👋 Getting hello message...")
            await cli.get_hello()

        if args.status or args.all:
            print("\n📊 Getting API status...")
            await cli.get_api_status()

        if not any([args.test, args.hello, args.status, args.all]):
            print("\n🚀 Welcome to Paladin AI CLI!")
            print("Use --help to see available options")
            print("Quick test: python main.py --all")


if __name__ == "__main__":
    asyncio.run(main())
