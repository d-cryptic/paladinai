"""
Action analysis utilities for action node.

This module provides utilities for analyzing action requests to determine
data requirements and action types.
"""

import json
import logging
from typing import Dict, Any

from llm.openai import openai
from prompts.workflows.analysis import get_action_analysis_prompt

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
        analysis_prompt = get_action_analysis_prompt(user_input)
        
        response = await openai.chat_completion(
            user_message=analysis_prompt,
            system_prompt="You are an expert SRE analyzing action requests for monitoring systems. Always include the word 'json' in your response when using JSON format.",
            temperature=0.1
        )

        if not response["success"]:
            raise Exception(response.get("error", "OpenAI request failed"))

        result = json.loads(response["content"])
        return result
        
    except Exception as e:
        logger.error(f"Error analyzing action requirements: {str(e)}")
        # Default to requiring metrics if analysis fails
        return {
            "needs_metrics": True,
            "action_type": "data_analysis",
            "data_requirements": {},
            "analysis_level": "basic",
            "reasoning": f"Analysis failed: {str(e)}"
        }