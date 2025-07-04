#!/usr/bin/env python3
"""
RQ Workers for processing Discord messages using PaladinAI server
Instead of direct memory storage, uses PaladinAI server endpoints
"""
import os
import json
import asyncio
from typing import Dict, Any, List
from pathlib import Path
from datetime import datetime

from dotenv import load_dotenv
import openai
import redis
from rq import Worker, Queue

# Load environment variables
load_dotenv(Path(__file__).parent.parent.parent.parent / '.env')

# Initialize OpenAI for guardrail checks
openai.api_key = os.getenv("OPENAI_API_KEY")

from .paladin_client import PaladinClient
from .prompts import PALADIN_GUARDRAIL_SYSTEM_PROMPT, PALADIN_KEYWORDS

class MessageProcessor:
    def __init__(self):
        self.paladin_client = PaladinClient()
        
    def check_paladin_relevance(self, message_data: Dict[str, Any]) -> tuple[bool, float, str]:
        """
        Check if message is related to PaladinAI operations using guardrail logic
        Returns: (is_relevant, confidence_score, reason)
        """
        content = message_data.get("content", "")
        channel = message_data.get("channel_name", "")
        
        print(f"[GUARDRAIL] Checking relevance for message...")
        print(f"[GUARDRAIL] Channel: {channel}")
        print(f"[GUARDRAIL] Content preview: {content[:100]}...")
        
        # Use keywords from prompts module
        paladin_keywords = PALADIN_KEYWORDS
        
        # Check for keywords
        content_lower = content.lower()
        found_keywords = [kw for kw in paladin_keywords if kw in content_lower]
        keyword_matches = len(found_keywords)
        
        print(f"[GUARDRAIL] Keyword scan found {keyword_matches} matches: {found_keywords}")
        print(f"[GUARDRAIL] Full content being checked: '{content}'")
        
        # Use OpenAI for more sophisticated relevance checking
        try:
            print(f"[GUARDRAIL] Using OpenAI {os.getenv('OPENAI_MODEL', 'gpt-4o-mini')} for advanced relevance check...")
            
            response = openai.chat.completions.create(
                model=os.getenv("OPENAI_MODEL", "gpt-4o-mini"),
                messages=[
                    {
                        "role": "system",
                        "content": PALADIN_GUARDRAIL_SYSTEM_PROMPT
                    },
                    {
                        "role": "user",
                        "content": f"Channel: {channel}\nMessage: {content}"
                    }
                ],
                temperature=0.1,
                response_format={"type": "json_object"}
            )
            
            result = json.loads(response.choices[0].message.content)
            print(f"[GUARDRAIL] OpenAI response: {result}")
            
            return result["relevant"], result["confidence"], result["reason"]
            
        except Exception as e:
            print(f"[GUARDRAIL ERROR] Failed to use OpenAI, falling back to keyword matching: {e}")
            # Fallback to keyword matching
            if keyword_matches >= 2:
                reason = f"Contains {keyword_matches} relevant keywords: {', '.join(found_keywords[:3])}"
                print(f"[GUARDRAIL] Fallback decision: relevant (confidence: 0.7)")
                return True, 0.7, reason
            elif keyword_matches == 1:
                reason = f"Contains 1 relevant keyword: {found_keywords[0]}"
                print(f"[GUARDRAIL] Fallback decision: possibly relevant (confidence: 0.4)")
                return True, 0.4, reason
            else:
                print(f"[GUARDRAIL] Fallback decision: not relevant (confidence: 0.2)")
                return False, 0.2, "No relevant keywords found"

