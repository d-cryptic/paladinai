"""
Chat completion routes for Paladin AI Server.
Contains endpoints for OpenAI chat completion functionality.
"""

from typing import Dict, Any
from fastapi import APIRouter, HTTPException

from models import ChatRequest
from llm.openai import openai

router = APIRouter()


@router.post("/api/v1/chat")
async def chat_completion(request: ChatRequest) -> Dict[str, Any]:
    """
    OpenAI chat completion endpoint.

    Accepts a message and optional parameters, returns OpenAI response.
    """
    try:
        result = await openai.chat_completion(
            user_message=request.message,
            system_prompt=request.system_prompt,
            model=request.model,
            max_tokens=request.max_tokens,
            temperature=request.temperature,
            additional_context=request.additional_context
        )
        return result
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"OpenAI API error: {str(e)}")
