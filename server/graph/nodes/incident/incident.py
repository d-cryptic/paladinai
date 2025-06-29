"""
Incident Node for PaladinAI LangGraph Workflow.

This module implements the incident node that handles incident-type requests
and routes metrics-related requests to the prometheus node.
"""

import logging
from typing import Dict, Any
from langfuse import observe

from ...state import WorkflowState, update_state_node
from .analyzers import analyze_incident_requirements, process_non_metrics_incident
from .processors import process_prometheus_result

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
            incident_analysis = await analyze_incident_requirements(state.user_input)
            
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
                result = await process_non_metrics_incident(state.user_input, incident_analysis)
                
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
    
    
    
    async def process_prometheus_result(self, state: WorkflowState, prometheus_data: Dict[str, Any]) -> WorkflowState:
        """
        Process results returned from prometheus node.
        
        Args:
            state: Current workflow state
            prometheus_data: Data returned from prometheus node
            
        Returns:
            Updated workflow state with processed results
        """
        return await process_prometheus_result(state, prometheus_data, self.node_name)
    
    
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
