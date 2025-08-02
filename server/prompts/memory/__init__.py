"""
Memory-Enhanced Prompts for PaladinAI Agent Orchestration System

This module contains prompts that leverage memory context for enhanced
decision making and context-aware operations.
"""

from .context_retrieval import (
    MEMORY_CONTEXT_RETRIEVAL_PROMPT,
    MEMORY_SIMILARITY_SEARCH_PROMPT,
    get_memory_context_prompt
)

from .decision_making import (
    MEMORY_GUIDED_DECISION_PROMPT,
    MEMORY_PATTERN_ANALYSIS_PROMPT,
    get_memory_guided_decision_prompt
)

from .analysis_enhancement import (
    MEMORY_ENHANCED_ANALYSIS_PROMPT,
    MEMORY_CORRELATION_PROMPT,
    get_memory_enhanced_analysis_prompt
)

__all__ = [
    "MEMORY_CONTEXT_RETRIEVAL_PROMPT",
    "MEMORY_SIMILARITY_SEARCH_PROMPT", 
    "get_memory_context_prompt",
    "MEMORY_GUIDED_DECISION_PROMPT",
    "MEMORY_PATTERN_ANALYSIS_PROMPT",
    "get_memory_guided_decision_prompt",
    "MEMORY_ENHANCED_ANALYSIS_PROMPT",
    "MEMORY_CORRELATION_PROMPT",
    "get_memory_enhanced_analysis_prompt"
]
