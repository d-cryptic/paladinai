#!/usr/bin/env python3
import asyncio
import os
import sys
from datetime import datetime
from typing import Dict, List, Optional, Any, Set
from pathlib import Path
import json
import redis
from rq import Queue

import discord
from discord.ext import commands
from dotenv import load_dotenv
import aiohttp
from mcp.server import Server
from mcp.types import (
    Tool,
    TextContent,
    ImageContent,
    EmbeddedResource,
    LoggingLevel
)
from mcp.server.stdio import stdio_server

# Load environment variables
load_dotenv(Path(__file__).parent.parent.parent.parent / '.env')

class ConversationContext:
    """Tracks conversation context for threads and message chains"""
    def __init__(self):
        self.threads: Dict[int, List[Dict[str, Any]]] = {}  # thread_id -> messages
        self.reply_chains: Dict[int, List[Dict[str, Any]]] = {}  # root_message_id -> chain
        self.user_conversations: Dict[int, List[Dict[str, Any]]] = {}  # user_id -> recent messages
        
    def add_to_thread(self, thread_id: int, message_data: Dict[str, Any]):
        """Add message to thread context"""
        if thread_id not in self.threads:
            self.threads[thread_id] = []
        self.threads[thread_id].append(message_data)
        # Keep last 100 messages per thread
        self.threads[thread_id] = self.threads[thread_id][-100:]
    
    def add_to_reply_chain(self, root_id: int, message_data: Dict[str, Any]):
        """Add message to reply chain"""
        if root_id not in self.reply_chains:
            self.reply_chains[root_id] = []
        self.reply_chains[root_id].append(message_data)
    
    def add_user_message(self, user_id: int, message_data: Dict[str, Any]):
        """Track user conversation history"""
        if user_id not in self.user_conversations:
            self.user_conversations[user_id] = []
        self.user_conversations[user_id].append(message_data)
        # Keep last 50 messages per user
        self.user_conversations[user_id] = self.user_conversations[user_id][-50:]
    
    def get_thread_context(self, thread_id: int) -> List[Dict[str, Any]]:
        """Get all messages in a thread"""
        return self.threads.get(thread_id, [])
    
    def get_conversation_context(self, message: discord.Message) -> Dict[str, Any]:
        """Get full conversation context for a message"""
        context = {
            "thread_messages": [],
            "reply_chain": [],
            "user_history": [],
            "related_messages": []
        }
        
        # Get thread context if in thread
        if hasattr(message.channel, 'parent') and message.channel.parent:
            context["thread_messages"] = self.get_thread_context(message.channel.id)
        
        # Get reply chain if replying
        if message.reference and message.reference.message_id:
            root_id = message.reference.message_id
            context["reply_chain"] = self.reply_chains.get(root_id, [])
        
        # Get user conversation history
        context["user_history"] = self.user_conversations.get(message.author.id, [])[-10:]
        
        return context

