#!/usr/bin/env python3
"""HTTP server for receiving alert reports from Paladin server"""

import asyncio
import os
from typing import Dict, Any
from aiohttp import web
from pathlib import Path
from dotenv import load_dotenv
import sys

# Load environment variables
load_dotenv(Path(__file__).parent.parent.parent.parent / '.env')


class AlertReportServer:
    """HTTP server to receive alert reports and forward to Discord."""
    
    def __init__(self, discord_server):
        self.discord_server = discord_server
        self.app = web.Application()
        self.setup_routes()
    
    def setup_routes(self):
        """Setup HTTP routes."""
        self.app.router.add_post('/send-alert-report', self.handle_alert_report)
        self.app.router.add_get('/health', self.health_check)
    
    async def health_check(self, request: web.Request) -> web.Response:
        """Health check endpoint."""
        return web.json_response({
            "status": "healthy",
            "service": "discord-alert-server",
            "discord_connected": self.discord_server.bot.is_ready()
        })
    
    async def handle_alert_report(self, request: web.Request) -> web.Response:
        """Handle incoming alert report from Paladin server."""
        try:
            # Parse JSON payload
            data = await request.json()
            
            print(f"[ALERT SERVER] Received alert report: {data.get('alert_id')}", file=sys.stderr)
            
            # Validate required fields
            required_fields = ['markdown_content', 'alert_id']
            missing_fields = [field for field in required_fields if field not in data]
            
            if missing_fields:
                return web.json_response({
                    "success": False,
                    "error": f"Missing required fields: {', '.join(missing_fields)}"
                }, status=400)
            
            # Forward to Discord handler
            result = await self.discord_server.handle_alert_report(data)
            
            if result.get("success"):
                return web.json_response({
                    "success": True,
                    "message": "Alert report sent to Discord",
                    "discord_url": result.get("message_url"),
                    "channel": result.get("channel")
                })
            else:
                return web.json_response({
                    "success": False,
                    "error": result.get("error", "Failed to send to Discord")
                }, status=500)
                
        except Exception as e:
            print(f"[ALERT SERVER ERROR] {e}", file=sys.stderr)
            return web.json_response({
                "success": False,
                "error": str(e)
            }, status=500)
    
    async def start(self, host: str = "0.0.0.0", port: int = 9000):
        """Start the HTTP server."""
        runner = web.AppRunner(self.app)
        await runner.setup()
        site = web.TCPSite(runner, host, port)
        await site.start()
        print(f"[ALERT SERVER] HTTP server started on {host}:{port}", file=sys.stderr)
        return runner


async def run_alert_server(discord_server, host: str = "0.0.0.0", port: int = 9000):
    """Run the alert report HTTP server."""
    server = AlertReportServer(discord_server)
    runner = await server.start(host, port)
    
    try:
        # Keep the server running
        await asyncio.Event().wait()
    except KeyboardInterrupt:
        pass
    finally:
        await runner.cleanup()