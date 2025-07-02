"""
Paladin AI CLI Client
Main client class combining all functionality modules.
"""

from .base import BaseHTTPClient
from .health import HealthMixin
from .openai import OpenAIMixin
from .memory import MemoryMixin
from .checkpoint import CheckpointMixin


class PaladinCLI(BaseHTTPClient, HealthMixin, OpenAIMixin, MemoryMixin, CheckpointMixin):
    """
    Main CLI client class for Paladin AI server communication.

    Combines base HTTP functionality with health checks and OpenAI features.
    """
    def __init__(self) -> None:
        """Initialize the PaladinCLI client."""
        super().__init__()  # Initialize BaseHTTPClient
