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
            
            # Extract data requirements
            needs_metrics = incident_analysis.get("needs_metrics", True)
            needs_logs = incident_analysis.get("needs_logs", True)
            
            logger.info(f"Data requirements - Metrics: {needs_metrics}, Logs: {needs_logs}")
            
            # Store data requirements and context
            state.metadata["data_requirements"] = incident_analysis
            state.metadata["originating_node"] = "incident"
            state.metadata["incident_context"] = {
                "workflow_type": "incident",
                "user_input": state.user_input,
                "incident_type": incident_analysis.get("incident_type", "general"),
                "severity": incident_analysis.get("severity", "medium"),
                "investigation_focus": incident_analysis.get("investigation_focus", []),
                "data_requirements": incident_analysis.get("data_requirements", {}),
                "processing_stage": "data_collection"
            }
            
            # Determine routing based on data needs
            if needs_metrics and needs_logs:
                logger.info("Incident requires both metrics and logs")
                # First collect metrics, then logs
                state.metadata["needs_prometheus"] = True
                state.metadata["needs_loki"] = True
                state.metadata["next_node"] = "prometheus"
                state.metadata["data_collection_sequence"] = ["prometheus", "loki"]
                
            elif needs_metrics:
                logger.info("Incident requires only metrics data")
                state.metadata["needs_prometheus"] = True
                state.metadata["next_node"] = "prometheus"
                
            elif needs_logs:
                logger.info("Incident requires only log data")
                state.metadata["needs_loki"] = True
                state.metadata["next_node"] = "loki"
                
            else:
                logger.info("Incident can be handled without external data (rare)")
                # Process incident directly without metrics or logs
                result = await process_non_metrics_incident(state.user_input)
                
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
                "needs_metrics": needs_metrics,
                "needs_logs": needs_logs,
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
    
    async def process_loki_result(self, state: WorkflowState, loki_data: Dict[str, Any]) -> WorkflowState:
        """
        Process results returned from loki node.
        
        Args:
            state: Current workflow state
            loki_data: Data returned from loki node
            
        Returns:
            Updated workflow state with processed results
        """
        # Import here to avoid circular dependency
        from .processors import process_loki_result
        return await process_loki_result(state, loki_data, self.node_name)
    
    
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

        # Check if we're in a data collection sequence
        collection_sequence = state.metadata.get("data_collection_sequence", [])
        
        # Check if prometheus processing is complete
        if state.metadata.get("prometheus_collection_complete"):
            # If we still need loki data and haven't collected it yet
            if state.metadata.get("needs_loki") and not state.metadata.get("loki_collection_complete"):
                return "loki"
            # Otherwise, route to output
            return state.metadata.get("next_node", "incident_output")
        
        # Check if loki processing is complete
        if state.metadata.get("loki_collection_complete"):
            # If we still need prometheus data and haven't collected it yet
            if state.metadata.get("needs_prometheus") and not state.metadata.get("prometheus_collection_complete"):
                return "prometheus"
            # Otherwise, route to output
            return state.metadata.get("next_node", "incident_output")

        # Initial routing based on needs
        if state.metadata.get("needs_prometheus") and not state.metadata.get("prometheus_collection_complete"):
            return "prometheus"
        elif state.metadata.get("needs_loki") and not state.metadata.get("loki_collection_complete"):
            return "loki"

        # Otherwise route to output
        return state.metadata.get("next_node", "incident_output")


# Create singleton instance
incident_node = IncidentNode()
