"""
Routes package for Paladin AI Server
Contains FastAPI route handlers organized by functionality.
"""

from .health import router as health_router
from .chat import router as chat_router

__all__ = ["health_router", "chat_router"]
