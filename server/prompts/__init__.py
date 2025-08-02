"""
Prompts package for Paladin AI Server
Contains prompt templates and utilities for LangGraph workflows.
"""

from .data_collection import (
    query_prompts,
    action_prompts,
    incident_prompts,
)
from .workflows import (
    planning,
    processing,
    tool_decision,
)

from .workflows.analysis import get_query_analysis_prompt, get_incident_analysis_prompt

__all__ = [
    "query_prompts",
    "action_prompts",
    "incident_prompts",
    "get_query_analysis_prompt",
    "get_incident_analysis_prompt",
    "planning",
    "processing",
    "tool_decision",
]
