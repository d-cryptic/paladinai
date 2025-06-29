"""
Prometheus Node for PaladinAI LangGraph Workflow.

This module implements the prometheus node that handles metrics data collection
using tools from server/tools/prometheus/ and processes data according to user requirements.
"""

import logging
from langfuse import observe

from ...state import WorkflowState, update_state_node
from .data_collector import data_collector
from .data_processor import data_processor

logger = logging.getLogger(__name__)


class PrometheusNode:
    """
    Node responsible for collecting and processing metrics data from Prometheus.
    
    This node receives requests from query/action/incident nodes, collects relevant
    metrics data, and returns processed results back to the originating node.
    """
    
    def __init__(self):
        """Initialize the prometheus node."""
        self.node_name = "prometheus"
    
    @observe(name="prometheus_node")
    async def execute(self, state: WorkflowState) -> WorkflowState:
        """
        Execute prometheus data collection and processing with simplified single-pass approach.

        Args:
            state: Current workflow state containing context from originating node

        Returns:
            Updated workflow state with prometheus data and routing back to originating node
        """
        logger.info(f"Executing prometheus node for {state.metadata.get('originating_node', 'unknown')} workflow")

        # Update state to indicate prometheus node execution
        state = update_state_node(state, self.node_name)

        try:
            # Get context from originating node
            originating_node = state.metadata.get("originating_node", "unknown")
            context_key = f"{originating_node}_context"
            context = state.metadata.get(context_key, {})

            # Initial data collection using the data collector
            collected_data = await data_collector.collect_metrics_data_ai_driven(
                state.user_input,
                context,
                str(originating_node)
            )

            if not collected_data.get("success", False):
                state.error_message = f"Failed to collect metrics data: {collected_data.get('error')}"
                state.metadata["next_node"] = "error_handler"
                return state

            # Simplified data collection - single pass without validation loop
            successful_metrics = [m for m in collected_data["data"].get("metrics", []) if "error" not in m]
            logger.info(f"Collected {len(successful_metrics)} successful metrics out of {len(collected_data['data'].get('metrics', []))} total")

            # Process and format the collected data using the data processor
            processed_data = await data_processor.process_collected_data(
                collected_data["data"],
                state.user_input,
                context,
                str(originating_node)
            )

            # Store processed data for originating node
            state.metadata["prometheus_data"] = processed_data
            state.metadata["prometheus_collection_complete"] = True

            # Route back to originating node for final processing
            state.metadata["next_node"] = f"{originating_node}_prometheus_return"

            # Store detailed execution info in metadata
            state.metadata[f"{self.node_name}_execution"] = {
                "timestamp": state.metadata.get("current_timestamp"),
                "status": "completed",
                "metrics_collected": len(processed_data.get("metrics", [])),
                "originating_node": originating_node,
                "data_points": processed_data.get("total_data_points", 0),
                "validation_iterations": 0  # No validation loop anymore
            }

        except Exception as e:
            logger.error(f"Error in prometheus node: {str(e)}")
            state.error_message = f"Prometheus data collection failed: {str(e)}"
            state.metadata["next_node"] = "error_handler"

            # Store error execution info in metadata
            state.metadata[f"{self.node_name}_execution"] = {
                "timestamp": state.metadata.get("current_timestamp"),
                "status": "error",
                "error": str(e)
            }

        return state

    def get_next_node(self, state: WorkflowState) -> str:
        """
        Determine the next node based on prometheus processing results.
        
        Args:
            state: Current workflow state
            
        Returns:
            Name of the next node to execute (back to originating node)
        """
        if state.error_message:
            return "error_handler"
        
        # Route back to originating node for final processing
        originating_node = state.metadata.get("originating_node")
        if originating_node:
            return f"{originating_node}_prometheus_return"
        
        # Fallback to result node if no originating node specified
        return "result"


# Create singleton instance
prometheus_node = PrometheusNode()
