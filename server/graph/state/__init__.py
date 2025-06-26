"""
PaladinAI LangGraph Workflow State Management Package.

This package provides comprehensive state management for LangGraph workflows,
including models, enums, and utility functions for workflow execution.

The state management system is organized into:
- Enums: Workflow types, complexity levels, and execution phases
- Models: Pydantic models for state, results, and configuration
- Utils: Helper functions for state manipulation and validation

Usage:
    from graph.state import WorkflowState, WorkflowType, create_initial_state
    
    # Create initial state
    state = create_initial_state("Is my CPU usage high?")
    
    # Update state during workflow execution
    state = update_state_node(state, "categorization")
    
    # Set categorization results
    state = set_categorization_result(
        state,
        WorkflowType.QUERY,
        0.95,
        "User asking direct question about metrics",
        "Quick metric check",
        ComplexityLevel.LOW
    )
"""

# Import enums
from .enums import (
    WorkflowType,
    ComplexityLevel,
    NodeStatus,
    ExecutionPhase
)

# Import models
from .models import (
    CategorizationResult,
    NodeResult,
    WorkflowState,
    GraphConfig
)

# Import utility functions
from .utils import (
    create_initial_state,
    generate_session_id,
    update_state_node,
    finalize_state,
    add_node_result,
    set_categorization_result,
    set_error,
    get_state_summary,
    validate_state_transition,
    create_default_config,
    merge_user_context
)

# Define public API
__all__ = [
    # Enums
    "WorkflowType",
    "ComplexityLevel", 
    "NodeStatus",
    "ExecutionPhase",
    
    # Models
    "CategorizationResult",
    "NodeResult",
    "WorkflowState",
    "GraphConfig",
    
    # Utility functions
    "create_initial_state",
    "generate_session_id",
    "update_state_node",
    "finalize_state",
    "add_node_result",
    "set_categorization_result",
    "set_error",
    "get_state_summary",
    "validate_state_transition",
    "create_default_config",
    "merge_user_context"
]

# Package metadata
__version__ = "0.1.0"
__author__ = "Barun Debnath"
__email__ = "barundebnath91@gmail.com"
__description__ = "State management for PaladinAI LangGraph workflows"

# Convenience functions for common operations
def quick_state_setup(user_input: str, session_id: str = None) -> WorkflowState:
    """
    Quick setup for a new workflow state with common defaults.
    
    Args:
        user_input: User's input query
        session_id: Optional session identifier
        
    Returns:
        Initialized WorkflowState ready for processing
    """
    return create_initial_state(user_input, session_id)


def is_workflow_complete(state: WorkflowState) -> bool:
    """
    Check if a workflow has completed (successfully or with errors).
    
    Args:
        state: Workflow state to check
        
    Returns:
        True if workflow is complete
    """
    return state.current_phase in {ExecutionPhase.COMPLETED, ExecutionPhase.ERROR}


def get_workflow_progress(state: WorkflowState) -> float:
    """
    Calculate workflow progress as a percentage.
    
    Args:
        state: Workflow state to analyze
        
    Returns:
        Progress percentage (0.0 to 1.0)
    """
    # Define expected phases and their weights
    phase_weights = {
        ExecutionPhase.INITIALIZATION: 0.1,
        ExecutionPhase.CATEGORIZATION: 0.3,
        ExecutionPhase.PROCESSING: 0.5,
        ExecutionPhase.FINALIZATION: 0.9,
        ExecutionPhase.COMPLETED: 1.0,
        ExecutionPhase.ERROR: 1.0  # Consider error as complete
    }
    
    return phase_weights.get(state.current_phase, 0.0)


def format_execution_summary(state: WorkflowState) -> str:
    """
    Format a human-readable execution summary.
    
    Args:
        state: Workflow state to summarize
        
    Returns:
        Formatted summary string
    """
    summary = get_state_summary(state)
    
    lines = [
        f"Session: {summary['session_id']}",
        f"Phase: {summary['current_phase']}",
        f"Progress: {get_workflow_progress(state):.1%}",
        f"Nodes: {summary['node_count']} executed",
        f"Path: {' → '.join(summary['execution_path'])}"
    ]
    
    if summary['execution_time_ms']:
        lines.append(f"Time: {summary['execution_time_ms']}ms")
    
    if summary['has_categorization']:
        cat_type = state.categorization.workflow_type.value
        cat_conf = state.categorization.confidence
        lines.append(f"Type: {cat_type} ({cat_conf:.1%} confidence)")
    
    if summary['has_errors']:
        lines.append("⚠️  Has errors")
    
    return "\n".join(lines)


# Backward compatibility aliases (for existing code)
# These maintain compatibility with the old state.py structure
def create_initial_state_legacy(user_input: str, session_id: str = None) -> WorkflowState:
    """Legacy alias for create_initial_state."""
    return create_initial_state(user_input, session_id)


def update_state_node_legacy(state: WorkflowState, node_name: str) -> WorkflowState:
    """Legacy alias for update_state_node."""
    return update_state_node(state, node_name)


def finalize_state_legacy(state: WorkflowState, result: dict, execution_time_ms: int) -> WorkflowState:
    """Legacy alias for finalize_state."""
    return finalize_state(state, result, execution_time_ms)
