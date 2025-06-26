"""
Utility Functions for PaladinAI LangGraph Workflow State Management.

This module provides helper functions for creating, updating, and
manipulating workflow state objects throughout the execution lifecycle.
"""

import uuid
from typing import Dict, List, Optional, Any
from datetime import datetime

from .models import WorkflowState, NodeResult, CategorizationResult, GraphConfig
from .enums import WorkflowType, ComplexityLevel, NodeStatus, ExecutionPhase


def create_initial_state(
    user_input: str, 
    session_id: Optional[str] = None,
    user_context: Optional[Dict[str, Any]] = None
) -> WorkflowState:
    """
    Create initial workflow state from user input.
    
    Args:
        user_input: The user's input/query
        session_id: Optional session identifier
        user_context: Optional user-specific context
        
    Returns:
        Initial WorkflowState object
    """
    if not session_id:
        session_id = generate_session_id()
    
    return WorkflowState(
        user_input=user_input,
        session_id=session_id,
        current_node="start",
        current_phase=ExecutionPhase.INITIALIZATION,
        execution_path=["start"],
        user_context=user_context or {},
        metadata={
            "created_at": datetime.now().isoformat(),
            "input_length": len(user_input),
            "input_preview": user_input[:100] + "..." if len(user_input) > 100 else user_input
        }
    )


def generate_session_id(prefix: str = "session") -> str:
    """
    Generate a unique session identifier.
    
    Args:
        prefix: Optional prefix for the session ID
        
    Returns:
        Unique session ID string
    """
    return f"{prefix}_{uuid.uuid4().hex[:12]}"


def update_state_node(
    state: WorkflowState, 
    node_name: str,
    phase: Optional[ExecutionPhase] = None
) -> WorkflowState:
    """
    Update state when transitioning to a new node.
    
    Args:
        state: Current workflow state
        node_name: Name of the node being entered
        phase: Optional execution phase to set
        
    Returns:
        Updated WorkflowState object
    """
    state.current_node = node_name
    
    # Add to execution path if not already present
    if node_name not in state.execution_path:
        state.execution_path.append(node_name)
    
    # Update execution phase if provided
    if phase:
        state.current_phase = phase
    
    # Update metadata
    state.metadata[f"{node_name}_entered_at"] = datetime.now().isoformat()
    
    return state


def finalize_state(
    state: WorkflowState, 
    result: Dict[str, Any], 
    execution_time_ms: int
) -> WorkflowState:
    """
    Finalize workflow state with results and timing.
    
    Args:
        state: Current workflow state
        result: Final workflow result
        execution_time_ms: Total execution time in milliseconds
        
    Returns:
        Finalized WorkflowState object
    """
    state.final_result = result
    state.execution_time_ms = execution_time_ms
    state.current_node = "completed"
    state.current_phase = ExecutionPhase.COMPLETED
    
    # Add to execution path if not already present
    if "completed" not in state.execution_path:
        state.execution_path.append("completed")
    
    # Update metadata
    state.metadata.update({
        "completed_at": datetime.now().isoformat(),
        "total_execution_time_ms": execution_time_ms,
        "final_node_count": len(state.node_results),
        "success": result.get("success", False)
    })
    
    return state


def add_node_result(
    state: WorkflowState,
    node_name: str,
    success: bool,
    data: Optional[Dict[str, Any]] = None,
    error: Optional[str] = None,
    execution_time_ms: Optional[int] = None,
    metadata: Optional[Dict[str, Any]] = None
) -> WorkflowState:
    """
    Add a node result to the workflow state.
    
    Args:
        state: Current workflow state
        node_name: Name of the node
        success: Whether the node execution was successful
        data: Optional output data from the node
        error: Optional error message
        execution_time_ms: Optional execution time
        metadata: Optional additional metadata
        
    Returns:
        Updated WorkflowState object
    """
    node_result = NodeResult(
        success=success,
        status=NodeStatus.COMPLETED if success else NodeStatus.FAILED,
        data=data,
        error=error,
        execution_time_ms=execution_time_ms,
        metadata=metadata or {}
    )
    
    state.add_node_result(node_name, node_result)
    return state


