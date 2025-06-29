"""
Analyzers for query node - handles query analysis logic.
"""

import json
import logging
from typing import Dict, Any
from llm.openai import openai
from prompts import get_query_analysis_prompt
from prompts.data_collection.query_prompts import get_query_prompt

logger = logging.getLogger(__name__)


async def analyze_metrics_requirement(user_input: str) -> bool:
    """
    Analyze if the user query requires metrics data from Prometheus.
    
    Args:
        user_input: The user's query
        
    Returns:
        Boolean indicating if metrics data is needed
    """
    try:
        # Use OpenAI to determine if metrics are needed
        analysis_prompt = get_query_analysis_prompt(user_input)
        
        response = await openai.chat_completion(
            user_message=analysis_prompt,
            system_prompt="You are an expert SRE analyzing monitoring queries. Always include the word 'json' in your response when using JSON format.",
            temperature=0.1
        )

        if not response["success"]:
            raise Exception(response.get("error", "OpenAI request failed"))

        result = json.loads(response["content"])
        return result.get("needs_metrics", False)
        
    except Exception as e:
        logger.error(f"Error analyzing metrics requirement: {str(e)}")
        # Default to requiring metrics if analysis fails
        return True


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
            system_prompt="You are an expert SRE providing direct answers to non-metrics queries. Always include the word 'json' in your response when using JSON format.",
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