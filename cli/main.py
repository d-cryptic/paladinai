"""
Paladin AI CLI Client
Entry point for the CLI application.
"""

import asyncio

from utils.banner import display_banner
from client import PaladinCLI
from args import create_parser, parse_context, execute_commands

async def main() -> None:
    """Main entry point for the CLI application."""
    # Create and parse arguments
    parser = create_parser()
    args = parser.parse_args()

    # Display banner
    display_banner()

    # Parse context if provided
    try:
        context = parse_context(args.context)
    except ValueError as e:
        print(f"❌ {e}")
        return

    # Execute commands with the CLI client
    async with PaladinCLI() as cli:
        await execute_commands(cli, args, context)


if __name__ == "__main__":
    asyncio.run(main())
