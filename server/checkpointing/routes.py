"""
Checkpoint management routes for Paladin AI Server.
Contains endpoints for managing workflow checkpoints stored in MongoDB.
"""

from typing import Dict, Any, Optional
from fastapi import APIRouter, HTTPException, Query
from pydantic import BaseModel

from .utils import validate_session_id, format_checkpoint_info

router = APIRouter(prefix="/api/v1/checkpoints", tags=["checkpoints"])

# Workflow instance will be set during app initialization
_workflow = None


def set_workflow(workflow_instance):
    """Set the workflow instance for checkpoint operations."""
    global _workflow
    _workflow = workflow_instance


def _ensure_workflow():
    """Ensure workflow is initialized, raise exception if not."""
    if not _workflow:
        raise HTTPException(
            status_code=503,
            detail="Workflow service not initialized. Please try again later."
        )


class CheckpointResponse(BaseModel):
    """Response model for checkpoint operations."""
    success: bool
    message: str
    data: Optional[Dict[str, Any]] = None


@router.get("/{session_id}")
async def get_checkpoint(session_id: str) -> CheckpointResponse:
    """
    Retrieve the latest checkpoint for a session.
    
    Args:
        session_id: The session identifier
        
    Returns:
        Checkpoint data if found
    """
    # Validate session ID
    if not validate_session_id(session_id):
        raise HTTPException(
            status_code=400,
            detail="Invalid session ID format"
        )
    
    _ensure_workflow()
    
    try:
        checkpoint = await _workflow.get_checkpoint(session_id)
        
        if checkpoint:
            # Format checkpoint info for better readability
            formatted_info = format_checkpoint_info(checkpoint)
            return CheckpointResponse(
                success=True,
                message=f"Checkpoint found for session {session_id}",
                data=formatted_info
            )
        else:
            return CheckpointResponse(
                success=False,
                message=f"No checkpoint found for session {session_id}",
                data=None
            )
            
    except Exception as e:
        raise HTTPException(
            status_code=500,
            detail=f"Failed to retrieve checkpoint: {str(e)}"
        )


@router.get("/{session_id}/exists")
async def check_checkpoint_exists(session_id: str) -> Dict[str, Any]:
    """
    Check if a checkpoint exists for a session.
    
    This is a lightweight endpoint that only checks existence without
    loading the full checkpoint data.
    
    Args:
        session_id: The session identifier
        
    Returns:
        JSON response with exists flag
    """
    # Validate session ID
    if not validate_session_id(session_id):
        raise HTTPException(
            status_code=400,
            detail="Invalid session ID format"
        )
    
    _ensure_workflow()
    
    try:
        checkpoint = await _workflow.get_checkpoint(session_id)
        
        return {
            "exists": checkpoint is not None,
            "session_id": session_id
        }
        
    except Exception as e:
        raise HTTPException(
            status_code=500,
            detail=f"Failed to check checkpoint existence: {str(e)}"
        )


@router.get("/")
async def list_checkpoints(
    session_id: Optional[str] = Query(None, description="Filter by session ID"),
    limit: int = Query(10, ge=1, le=100, description="Maximum number of checkpoints to return")
) -> CheckpointResponse:
    """
    List available checkpoints.
    
    Args:
        session_id: Optional session ID to filter by
        limit: Maximum number of checkpoints to return (1-100)
        
    Returns:
        List of checkpoint metadata
    """
    # Validate session ID if provided
    if session_id and not validate_session_id(session_id):
        raise HTTPException(
            status_code=400,
            detail="Invalid session ID format"
        )
    
    _ensure_workflow()
    
    try:
        checkpoints = await _workflow.list_checkpoints(
            session_id=session_id,
            limit=limit
        )
        
        # Format each checkpoint for display
        formatted_checkpoints = [format_checkpoint_info(cp) for cp in checkpoints]
        
        return CheckpointResponse(
            success=True,
            message=f"Found {len(checkpoints)} checkpoints",
            data={"checkpoints": formatted_checkpoints, "count": len(checkpoints)}
        )
        
    except Exception as e:
        raise HTTPException(
            status_code=500,
            detail=f"Failed to list checkpoints: {str(e)}"
        )


@router.delete("/{session_id}")
async def delete_checkpoint(session_id: str) -> CheckpointResponse:
    """
    Delete all checkpoints for a session.
    
    Args:
        session_id: The session identifier
        
    Returns:
        Success status
    """
    # Validate session ID
    if not validate_session_id(session_id):
        raise HTTPException(
            status_code=400,
            detail="Invalid session ID format"
        )
    
    _ensure_workflow()
    
    try:
        success = await _workflow.delete_checkpoint(session_id)
        
        if success:
            return CheckpointResponse(
                success=True,
                message=f"Checkpoints deleted for session {session_id}",
                data=None
            )
        else:
            return CheckpointResponse(
                success=False,
                message=f"No checkpoints found to delete for session {session_id}",
                data=None
            )
            
    except Exception as e:
        raise HTTPException(
            status_code=500,
            detail=f"Failed to delete checkpoint: {str(e)}"
        )