#!/usr/bin/env python3
import asyncio
import os
import sys
from datetime import datetime
from typing import Dict, List, Optional, Any, Set
from pathlib import Path
import json
import re
import redis
from rq import Queue
import openai
import tempfile
from io import BytesIO
from reportlab.lib import colors
from reportlab.lib.pagesizes import letter, A4
from reportlab.platypus import SimpleDocTemplate, Paragraph, Spacer, PageBreak, Preformatted
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.units import inch
from reportlab.lib.enums import TA_JUSTIFY, TA_CENTER
import markdown2

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

# Import prompts
from .prompts.discord_bot_prompts import (
    ACKNOWLEDGMENT_SYSTEM_PROMPT,
    DISCORD_FORMATTING_SYSTEM_PROMPT,
    get_acknowledgment_user_prompt,
    get_formatting_user_prompt
)

# Load environment variables
load_dotenv(Path(__file__).parent.parent.parent.parent / '.env')

# Initialize OpenAI
openai.api_key = os.getenv("OPENAI_API_KEY")

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
        self.paladin_url = f"{os.getenv('SERVER_HOST', '127.0.0.1')}:{os.getenv('SERVER_PORT', '8000')}"
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
            
            # Check if this is the docs channel and has document attachments
            if message.channel.name == "docs" and message.attachments:
                # Auto-add docs channel to monitored channels if not already monitored
                if message.channel.id not in self.monitored_channels:
                    self.monitored_channels[message.channel.id] = message.channel.name
                    print(f"[DOCS] Auto-monitoring #docs channel (ID: {message.channel.id})", file=sys.stderr)
                # Filter for PDF and Markdown files
                doc_attachments = [
                    att for att in message.attachments 
                    if att.filename.lower().endswith(('.pdf', '.md'))
                ]
                
                if doc_attachments:
                    print(f"[DOCS] Document detected in #docs channel: {[att.filename for att in doc_attachments]}", file=sys.stderr)
                    await self.handle_document_upload(message, doc_attachments)
                    return  # Skip normal processing for document uploads
            
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
            except Exception:
                pass
        
        # Fetch replied message if replying
        replied_message = None
        if message.reference and message.reference.message_id:
            try:
                replied_message = await message.channel.fetch_message(message.reference.message_id)
            except Exception:
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
        
        try:
            # Step 1: Create thread for the conversation
            thread = None
            if hasattr(message.channel, 'create_thread'):  # Text channel
                thread_name = f"{message.author.name} - {content[:50]}..." if len(content) > 50 else f"{message.author.name} - {content}"
                thread = await message.create_thread(
                    name=thread_name,
                    auto_archive_duration=60  # Auto-archive after 1 hour of inactivity
                )
                print(f"[THREAD] Created thread: {thread.name}", file=sys.stderr)
                reply_channel = thread
            else:
                # Already in a thread or DM channel
                reply_channel = message.channel
                print(f"[THREAD] Using existing channel: {reply_channel.name}", file=sys.stderr)
            
            # Step 2: Ask user for response format preference
            format_question = await reply_channel.send(
                f"<@{message.author.id}> Would you like the response as:\n"
                f"💬 **Text** (in Discord)\n"
                f"📊 **PDF Report** (downloadable file)\n\n"
                f"React with 💬 for text or 📊 for PDF report."
            )
            await format_question.add_reaction('💬')
            await format_question.add_reaction('📊')
            
            # Wait for user reaction
            def check(reaction, user):
                return user == message.author and str(reaction.emoji) in ['💬', '📊'] and reaction.message.id == format_question.id
            
            format_choice = 'text'  # Default
            try:
                reaction, user = await self.bot.wait_for('reaction_add', timeout=60.0, check=check)
                format_choice = 'pdf' if str(reaction.emoji) == '📊' else 'text'
                print(f"[FORMAT] User selected: {format_choice}", file=sys.stderr)
            except (asyncio.TimeoutError, TimeoutError):
                # Timeout - use default
                format_choice = 'text'
                await reply_channel.send("No response received, defaulting to text format.")
            except Exception as e:
                # Other exceptions - log and use default
                print(f"[ERROR] Failed to get format choice: {e}", file=sys.stderr)
                format_choice = 'text'
            
            # Step 3: Generate acknowledgment
            print(f"[ACKNOWLEDGMENT] Generating acknowledgment for: {content[:100]}...", file=sys.stderr)
            ack_message = await self.generate_acknowledgment(content, message.author.name)
            
            # Send acknowledgment to thread/channel
            ack_reply = await reply_channel.send(f"<@{message.author.id}> {ack_message}")
            print(f"[ACKNOWLEDGMENT] Sent: {ack_message}", file=sys.stderr)
            
            # Step 4: Process with PaladinAI
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
            
            # Send typing indicator while processing
            async with reply_channel.typing():
                print(f"[PALADIN] Processing message with PaladinAI server...", file=sys.stderr)
                response = await self.send_to_paladin(request_data)
            
            # Step 5: Handle response based on format choice
            if format_choice == 'pdf':
                try:
                    # Generate PDF report
                    print(f"[PDF] Starting PDF generation...", file=sys.stderr)
                    pdf_buffer = await self.generate_pdf_report(
                        user_query=content,
                        response=response,
                        user_name=message.author.name,
                        timestamp=datetime.now()
                    )
                    print(f"[PDF] PDF generated successfully, size: {pdf_buffer.getbuffer().nbytes} bytes", file=sys.stderr)
                    
                    # Send PDF file
                    pdf_file = discord.File(
                        pdf_buffer,
                        filename=f"paladin_report_{datetime.now().strftime('%Y%m%d_%H%M%S')}.pdf"
                    )
                    await reply_channel.send(
                        f"<@{message.author.id}> Here's your PaladinAI report:",
                        file=pdf_file
                    )
                    print(f"[PDF] PDF sent successfully", file=sys.stderr)
                except Exception as pdf_error:
                    print(f"[PDF ERROR] Failed to generate/send PDF: {pdf_error}", file=sys.stderr)
                    import traceback
                    print(f"[PDF ERROR] Traceback:\n{traceback.format_exc()}", file=sys.stderr)
                    # Fall back to text format
                    await reply_channel.send(f"<@{message.author.id}> Sorry, I couldn't generate the PDF. Here's the text response instead:")
                    formatted_response = await self.format_discord_response(response, message.author.name)
                    await self.send_chunked_response(reply_channel, formatted_response, message.author.id)
            else:
                # Text format (existing flow)
                formatted_response = await self.format_discord_response(response, message.author.name)
                await self.send_chunked_response(reply_channel, formatted_response, message.author.id)
            
        except Exception as e:
            # Error handling - notify user in Discord
            import traceback
            error_details = traceback.format_exc()
            print(f"[ERROR] Exception in handle_mention: {e}", file=sys.stderr)
            print(f"[ERROR] Full traceback:\n{error_details}", file=sys.stderr)
            error_msg = f"Sorry <@{message.author.id}>, I encountered an error while processing your request: {str(e)}"
            
            try:
                # Try to send error to appropriate channel
                if 'thread' in locals() and thread:
                    await thread.send(error_msg)
                elif 'reply_channel' in locals() and reply_channel:
                    await reply_channel.send(error_msg)
                else:
                    await message.reply(error_msg, mention_author=True)
            except Exception as fallback_error:
                # Fallback to channel if all else fails
                print(f"[ERROR] Failed to send error message: {fallback_error}", file=sys.stderr)
                try:
                    await message.channel.send(error_msg)
                except Exception:
                    pass  # Give up

    async def generate_acknowledgment(self, message_content: str, user_name: str) -> str:
        """Generate a quick acknowledgment message using OpenAI"""
        try:
            response = openai.chat.completions.create(
                model=os.getenv("OPENAI_MODEL", "gpt-4o-mini"),
                messages=[
                    {
                        "role": "system",
                        "content": ACKNOWLEDGMENT_SYSTEM_PROMPT
                    },
                    {
                        "role": "user",
                        "content": get_acknowledgment_user_prompt(user_name, message_content)
                    }
                ],
                temperature=0.7,
                max_tokens=50
            )
            
            return response.choices[0].message.content
            
        except Exception as e:
            print(f"[ACKNOWLEDGMENT ERROR] Failed to generate acknowledgment: {e}", file=sys.stderr)
            # Fallback acknowledgments
            fallbacks = [
                "Got it! Let me process that for you...",
                "On it! Give me a moment...",
                "I'll help with that! Processing now...",
                "Sure thing! Looking into this..."
            ]
            import random
            return random.choice(fallbacks)

    async def send_to_paladin(self, request_data: Dict[str, Any]) -> str:
        """Send message to PaladinAI for processing"""
        url = f"http://{self.paladin_url}/api/v1/chat"
        
        # Convert request format to match PaladinAI server expectations
        paladin_request = {
            "message": request_data["user_input"],
            "additional_context": {
                "session_id": f"discord_{request_data['context']['user']}_{request_data['context']['timestamp']}",
                "discord_context": request_data["context"]
            }
        }
        
        async with aiohttp.ClientSession() as session:
            try:
                print(f"[PALADIN] Full URL: {url}", file=sys.stderr)
                print(f"[PALADIN] Sending request to {url}", file=sys.stderr)
                print(f"[PALADIN] Request: {paladin_request['message'][:100]}...", file=sys.stderr)
                
                async with session.post(
                    url,
                    json=paladin_request,
                    timeout=aiohttp.ClientTimeout(total=600)  # 10 minutes
                ) as response:
                    if response.status == 200:
                        result = await response.json()
                        print(f"[PALADIN] Response received, success: {result.get('success', False)}", file=sys.stderr)
                        
                        # Extract the formatted response
                        if "content" in result:
                            # The server returns markdown in the 'content' field
                            return result["content"]
                        elif "raw_result" in result and "formatted_markdown" in result["raw_result"]:
                            # Fallback to raw_result if needed
                            return result["raw_result"]["formatted_markdown"]
                        else:
                            # Final fallback
                            return "I've processed your request but couldn't format the response properly."
                    else:
                        error_text = await response.text()
                        print(f"[PALADIN ERROR] Status {response.status}: {error_text}", file=sys.stderr)
                        return f"Sorry, I encountered an error while processing your request (Status: {response.status})."
            except (asyncio.TimeoutError, TimeoutError) as e:
                print(f"[PALADIN ERROR] Request timeout: {e}", file=sys.stderr)
                return "Sorry, the request took too long to process (timeout: 10 minutes). Please try again."
            except aiohttp.ClientError as e:
                print(f"[PALADIN ERROR] Client error: {e}", file=sys.stderr)
                return "Sorry, I'm having trouble connecting to the processing server."
            except Exception as e:
                print(f"[PALADIN ERROR] {type(e).__name__}: {e}", file=sys.stderr)
                return "Sorry, I encountered an unexpected error while processing your request."

    async def format_discord_response(self, response: str, user_name: str) -> str:
        """Format response for Discord using OpenAI"""
        try:
            # If response is already short enough and well-formatted, return as-is
            if len(response) <= 2000 and not response.count('```') > 6:
                return response
            
            format_response = openai.chat.completions.create(
                model=os.getenv("OPENAI_MODEL", "gpt-4o-mini"),
                messages=[
                    {
                        "role": "system",
                        "content": DISCORD_FORMATTING_SYSTEM_PROMPT
                    },
                    {
                        "role": "user",
                        "content": get_formatting_user_prompt(user_name, response)
                    }
                ],
                temperature=0.3,
                max_tokens=4000
            )
            
            return format_response.choices[0].message.content
            
        except Exception as e:
            print(f"[FORMAT ERROR] Failed to format response: {e}", file=sys.stderr)
            # Return original response if formatting fails
            return response

    async def handle_document_upload(self, message: discord.Message, doc_attachments: List[discord.Attachment]):
        """Handle document uploads in the docs channel"""
        try:
            # Add reaction to acknowledge receipt
            await message.add_reaction('📥')
            
            # Step 1: Create a thread for the document processing
            thread_name = f"Document: {doc_attachments[0].filename[:50]}"
            thread = await message.create_thread(
                name=thread_name,
                auto_archive_duration=60  # Auto-archive after 1 hour
            )
            print(f"[DOCS] Created thread: {thread.name}", file=sys.stderr)
            
            # Step 2: Send initial acknowledgment
            doc_names = ", ".join([att.filename for att in doc_attachments])
            
            # Generate AI acknowledgment
            ack_prompt = f"User uploaded document(s): {doc_names}. Acknowledge that you received the document(s) and will process them."
            ack_message = await self.generate_acknowledgment(ack_prompt, message.author.name)
            
            await thread.send(f"<@{message.author.id}> {ack_message}")
            
            # Step 3: Process each document
            for attachment in doc_attachments:
                try:
                    # Download the document
                    print(f"[DOCS] Downloading {attachment.filename}...", file=sys.stderr)
                    doc_bytes = await attachment.read()
                    
                    # Send to Paladin RAG endpoint
                    await thread.send(f"📄 Processing `{attachment.filename}`...")
                    
                    async with aiohttp.ClientSession() as session:
                        # Prepare form data for file upload
                        form_data = aiohttp.FormData()
                        form_data.add_field(
                            'file',
                            doc_bytes,
                            filename=attachment.filename,
                            content_type='application/octet-stream'
                        )
                        
                        # Send to Paladin RAG endpoint
                        url = f"http://{self.paladin_url}/api/v1/documents/upload"
                        print(f"[DOCS] Uploading to Paladin: {url}", file=sys.stderr)
                        
                        async with session.post(url, data=form_data, timeout=aiohttp.ClientTimeout(total=300)) as response:
                            if response.status == 200:
                                result = await response.json()
                                print(f"[DOCS] Upload successful: {result}", file=sys.stderr)
                                
                                # Step 4: Generate analysis of the response using OpenAI
                                analysis_prompt = f"""The document '{attachment.filename}' was processed with the following result:
                                
Status: {result.get('status')}
Message: {result.get('message')}
Chunks created: {result.get('chunks_created', 'N/A')}
Document ID: {result.get('document_id')}
Collection: {result.get('collection_name')}

Please provide a user-friendly summary of this processing result, explaining what happened and what this means for the user."""
                                
                                try:
                                    analysis_response = openai.chat.completions.create(
                                        model=os.getenv("OPENAI_MODEL", "gpt-4o-mini"),
                                        messages=[
                                            {
                                                "role": "system",
                                                "content": "You are a helpful assistant explaining document processing results to users. Be clear, concise, and friendly."
                                            },
                                            {
                                                "role": "user",
                                                "content": analysis_prompt
                                            }
                                        ],
                                        temperature=0.7,
                                        max_tokens=500
                                    )
                                    
                                    analysis = analysis_response.choices[0].message.content
                                    
                                    # Send formatted response
                                    await thread.send(f"""✅ **Document Processed Successfully**

{analysis}

📊 **Technical Details:**
• Document ID: `{result.get('document_id')}`
• Chunks created: {result.get('chunks_created', 'N/A')}
• Collection: `{result.get('collection_name')}`""")
                                    
                                except Exception as e:
                                    print(f"[DOCS ERROR] Failed to analyze response: {e}", file=sys.stderr)
                                    # Fallback to simple response
                                    await thread.send(f"""✅ **Document Processed Successfully**

Your document `{attachment.filename}` has been successfully stored in the knowledge base.

📊 **Details:**
• Document ID: `{result.get('document_id')}`
• Chunks created: {result.get('chunks_created', 'N/A')}
• Status: {result.get('status')}""")
                                
                            else:
                                error_text = await response.text()
                                print(f"[DOCS ERROR] Upload failed: {response.status} - {error_text}", file=sys.stderr)
                                await thread.send(f"❌ Failed to process `{attachment.filename}`: {error_text}")
                                
                except Exception as e:
                    print(f"[DOCS ERROR] Error processing {attachment.filename}: {e}", file=sys.stderr)
                    await thread.send(f"❌ Error processing `{attachment.filename}`: {str(e)}")
                    
            # Step 5: Final summary if multiple documents
            if len(doc_attachments) > 1:
                await thread.send(f"""
📚 **Summary**
Processed {len(doc_attachments)} document(s). They are now available in the knowledge base for future queries.

You can ask questions about these documents by mentioning me in any channel!""")
            
            # Add success reaction
            await message.add_reaction('✅')
                
        except Exception as e:
            print(f"[DOCS ERROR] Failed to handle document upload: {e}", file=sys.stderr)
            try:
                await message.add_reaction('❌')
                await message.channel.send(f"<@{message.author.id}> Sorry, I encountered an error processing your document(s): {str(e)}")
            except:
                pass

    async def send_chunked_response(self, channel: discord.TextChannel, response: str, user_id: int):
        """Send response in chunks if it exceeds Discord's limit"""
        MAX_LENGTH = 2000
        
        if len(response) <= MAX_LENGTH:
            # Single message
            await channel.send(response)
            return
        
        # Split into chunks while preserving code blocks
        chunks = []
        current_chunk = ""
        in_code_block = False
        code_block_lang = ""
        
        lines = response.split('\n')
        
        for line in lines:
            # Track code blocks
            if line.startswith('```'):
                if not in_code_block:
                    in_code_block = True
                    # Extract language identifier if present
                    code_block_lang = line[3:].strip()
                else:
                    in_code_block = False
            
            # Check if adding this line would exceed limit
            potential_length = len(current_chunk) + len(line) + 1
            
            # If we're in a code block and close to limit, close it properly
            if in_code_block and potential_length > MAX_LENGTH - 100:
                current_chunk += "```\n"
                chunks.append(current_chunk.strip())
                # Start new chunk with code block
                current_chunk = f"```{code_block_lang}\n{line}\n"
            elif potential_length > MAX_LENGTH - 50:
                # Normal chunk boundary
                if current_chunk:
                    chunks.append(current_chunk.strip())
                current_chunk = line + '\n'
            else:
                current_chunk += line + '\n'
        
        # Add remaining content
        if current_chunk:
            # Close any open code blocks
            if in_code_block:
                current_chunk += "```\n"
            chunks.append(current_chunk.strip())
        
        # Send chunks with indicators
        total_chunks = len(chunks)
        for i, chunk in enumerate(chunks):
            chunk_indicator = f"**[Part {i+1}/{total_chunks}]**\n\n" if total_chunks > 1 else ""
            
            # Add mention in first chunk
            if i == 0:
                chunk_text = f"<@{user_id}> {chunk_indicator}{chunk}"
            else:
                chunk_text = f"{chunk_indicator}{chunk}"
            
            try:
                await channel.send(chunk_text)
                # Small delay between chunks to avoid rate limiting
                if i < total_chunks - 1:
                    await asyncio.sleep(0.5)
            except discord.HTTPException as e:
                print(f"[CHUNK ERROR] Failed to send chunk {i+1}: {e}", file=sys.stderr)
                # Try to send error notification
                try:
                    await channel.send(f"<@{user_id}> Sorry, part {i+1} of the response was too long to send.")
                except Exception as chunk_error:
                    print(f"[CHUNK ERROR] Failed to send error notification: {chunk_error}", file=sys.stderr)
                    pass
            except Exception as e:
                # Log unexpected errors but continue
                print(f"[CHUNK ERROR] Unexpected error sending chunk {i+1}: {e}", file=sys.stderr)
                pass

    async def generate_pdf_report(self, user_query: str, response: str, user_name: str, timestamp: datetime) -> BytesIO:
        """Generate a PDF report from the response"""
        buffer = BytesIO()
        
        # Create PDF document
        doc = SimpleDocTemplate(
            buffer,
            pagesize=letter,
            rightMargin=72,
            leftMargin=72,
            topMargin=72,
            bottomMargin=72
        )
        
        # Get styles
        styles = getSampleStyleSheet()
        
        # Custom styles
        title_style = ParagraphStyle(
            'CustomTitle',
            parent=styles['Title'],
            fontSize=24,
            textColor=colors.HexColor('#1a73e8'),
            spaceAfter=30,
            alignment=TA_CENTER
        )
        
        heading_style = ParagraphStyle(
            'CustomHeading',
            parent=styles['Heading1'],
            fontSize=16,
            textColor=colors.HexColor('#1a73e8'),
            spaceAfter=12,
            spaceBefore=12
        )
        
        subheading_style = ParagraphStyle(
            'CustomSubHeading',
            parent=styles['Heading2'],
            fontSize=14,
            textColor=colors.HexColor('#5f6368'),
            spaceAfter=10,
            spaceBefore=10
        )
        
        body_style = ParagraphStyle(
            'CustomBody',
            parent=styles['BodyText'],
            fontSize=11,
            leading=14,
            alignment=TA_JUSTIFY
        )
        
        code_style = ParagraphStyle(
            'Code',
            parent=styles['Code'],
            fontSize=9,
            leftIndent=20,
            rightIndent=20,
            backColor=colors.HexColor('#f5f5f5'),
            borderColor=colors.HexColor('#e0e0e0'),
            borderWidth=1,
            borderPadding=10
        )
        
        # Build content
        story = []
        
        # Title
        story.append(Paragraph("PaladinAI Analysis Report", title_style))
        story.append(Spacer(1, 20))
        
        # Metadata
        metadata = f"""
        <para><b>Generated for:</b> {user_name}<br/>
        <b>Date:</b> {timestamp.strftime('%B %d, %Y at %I:%M %p')}<br/>
        <b>Request Type:</b> Discord Query</para>
        """
        story.append(Paragraph(metadata, body_style))
        story.append(Spacer(1, 20))
        
        # User Query Section
        story.append(Paragraph("User Query", heading_style))
        story.append(Paragraph(user_query, body_style))
        story.append(Spacer(1, 20))
        
        # Response Section
        story.append(Paragraph("Analysis Results", heading_style))
        
        # Convert markdown response to PDF elements
        # Split response into sections for better formatting
        lines = response.split('\n')
        current_code_block = []
        in_code_block = False
        
        for line in lines:
            if line.startswith('```'):
                if not in_code_block:
                    in_code_block = True
                else:
                    # End of code block
                    if current_code_block:
                        code_text = '\n'.join(current_code_block)
                        story.append(Preformatted(code_text, code_style))
                        current_code_block = []
                    in_code_block = False
                continue
            
            if in_code_block:
                current_code_block.append(line)
            else:
                # Process markdown formatting
                if line.startswith('###'):
                    story.append(Paragraph(line[3:].strip(), subheading_style))
                elif line.startswith('##'):
                    story.append(Paragraph(line[2:].strip(), heading_style))
                elif line.startswith('#'):
                    story.append(Paragraph(line[1:].strip(), heading_style))
                elif line.strip():
                    # Convert markdown to HTML for reportlab
                    line_html = self._markdown_to_html(line)
                    try:
                        story.append(Paragraph(line_html, body_style))
                    except Exception as e:
                        # Fallback for problematic content
                        print(f"[PDF] Failed to parse line as HTML: {e}", file=sys.stderr)
                        story.append(Paragraph(line, body_style))
                else:
                    story.append(Spacer(1, 6))
        
        # Footer
        story.append(Spacer(1, 30))
        footer_text = """
        <para align="center"><font size="9" color="#666666">
        This report was generated by PaladinAI via Discord integration.<br/>
        For questions or support, please contact your system administrator.
        </font></para>
        """
        story.append(Paragraph(footer_text, body_style))
        
        # Build PDF
        doc.build(story)
        buffer.seek(0)
        
        return buffer
    
    def _markdown_to_html(self, text: str) -> str:
        """Convert markdown formatting to HTML for reportlab"""
        # First escape special XML characters
        text = text.replace('&', '&amp;')
        text = text.replace('<', '&lt;')
        text = text.replace('>', '&gt;')
        
        # Handle bold (must be done before italic to handle ***text***)
        text = re.sub(r'\*\*(.+?)\*\*', r'<b>\1</b>', text)
        
        # Handle italic
        text = re.sub(r'\*(.+?)\*', r'<i>\1</i>', text)
        
        # Handle inline code
        text = re.sub(r'`(.+?)`', r'<font name="Courier">\1</font>', text)
        
        # Handle bullet points
        if text.strip().startswith('- '):
            text = f"• {text[2:]}"
        elif text.strip().startswith('* '):
            text = f"• {text[2:]}"
        elif re.match(r'^\d+\.\s', text.strip()):
            # Numbered lists
            text = re.sub(r'^(\d+)\.\s', r'\1. ', text.strip())
        
        return text

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
                except Exception as ref_error:
                    print(f"[REFERENCE ERROR] Failed to fetch reply message: {ref_error}", file=sys.stderr)
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