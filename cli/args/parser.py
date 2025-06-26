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
        "--context", type=str, help="Additional context as JSON string"
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
    print("  python main.py --chat 'Check system health' --context '{\"environment\": \"production\"}'")


def has_any_command(args: argparse.Namespace) -> bool:
    """Check if any command arguments were provided."""
    return any([args.test, args.hello, args.status, args.all, args.chat])