def set_categorization_result(
    state: WorkflowState,
    workflow_type: WorkflowType,
    confidence: float,
    reasoning: str,
    suggested_approach: str,
    estimated_complexity: ComplexityLevel,
    **kwargs
) -> WorkflowState:
    """
    Set the categorization result in the workflow state.
    
    Args:
        state: Current workflow state
        workflow_type: Categorized workflow type
        confidence: Confidence score (0.0 to 1.0)
        reasoning: Explanation of the categorization
        suggested_approach: Recommended approach
        estimated_complexity: Estimated complexity level
        **kwargs: Additional categorization metadata
        
    Returns:
        Updated WorkflowState object
    """
    state.categorization = CategorizationResult(
        workflow_type=workflow_type,
        confidence=confidence,
        reasoning=reasoning,
        suggested_approach=suggested_approach,
        estimated_complexity=estimated_complexity,
        **kwargs
    )
    
    # Update phase to categorization complete
    state.current_phase = ExecutionPhase.PROCESSING
    
    # Add categorization metadata
    state.metadata.update({
        "categorization_completed_at": datetime.now().isoformat(),
        "categorization_type": workflow_type.value,
        "categorization_confidence": confidence
    })
    
    return state


def set_error(
    state: WorkflowState,
    error_message: str,
    node_name: Optional[str] = None,
    error_metadata: Optional[Dict[str, Any]] = None
) -> WorkflowState:
    """
    Set an error in the workflow state.
    
    Args:
        state: Current workflow state
        error_message: Error message to set
        node_name: Optional name of the node where error occurred
        error_metadata: Optional additional error metadata
        
    Returns:
        Updated WorkflowState object with error information
    """
    state.error_message = error_message
    state.current_phase = ExecutionPhase.ERROR
    
    # Update metadata
    error_info = {
        "error_occurred_at": datetime.now().isoformat(),
        "error_message": error_message,
        "error_node": node_name or state.current_node
    }
    
    if error_metadata:
        error_info.update(error_metadata)
    
    state.metadata.update(error_info)
    
    return state


def get_state_summary(state: WorkflowState) -> Dict[str, Any]:
    """
    Get a concise summary of the workflow state.
    
    Args:
        state: Workflow state to summarize
        
    Returns:
        Dictionary containing state summary
    """
    return {
        "session_id": state.session_id,
        "current_node": state.current_node,
        "current_phase": state.current_phase.value,
        "execution_path": state.execution_path,
        "has_categorization": state.categorization is not None,
        "has_final_result": state.final_result is not None,
        "has_errors": state.has_errors(),
        "execution_time_ms": state.execution_time_ms,
        "node_count": len(state.node_results),
        "api_calls": state.total_api_calls,
        "tokens_used": state.total_tokens_used
    }


def validate_state_transition(
    current_state: WorkflowState,
    target_node: str
) -> bool:
    """
    Validate if a state transition to a target node is allowed.
    
    Args:
        current_state: Current workflow state
        target_node: Target node to transition to
        
    Returns:
        True if the transition is valid, False otherwise
    """
    # Define valid transitions
    valid_transitions = {
        "start": ["categorization", "error_handler"],
        "categorization": ["result", "error_handler"],
        "result": [],  # Terminal node
        "error_handler": []  # Terminal node
    }
    
    current_node = current_state.current_node
    allowed_targets = valid_transitions.get(current_node, [])
    
    return target_node in allowed_targets


def create_default_config() -> GraphConfig:
    """
    Create a default graph configuration.
    
    Returns:
        Default GraphConfig object
    """
    return GraphConfig()


def merge_user_context(
    state: WorkflowState,
    additional_context: Dict[str, Any]
) -> WorkflowState:
    """
    Merge additional user context into the workflow state.
    
    Args:
        state: Current workflow state
        additional_context: Additional context to merge
        
    Returns:
        Updated WorkflowState object
    """
    state.user_context.update(additional_context)
    state.metadata["context_updated_at"] = datetime.now().isoformat()
    return state
