#!/usr/bin/env python3
"""
Start Discord message processing worker (Server Mode)
This worker sends messages to PaladinAI server for storage
"""
import sys
import os
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from discord_mcp.workers_server import start_worker

if __name__ == "__main__":
    print("Starting Discord Worker in Server Mode...")
    print("Messages will be sent to PaladinAI server for memory storage")
    start_worker()