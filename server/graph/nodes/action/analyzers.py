"""
Action analysis utilities for action node.

This module provides utilities for analyzing action requests to determine
data requirements and action types.
"""

import json
import logging
from typing import Dict, Any

from llm.openai import openai
from prompts.workflows.analyzer_prompts import get_analyzer_prompt, ANALYZER_SYSTEM_PROMPT

logger = logging.getLogger(__name__)


async def analyze_action_requirements(user_input: str) -> Dict[str, Any]:
    """
    Analyze the action request to determine data requirements and action type.
    
    Args:
        user_input: The user's action request
        
    Returns:
        Dictionary containing action analysis results
    """
    try:
        # Use the centralized analyzer prompt
        analysis_prompt = get_analyzer_prompt(
            "ACTION",
            user_input=user_input
        )
        
        response = await openai.chat_completion(
            user_message=analysis_prompt,
            system_prompt=ANALYZER_SYSTEM_PROMPT,
            temperature=0.1
        )

        if not response["success"]:
            raise Exception(response.get("error", "OpenAI request failed"))

        result = json.loads(response["content"])
        
        # Double-check log requirement - only if explicitly mentioned
        needs_logs = result.get("needs_logs", False)
        if needs_logs:
            # Verify user actually asked for logs
            log_keywords = ["log", "logs", "error message", "stack trace", "exception", "debug", "console output", "logging"]
            user_input_lower = user_input.lower()
            has_log_keyword = any(keyword in user_input_lower for keyword in log_keywords)
            
            if not has_log_keyword:
                logger.info(f"Overriding needs_logs=True to False - no explicit log keywords found in: {user_input[:100]}")
                needs_logs = False
                result["needs_logs"] = False
        
        return result
        
    except Exception as e:
        logger.error(f"Error analyzing action requirements: {str(e)}")
        # Default to not requiring either if analysis fails
        return {
            "needs_metrics": False,
            "needs_logs": False,
            "action_type": "data_analysis",
            "data_requirements": {},
            "analysis_level": "basic",
            "reasoning": f"Analysis failed: {str(e)}"
        }