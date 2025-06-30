"""
Processors for incident node - handles data processing and prometheus result handling.
"""

import json
import logging
from typing import Dict, Any
from langfuse import observe
from llm.openai import openai
from prompts.data_collection.incident_prompts import get_incident_prompt
from prompts.workflows.processor_prompts import get_processor_system_prompt
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
            system_prompt=get_processor_system_prompt("INCIDENT", "investigation"),
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
            system_prompt=get_processor_system_prompt("INCIDENT", "report"),
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


@observe(name="process_loki_result_incident")
async def process_loki_result(state: WorkflowState, loki_data: Dict[str, Any], node_name: str) -> WorkflowState:
    """
    Process results returned from loki node for incident workflows.

    Args:
        state: Current workflow state
        loki_data: Data returned from loki node
        node_name: Name of the incident node

    Returns:
        Updated workflow state with processed results
    """
    logger.info("Processing loki results in incident node")

    try:
        # Get incident analysis
        incident_analysis = state.metadata.get("incident_analysis", {})
        incident_type = incident_analysis.get("incident_type", "general")
        severity = incident_analysis.get("severity", "medium")
        
        # Check if we have both prometheus and loki data
        has_prometheus_data = state.metadata.get("prometheus_data") is not None
        
        # Prepare collected data
        collected_data = {}
        data_sources = []
        
        if has_prometheus_data:
            # Serialize prometheus data
            prometheus_data = state.metadata.get("prometheus_data", {})
            from .serializers import serialize_prometheus_data
            serialized_prometheus = serialize_prometheus_data(prometheus_data)
            collected_data["metrics"] = serialized_prometheus
            data_sources.append("Prometheus")
        
        # Add loki data
        collected_data["logs"] = loki_data
        data_sources.append("Loki")
        
        # Extract combined timeline from both sources
        combined_timeline = extract_combined_timeline(collected_data)
        
        # Format the final incident investigation report
        from prompts.data_collection.incident_prompts import get_incident_prompt
        prompt = get_incident_prompt(
            "output_formatting",
            user_input=state.user_input,
            collected_data=json.dumps(collected_data, indent=2),
            incident_type=incident_type,
            severity=severity,
            timeline=json.dumps(combined_timeline, indent=2)
        )

        response = await openai.chat_completion(
            user_message=prompt,
            system_prompt=get_processor_system_prompt("INCIDENT", "combined"),
            temperature=0.3
        )

        if not response["success"]:
            raise Exception(response.get("error", "OpenAI request failed"))

        result = json.loads(response["content"])

        # Set incident result and route to output
        state.metadata["incident_result"] = result
        state.metadata["next_node"] = "incident_output"

        # Clear loki routing flags to prevent loops
        state.metadata["needs_loki"] = False
        state.metadata["loki_collection_complete"] = False

        # Store loki processing info in metadata
        state.metadata[f"{node_name}_loki_processing"] = {
            "timestamp": state.metadata.get("current_timestamp"),
            "status": "completed",
            "data_processed": True,
            "incident_type": incident_type,
            "severity": severity
        }

    except Exception as e:
        logger.error(f"Error processing loki results: {str(e)}")
        state.error_message = f"Failed to process loki results: {str(e)}"
        state.metadata["next_node"] = "error_handler"
        # Clear loki routing flags even on error
        state.metadata["needs_loki"] = False
        state.metadata["loki_collection_complete"] = False

    return state


def extract_combined_timeline(collected_data: Dict[str, Any]) -> Dict[str, Any]:
    """
    Extract a combined timeline from both metrics and logs data.
    
    Args:
        collected_data: Dictionary containing both metrics and logs
        
    Returns:
        Combined timeline information
    """
    try:
        timeline: Dict[str, Any] = {
            "events": [],
            "data_sources": [],
            "time_range": {}
        }
        
        # Extract events from logs
        if "logs" in collected_data:
            logs = collected_data["logs"].get("logs", [])
            for log in logs[:50]:  # Limit to first 50 for timeline
                timeline["events"].append({
                    "timestamp": log.get("timestamp"),
                    "source": "logs",
                    "message": log.get("message", "")[:200],
                    "severity": determine_log_severity(log.get("message", ""))
                })
            timeline["data_sources"].append("Loki")
        
        # Extract events from metrics anomalies
        if "metrics" in collected_data:
            metrics = collected_data["metrics"]
            # Add any metric anomalies to timeline
            if "anomalies" in metrics:
                for anomaly in metrics["anomalies"]:
                    timeline["events"].append({
                        "timestamp": anomaly.get("timestamp"),
                        "source": "metrics",
                        "message": f"Anomaly detected: {anomaly.get('description', 'Metric anomaly')}",
                        "severity": "high"
                    })
            timeline["data_sources"].append("Prometheus")
        
        # Sort events by timestamp
        timeline["events"].sort(key=lambda x: x.get("timestamp", ""))
        
        return timeline
        
    except Exception as e:
        logger.error(f"Error creating combined timeline: {str(e)}")
        return {"error": str(e)}


def determine_log_severity(message: str) -> str:
    """Determine severity of a log message."""
    message_lower = message.lower()
    if any(term in message_lower for term in ["fatal", "panic", "critical"]):
        return "critical"
    elif any(term in message_lower for term in ["error", "exception", "fail"]):
        return "high"
    elif "warn" in message_lower:
        return "medium"
    else:
        return "low"