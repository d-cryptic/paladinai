"""
Processors for incident node - handles data processing and prometheus result handling.
"""

import json
import logging
from typing import Dict, Any
from langfuse import observe
from llm.openai import openai
from prompts.data_collection.incident_prompts import get_incident_prompt
from graph.state import WorkflowState
from .serializers import serialize_prometheus_data

logger = logging.getLogger(__name__)


@observe(name="process_prometheus_result")
async def process_prometheus_result(state: WorkflowState, prometheus_data: Dict[str, Any], node_name: str) -> WorkflowState:
    """
    Process results returned from prometheus node.
    
    Args:
        state: Current workflow state
        prometheus_data: Data returned from prometheus node
        node_name: Name of the incident node
        
    Returns:
        Updated workflow state with processed results
    """
    logger.info("Processing prometheus results in incident node")
    
    try:
        incident_context = state.metadata.get("incident_context", {})
        incident_type = incident_context.get("incident_type", "general")
        severity = incident_context.get("severity", "medium")
        
        # Create timeline from prometheus data if available
        timeline = extract_timeline_from_data(prometheus_data)
        
        # Serialize prometheus data to handle Pydantic objects
        serialized_prometheus_data = serialize_prometheus_data(prometheus_data)

        # Limit the size of data passed to OpenAI to prevent token overflow
        data_str = json.dumps(serialized_prometheus_data, indent=2)
        if len(data_str) > 10000:  # Limit to ~10k characters
            # Truncate and add summary
            truncated_data = data_str[:10000] + "\n... [Data truncated for processing]"
            logger.warning(f"Prometheus data truncated from {len(data_str)} to 10000 characters")
            data_for_prompt = truncated_data
        else:
            data_for_prompt = data_str

        # Use incident investigation prompt for detailed analysis
        investigation_prompt = get_incident_prompt(
            "investigation",
            user_input=state.user_input,
            collected_data=data_for_prompt
        )
        
        investigation_response = await openai.chat_completion(
            user_message=investigation_prompt,
            system_prompt="You are an expert SRE conducting incident investigation with metrics data. Always include the word 'json' in your response when using JSON format.",
            temperature=0.2
        )

        if not investigation_response["success"]:
            raise Exception(investigation_response.get("error", "OpenAI request failed"))

        investigation_result = json.loads(investigation_response["content"])
        
        # Now create the final incident report
        # Use the same truncated data for consistency
        timeline_str = json.dumps(timeline, indent=2)
        if len(timeline_str) > 5000:  # Limit timeline data too
            timeline_str = timeline_str[:5000] + "\n... [Timeline truncated]"

        report_prompt = get_incident_prompt(
            "output_formatting",
            user_input=state.user_input,
            collected_data=data_for_prompt,
            timeline=timeline_str
        )
        
        report_response = await openai.chat_completion(
            user_message=report_prompt,
            system_prompt="You are an expert SRE creating comprehensive incident reports. Always include the word 'json' in your response when using JSON format.",
            temperature=0.3,
            max_tokens=16000  # Increase token limit for incident reports
        )

        if not report_response["success"]:
            raise Exception(report_response.get("error", "OpenAI request failed"))

        # Parse JSON response with better error handling
        try:
            report_result = json.loads(report_response["content"])
        except json.JSONDecodeError as e:
            logger.error(f"Failed to parse OpenAI response as JSON: {str(e)}")
            logger.error(f"Response content: {report_response['content'][:500]}...")  # Log first 500 chars
            # Return a fallback response
            report_result = {
                "error": "Failed to parse OpenAI response",
                "incident_summary": "Unable to generate incident report due to JSON parsing error",
                "severity": "unknown",
                "recommendations": ["Please try the request again", "Check system logs for details"]
            }
        
        # Combine investigation and report results
        final_result = {
            **report_result,
            "investigation_details": investigation_result,
            "incident_metadata": {
                "type": incident_type,
                "severity": severity,
                "data_sources": ["Prometheus"],
                "investigation_timestamp": state.metadata.get("current_timestamp")
            }
        }
        
        # Store formatted result
        state.metadata["incident_result"] = final_result
        state.metadata["next_node"] = "incident_output"

        # Clear prometheus routing flags to prevent loops
        state.metadata["needs_prometheus"] = False
        state.metadata["prometheus_collection_complete"] = False

        # Store prometheus processing info in metadata
        state.metadata[f"{node_name}_prometheus_processing"] = {
            "timestamp": state.metadata.get("current_timestamp"),
            "status": "completed",
            "data_processed": True,
            "incident_type": incident_type,
            "severity": severity
        }

    except Exception as e:
        logger.error(f"Error processing prometheus results: {str(e)}")
        state.error_message = f"Failed to process prometheus results: {str(e)}"
        state.metadata["next_node"] = "error_handler"
        # Clear prometheus routing flags even on error
        state.metadata["needs_prometheus"] = False
        state.metadata["prometheus_collection_complete"] = False
    
    return state


def extract_timeline_from_data(prometheus_data: Dict[str, Any]) -> Dict[str, Any]:
    """
    Extract timeline information from prometheus data.
    
    Args:
        prometheus_data: Data from prometheus node
        
    Returns:
        Timeline information extracted from the data
    """
    try:
        # Extract basic timeline info from prometheus data structure
        timeline = {
            "data_collection_time": prometheus_data.get("timestamp"),
            "data_sources": ["Prometheus"],
            "metrics_collected": len(prometheus_data.get("metrics", [])),
            "time_range": prometheus_data.get("time_range", "recent")
        }
        
        # Add any specific timeline events if available in the data
        if "events" in prometheus_data:
            timeline["events"] = prometheus_data["events"]
        
        return timeline
        
    except Exception as e:
        logger.error(f"Error extracting timeline: {str(e)}")
        return {"error": f"Timeline extraction failed: {str(e)}"}