"""
Action processing utilities for action node.

This module provides utilities for processing different types of actions,
including non-metrics actions and prometheus result processing.
"""

import json
import logging
from typing import Dict, Any

from llm.openai import openai
from prompts.data_collection.action_prompts import get_action_prompt
from prompts.workflows.response_type import get_response_type_prompt
from .serializers import serialize_prometheus_data
from ...state import WorkflowState

logger = logging.getLogger(__name__)


async def process_non_metrics_action(user_input: str, action_analysis: Dict[str, Any]) -> Dict[str, Any]:
    """
    Process actions that don't require metrics data.
    
    Args:
        user_input: The user's action request
        action_analysis: Analysis results from action requirements
        
    Returns:
        Dictionary with processing results
    """
    try:
        # Use action output formatting prompt for non-metrics actions
        prompt = get_action_prompt(
            "output_formatting",
            user_input=user_input,
            collected_data="No metrics data required for this action",
            action_type=action_analysis.get("action_type", "general")
        )
        
        response = await openai.chat_completion(
            user_message=prompt,
            system_prompt="You are an expert SRE handling non-metrics action requests. Always include the word 'json' in your response when using JSON format.",
            temperature=0.3
        )

        if not response["success"]:
            raise Exception(response.get("error", "OpenAI request failed"))

        # Parse JSON response with better error handling
        try:
            result = json.loads(response["content"])
        except json.JSONDecodeError as e:
            logger.error(f"Failed to parse OpenAI response as JSON: {str(e)}")
            logger.error(f"Response content: {response['content'][:500]}...")  # Log first 500 chars
            # Return a fallback response
            result = {
                "error": "Failed to parse OpenAI response",
                "action_type": "unknown",
                "execution_status": "failed",
                "message": "Unable to process the action due to JSON parsing error"
            }
        
        return {
            "success": True,
            "data": result
        }
        
    except Exception as e:
        logger.error(f"Error processing non-metrics action: {str(e)}")
        return {
            "success": False,
            "error": str(e)
        }


