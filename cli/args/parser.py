"""
Argument parser for Paladin AI CLI
Handles command-line argument parsing and validation.
"""

import argparse
import json
from typing import Optional, Dict, Any


def create_parser() -> argparse.ArgumentParser:
    """Create and configure the argument parser for Paladin AI CLI."""
    parser = argparse.ArgumentParser(description="Paladin AI CLI Client")
    
    # Connection testing arguments
    parser.add_argument(
        "--test", action="store_true", help="Test server connection"
    )
    parser.add_argument(
        "--hello", action="store_true", help="Get hello message from server"
    )
    parser.add_argument(
        "--status", action="store_true", help="Get API status from server"
    )
    parser.add_argument("--all", action="store_true", help="Run all tests")

    # OpenAI functionality arguments
    parser.add_argument(
        "--chat", type=str, help="Send a message to OpenAI via server"
    )
    parser.add_argument(
        "--analysis-only", action="store_true", help="Show only analysis section in responses"
    )
    parser.add_argument(
        "--interactive", action="store_true", help="Start interactive chat mode"
    )
    parser.add_argument(
        "--context", type=str, help="Additional context as JSON string"
    )
    
    # Memory functionality arguments
    parser.add_argument(
        "--memory-store", type=str, help="Store an instruction in memory"
    )
    parser.add_argument(
        "--memory-search", type=str, help="Search memories using semantic similarity"
    )
    parser.add_argument(
        "--memory-context", type=str, help="Get contextual memories for given situation"
    )
    parser.add_argument(
        "--memory-types", action="store_true", help="Show available memory types"
    )
    parser.add_argument(
        "--memory-health", action="store_true", help="Check memory service health"
    )
    parser.add_argument(
        "--memory-help", action="store_true", help="Show memory features help"
    )
    parser.add_argument(
        "--workflow-type", type=str, choices=["QUERY", "ACTION", "INCIDENT"], 
        default="QUERY", help="Workflow type for contextual memories"
    )
    parser.add_argument(
        "--limit", type=int, default=10, help="Maximum number of results for memory operations"
    )
    parser.add_argument(
        "--confidence", type=float, default=0.5, help="Minimum confidence threshold for memory search"
    )
    
    return parser


def parse_context(context_str: Optional[str]) -> Optional[Dict[str, Any]]:
    """
    Parse context string as JSON.
    
    Args:
        context_str: JSON string to parse
        
    Returns:
        Parsed context dictionary or None if parsing fails
        
    Raises:
        ValueError: If JSON parsing fails
    """
    if not context_str:
        return None
        
    try:
        return json.loads(context_str)
    except json.JSONDecodeError as e:
        raise ValueError(f"Invalid JSON format for context: {e}")


def show_help_message() -> None:
    """Display help message when no arguments are provided."""
    print("\n🚀 Welcome to Paladin AI CLI!")
    print("Use --help to see available options")
    print("Quick test: python main.py --all")
    print("\nOpenAI Examples:")
    print("  python main.py --chat 'What is the status of our system?'")
    print("  python main.py --interactive  # Start continuous chat mode")
    print("  python main.py --chat 'Check system health' --context '{\"environment\": \"production\"}'")
    print("\nMemory Examples:")
    print("  python main.py --memory-store 'Always check systemd logs for app errors'")
    print("  python main.py --memory-search 'CPU alerts' --limit 5")
    print("  python main.py --memory-context 'high memory usage' --workflow-type ACTION")
    print("  python main.py --memory-help  # Show detailed memory features")
    print("  python main.py --memory-health  # Check memory service status")


def has_any_command(args: argparse.Namespace) -> bool:
    """Check if any command arguments were provided."""
    return any([
        args.test, args.hello, args.status, args.all, args.chat, args.interactive,
        args.memory_store, args.memory_search, args.memory_context, args.memory_types,
        args.memory_health, args.memory_help
    ])
