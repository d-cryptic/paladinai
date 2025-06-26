"""
LangGraph Package for PaladinAI Workflow Orchestration.

This package contains the LangGraph workflow implementation for
categorizing and processing user requests in the PaladinAI system.
"""

from .workflow import PaladinWorkflow, workflow
from .state import (
    WorkflowState,
    CategorizationResult,
    NodeResult,
    GraphConfig,
    WorkflowType,
    ComplexityLevel,
    NodeStatus,
    ExecutionPhase,
    create_initial_state,
    update_state_node,
    finalize_state,
    set_categorization_result,
    set_error,
    get_state_summary
)
from .nodes import (
    start_node,
    categorization_node,
    result_node,
    error_handler_node
)

__all__ = [
    # Main workflow
    "PaladinWorkflow",
    "workflow",

    # State management - Core models
    "WorkflowState",
    "CategorizationResult",
    "NodeResult",
    "GraphConfig",

    # State management - Enums
    "WorkflowType",
    "ComplexityLevel",
    "NodeStatus",
    "ExecutionPhase",

    # State management - Utilities
    "create_initial_state",
    "update_state_node",
    "finalize_state",
    "set_categorization_result",
    "set_error",
    "get_state_summary",

    # Nodes
    "start_node",
    "categorization_node",
    "result_node",
    "error_handler_node"
]
