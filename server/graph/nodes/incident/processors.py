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
from utils.data_reduction import data_reducer

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
        
        # Check if we have alertmanager data to include
        has_alertmanager_data = state.metadata.get("alertmanager_data") is not None
        collected_data = {}
        data_sources = []
        
        # Serialize prometheus data to handle Pydantic objects
        serialized_prometheus_data = serialize_prometheus_data(prometheus_data)
        collected_data["metrics"] = serialized_prometheus_data
        data_sources.append("Prometheus")
        
        if has_alertmanager_data:
            alert_data = state.metadata.get("alertmanager_data", {})
            collected_data["alerts"] = alert_data
            data_sources.append("Alertmanager")
            logger.info(f"Including Alertmanager data with {len(alert_data.get('alerts', []))} alerts")
        
        # Create timeline from collected data if available
        timeline = extract_combined_timeline(collected_data)

        # Apply data reduction to prevent token limit issues
        logger.info("Applying data reduction to monitoring data...")
        
        # Reduce prometheus data
        if "metrics" in collected_data and collected_data["metrics"]:
            original_size = data_reducer.estimate_tokens(collected_data["metrics"])
            reduced_prometheus = data_reducer.reduce_prometheus_data(
                {"metrics": collected_data["metrics"]},
                priority="anomalies"  # For incidents, focus on anomalies
            )
            collected_data["metrics"] = reduced_prometheus
            reduced_size = data_reducer.estimate_tokens(reduced_prometheus)
            logger.info(f"Reduced Prometheus data from ~{original_size} to ~{reduced_size} tokens")
        
        # Reduce alertmanager data if present
        if "alerts" in collected_data and collected_data["alerts"]:
            original_size = data_reducer.estimate_tokens(collected_data["alerts"])
            reduced_alerts = data_reducer.reduce_alertmanager_data(collected_data["alerts"])
            collected_data["alerts"] = reduced_alerts
            reduced_size = data_reducer.estimate_tokens(reduced_alerts)
            logger.info(f"Reduced Alertmanager data from ~{original_size} to ~{reduced_size} tokens")
        
        data_for_prompt = json.dumps(collected_data, indent=2)

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
                "data_sources": data_sources,
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
        
        # Check if we have multiple data sources
        has_prometheus_data = state.metadata.get("prometheus_data") is not None
        has_alertmanager_data = state.metadata.get("alertmanager_data") is not None
        
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
        logger.info(f"Loki data contains: {len(loki_data.get('logs', []))} logs")
        collected_data["logs"] = loki_data
        data_sources.append("Loki")
        
        # Add alertmanager data if available
        if has_alertmanager_data:
            alert_data = state.metadata.get("alertmanager_data", {})
            collected_data["alerts"] = alert_data
            data_sources.append("Alertmanager")
            logger.info(f"Alertmanager data contains: {len(alert_data.get('alerts', []))} alerts")
        
        # Extract combined timeline from both sources
        combined_timeline = extract_combined_timeline(collected_data)
        
        # Apply data reduction to prevent token limit issues
        logger.info("Applying data reduction to combined monitoring data...")
        
        # Reduce prometheus data
        if "metrics" in collected_data and collected_data["metrics"]:
            original_size = data_reducer.estimate_tokens(collected_data["metrics"])
            reduced_prometheus = data_reducer.reduce_prometheus_data(
                {"metrics": collected_data["metrics"]},
                priority="anomalies"
            )
            collected_data["metrics"] = reduced_prometheus
            reduced_size = data_reducer.estimate_tokens(reduced_prometheus)
            logger.info(f"Reduced Prometheus data from ~{original_size} to ~{reduced_size} tokens")
        
        # Reduce loki logs
        if "logs" in collected_data and collected_data["logs"]:
            original_size = data_reducer.estimate_tokens(collected_data["logs"])
            reduced_logs = data_reducer.reduce_loki_logs(collected_data["logs"])
            collected_data["logs"] = reduced_logs
            reduced_size = data_reducer.estimate_tokens(reduced_logs)
            logger.info(f"Reduced Loki logs from ~{original_size} to ~{reduced_size} tokens")
        
        # Reduce alertmanager data
        if "alerts" in collected_data and collected_data["alerts"]:
            original_size = data_reducer.estimate_tokens(collected_data["alerts"])
            reduced_alerts = data_reducer.reduce_alertmanager_data(collected_data["alerts"])
            collected_data["alerts"] = reduced_alerts
            reduced_size = data_reducer.estimate_tokens(reduced_alerts)
            logger.info(f"Reduced Alertmanager data from ~{original_size} to ~{reduced_size} tokens")
        
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
            log_data = collected_data["logs"]
            
            # Handle reduced data format
            if isinstance(log_data, dict) and "logs" in log_data:
                # Reduced format
                logs = log_data.get("logs", [])
                if isinstance(logs, list):
                    for log_entry in logs[:50]:  # Limit to first 50 for timeline
                        if isinstance(log_entry, dict):
                            # Handle grouped logs
                            if log_entry.get("type") == "grouped":
                                # Add representative example from grouped logs
                                for example in log_entry.get("examples", [])[:1]:
                                    if isinstance(example, dict):
                                        timeline["events"].append({
                                            "timestamp": example.get("timestamp", ""),
                                            "source": "logs",
                                            "message": f"[{log_entry.get('count', 1)}x] {example.get('message', '')[:200]}",
                                            "severity": determine_log_severity(example.get("message", ""))
                                        })
                            else:
                                # Regular log entry
                                timeline["events"].append({
                                    "timestamp": log_entry.get("timestamp", ""),
                                    "source": "logs",
                                    "message": log_entry.get("message", "")[:200] if isinstance(log_entry.get("message"), str) else str(log_entry.get("message", ""))[:200],
                                    "severity": determine_log_severity(str(log_entry.get("message", "")))
                                })
                        elif isinstance(log_entry, str):
                            # Handle string logs
                            timeline["events"].append({
                                "timestamp": "",
                                "source": "logs",
                                "message": log_entry[:200],
                                "severity": determine_log_severity(log_entry)
                            })
            elif isinstance(log_data, list):
                # Original format - list of logs
                for log in log_data[:50]:
                    if isinstance(log, dict):
                        timeline["events"].append({
                            "timestamp": log.get("timestamp", ""),
                            "source": "logs",
                            "message": log.get("message", "")[:200],
                            "severity": determine_log_severity(log.get("message", ""))
                        })
            
            timeline["data_sources"].append("Loki")
        
        # Extract events from metrics anomalies
        if "metrics" in collected_data:
            metrics = collected_data["metrics"]
            
            # Handle reduced data format
            if isinstance(metrics, dict):
                # Check for summary info in reduced format
                if "summary" in metrics and isinstance(metrics["summary"], dict):
                    if metrics["summary"].get("has_anomalies"):
                        timeline["events"].append({
                            "timestamp": "",
                            "source": "metrics",
                            "message": "Anomalies detected in metrics data",
                            "severity": "high"
                        })
                
                # Check for specific metrics data
                if "metrics" in metrics:
                    # Extract sample events from aggregated metrics
                    for metric_name, metric_data in list(metrics.get("metrics", {}).items())[:10]:
                        if isinstance(metric_data, dict) and "max" in metric_data and "avg" in metric_data:
                            if metric_data["max"] > metric_data["avg"] * 2:  # Simple anomaly detection
                                timeline["events"].append({
                                    "timestamp": "",
                                    "source": "metrics",
                                    "message": f"High value detected in {metric_name}: max={metric_data['max']:.2f}, avg={metric_data['avg']:.2f}",
                                    "severity": "medium"
                                })
            
            timeline["data_sources"].append("Prometheus")
        
        # Extract events from alerts
        if "alerts" in collected_data:
            alert_data = collected_data["alerts"]
            
            # Handle reduced data format
            if isinstance(alert_data, dict) and "alerts" in alert_data:
                alerts = alert_data.get("alerts", [])
                for alert in alerts[:30]:  # Limit to first 30 alerts for timeline
                    if isinstance(alert, dict):
                        # Handle reduced alert format
                        timestamp = alert.get("startsAt", alert.get("timestamp", ""))
                        severity = alert.get("severity", alert.get("labels", {}).get("severity", "medium")) if isinstance(alert.get("labels"), dict) else "medium"
                        alertname = alert.get("name", alert.get("labels", {}).get("alertname", "Unknown")) if isinstance(alert.get("labels"), dict) else "Unknown"
                        summary = alert.get("summary", alert.get("annotations", {}).get("summary", "No summary")) if isinstance(alert.get("annotations"), dict) else "No summary"
                        
                        timeline["events"].append({
                            "timestamp": timestamp,
                            "source": "alerts",
                            "message": f"Alert: {alertname} - {summary}",
                            "severity": severity
                        })
                
                # Add summary info if available
                if "summary" in alert_data and isinstance(alert_data["summary"], dict):
                    summary = alert_data["summary"]
                    if summary.get("active_alerts", 0) > 0:
                        timeline["events"].insert(0, {
                            "timestamp": "",
                            "source": "alerts",
                            "message": f"Alert Summary: {summary.get('active_alerts', 0)} active alerts, {summary.get('total_alerts', 0)} total",
                            "severity": "critical" if summary.get("by_severity", {}).get("critical", 0) > 0 else "high"
                        })
            elif isinstance(alert_data, list):
                # Original format
                for alert in alert_data[:30]:
                    if isinstance(alert, dict):
                        timeline["events"].append({
                            "timestamp": alert.get("startsAt", alert.get("timestamp", "")),
                            "source": "alerts",
                            "message": f"Alert: {alert.get('labels', {}).get('alertname', 'Unknown')} - {alert.get('annotations', {}).get('summary', 'No summary')}",
                            "severity": alert.get("labels", {}).get("severity", "medium")
                        })
            
            timeline["data_sources"].append("Alertmanager")
        
        # Sort events by timestamp
        timeline["events"].sort(key=lambda x: x.get("timestamp", ""))
        
        return timeline
        
    except Exception as e:
        logger.error(f"Error creating combined timeline: {str(e)}")
        return {"error": str(e)}


def determine_log_severity(message: str) -> str:
    """Determine severity of a log message."""
    if not isinstance(message, str):
        message = str(message)
    message_lower = message.lower()
    if any(term in message_lower for term in ["fatal", "panic", "critical"]):
        return "critical"
    elif any(term in message_lower for term in ["error", "exception", "fail"]):
        return "high"
    elif "warn" in message_lower:
        return "medium"
    else:
        return "low"