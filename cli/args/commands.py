"""
Command execution logic for Paladin AI CLI
Handles the execution of different CLI commands.
"""

import argparse
from typing import Optional, Dict, Any

from .parser import has_any_command, show_help_message


async def execute_commands(cli, args: argparse.Namespace, context: Optional[Dict[str, Any]] = None) -> None:
    """
    Execute CLI commands based on parsed arguments.
    
    Args:
        cli: PaladinCLI instance
        args: Parsed command-line arguments
        context: Optional context dictionary for chat commands
    """
    # Connection testing commands
    if args.test or args.all:
        print("\n🔍 Testing server connection...")
        await cli.test_connection()

    if args.hello or args.all:
        print("\n👋 Getting hello message...")
        await cli.get_hello()

    if args.status or args.all:
        print("\n📊 Getting API status...")
        await cli.get_api_status()

    # OpenAI functionality
    if args.chat:
        print(f"\n🤖 Sending message to OpenAI via server: {args.chat}")
        await cli.chat_with_openai(args.chat, context, analysis_only=args.analysis_only)

    if args.interactive:
        print("\n🤖 Starting interactive chat mode...")
        print("💡 Type 'exit' or 'bye' to quit")
        if args.analysis_only:
            print("📊 Analysis-only mode enabled")
        print("=" * 50)
        await interactive_chat(cli, context, analysis_only=args.analysis_only)

    # Show help if no commands were provided
    if not has_any_command(args):
        show_help_message()


async def interactive_chat(cli, context: Optional[Dict[str, Any]] = None, analysis_only: bool = False) -> None:
    """
    Start interactive chat mode with continuous user input.

    Args:
        cli: PaladinCLI instance
        context: Optional context dictionary for chat commands
        analysis_only: If True, shows only analysis section of responses
    """
    while True:
        try:
            # Get user input
            user_input = input("\n💬 You: ").strip()

            # Check for exit commands
            if user_input.lower() in ['exit', 'bye', 'quit']:
                print("\n👋 Goodbye! Thanks for using Paladin AI!")
                break

            # Skip empty inputs
            if not user_input:
                continue

            # Send message to OpenAI with loading screen
            await cli.chat_with_openai(user_input, context, interactive=True, analysis_only=analysis_only)

        except KeyboardInterrupt:
            print("\n\n👋 Goodbye! Thanks for using Paladin AI!")
            break
        except EOFError:
            print("\n\n👋 Goodbye! Thanks for using Paladin AI!")
            break
