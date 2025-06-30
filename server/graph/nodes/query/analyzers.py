"""
Analyzers for query node - handles query analysis logic.
"""

import json
import logging
from typing import Dict, Any
from llm.openai import openai
from prompts.data_collection.query_prompts import get_query_prompt
from prompts.workflows.analyzer_prompts import get_analyzer_prompt, ANALYZER_SYSTEM_PROMPT
from prompts.workflows.processor_prompts import get_processor_system_prompt

logger = logging.getLogger(__name__)


async def analyze_data_requirements(user_input: str) -> Dict[str, bool]:
    """
    Analyze if the user query requires metrics data from Prometheus and/or logs from Loki.
    
    Args:
        user_input: The user's query
        
    Returns:
        Dictionary with flags for needs_metrics and needs_logs
    """
    try:
        # Use the imported analyzer prompt
        analysis_prompt = get_analyzer_prompt(
            "QUERY",
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
        return {
            "needs_metrics": result.get("needs_metrics", False),
            "needs_logs": result.get("needs_logs", False)
        }
        
    except Exception as e:
        logger.error(f"Error analyzing data requirements: {str(e)}")
        # Default to not requiring either if analysis fails
        return {"needs_metrics": False, "needs_logs": False}


async def process_non_metrics_query(user_input: str) -> Dict[str, Any]:
    """
    Process queries that don't require metrics data.
    
    Args:
        user_input: The user's query
        
    Returns:
        Dictionary with processing results
    """
    try:
        # Use query output formatting prompt for non-metrics queries
        prompt = get_query_prompt(
            "output_formatting",
            user_input=user_input,
            collected_data="No metrics data required for this query",
            data_sources="Direct processing"
        )
        
        response = await openai.chat_completion(
            user_message=prompt,
            system_prompt=get_processor_system_prompt("QUERY", "non_metrics"),
            temperature=0.3
        )

        if not response["success"]:
            raise Exception(response.get("error", "OpenAI request failed"))

        result = json.loads(response["content"])
        
        return {
            "success": True,
            "data": result
        }
        
    except Exception as e:
        logger.error(f"Error processing non-metrics query: {str(e)}")
        return {
            "success": False,
            "error": str(e)
        }