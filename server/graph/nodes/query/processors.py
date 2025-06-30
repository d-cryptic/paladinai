"""
Processors for query node - handles data processing and prometheus result handling.
"""

import json
import logging
from typing import Dict, Any
from langfuse import observe
from llm.openai import openai
from prompts.data_collection.query_prompts import get_query_prompt
from prompts.workflows.processor_prompts import get_processor_system_prompt
from graph.state import WorkflowState
from .serializers import serialize_prometheus_data, serialize_loki_data

logger = logging.getLogger(__name__)


@observe(name="process_prometheus_result")
async def process_prometheus_result(state: WorkflowState, prometheus_data: Dict[str, Any], node_name: str) -> WorkflowState:
    """
    Process results returned from prometheus node.

    Args:
        state: Current workflow state
        prometheus_data: Data returned from prometheus node
        node_name: Name of the query node

    Returns:
        Updated workflow state with processed results
    """
    logger.info("Processing prometheus results in query node")

    try:
        # Serialize prometheus data to handle Pydantic objects
        serialized_prometheus_data = serialize_prometheus_data(prometheus_data)

        # Use query output formatting prompt to format the final response
        prompt = get_query_prompt(
            "output_formatting",
            user_input=state.user_input,
            collected_data=json.dumps(serialized_prometheus_data, indent=2),
            data_sources="Prometheus"
        )

        response = await openai.chat_completion(
            user_message=prompt,
            system_prompt=get_processor_system_prompt("QUERY", "metrics"),
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
                "analysis_results": "Unable to process the query results due to JSON parsing error",
                "data_summary": "Query processing failed",
                "recommendations": ["Please try the request again", "Check system logs for details"]
            }

        # Store formatted result
        state.metadata["query_result"] = result
        state.metadata["next_node"] = "query_output"

        # Clear prometheus routing flags to prevent loops
        state.metadata["needs_prometheus"] = False
        state.metadata["prometheus_collection_complete"] = False

        # Store prometheus processing info in metadata
        state.metadata[f"{node_name}_prometheus_processing"] = {
            "timestamp": state.metadata.get("current_timestamp"),
            "status": "completed",
            "data_processed": True
        }

    except Exception as e:
        logger.error(f"Error processing prometheus results: {str(e)}")
        state.error_message = f"Failed to process prometheus results: {str(e)}"
        state.metadata["next_node"] = "error_handler"
        # Clear prometheus routing flags even on error
        state.metadata["needs_prometheus"] = False
        state.metadata["prometheus_collection_complete"] = False

    return state


@observe(name="process_loki_result")
async def process_loki_result(state: WorkflowState, loki_data: Dict[str, Any], node_name: str) -> WorkflowState:
    """
    Process results returned from loki node.

    Args:
        state: Current workflow state
        loki_data: Data returned from loki node
        node_name: Name of the query node

    Returns:
        Updated workflow state with processed results
    """
    logger.info("Processing loki results in query node")

    try:
       # Check if we have multiple data sources
        has_prometheus_data = state.metadata.get("prometheus_data") is not None
        has_alertmanager_data = state.metadata.get("alertmanager_data") is not None
        
        # Prepare collected data
        collected_data = {}
        data_sources = []
        
        if has_prometheus_data:
            # Serialize prometheus data
            prometheus_data = state.metadata.get("prometheus_data", {})
            serialized_prometheus = serialize_prometheus_data(prometheus_data)
            collected_data["metrics"] = serialized_prometheus
            data_sources.append("Prometheus")
        
        # Add loki data
        logger.info(f"Loki data contains: {len(loki_data.get('logs', []))} logs")
        collected_data["logs"] = loki_data
        data_sources.append("Loki")
        
        # Add alertmanager data if available
        if has_alertmanager_data:
            alert_data = state.metadata.get("alertmanager_data", {})
            collected_data["alerts"] = alert_data
            data_sources.append("Alertmanager")
            logger.info(f"Alertmanager data contains: {len(alert_data.get('alerts', []))} alerts")
        
        # Format the final response with all collected data
        prompt = get_query_prompt(
            "output_formatting",
            user_input=state.user_input,
            collected_data=json.dumps(collected_data, indent=2),
            data_sources=", ".join(data_sources)
        )

        response = await openai.chat_completion(
            user_message=prompt,
            system_prompt=get_processor_system_prompt("QUERY", "combined"),
            temperature=0.3
        )

        if not response["success"]:
            raise Exception(response.get("error", "OpenAI request failed"))

        result = json.loads(response["content"])
        
        logger.info(f"Query result formatted with keys: {list(result.keys())}")

        # Set query result and route to output
        state.metadata["query_result"] = result
        state.metadata["next_node"] = "query_output"

        # Clear loki routing flags to prevent loops
        state.metadata["needs_loki"] = False
        state.metadata["loki_collection_complete"] = False

        # Store loki processing info in metadata
        state.metadata[f"{node_name}_loki_processing"] = {
            "timestamp": state.metadata.get("current_timestamp"),
            "status": "completed",
            "data_processed": True
        }

    except Exception as e:
        logger.error(f"Error processing loki results: {str(e)}")
        state.error_message = f"Failed to process loki results: {str(e)}"
        state.metadata["next_node"] = "error_handler"
        # Clear loki routing flags even on error
        state.metadata["needs_loki"] = False
        state.metadata["loki_collection_complete"] = False

    return state