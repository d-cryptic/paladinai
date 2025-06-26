#!/usr/bin/env python3
"""
PaladinAI CLI Banner and Help System

This module provides banner display and help
text functionality for the PaladinAI CLI.
"""


def print_banner() -> None:
    """Print the PaladinAI banner."""
    banner = """
╔═════════════════════════════════════════════════════════════════════════════╗
║                                                                             ║
║  ██████╗  █████╗ ██╗      █████╗ ██████╗ ██╗███╗   ██╗ █████╗ ██╗          ║
║  ██╔══██╗██╔══██╗██║     ██╔══██╗██╔══██╗██║████╗  ██║██╔══██╗██║           ║
║  ██████╔╝███████║██║     ███████║██║  ██║██║██╔██╗ ██║███████║██║           ║
║  ██╔═══╝ ██╔══██║██║     ██╔══██║██║  ██║██║██║╚██╗██║██╔══██║██║           ║
║  ██║     ██║  ██║███████╗██║  ██║██████╔╝██║██║ ╚████║██║  ██║██║           ║
║  ╚═╝     ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚═════╝ ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝╚═╝           ║
║                                                                             ║
║              AI-Powered Monitoring & Incident Response Platform            ║
║                                                                             ║
╚═════════════════════════════════════════════════════════════════════════════╝
"""
    print(banner)


def display_banner() -> None:
    """
    Display the PaladinAI banner - alias
    for print_banner for CLI compatibility.
    """
    print_banner()
