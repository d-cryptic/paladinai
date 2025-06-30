"""
Chat completion routes for Paladin AI Server.
Contains endpoints for LangGraph workflow execution and categorization.
"""

import uuid
from typing import Dict, Any
from fastapi import APIRouter, HTTPException
from fastapi.responses import StreamingResponse

from models import ChatRequest
from graph import workflow

router = APIRouter()


@router.post("/api/v1/chat")
async def chat_completion(request: ChatRequest) -> Dict[str, Any]:
    """
    LangGraph workflow execution endpoint.

    Accepts a message and executes the categorization workflow,
    returning the categorized result with workflow metadata.
    """
    try:
        # Generate session ID if not provided in additional_context
        session_id = None
        if request.additional_context and "session_id" in request.additional_context:
            session_id = request.additional_context["session_id"]
        else:
            session_id = f"chat_{uuid.uuid4().hex[:12]}"

        # Execute the LangGraph workflow
        result = await workflow.execute(
            user_input=request.message,
            session_id=session_id
        )
        
        # If formatted markdown is available, return it as the primary response
        if isinstance(result, dict) and 'formatted_markdown' in result:
            return {
                "success": result.get("success", True),
                "session_id": session_id,
                "content": result["formatted_markdown"],
                "metadata": {
                    "workflow_type": result.get("categorization", {}).get("workflow_type", "unknown"),
                    "execution_time_ms": result.get("execution_time_ms", 0),
                    "execution_path": result.get("execution_metadata", {}).get("execution_path", [])
                },
                "raw_result": result  # Include full result for debugging if needed
            }
        else:
            # Fallback to returning the raw result
            return result

    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Workflow execution error: {str(e)}")


@router.post("/api/v1/chat/stream")
async def chat_stream(request: ChatRequest) -> StreamingResponse:
    """
    LangGraph workflow streaming endpoint.

    Accepts a message and streams the workflow execution in real-time,
    returning state updates as they occur.
    """
    import json

    async def generate_stream():
        session_id = None
        try:
            # Generate session ID if not provided in additional_context
            if request.additional_context and "session_id" in request.additional_context:
                session_id = request.additional_context["session_id"]
            else:
                session_id = f"stream_{uuid.uuid4().hex[:12]}"

            # Stream the LangGraph workflow execution
            async for state_update in workflow.stream(
                user_input=request.message,
                session_id=session_id
            ):
                # Format state update as JSON
                yield f"data: {json.dumps(state_update)}\n\n"

        except Exception as e:
            # Send error as final message
            error_response = {
                "error": {
                    "success": False,
                    "error": f"Workflow streaming error: {str(e)}",
                    "session_id": session_id
                }
            }
            yield f"data: {json.dumps(error_response)}\n\n"

    return StreamingResponse(
        generate_stream(),
        media_type="text/plain",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "Content-Type": "text/plain; charset=utf-8"
        }
    )