def process_message(message_data: Dict[str, Any]):
    """Main function called by RQ worker to process a message"""
    print("\n" + "="*80)
    print(f"[WORKER START] Processing new message (Server Mode)")
    print(f"[MESSAGE INFO] ID: {message_data['id']}")
    print(f"[MESSAGE INFO] Author: {message_data['author_name']} (ID: {message_data['author_id']})")
    print(f"[MESSAGE INFO] Channel: #{message_data['channel_name']} (ID: {message_data['channel_id']})")
    print(f"[MESSAGE INFO] Content: {message_data['content'][:200]}{'...' if len(message_data['content']) > 200 else ''}")
    print(f"[MESSAGE INFO] Timestamp: {message_data['timestamp']}")
    
    if message_data.get('is_thread'):
        print(f"[MESSAGE INFO] Thread: {message_data['thread_name']}")
    if message_data.get('is_reply'):
        print(f"[MESSAGE INFO] Reply to: {message_data['replied_to']['author']}")
    
    # Show context if available
    context = message_data.get('context', {})
    if context.get('thread_messages'):
        print(f"[CONTEXT] Thread has {len(context['thread_messages'])} messages")
    if context.get('user_history'):
        print(f"[CONTEXT] User has {len(context['user_history'])} recent messages")
    
    print("-"*80)
    
    processor = MessageProcessor()
    
    # Check if message is relevant to PaladinAI
    print("[STEP 1] Checking message relevance...")
    is_relevant, confidence, reason = processor.check_paladin_relevance(message_data)
    
    print(f"[RELEVANCE CHECK] Relevant: {is_relevant}")
    print(f"[RELEVANCE CHECK] Confidence: {confidence:.2f}")
    print(f"[RELEVANCE CHECK] Reason: {reason}")
    print(f"[RELEVANCE CHECK] Threshold: {os.getenv('MIN_CONFIDENCE_FOR_ACTION', '0.7')}")
    
    if is_relevant and confidence >= float(os.getenv("MIN_CONFIDENCE_FOR_ACTION", "0.7")):
        print(f"[DECISION] ✅ Message meets relevance threshold, proceeding with processing")
        print("-"*80)
        
        # Store in PaladinAI memory via server
        print("[STEP 2] Sending to PaladinAI server for memory storage...")
        
        # Run async function in sync context
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        
        try:
            # Extract and store memory
            print(f"[PALADIN CLIENT] Calling extract_and_store_memory...")
            print(f"[PALADIN CLIENT] Server URL: {processor.paladin_client.base_url}")
            
            result = loop.run_until_complete(
                processor.paladin_client.extract_and_store_memory(message_data)
            )
            
            print(f"[PALADIN CLIENT] Server response received: {result}")
            
            if result["success"]:
                print(f"[SERVER RESPONSE] ✅ Memory stored successfully!")
                print(f"[SERVER RESPONSE] Memory ID: {result.get('memory_id')}")
                print(f"[SERVER RESPONSE] Memory type: {result.get('memory_type')}")
                print(f"[SERVER RESPONSE] Relationships: {result.get('relationships_count', 0)}")
                
                print(f"[WORKER COMPLETE] ✅ Message processed successfully via PaladinAI server")
                print("="*80 + "\n")
                
                return {
                    "status": "processed",
                    "message_id": message_data["id"],
                    "relevant": True,
                    "confidence": confidence,
                    "server_response": result
                }
            else:
                print(f"[SERVER ERROR] ❌ Failed to store memory: {result.get('error', 'Unknown error')}")
                print(f"[WORKER COMPLETE] Message processing failed")
                print("="*80 + "\n")
                
                return {
                    "status": "failed",
                    "message_id": message_data["id"],
                    "relevant": True,
                    "confidence": confidence,
                    "error": result.get('error', 'Server storage failed')
                }
                
        except Exception as e:
            print(f"[ERROR] ❌ Exception during server communication: {e}")
            import traceback
            traceback.print_exc()
            
            return {
                "status": "error",
                "message_id": message_data["id"],
                "error": str(e)
            }
        finally:
            loop.close()
            
    else:
        print(f"[DECISION] ❌ Message below relevance threshold, skipping")
        print(f"[WORKER COMPLETE] Message skipped")
        print("="*80 + "\n")
        
        return {
            "status": "skipped",
            "message_id": message_data["id"],
            "relevant": False,
            "confidence": confidence,
            "reason": reason
        }

def start_worker():
    """Start the RQ worker"""
    print("\n" + "="*80)
    print("[DISCORD WORKER] Starting message processing worker (Server Mode)")
    print("[DISCORD WORKER] Messages will be sent to PaladinAI server for storage")
    print(f"[DISCORD WORKER] Process PID: {os.getpid()}")
    
    # Use VALKEY_HOST from env (which is set to HOST_IP)
    host = os.getenv("VALKEY_HOST", "localhost")
    
    print("[DISCORD WORKER] Configuration:")
    print(f"  - Valkey/Redis: {host}:{os.getenv('VALKEY_PORT', '6379')}")
    print(f"  - Queue: discord_messages")
    print(f"  - Worker prefix: {os.getenv('WORKER_NAME_PREFIX', 'discord-worker')}")
    print(f"  - Min confidence threshold: {os.getenv('MIN_CONFIDENCE_FOR_ACTION', '0.7')}")
    print(f"  - OpenAI Model: {os.getenv('OPENAI_MODEL', 'gpt-4o-mini')}")
    print(f"  - PaladinAI Server: http://{os.getenv('SERVER_HOST', '127.0.0.1')}:{os.getenv('SERVER_PORT', '8000')}")
    print("="*80 + "\n")
    
    redis_conn = redis.Redis(
        host=host,
        port=int(os.getenv("VALKEY_PORT", "6379")),
        password=None,  # No password in current setup
        db=int(os.getenv("VALKEY_DB", "0"))
    )
    
    try:
        # Test Redis connection
        redis_conn.ping()
        print("[REDIS] ✅ Successfully connected to Valkey/Redis")
    except Exception as e:
        print(f"[REDIS ERROR] ❌ Failed to connect to Valkey/Redis: {e}")
        return
    
    worker = Worker(
        ['discord_messages'],
        connection=redis_conn,
        name=f"{os.getenv('WORKER_NAME_PREFIX', 'discord-worker')}-{os.getpid()}"
    )
    print(f"[WORKER] Starting worker: {worker.name}")
    print("[WORKER] Listening for messages...\n")
    worker.work()

if __name__ == "__main__":
    start_worker()