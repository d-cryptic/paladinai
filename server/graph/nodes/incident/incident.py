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
        # Use enhanced input if available
        incident_input = state.enhanced_input or state.user_input
        logger.info(f"Executing incident node for input: {incident_input[:100]}...")
        
        # Update state to indicate incident node execution
        state = update_state_node(state, self.node_name)
        
        try:
            # Analyze the incident to determine investigation requirements
            incident_analysis = await analyze_incident_requirements(incident_input)
            
            # Extract data requirements
            needs_metrics = incident_analysis.get("needs_metrics", True)
            needs_logs = incident_analysis.get("needs_logs", True)
            needs_alerts = incident_analysis.get("needs_alerts", False)
            
            logger.info(f"Data requirements - Metrics: {needs_metrics}, Logs: {needs_logs}, Alerts: {needs_alerts}")
            
            # Store data requirements and context
            state.metadata["data_requirements"] = incident_analysis
            state.metadata["originating_node"] = "incident"
            state.metadata["incident_context"] = {
                "workflow_type": "incident",
                "user_input": incident_input,
                "incident_type": incident_analysis.get("incident_type", "general"),
                "severity": incident_analysis.get("severity", "medium"),
                "investigation_focus": incident_analysis.get("investigation_focus", []),
                "data_requirements": incident_analysis.get("data_requirements", {}),
                "processing_stage": "data_collection"
            }
            
            # Determine routing based on data needs
            data_sources_needed = []
            if needs_metrics:
                data_sources_needed.append("prometheus")
                state.metadata["needs_prometheus"] = True
            if needs_logs:
                data_sources_needed.append("loki")
                state.metadata["needs_loki"] = True
            if needs_alerts:
                data_sources_needed.append("alertmanager")
                state.metadata["needs_alertmanager"] = True
                state.metadata["alertmanager_requested_by"] = "incident"
            
            if data_sources_needed:
                logger.info(f"Incident requires data from: {', '.join(data_sources_needed)}")
                # Set up data collection sequence
                state.metadata["data_collection_sequence"] = data_sources_needed
                state.metadata["next_node"] = data_sources_needed[0]
                
            else:
                logger.info("Incident can be handled without external data (rare)")
                # Process incident directly without metrics or logs
                result = await process_non_metrics_incident(incident_input)
                
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
                "needs_alerts": needs_alerts,
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
    
    async def process_alertmanager_result(self, state: WorkflowState, alert_data: Dict[str, Any]) -> WorkflowState:
        """
        Process results returned from alertmanager node.
        
        Args:
            state: Current workflow state
            alert_data: Data returned from alertmanager node
            
        Returns:
            Updated workflow state with processed results
        """
        # For now, just store the alert data
        # In the future, we might want to add specific processing
        state.metadata["alertmanager_data"] = alert_data
        state.metadata["alertmanager_collection_complete"] = True
        
        # Clear the next_node set by alertmanager to prevent routing conflicts
        if "next_node" in state.metadata:
            del state.metadata["next_node"]
        
        # Continue with the data collection sequence
        return state
    
    
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
        
        # Check what data has been collected
        prometheus_done = state.metadata.get("prometheus_collection_complete", False)
        loki_done = state.metadata.get("loki_collection_complete", False)
        alertmanager_done = state.metadata.get("alertmanager_collection_complete", False)
        
        # Route to next data source if needed
        if state.metadata.get("needs_prometheus") and not prometheus_done:
            return "prometheus"
        elif state.metadata.get("needs_loki") and not loki_done:
            return "loki"
        elif state.metadata.get("needs_alertmanager") and not alertmanager_done:
            return "alertmanager"
        
        # All data collected, route to output
        return state.metadata.get("next_node", "incident_output")


# Create singleton instance
incident_node = IncidentNode()
