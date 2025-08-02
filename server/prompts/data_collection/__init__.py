"""
Data Collection Prompts Package for PaladinAI LangGraph Workflows.

This package contains workflow-specific prompts for data collection,
evaluation, and processing across QUERY, INCIDENT, and ACTION workflows.
"""

from .query_prompts import get_query_prompt, get_query_examples
from .incident_prompts import get_incident_prompt, get_incident_examples
from .action_prompts import get_action_prompt, get_action_examples

__all__ = [
    "get_query_prompt",
    "get_query_examples",
    "get_incident_prompt", 
    "get_incident_examples",
    "get_action_prompt",
    "get_action_examples"
]
