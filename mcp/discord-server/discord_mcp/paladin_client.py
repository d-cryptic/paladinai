#!/usr/bin/env python3
"""
PaladinAI client for Discord MCP server
Handles communication with PaladinAI server for memory storage
"""
import os
import aiohttp
import asyncio
from typing import Dict, Any, Optional
from pathlib import Path
from dotenv import load_dotenv

# Load environment variables
load_dotenv(Path(__file__).parent.parent.parent.parent / '.env')

class PaladinClient:
    """Client for interacting with PaladinAI server"""
    
    def __init__(self):
        self.base_url = f"http://{os.getenv('SERVER_HOST', '127.0.0.1')}:{os.getenv('SERVER_PORT', '8000')}"
        self.timeout = aiohttp.ClientTimeout(total=60)
    
    async def store_memory_instruction(self, instruction: str, user_id: str, context: Dict[str, Any]) -> Dict[str, Any]:
        """
        Store an instruction in PaladinAI memory via server endpoint
        
        Args:
            instruction: The instruction/message to store
            user_id: Discord user ID
            context: Additional context (channel, timestamp, etc.)
            
        Returns:
            Response from server
        """
        url = f"{self.base_url}/api/memory/instruction"
        
        payload = {
            "instruction": instruction,
            "user_id": f"discord_{user_id}",
            "context": context
        }
        
        async with aiohttp.ClientSession() as session:
            try:
                async with session.post(url, json=payload, timeout=self.timeout) as response:
                    result = await response.json()
                    
                    if response.status == 200:
                        return {
                            "success": True,
                            "memory_id": result.get("memory_id"),
                            "relationships_count": result.get("relationships_count", 0),
                            "memory_type": result.get("memory_type")
                        }
                    else:
                        return {
                            "success": False,
                            "error": f"Server returned {response.status}: {result.get('detail', 'Unknown error')}"
                        }
                        
            except asyncio.TimeoutError:
                return {"success": False, "error": "Request timeout"}
            except Exception as e:
                return {"success": False, "error": str(e)}
    
    async def extract_and_store_memory(self, message_data: Dict[str, Any]) -> Dict[str, Any]:
        """
        Store Discord message as instruction memory via server endpoint
        
        Args:
            message_data: Complete Discord message data
            
        Returns:
            Response from server
        """
        # Use instruction endpoint for Discord messages
        url = f"{self.base_url}/api/memory/instruction"
        
        # Prepare the content for extraction
        content = message_data["content"]
        
        # Include thread context if available
        if message_data.get("context", {}).get("thread_messages"):
            thread_context = "\n".join([
                f"{m['author_name']}: {m['content']}" 
                for m in message_data["context"]["thread_messages"][-5:]
            ])
            content = f"Thread context:\n{thread_context}\n\nCurrent message:\n{content}"
        
        # Format as instruction with context
        instruction = f"[Discord #{message_data['channel_name']}] {message_data['author_name']}: {content}"
        
        payload = {
            "instruction": instruction,
            "user_id": f"discord_{message_data['author_id']}",
            "context": {
                "source": "discord",
                "channel": message_data["channel_name"],
                "channel_id": message_data["channel_id"],
                "message_id": message_data["id"],
                "author": message_data["author_name"],
                "timestamp": message_data["timestamp"],
                "is_thread": message_data.get("is_thread", False),
                "has_context": bool(message_data.get("context", {}).get("thread_messages"))
            }
        }
        
        print(f"[PALADIN CLIENT] Sending to server: {url}")
        print(f"[PALADIN CLIENT] Instruction: {instruction}")
        print(f"[PALADIN CLIENT] User ID: {payload['user_id']}")
        
        async with aiohttp.ClientSession() as session:
            try:
                print(f"[PALADIN CLIENT] Making POST request to {url}...")
                async with session.post(url, json=payload, timeout=self.timeout) as response:
                    response_text = await response.text()
                    print(f"[PALADIN CLIENT] Response status: {response.status}")
                    print(f"[PALADIN CLIENT] Response text: {response_text[:500]}..." if len(response_text) > 500 else f"[PALADIN CLIENT] Response text: {response_text}")
                    
                    result = await response.json(content_type=None)
                    print(f"[PALADIN CLIENT] Parsed response: {result}")
                    
                    if response.status == 200:
                        return {
                            "success": True,
                            "memory_id": result.get("memory_id"),
                            "memory_type": result.get("memory_type", "instruction"),
                            "relationships_count": result.get("relationships_count", 0)
                        }
                    else:
                        print(f"[PALADIN CLIENT] Error response: {result}")
                        return {
                            "success": False,
                            "error": f"Server returned {response.status}: {result.get('detail', 'Unknown error')}"
                        }
                        
            except asyncio.TimeoutError:
                return {"success": False, "error": "Request timeout"}
            except Exception as e:
                return {"success": False, "error": str(e)}
    
    async def search_memories(self, query: str, user_id: Optional[str] = None, limit: int = 10) -> Dict[str, Any]:
        """
        Search memories via server endpoint
        
        Args:
            query: Search query
            user_id: Optional user ID filter
            limit: Maximum results
            
        Returns:
            Search results from server
        """
        url = f"{self.base_url}/api/memory/search"
        
        payload = {
            "query": query,
            "user_id": f"discord_{user_id}" if user_id else None,
            "limit": limit,
            "memory_types": ["instruction", "extracted", "discord"]
        }
        
        async with aiohttp.ClientSession() as session:
            try:
                async with session.post(url, json=payload, timeout=self.timeout) as response:
                    result = await response.json()
                    
                    if response.status == 200:
                        return {
                            "success": True,
                            "total_results": result.get("total_results", 0),
                            "memories": result.get("memories", [])
                        }
                    else:
                        return {
                            "success": False,
                            "error": f"Server returned {response.status}: {result.get('detail', 'Unknown error')}"
                        }
                        
            except asyncio.TimeoutError:
                return {"success": False, "error": "Request timeout"}
            except Exception as e:
                return {"success": False, "error": str(e)}