class EnhancedDiscordMCPServer:
    def __init__(self):
        self.server = Server("discord-mcp-server")
        self.bot = commands.Bot(
            command_prefix='!',
            intents=discord.Intents.all()
        )
        self.message_history: List[Dict[str, Any]] = []
        self.monitored_channels: Dict[int, str] = {}
        self.paladin_url = os.getenv("SERVER_HOST", "127.0.0.1") + ":" + os.getenv("SERVER_PORT", "8000")
        self.conversation_context = ConversationContext()
        
        # Valkey/Redis configuration
        self.setup_queue()
        
        self.setup_handlers()
        self.setup_bot_events()
    
    def setup_queue(self):
        """Setup Valkey/Redis queue for message processing"""
        try:
            # Use VALKEY_HOST from env (which is set to HOST_IP)
            host = os.getenv("VALKEY_HOST", "localhost")
            
            self.redis_conn = redis.Redis(
                host=host,
                port=int(os.getenv("VALKEY_PORT", "6379")),
                password=None,  # No password in current setup
                db=int(os.getenv("VALKEY_DB", "0")),
                decode_responses=True
            )
            # Test connection
            self.redis_conn.ping()
            
            self.message_queue = Queue('discord_messages', connection=self.redis_conn)
            print(f"[QUEUE] Connected to Valkey/Redis at {host}:{os.getenv('VALKEY_PORT')}", file=sys.stderr)
        except Exception as e:
            print(f"[QUEUE ERROR] Failed to connect to Valkey/Redis at {host}: {e}", file=sys.stderr)
            self.redis_conn = None
            self.message_queue = None

    def setup_handlers(self):
        @self.server.list_tools()
        async def list_tools() -> List[Tool]:
            return [
                Tool(
                    name="get_channel_messages",
                    description="Get recent messages from a Discord channel with full context",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "channel_id": {
                                "type": "string",
                                "description": "Discord channel ID"
                            },
                            "limit": {
                                "type": "integer",
                                "description": "Number of messages to retrieve (max 100)",
                                "default": 50
                            },
                            "include_thread_context": {
                                "type": "boolean",
                                "description": "Include thread context if available",
                                "default": True
                            }
                        },
                        "required": ["channel_id"]
                    }
                ),
                Tool(
                    name="send_message",
                    description="Send a message to a Discord channel or thread",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "channel_id": {
                                "type": "string",
                                "description": "Discord channel or thread ID"
                            },
                            "content": {
                                "type": "string",
                                "description": "Message content to send"
                            },
                            "reply_to_id": {
                                "type": "string",
                                "description": "Message ID to reply to (optional)"
                            }
                        },
                        "required": ["channel_id", "content"]
                    }
                ),
                Tool(
                    name="get_thread_messages",
                    description="Get all messages from a Discord thread",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "thread_id": {
                                "type": "string",
                                "description": "Discord thread ID"
                            }
                        },
                        "required": ["thread_id"]
                    }
                ),
                Tool(
                    name="get_conversation_context",
                    description="Get full conversation context for a user or message",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "user_id": {
                                "type": "string",
                                "description": "User ID to get conversation history"
                            },
                            "message_id": {
                                "type": "string",
                                "description": "Message ID to get context for"
                            }
                        }
                    }
                ),
                Tool(
                    name="monitor_channel",
                    description="Start monitoring a Discord channel for messages",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "channel_id": {
                                "type": "string",
                                "description": "Discord channel ID to monitor"
                            },
                            "channel_name": {
                                "type": "string",
                                "description": "Optional channel name for reference"
                            }
                        },
                        "required": ["channel_id"]
                    }
                ),
                Tool(
                    name="get_queue_status",
                    description="Get status of the message processing queue",
                    inputSchema={
                        "type": "object",
                        "properties": {}
                    }
                )
            ]

        @self.server.call_tool()
        async def call_tool(name: str, arguments: Dict[str, Any]) -> List[TextContent]:
            if name == "get_channel_messages":
                return await self.get_channel_messages(
                    arguments.get("channel_id"),
                    arguments.get("limit", 50),
                    arguments.get("include_thread_context", True)
                )
            elif name == "send_message":
                return await self.send_message(
                    arguments.get("channel_id"),
                    arguments.get("content"),
                    arguments.get("reply_to_id")
                )
            elif name == "get_thread_messages":
                return await self.get_thread_messages(
                    arguments.get("thread_id")
                )
            elif name == "get_conversation_context":
                return await self.get_conversation_context_tool(
                    arguments.get("user_id"),
                    arguments.get("message_id")
                )
            elif name == "monitor_channel":
                return await self.monitor_channel(
                    arguments.get("channel_id"),
                    arguments.get("channel_name")
                )
            elif name == "get_queue_status":
                return await self.get_queue_status()
            else:
                return [TextContent(type="text", text=f"Unknown tool: {name}")]

    def setup_bot_events(self):
        @self.bot.event
        async def on_ready():
            print(f'{self.bot.user} has connected to Discord!', file=sys.stderr)
            # Log guilds and channels for debugging
            for guild in self.bot.guilds:
                print(f'Connected to server: {guild.name} (ID: {guild.id})', file=sys.stderr)
                for channel in guild.text_channels:
                    print(f'  - Channel: #{channel.name} (ID: {channel.id})', file=sys.stderr)
            
            # Auto-monitor all channels
            print("\n[AUTO-MONITOR] Automatically monitoring all text channels:", file=sys.stderr)
            for guild in self.bot.guilds:
                for channel in guild.text_channels:
                    self.monitored_channels[channel.id] = channel.name
                    print(f"  ✓ Monitoring #{channel.name} (ID: {channel.id})", file=sys.stderr)

        @self.bot.event
        async def on_message(message: discord.Message):
            # Don't respond to ourselves
            if message.author == self.bot.user:
                return

            # Create message data with full context
            message_data = await self.create_message_data(message)
            
            # Print message info
            print(f"\n[NEW MESSAGE] #{message.channel.name} | {message.author.name}: {message.content}", file=sys.stderr)
            
            # Store in conversation context
            if hasattr(message.channel, 'parent') and message.channel.parent:
                self.conversation_context.add_to_thread(message.channel.id, message_data)
                print(f"[THREAD] Message added to thread #{message.channel.name}", file=sys.stderr)
            
            if message.reference and message.reference.message_id:
                self.conversation_context.add_to_reply_chain(message.reference.message_id, message_data)
                print(f"[REPLY] Message added to reply chain", file=sys.stderr)
            
            self.conversation_context.add_user_message(message.author.id, message_data)
            
            # Store message if channel is monitored
            if message.channel.id in self.monitored_channels:
                self.store_message(message_data)
                print(f"[STORED] Message stored from monitored channel #{message.channel.name}", file=sys.stderr)
                
                # Queue non-tagged messages for processing
                if self.bot.user not in message.mentions:
                    await self.queue_message_for_processing(message_data)
            else:
                print(f"[NOT MONITORED] Channel #{message.channel.name} is not being monitored", file=sys.stderr)

            # Handle bot mentions
            if self.bot.user in message.mentions:
                print(f"[BOT MENTIONED] Bot was mentioned by {message.author.name}", file=sys.stderr)
                await self.handle_mention_with_context(message)

            # Process commands
            await self.bot.process_commands(message)

    async def create_message_data(self, message: discord.Message) -> Dict[str, Any]:
        """Create comprehensive message data including context"""
        # Get conversation context
        context = self.conversation_context.get_conversation_context(message)
        
        # Fetch thread starter if in thread
        thread_starter = None
        if hasattr(message.channel, 'parent') and message.channel.parent:
            try:
                thread_starter = await message.channel.parent.fetch_message(message.channel.id)
            except:
                pass
        
        # Fetch replied message if replying
        replied_message = None
        if message.reference and message.reference.message_id:
            try:
                replied_message = await message.channel.fetch_message(message.reference.message_id)
            except:
                pass
        
        message_data = {
            "id": str(message.id),
            "channel_id": str(message.channel.id),
            "channel_name": message.channel.name,
            "author_id": str(message.author.id),
            "author_name": message.author.name,
            "author_discriminator": message.author.discriminator,
            "content": message.content,
            "timestamp": message.created_at.isoformat(),
            "attachments": [
                {
                    "filename": att.filename,
                    "url": att.url,
                    "size": att.size
                } for att in message.attachments
            ],
            "embeds": [embed.to_dict() for embed in message.embeds],
            "mentions": [str(user.id) for user in message.mentions],
            "mention_roles": [str(role.id) for role in message.role_mentions],
            "mention_everyone": message.mention_everyone,
            "is_thread": hasattr(message.channel, 'parent') and message.channel.parent is not None,
            "thread_name": message.channel.name if hasattr(message.channel, 'parent') else None,
            "thread_starter_content": thread_starter.content if thread_starter else None,
            "is_reply": message.reference is not None,
            "replied_to": {
                "message_id": str(replied_message.id),
                "author": replied_message.author.name,
                "content": replied_message.content
            } if replied_message else None,
            "context": context
        }
        
        return message_data

    def store_message(self, message_data: Dict[str, Any]):
        """Store a message in the history"""
        self.message_history.append(message_data)
        
        # Print stored message details
        print(f"[MESSAGE DATA] Stored message #{len(self.message_history)}:", file=sys.stderr)
        print(f"  - Author: {message_data['author_name']} (ID: {message_data['author_id']})", file=sys.stderr)
        print(f"  - Channel: #{message_data['channel_name']} (ID: {message_data['channel_id']})", file=sys.stderr)
        print(f"  - Content: {message_data['content'][:100]}{'...' if len(message_data['content']) > 100 else ''}", file=sys.stderr)
        print(f"  - Timestamp: {message_data['timestamp']}", file=sys.stderr)
        if message_data.get('is_thread'):
            print(f"  - Thread: {message_data['thread_name']}", file=sys.stderr)
        if message_data.get('is_reply'):
            print(f"  - Reply to: {message_data['replied_to']['author']}", file=sys.stderr)
        if message_data['mentions']:
            print(f"  - Mentions: {message_data['mentions']}", file=sys.stderr)
        
        # Keep only last 10000 messages to prevent memory issues
        if len(self.message_history) > 10000:
            self.message_history = self.message_history[-10000:]

    async def queue_message_for_processing(self, message_data: Dict[str, Any]):
        """Queue non-tagged messages for processing"""
        if self.message_queue:
            try:
                print(f"\n[QUEUE DEBUG] {'='*60}", file=sys.stderr)
                print(f"[QUEUE] Attempting to queue message for guardrail check...", file=sys.stderr)
                print(f"[QUEUE] Message ID: {message_data['id']}", file=sys.stderr)
                print(f"[QUEUE] Author: {message_data['author_name']}", file=sys.stderr)
                print(f"[QUEUE] Content: {message_data['content']}", file=sys.stderr)
                print(f"[QUEUE] Full content length: {len(message_data['content'])} chars", file=sys.stderr)
                
                # Test Redis connection before queuing
                try:
                    self.redis_conn.ping()
                    print(f"[QUEUE] Redis ping successful", file=sys.stderr)
                except Exception as e:
                    print(f"[QUEUE] Redis ping failed: {e}", file=sys.stderr)
                
                # Queue message for guardrail check and memory storage via PaladinAI server
                job = self.message_queue.enqueue(
                    'discord_mcp.workers_server.process_message',
                    message_data,
                    job_timeout=os.getenv("QUEUE_JOB_TIMEOUT", "600s"),
                    result_ttl=os.getenv("QUEUE_RESULT_TTL", "3600")
                )
                
                print(f"[QUEUE] ✅ Message successfully queued for processing!", file=sys.stderr)
                print(f"[QUEUE] Job ID: {job.id}", file=sys.stderr)
                print(f"[QUEUE] Job status: {job.get_status()}", file=sys.stderr)
                print(f"[QUEUE] Queue length: {len(self.message_queue)}", file=sys.stderr)
                print(f"[QUEUE] Message will go through: Worker -> Guardrail -> Memory (if relevant)", file=sys.stderr)
                print(f"[QUEUE DEBUG] {'='*60}\n", file=sys.stderr)
                
            except Exception as e:
                print(f"[QUEUE ERROR] ❌ Failed to queue message: {e}", file=sys.stderr)
                print(f"[QUEUE ERROR] Error type: {type(e).__name__}", file=sys.stderr)
                import traceback
                traceback.print_exc(file=sys.stderr)
        else:
            print(f"[QUEUE ERROR] ❌ Message queue not initialized!", file=sys.stderr)

    async def handle_mention_with_context(self, message: discord.Message):
        """Handle bot mentions with full conversation context"""
        # Get conversation context
        context = self.conversation_context.get_conversation_context(message)
        
        # Remove the bot mention from the message
        content = message.content.replace(f'<@{self.bot.user.id}>', '').strip()
        
        # Prepare request with context
        request_data = {
            "user_input": content,
            "context": {
                "source": "discord",
                "channel": message.channel.name,
                "user": message.author.name,
                "timestamp": message.created_at.isoformat(),
                "thread_messages": context["thread_messages"][-10:],  # Last 10 thread messages
                "user_history": context["user_history"][-5:],  # Last 5 user messages
                "reply_chain": context["reply_chain"][-5:]  # Last 5 in reply chain
            }
        }
        
        # Send typing indicator
        async with message.channel.typing():
            # Process with PaladinAI
            response = await self.send_to_paladin(request_data)
        
        # Send response
        await message.reply(response, mention_author=True)

    async def send_to_paladin(self, request_data: Dict[str, Any]) -> str:
        """Send message to PaladinAI for processing"""
        url = f"http://{self.paladin_url}/chat"
        
        async with aiohttp.ClientSession() as session:
            try:
                async with session.post(
                    url,
                    json=request_data,
                    timeout=aiohttp.ClientTimeout(total=60)
                ) as response:
                    if response.status == 200:
                        result = await response.json()
                        return result.get("response", "Sorry, I couldn't process that request.")
                    else:
                        return f"Error: PaladinAI returned status {response.status}"
            except Exception as e:
                print(f"[PALADIN ERROR] {e}", file=sys.stderr)
                return "Sorry, I'm having trouble connecting to PaladinAI right now."

    async def get_channel_messages(self, channel_id: str, limit: int, include_thread_context: bool) -> List[TextContent]:
        """Get recent messages from a channel with context"""
        try:
            channel = self.bot.get_channel(int(channel_id))
            if not channel:
                return [TextContent(type="text", text=f"Channel {channel_id} not found")]
            
            messages = []
            async for message in channel.history(limit=min(limit, 100)):
                message_data = await self.create_message_data(message)
                messages.append(message_data)
            
            # Format response
            formatted = []
            for msg in messages:
                base = f"[{msg['timestamp']}] {msg['author_name']}: {msg['content']}"
                if msg.get('is_thread'):
                    base += f" (in thread: {msg['thread_name']})"
                if msg.get('is_reply'):
                    base += f" (replying to {msg['replied_to']['author']})"
                formatted.append(base)
            
            return [TextContent(
                type="text",
                text=f"Retrieved {len(messages)} messages from {channel.name}:\n" + 
                     "\n".join(formatted)
            )]
        except Exception as e:
            return [TextContent(type="text", text=f"Error getting messages: {str(e)}")]

    async def send_message(self, channel_id: str, content: str, reply_to_id: Optional[str] = None) -> List[TextContent]:
        """Send a message to a channel with optional reply"""
        try:
            channel = self.bot.get_channel(int(channel_id))
            if not channel:
                return [TextContent(type="text", text=f"Channel {channel_id} not found")]
            
            # Create reference if replying
            reference = None
            if reply_to_id:
                try:
                    reply_msg = await channel.fetch_message(int(reply_to_id))
                    reference = reply_msg
                except:
                    pass
            
            message = await channel.send(content, reference=reference)
            return [TextContent(
                type="text",
                text=f"Message sent to {channel.name}: {message.jump_url}"
            )]
        except Exception as e:
            return [TextContent(type="text", text=f"Error sending message: {str(e)}")]

    async def get_thread_messages(self, thread_id: str) -> List[TextContent]:
        """Get all messages from a thread"""
        try:
            thread_messages = self.conversation_context.get_thread_context(int(thread_id))
            
            if not thread_messages:
                # Try to fetch from Discord
                thread = self.bot.get_channel(int(thread_id))
                if thread and hasattr(thread, 'parent'):
                    messages = []
                    async for message in thread.history(limit=100):
                        message_data = await self.create_message_data(message)
                        messages.append(message_data)
                        self.conversation_context.add_to_thread(int(thread_id), message_data)
                    thread_messages = messages
            
            if not thread_messages:
                return [TextContent(type="text", text=f"No messages found in thread {thread_id}")]
            
            # Format messages
            formatted = []
            for msg in thread_messages:
                formatted.append(
                    f"[{msg['timestamp']}] {msg['author_name']}: {msg['content']}"
                )
            
            return [TextContent(
                type="text",
                text=f"Thread messages ({len(formatted)} total):\n" + "\n".join(formatted)
            )]
        except Exception as e:
            return [TextContent(type="text", text=f"Error getting thread messages: {str(e)}")]

    async def get_conversation_context_tool(self, user_id: Optional[str] = None, message_id: Optional[str] = None) -> List[TextContent]:
        """Get conversation context for a user or message"""
        try:
            if user_id:
                user_history = self.conversation_context.user_conversations.get(int(user_id), [])
                if not user_history:
                    return [TextContent(type="text", text=f"No conversation history for user {user_id}")]
                
                formatted = []
                for msg in user_history[-20:]:
                    formatted.append(
                        f"[{msg['timestamp']}] #{msg['channel_name']}: {msg['content']}"
                    )
                
                return [TextContent(
                    type="text",
                    text=f"User conversation history ({len(formatted)} messages):\n" + "\n".join(formatted)
                )]
            
            elif message_id:
                # Find message in history
                for msg in self.message_history:
                    if msg['id'] == message_id:
                        context = msg.get('context', {})
                        return [TextContent(
                            type="text",
                            text=f"Context for message {message_id}:\n" + json.dumps(context, indent=2)
                        )]
                
                return [TextContent(type="text", text=f"Message {message_id} not found in history")]
            
            else:
                return [TextContent(type="text", text="Please provide either user_id or message_id")]
            
        except Exception as e:
            return [TextContent(type="text", text=f"Error getting context: {str(e)}")]

    async def monitor_channel(self, channel_id: str, channel_name: Optional[str] = None) -> List[TextContent]:
        """Start monitoring a channel"""
        try:
            channel = self.bot.get_channel(int(channel_id))
            if not channel:
                return [TextContent(type="text", text=f"Channel {channel_id} not found")]
            
            self.monitored_channels[int(channel_id)] = channel_name or channel.name
            return [TextContent(
                type="text",
                text=f"Now monitoring channel: {channel.name} ({channel_id})"
            )]
        except Exception as e:
            return [TextContent(type="text", text=f"Error monitoring channel: {str(e)}")]

    async def get_queue_status(self) -> List[TextContent]:
        """Get status of the message processing queue"""
        if not self.message_queue:
            return [TextContent(type="text", text="Queue not connected")]
        
        try:
            queue_info = {
                "name": self.message_queue.name,
                "job_count": len(self.message_queue),
                "failed_job_count": len(self.message_queue.failed_job_registry),
                "connection": f"{os.getenv('VALKEY_HOST')}:{os.getenv('VALKEY_PORT')}"
            }
            
            return [TextContent(
                type="text",
                text=f"Queue Status:\n" + json.dumps(queue_info, indent=2)
            )]
        except Exception as e:
            return [TextContent(type="text", text=f"Error getting queue status: {str(e)}")]

    async def run(self):
        """Run both the MCP server and Discord bot"""
        # Get Discord token
        token = os.getenv("DISCORD_BOT_TOKEN")
        if not token:
            print("Error: DISCORD_BOT_TOKEN not found in environment variables", file=sys.stderr)
            sys.exit(1)
        
        # Start Discord bot in the background
        bot_task = asyncio.create_task(self.bot.start(token))
        
        # Wait for bot to be ready
        print("Waiting for Discord bot to connect...", file=sys.stderr)
        while not self.bot.is_ready():
            await asyncio.sleep(0.1)
        print(f"Discord bot connected as {self.bot.user}", file=sys.stderr)
        
        # Run MCP server
        async with stdio_server() as (read_stream, write_stream):
            await self.server.run(
                read_stream,
                write_stream,
                self.server.create_initialization_options()
            )
        
        # Clean up
        await self.bot.close()
        await bot_task

def main():
    """Main entry point"""
    server = EnhancedDiscordMCPServer()
    asyncio.run(server.run())

if __name__ == "__main__":
    main()