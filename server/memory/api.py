"""
Memory API endpoints for PaladinAI.

This module provides REST API endpoints for memory management,
including instruction storage, memory search, and extraction.
"""

import logging
from typing import Dict, Any, List
from fastapi import APIRouter, HTTPException, Query
from pydantic import BaseModel

from .models import (
    MemoryInstructionRequest, MemorySearchQuery,
    MemoryExtractionRequest
)
from .service import get_memory_service
from .extractor import memory_extractor

logger = logging.getLogger(__name__)

# Create router for memory endpoints
memory_router = APIRouter(prefix="/api/memory", tags=["memory"])


class MemoryInstructionResponse(BaseModel):
    """Response model for instruction storage."""
    success: bool
    memory_id: str = None
    relationships_count: int = 0
    memory_type: str = None
    error: str = None


class MemorySearchResponse(BaseModel):
    """Response model for memory search."""
    success: bool
    total_results: int = 0
    memories: List[Dict[str, Any]] = []
    query: str = None
    error: str = None


class ContextualMemoriesResponse(BaseModel):
    """Response model for contextual memories."""
    success: bool
    memories: List[Dict[str, Any]] = []
    context: str = None
    workflow_type: str = None
    error: str = None


@memory_router.post("/instruction", response_model=MemoryInstructionResponse)
async def store_instruction(request: MemoryInstructionRequest) -> MemoryInstructionResponse:
    """
    Store an explicit instruction as memory.
    
    This endpoint allows users to explicitly provide instructions that should
    be remembered for future workflow executions.
    
    Args:
        request: Instruction storage request
        
    Returns:
        Storage result with memory ID and metadata
    """
    try:
        logger.info(f"Storing instruction for user: {request.user_id}")
        
        result = await get_memory_service().store_instruction(request)
        
        if result["success"]:
            return MemoryInstructionResponse(
                success=True,
                memory_id=result.get("memory_id"),
                relationships_count=result.get("relationships_count", 0),
                memory_type=result.get("memory_type")
            )
        else:
            raise HTTPException(
                status_code=500,
                detail=f"Failed to store instruction: {result.get('error')}"
            )
    
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error storing instruction: {str(e)}")
        error_msg = f"Failed to store instruction: {str(e)}"
        raise HTTPException(status_code=500, detail=error_msg)


@memory_router.post("/search", response_model=MemorySearchResponse)
async def search_memories(query: MemorySearchQuery) -> MemorySearchResponse:
    """
    Search memories using vector similarity and filters.
    
    This endpoint provides semantic search capabilities across stored memories
    with support for filtering by memory type, user, and confidence thresholds.
    
    Args:
        query: Search query parameters
        
    Returns:
        Search results with relevant memories
    """
    try:
        logger.info(f"Searching memories for query: {query.query[:50]}...")
        
        result = await get_memory_service().search_memories(query)
        
        if result["success"]:
            return MemorySearchResponse(
                success=True,
                total_results=result.get("total_results", 0),
                memories=result.get("memories", []),
                query=result.get("query")
            )
        else:
            raise HTTPException(
                status_code=500,
                detail=f"Failed to search memories: {result.get('error')}"
            )
    
    except Exception as e:
        logger.error(f"Error searching memories: {str(e)}")
        raise HTTPException(status_code=500, detail=str(e))


@memory_router.get("/contextual", response_model=ContextualMemoriesResponse)
async def get_contextual_memories(
    context: str = Query(..., description="Context or situation to get memories for"),
    workflow_type: str = Query(..., description="Type of workflow (QUERY, ACTION, INCIDENT)"),
    limit: int = Query(5, description="Maximum number of memories to return")
) -> ContextualMemoriesResponse:
    """
    Get contextually relevant memories for a given situation.
    
    This endpoint provides memories that are relevant to the current context
    and workflow type, helping to inform decision-making.
    
    Args:
        context: Current context or situation
        workflow_type: Type of workflow
        limit: Maximum number of memories to return
        
    Returns:
        List of contextually relevant memories
    """
    try:
        logger.info(f"Getting contextual memories for {workflow_type}: {context[:50]}...")
        
        memories = await memory_extractor.get_contextual_memories_for_workflow(
            user_input=context,
            workflow_type=workflow_type,
            limit=limit
        )
        
        return ContextualMemoriesResponse(
            success=True,
            memories=memories,
            context=context,
            workflow_type=workflow_type
        )
    
    except Exception as e:
        logger.error(f"Error getting contextual memories: {str(e)}")
        raise HTTPException(status_code=500, detail=str(e))


@memory_router.post("/extract")
async def extract_memories(request: MemoryExtractionRequest) -> Dict[str, Any]:
    """
    Extract and store memories from workflow interactions.
    
    This endpoint analyzes workflow content and extracts valuable memories
    for future reference and learning.
    
    Args:
        request: Memory extraction request
        
    Returns:
        Extraction and storage results
    """
    try:
        logger.info(f"Extracting memories from {request.workflow_type} workflow")
        
        result = await get_memory_service().extract_and_store_memories(request)
        
        return result
    
    except Exception as e:
        logger.error(f"Error extracting memories: {str(e)}")
        raise HTTPException(status_code=500, detail=str(e))


@memory_router.get("/types")
async def get_memory_types() -> Dict[str, Any]:
    """
    Get available memory types.
    
    Returns:
        Dictionary with available memory types and their descriptions
    """
    return {
        "memory_types": "Dynamic - types are created based on content",
        "examples": [
            "instruction", "pattern", "preference", "knowledge", 
            "operational", "deployment_pattern", "security_configuration",
            "incident_resolution_step", "performance_baseline", "error_pattern"
        ],
        "descriptions": {
            "instruction": "Explicit instructions from users",
            "pattern": "Learned patterns from interactions",
            "preference": "User preferences and settings",
            "knowledge": "Factual knowledge learned from workflows",
            "operational": "Operational procedures and workflows",
            "dynamic": "New types are created automatically based on the nature of the memory"
        }
    }


@memory_router.get("/health")
async def memory_health_check() -> Dict[str, Any]:
    """
    Check memory service health.
    
    Returns:
        Health status of memory backends
    """
    try:
        # Test memory service connectivity
        test_query = MemorySearchQuery(
            query="health check",
            limit=1,
            confidence_threshold=0.9
        )
        
        result = await get_memory_service().search_memories(test_query)
        
        return {
            "status": "healthy" if result["success"] else "unhealthy",
            "backends": {
                "mem0": "connected",
                "qdrant": "connected",
                "neo4j": "connected"
            },
            "search_test": result["success"]
        }
    
    except Exception as e:
        logger.error(f"Memory health check failed: {str(e)}")
        return {
            "status": "unhealthy",
            "error": str(e)
        }