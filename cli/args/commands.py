"""
Command execution logic for Paladin AI CLI
Handles the execution of different CLI commands.
"""

import argparse
from typing import Optional, Dict, Any

from .parser import has_any_command, show_help_message, parse_context


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
        print("🧠 Memory integration enabled - contextual memories will be shown")
        if args.analysis_only:
            print("📊 Analysis-only mode enabled")
        print("=" * 50)
        await interactive_chat(cli, context, analysis_only=args.analysis_only)

    # Memory functionality commands
    if args.memory_help:
        await cli.show_memory_help()
    
    if args.memory_store:
        context = parse_context(args.context) if args.context else None
        await cli.store_instruction(args.memory_store, context=context)
    
    if args.memory_search:
        await cli.search_memories(
            query=args.memory_search,
            limit=args.limit,
            confidence_threshold=args.confidence
        )
    
    if args.memory_context:
        await cli.get_contextual_memories(
            context=args.memory_context,
            workflow_type=args.workflow_type,
            limit=args.limit
        )
    
    if args.memory_types:
        await cli.show_memory_types()
    
    if args.memory_health:
        await cli.check_memory_health()

    # Show help if no commands were provided
    if not has_any_command(args):
        show_help_message()


async def interactive_chat(cli, context: Optional[Dict[str, Any]] = None, analysis_only: bool = False) -> None:
    """
    Start interactive chat mode with continuous user input and memory integration.

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

            # Check for memory commands in interactive mode
            if user_input.startswith("/memory"):
                await handle_interactive_memory_command(cli, user_input)
                continue

            # Show contextual memories before processing (if not analysis_only)
            if not analysis_only:
                try:
                    print("\n🧠 Checking for relevant memories...")
                    
                    # Determine workflow type based on user input
                    workflow_type = "QUERY"
                    if any(keyword in user_input.lower() for keyword in ["fix", "resolve", "troubleshoot", "issue", "problem", "error"]):
                        workflow_type = "INCIDENT"
                    elif any(keyword in user_input.lower() for keyword in ["fetch", "get", "collect", "action", "restart", "start", "stop"]):
                        workflow_type = "ACTION"
                    
                    # Get contextual memories (limit to 2 for interactive mode)
                    await cli.get_contextual_memories(
                        context=user_input,
                        workflow_type=workflow_type,
                        limit=2
                    )
                except Exception as e:
                    print(f"⚠️  Memory lookup failed: {str(e)}")

            # Send message to OpenAI with loading screen
            await cli.chat_with_openai(user_input, context, interactive=True, analysis_only=analysis_only)

        except KeyboardInterrupt:
            print("\n\n👋 Goodbye! Thanks for using Paladin AI!")
            break
        except EOFError:
            print("\n\n👋 Goodbye! Thanks for using Paladin AI!")
            break


async def handle_interactive_memory_command(cli, command: str) -> None:
    """
    Handle memory commands in interactive mode.
    
    Args:
        cli: PaladinCLI instance
        command: Memory command string
    """
    try:
        parts = command.split(maxsplit=2)
        
        if len(parts) < 2:
            print("🧠 Memory commands:")
            print("  /memory help - Show memory help")
            print("  /memory search <query> - Search memories")
            print("  /memory store <instruction> - Store instruction")
            print("  /memory types - Show memory types")
            print("  /memory health - Check memory health")
            return
        
        cmd = parts[1].lower()
        
        if cmd == "help":
            await cli.show_memory_help()
        elif cmd == "search" and len(parts) > 2:
            await cli.search_memories(parts[2], limit=5)
        elif cmd == "store" and len(parts) > 2:
            await cli.store_instruction(parts[2])
        elif cmd == "types":
            await cli.show_memory_types()
        elif cmd == "health":
            await cli.check_memory_health()
        else:
            print("❌ Unknown memory command. Use '/memory help' for available commands.")
            
    except Exception as e:
        print(f"❌ Error executing memory command: {str(e)}")
