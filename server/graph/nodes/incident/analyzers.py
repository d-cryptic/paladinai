"""
Analyzers for incident node - handles incident analysis logic.
"""

import json
import logging
from typing import Dict, Any
from llm.openai import openai
from prompts.workflows.analyzer_prompts import get_analyzer_prompt, ANALYZER_SYSTEM_PROMPT
from prompts.data_collection.incident_prompts import get_incident_prompt

logger = logging.getLogger(__name__)


async def analyze_incident_requirements(user_input: str) -> Dict[str, Any]:
    """
    Analyze the incident to determine investigation requirements.
    
    Args:
        user_input: The user's incident description
        
    Returns:
        Dictionary containing incident analysis results
    """
    try:
        # Use the centralized analyzer prompt
        analysis_prompt = get_analyzer_prompt(
            "INCIDENT",
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
        
        # Double-check log requirement for incidents - be more selective
        needs_logs = result.get("needs_logs", False)
        if needs_logs:
            # For incidents, check if it's really about errors/failures or just performance
            error_keywords = ["error", "exception", "fail", "crash", "bug", "log", "stack trace", "debug", "500", "404"]
            user_input_lower = user_input.lower()
            has_error_keyword = any(keyword in user_input_lower for keyword in error_keywords)
            
            # Also check incident type
            incident_type = result.get("incident_type", "").lower()
            error_related_types = ["error", "failure", "crash", "bug", "application"]
            is_error_incident = any(etype in incident_type for etype in error_related_types)
            
            if not has_error_keyword and not is_error_incident:
                logger.info(f"Overriding needs_logs=True to False for incident - appears to be performance/availability focused: {user_input[:100]}")
                needs_logs = False
                result["needs_logs"] = False
        
        return result
        
    except Exception as e:
        logger.error(f"Error analyzing incident requirements: {str(e)}")
        # Default to metrics and alerts only when analysis fails
        return {
            "needs_metrics": True,
            "needs_logs": False,  # Conservative default - no logs unless explicitly needed
            "needs_alerts": True,
            "incident_type": "general",
            "severity": "medium",
            "investigation_focus": ["performance", "availability"],
            "urgency": "normal",
            "reasoning": f"Analysis failed: {str(e)}. Defaulting to metrics and alerts only.",
            "data_requirements": {
                "metrics": ["cpu", "memory", "error_rate"],
                "logs": [],
                "alerts": ["active_alerts"]
            }
        }


async def process_non_metrics_incident(user_input: str) -> Dict[str, Any]:
    """
    Process incidents that don't require metrics data (rare case).
    
    Args:
        user_input: The user's incident description
        
    Returns:
        Dictionary with processing results
    """
    try:
        # Use incident output formatting prompt for non-metrics incidents
        prompt = get_incident_prompt(
            "output_formatting",
            user_input=user_input,
            collected_data="No metrics data required for this incident",
            timeline="No timeline data available"
        )
        
        response = await openai.chat_completion(
            user_message=prompt,
            system_prompt="You are an expert SRE handling non-metrics incident requests. Always include the word 'json' in your response when using JSON format.",
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
        logger.error(f"Error processing non-metrics incident: {str(e)}")
        return {
            "success": False,
            "error": str(e)
        }