async def process_prometheus_result(state: WorkflowState, prometheus_data: Dict[str, Any], node_name: str) -> WorkflowState:
    """
    Process results returned from prometheus node.
    
    Args:
        state: Current workflow state
        prometheus_data: Data returned from prometheus node
        node_name: Name of the action node for metadata storage
        
    Returns:
        Updated workflow state with processed results
    """
    logger.info("Processing prometheus results in action node")
    
    try:
        action_context = state.metadata.get("action_context", {})
        action_type = action_context.get("action_type", "data_analysis")
        
        # Serialize prometheus data to handle Pydantic objects
        serialized_prometheus_data = serialize_prometheus_data(prometheus_data)

        # Use OpenAI to determine the appropriate response type based on categorization
        categorization = state.categorization

        # Build categorization data safely
        categorization_data = {}
        if categorization:
            categorization_data = {
                "workflow_type": getattr(categorization, 'workflow_type', 'ACTION'),
                "confidence": getattr(categorization, 'confidence', 0.0),
                "reasoning": getattr(categorization, 'reasoning', ''),
                "suggested_approach": getattr(categorization, 'suggested_approach', ''),
                "estimated_complexity": getattr(categorization, 'estimated_complexity', 'MEDIUM')
            }

        response_type_prompt = get_response_type_prompt(
            user_input=state.user_input,
            categorization_data=categorization_data
        )

        response_type_response = await openai.chat_completion(
            user_message=response_type_prompt,
            system_prompt="You are an expert SRE determining appropriate response types. Always include the word 'json' in your response when using JSON format.",
            temperature=0.1
        )

        if not response_type_response["success"]:
            # Default to simple data if decision fails
            response_type = "simple_data"
        else:
            try:
                response_decision = json.loads(response_type_response["content"])
                response_type = response_decision.get("response_type", "simple_data")
            except:
                response_type = "simple_data"

        # Use appropriate prompt based on OpenAI decision
        if response_type == "comprehensive_analysis":
            prompt = get_action_prompt(
                "analysis",
                user_input=state.user_input,
                collected_data=json.dumps(serialized_prometheus_data, indent=2),
                analysis_scope=action_type
            )
        elif response_type == "reporting":
            prompt = get_action_prompt(
                "reporting",
                user_input=state.user_input,
                data_sources="Prometheus",
                time_range=action_context.get("data_requirements", {}).get("time_range", "recent")
            )
        else:  # simple_data
            prompt = get_action_prompt(
                "output_formatting",
                user_input=state.user_input,
                collected_data=json.dumps(serialized_prometheus_data, indent=2),
                action_type=action_type
            )
        
        response = await openai.chat_completion(
            user_message=prompt,
            system_prompt="You are an expert SRE processing action results with metrics data. Always include the word 'json' in your response when using JSON format.",
            temperature=0.3
        )

        if not response["success"]:
            raise Exception(response.get("error", "OpenAI request failed"))

        # Parse JSON response with better error handling
        try:
            result = json.loads(response["content"])

            # Log the OpenAI response for debugging
            logger.info(f"OpenAI response for action formatting: {json.dumps(result, indent=2)}")

            # Ensure supporting_metrics is populated from prometheus data if missing
            if "supporting_metrics" not in result or not result["supporting_metrics"]:
                # Extract metrics from prometheus data
                prometheus_metrics = {}
                if "processed_metrics" in serialized_prometheus_data:
                    prometheus_metrics.update(serialized_prometheus_data["processed_metrics"])
                if "current_values" in serialized_prometheus_data:
                    prometheus_metrics.update(serialized_prometheus_data["current_values"])
                if "basic_statistics" in serialized_prometheus_data:
                    prometheus_metrics.update(serialized_prometheus_data["basic_statistics"])

                if prometheus_metrics:
                    result["supporting_metrics"] = prometheus_metrics
                    logger.info(f"Added supporting_metrics from prometheus data: {prometheus_metrics}")
                else:
                    logger.warning(f"No metrics found in prometheus data. Available keys: {list(serialized_prometheus_data.keys())}")
            else:
                logger.info(f"OpenAI response already contains supporting_metrics: {result['supporting_metrics']}")

        except json.JSONDecodeError as e:
            logger.error(f"Failed to parse OpenAI response as JSON: {str(e)}")
            logger.error(f"Response content: {response['content'][:500]}...")  # Log first 500 chars
            # Return a fallback response with metrics from prometheus data if available
            result = {
                "error": "Failed to parse OpenAI response",
                "action_type": action_type,
                "execution_status": "failed",
                "analysis_results": "Unable to process the action results due to JSON parsing error"
            }

            # Try to extract metrics from prometheus data even on error
            prometheus_metrics = {}
            if "processed_metrics" in serialized_prometheus_data:
                prometheus_metrics.update(serialized_prometheus_data["processed_metrics"])
            if "current_values" in serialized_prometheus_data:
                prometheus_metrics.update(serialized_prometheus_data["current_values"])
            if prometheus_metrics:
                result["supporting_metrics"] = prometheus_metrics

        # Store formatted result
        state.metadata["action_result"] = result
        state.metadata["next_node"] = "action_output"

        # Clear prometheus routing flags to prevent loops
        state.metadata["needs_prometheus"] = False
        state.metadata["prometheus_collection_complete"] = False

        # Store prometheus processing info in metadata
        state.metadata[f"{node_name}_prometheus_processing"] = {
            "timestamp": state.metadata.get("current_timestamp"),
            "status": "completed",
            "data_processed": True,
            "action_type": action_type
        }

    except Exception as e:
        logger.error(f"Error processing prometheus results: {str(e)}")
        state.error_message = f"Failed to process prometheus results: {str(e)}"
        state.metadata["next_node"] = "error_handler"
        # Clear prometheus routing flags even on error
        state.metadata["needs_prometheus"] = False
        state.metadata["prometheus_collection_complete"] = False
    
    return state