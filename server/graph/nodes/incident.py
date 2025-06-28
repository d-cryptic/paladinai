"""
Incident Node for PaladinAI LangGraph Workflow.

This module implements the incident node that handles incident-type requests
and routes metrics-related requests to the prometheus node.
"""

import json
import logging
from typing import Dict, Any, Optional
from langfuse import observe
from prompts.workflows.analysis import get_query_analysis_prompt, get_incident_analysis_prompt

from ..state import WorkflowState, NodeResult, update_state_node
from llm.openai import openai
from prompts.data_collection.incident_prompts import get_incident_prompt

logger = logging.getLogger(__name__)


class IncidentNode:
    """
    Node responsible for handling incident-type workflow requests.
    
    This node processes incident reports and investigations, determines if metrics
    data is needed, and routes to prometheus node for comprehensive data collection.
    """
    
    def __init__(self):
        """Initialize the incident node."""
        self.node_name = "incident"

    def _serialize_prometheus_data(self, prometheus_data: Dict[str, Any]) -> Dict[str, Any]:
        """
        Serialize prometheus data containing Pydantic objects to JSON-compatible format.

        Args:
            prometheus_data: Prometheus data that may contain Pydantic objects

        Returns:
            JSON-serializable dictionary
        """
        try:
            from pydantic import BaseModel

            def serialize_item(item):
                """Recursively serialize items, handling Pydantic models."""
                if isinstance(item, BaseModel):
                    return item.model_dump()  # Convert Pydantic model to dict
                elif isinstance(item, dict):
                    return {k: serialize_item(v) for k, v in item.items()}
                elif isinstance(item, list):
                    return [serialize_item(i) for i in item]
                else:
                    return item

            return serialize_item(prometheus_data)

        except Exception as e:
            logger.warning(f"Failed to serialize prometheus data: {str(e)}")
            # Return a simplified version without problematic objects
            return {
                "serialization_error": str(e),
                "data_summary": "Prometheus data available but not serializable"
            }
    
    @observe(name="incident_node")
    async def execute(self, state: WorkflowState) -> WorkflowState:
        """
        Execute incident workflow processing.
        
        Args:
            state: Current workflow state containing user input and categorization
            
        Returns:
            Updated workflow state with incident processing results
        """
        logger.info(f"Executing incident node for input: {state.user_input[:100]}...")
        
        # Update state to indicate incident node execution
        state = update_state_node(state, self.node_name)
        
        try:
            # Analyze the incident to determine investigation requirements
            incident_analysis = await self._analyze_incident_requirements(state.user_input)
            
            if incident_analysis.get("needs_metrics", True):  # Default to True for incidents
                logger.info("Incident requires metrics data - routing to prometheus node")
                # Set flag to route to prometheus node
                state.metadata["needs_prometheus"] = True
                state.metadata["originating_node"] = "incident"
                
                # Store incident context for prometheus node
                state.metadata["incident_context"] = {
                    "workflow_type": "incident",
                    "user_input": state.user_input,
                    "incident_type": incident_analysis.get("incident_type", "general"),
                    "severity": incident_analysis.get("severity", "medium"),
                    "investigation_focus": incident_analysis.get("investigation_focus", []),
                    "processing_stage": "data_collection"
                }
                
                # Set next node to prometheus
                state.metadata["next_node"] = "prometheus"
                
            else:
                logger.info("Incident can be handled without metrics data")
                # Process incident directly without metrics (rare case)
                result = await self._process_non_metrics_incident(state.user_input, incident_analysis)
                
                if result["success"]:
                    state.metadata["incident_result"] = result["data"]
                    state.metadata["next_node"] = "incident_output"
                else:
                    state.error_message = result.get("error", "Failed to process incident")
                    state.metadata["next_node"] = "error_handler"
            
            # Store incident analysis for later use
            state.metadata["incident_analysis"] = incident_analysis
            
            # Store detailed execution info in metadata
            state.metadata[f"{self.node_name}_execution"] = {
                "timestamp": state.metadata.get("current_timestamp"),
                "status": "completed" if not state.error_message else "error",
                "needs_metrics": incident_analysis.get("needs_metrics", True),
                "incident_type": incident_analysis.get("incident_type", "general"),
                "severity": incident_analysis.get("severity", "medium")
            }
            
        except Exception as e:
            logger.error(f"Error in incident node: {str(e)}")
            state.error_message = f"Incident processing failed: {str(e)}"
            state.metadata["next_node"] = "error_handler"
            
            # Store error execution info in metadata
            state.metadata[f"{self.node_name}_execution"] = {
                "timestamp": state.metadata.get("current_timestamp"),
                "status": "error",
                "error": str(e)
            }
        
        return state
    
    async def _analyze_incident_requirements(self, user_input: str) -> Dict[str, Any]:
        """
        Analyze the incident to determine investigation requirements.
        
        Args:
            user_input: The user's incident description
            
        Returns:
            Dictionary containing incident analysis results
        """
        try:
            analysis_prompt = get_incident_analysis_prompt(user_input)
            
            response = await openai.chat_completion(
                user_message=analysis_prompt,
                system_prompt="You are an expert SRE analyzing incident reports for investigation planning. Always include the word 'json' in your response when using JSON format.",
                temperature=0.1
            )

            if not response["success"]:
                raise Exception(response.get("error", "OpenAI request failed"))

            result = json.loads(response["content"])
            return result
            
        except Exception as e:
            logger.error(f"Error analyzing incident requirements: {str(e)}")
            # Default to requiring metrics for incidents
            return {
                "needs_metrics": True,
                "incident_type": "general",
                "severity": "medium",
                "investigation_focus": ["performance", "availability"],
                "urgency": "normal",
                "reasoning": f"Analysis failed: {str(e)}"
            }
    
    async def _process_non_metrics_incident(self, user_input: str, incident_analysis: Dict[str, Any]) -> Dict[str, Any]:
        """
        Process incidents that don't require metrics data (rare case).
        
        Args:
            user_input: The user's incident description
            incident_analysis: Analysis results from incident requirements
            
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
    
    async def process_prometheus_result(self, state: WorkflowState, prometheus_data: Dict[str, Any]) -> WorkflowState:
        """
        Process results returned from prometheus node.
        
        Args:
            state: Current workflow state
            prometheus_data: Data returned from prometheus node
            
        Returns:
            Updated workflow state with processed results
        """
        logger.info("Processing prometheus results in incident node")
        
        try:
            incident_context = state.metadata.get("incident_context", {})
            incident_type = incident_context.get("incident_type", "general")
            severity = incident_context.get("severity", "medium")
            
            # Create timeline from prometheus data if available
            timeline = self._extract_timeline_from_data(prometheus_data)
            
            # Serialize prometheus data to handle Pydantic objects
            serialized_prometheus_data = self._serialize_prometheus_data(prometheus_data)

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
            state.metadata[f"{self.node_name}_prometheus_processing"] = {
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
    
    def _extract_timeline_from_data(self, prometheus_data: Dict[str, Any]) -> Dict[str, Any]:
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
    
    def get_next_node(self, state: WorkflowState) -> str:
        """
        Determine the next node based on incident processing results.

        Args:
            state: Current workflow state

        Returns:
            Name of the next node to execute
        """
        if state.error_message:
            return "error_handler"

        # Check if prometheus processing is complete first
        if state.metadata.get("prometheus_collection_complete"):
            # Prometheus data has been collected and processed, route to output
            return state.metadata.get("next_node", "incident_output")

        # Check if we need to route to prometheus
        if state.metadata.get("needs_prometheus"):
            return "prometheus"

        # Otherwise route to output
        return state.metadata.get("next_node", "incident_output")


# Create singleton instance
incident_node = IncidentNode()
