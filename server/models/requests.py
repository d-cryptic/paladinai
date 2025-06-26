"""
Request models for Paladin AI Server API endpoints.
Contains Pydantic models for validating incoming API requests.
"""

from typing import Dict, Any, Optional
from pydantic import BaseModel


class ChatRequest(BaseModel):
    """Request model for chat completion."""
    message: str
    system_prompt: Optional[str] = None
    model: Optional[str] = None
    max_tokens: Optional[int] = None
    temperature: Optional[float] = None
    additional_context: Optional[Dict[str, Any]] = None
