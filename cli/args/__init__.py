"""
Args package for Paladin AI CLI
Contains argument parsing and command execution logic.
"""

from .parser import create_parser, parse_context
from .commands import execute_commands

__all__ = ["create_parser", "parse_context", "execute_commands"]
