"""
Client package for Paladin AI CLI
Contains HTTP client functionality organized by feature.
"""

from .main import PaladinCLI
from .base import BaseHTTPClient
from .health import HealthMixin
from .openai import OpenAIMixin

__all__ = ["PaladinCLI", "BaseHTTPClient", "HealthMixin", "OpenAIMixin"]
