"""
Action node for PaladinAI LangGraph Workflow.

This package implements the action node that handles action-type requests
and routes metrics-related requests to the prometheus node.
"""

from .action import action_node, ActionNode

__all__ = ["action_node", "ActionNode"]