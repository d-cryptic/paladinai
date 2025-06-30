"""
Query Node for PaladinAI LangGraph Workflow.

This module implements the query node that handles query-type requests
and routes metrics-related requests to the prometheus node.
"""

import logging
from typing import Dict, Any
from langfuse import observe

from ...state import WorkflowState, update_state_node
from .analyzers import analyze_data_requirements, process_non_metrics_query
from .processors import process_prometheus_result

logger = logging.getLogger(__name__)


class QueryNode:
    """
    Node responsible for handling query-type workflow requests.
    
    This node processes user queries, determines if metrics data is needed,
    and routes to prometheus node when necessary for data collection.
    """
    
    def __init__(self):
        """Initialize the query node."""
        self.node_name = "query"

    
    @observe(name="query_node")
    async def execute(self, state: WorkflowState) -> WorkflowState:
        """
        Execute query workflow processing.
        
        Args:
            state: Current workflow state containing user input and categorization
            
        Returns:
            Updated workflow state with query processing results
        """
        logger.info(f"Executing query node for input: {state.user_input[:100]}...")
        
        # Update state to indicate query node execution
        state = update_state_node(state, self.node_name)
        
        try:
            # Analyze what data sources are needed
            data_requirements = await analyze_data_requirements(state.user_input)
            needs_metrics = data_requirements.get("needs_metrics", False)
            needs_logs = data_requirements.get("needs_logs", False)
            needs_alerts = data_requirements.get("needs_alerts", False)
            
            logger.info(f"Data requirements - Metrics: {needs_metrics}, Logs: {needs_logs}, Alerts: {needs_alerts}")
            
            # Store data requirements in metadata
            state.metadata["data_requirements"] = data_requirements
            state.metadata["originating_node"] = "query"
            
            # Store query context for data collection nodes
            state.metadata["query_context"] = {
                "workflow_type": "query",
                "user_input": state.user_input,
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
                state.metadata["alertmanager_requested_by"] = "query"
            
            if data_sources_needed:
                logger.info(f"Query requires data from: {', '.join(data_sources_needed)}")
                # Set up data collection sequence
                state.metadata["data_collection_sequence"] = data_sources_needed
                state.metadata["next_node"] = data_sources_needed[0]
                
            else:
                logger.info("Query can be handled without external data")
                # Process query directly without metrics, logs, or alerts
                result = await process_non_metrics_query(state.user_input)
                
                if result["success"]:
                    state.metadata["query_result"] = result["data"]
                    state.metadata["next_node"] = "query_output"
                else:
                    state.error_message = result.get("error", "Failed to process query")
                    state.metadata["next_node"] = "error_handler"
            
            # Store detailed execution info in metadata
            state.metadata[f"{self.node_name}_execution"] = {
                "timestamp": state.metadata.get("current_timestamp"),
                "status": "completed" if not state.error_message else "error",
                "needs_metrics": needs_metrics,
                "needs_logs": needs_logs
            }
            
        except Exception as e:
            logger.error(f"Error in query node: {str(e)}")
            state.error_message = f"Query processing failed: {str(e)}"
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
        
        # Continue with the data collection sequence
        return state
    
    def get_next_node(self, state: WorkflowState) -> str:
        """
        Determine the next node based on query processing results.

        Args:
            state: Current workflow state

        Returns:
            Name of the next node to execute
        """
        if state.error_message:
            return "error_handler"

        # Check if we're in a data collection sequence
        collection_sequence = state.metadata.get("data_collection_sequence", [])
        
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
        return state.metadata.get("next_node", "query_output")


# Create singleton instance
query_node = QueryNode()
