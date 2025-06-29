"""
Query Node for PaladinAI LangGraph Workflow.

This module implements the query node that handles query-type requests
and routes metrics-related requests to the prometheus node.
"""

import logging
from typing import Dict, Any
from langfuse import observe

from ...state import WorkflowState, update_state_node
from .analyzers import analyze_metrics_requirement, process_non_metrics_query
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
            # Determine if this query requires metrics data
            needs_metrics = await analyze_metrics_requirement(state.user_input)
            
            if needs_metrics:
                logger.info("Query requires metrics data - routing to prometheus node")
                # Set flag to route to prometheus node
                state.metadata["needs_prometheus"] = True
                state.metadata["originating_node"] = "query"
                
                # Store query context for prometheus node
                state.metadata["query_context"] = {
                    "workflow_type": "query",
                    "user_input": state.user_input,
                    "processing_stage": "data_collection"
                }
                
                # Set next node to prometheus
                state.metadata["next_node"] = "prometheus"
                
            else:
                logger.info("Query can be handled without metrics data")
                # Process query directly without metrics
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
                "needs_metrics": needs_metrics
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

        # Check if prometheus processing is complete first
        if state.metadata.get("prometheus_collection_complete"):
            # Prometheus data has been collected and processed, route to output
            return state.metadata.get("next_node", "query_output")

        # Check if we need to route to prometheus
        if state.metadata.get("needs_prometheus"):
            return "prometheus"

        # Otherwise route to output
        return state.metadata.get("next_node", "query_output")


# Create singleton instance
query_node = QueryNode()
