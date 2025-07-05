"""
Routes package for Paladin AI Server
Contains FastAPI route handlers organized by functionality.
"""

from .health import router as health_router
from .chat import router as chat_router
from .documents import router as documents_router
from .alerts import router as alerts_router

__all__ = ["health_router", "chat_router", "documents_router", "alerts_router"]
