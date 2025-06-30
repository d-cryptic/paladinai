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
            
            logger.info(f"Data requirements - Metrics: {needs_metrics}, Logs: {needs_logs}")
            
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
            if needs_metrics and needs_logs:
                logger.info("Query requires both metrics and logs")
                # First collect metrics, then logs
                state.metadata["needs_prometheus"] = True
                state.metadata["needs_loki"] = True
                state.metadata["next_node"] = "prometheus"
                state.metadata["data_collection_sequence"] = ["prometheus", "loki"]
                
            elif needs_metrics:
                logger.info("Query requires only metrics data")
                state.metadata["needs_prometheus"] = True
                state.metadata["next_node"] = "prometheus"
                
            elif needs_logs:
                logger.info("Query requires only log data")
                state.metadata["needs_loki"] = True
                state.metadata["next_node"] = "loki"
                
            else:
                logger.info("Query can be handled without external data")
                # Process query directly without metrics or logs
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
        
        # Check if prometheus processing is complete
        if state.metadata.get("prometheus_collection_complete"):
            # If we still need loki data and haven't collected it yet
            if state.metadata.get("needs_loki") and not state.metadata.get("loki_collection_complete"):
                return "loki"
            # Otherwise, route to output
            return state.metadata.get("next_node", "query_output")
        
        # Check if loki processing is complete
        if state.metadata.get("loki_collection_complete"):
            # If we still need prometheus data and haven't collected it yet
            if state.metadata.get("needs_prometheus") and not state.metadata.get("prometheus_collection_complete"):
                return "prometheus"
            # Otherwise, route to output
            return state.metadata.get("next_node", "query_output")

        # Initial routing based on needs
        if state.metadata.get("needs_prometheus") and not state.metadata.get("prometheus_collection_complete"):
            return "prometheus"
        elif state.metadata.get("needs_loki") and not state.metadata.get("loki_collection_complete"):
            return "loki"

        # Otherwise route to output
        return state.metadata.get("next_node", "query_output")


# Create singleton instance
query_node = QueryNode()
