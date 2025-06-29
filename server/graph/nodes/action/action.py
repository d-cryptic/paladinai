"""
Action Node for PaladinAI LangGraph Workflow.

This module implements the action node that handles action-type requests
and routes metrics-related requests to the prometheus node.
"""

import logging
from typing import Dict, Any
from langfuse import observe

from ...state import WorkflowState, update_state_node
from .analyzers import analyze_action_requirements
from .processors import process_non_metrics_action, process_prometheus_result

logger = logging.getLogger(__name__)


class ActionNode:
    """
    Node responsible for handling action-type workflow requests.
    
    This node processes user action requests (data retrieval, analysis, reporting),
    determines if metrics data is needed, and routes to prometheus node when necessary.
    """
    
    def __init__(self):
        """Initialize the action node."""
        self.node_name = "action"

    
    @observe(name="action_node")
    async def execute(self, state: WorkflowState) -> WorkflowState:
        """
        Execute action workflow processing.
        
        Args:
            state: Current workflow state containing user input and categorization
            
        Returns:
            Updated workflow state with action processing results
        """
        logger.info(f"Executing action node for input: {state.user_input[:100]}...")
        
        # Update state to indicate action node execution
        state = update_state_node(state, self.node_name)
        
        try:
            # Analyze the action request to determine data requirements
            action_analysis = await analyze_action_requirements(state.user_input)
            
            if action_analysis.get("needs_metrics", False):
                logger.info("Action requires metrics data - routing to prometheus node")
                # Set flag to route to prometheus node
                state.metadata["needs_prometheus"] = True
                state.metadata["originating_node"] = "action"
                
                # Store action context for prometheus node
                state.metadata["action_context"] = {
                    "workflow_type": "action",
                    "user_input": state.user_input,
                    "action_type": action_analysis.get("action_type", "data_analysis"),
                    "data_requirements": action_analysis.get("data_requirements", {}),
                    "processing_stage": "data_collection"
                }
                
                # Set next node to prometheus
                state.metadata["next_node"] = "prometheus"
                
            else:
                logger.info("Action can be handled without metrics data")
                # Process action directly without metrics
                result = await process_non_metrics_action(state.user_input, action_analysis)
                
                if result["success"]:
                    state.metadata["action_result"] = result["data"]
                    state.metadata["next_node"] = "action_output"
                else:
                    state.error_message = result.get("error", "Failed to process action")
                    state.metadata["next_node"] = "error_handler"
            
            # Store action analysis for later use
            state.metadata["action_analysis"] = action_analysis
            
            # Store detailed execution info in metadata
            state.metadata[f"{self.node_name}_execution"] = {
                "timestamp": state.metadata.get("current_timestamp"),
                "status": "completed" if not state.error_message else "error",
                "needs_metrics": action_analysis.get("needs_metrics", False),
                "action_type": action_analysis.get("action_type", "unknown")
            }
            
        except Exception as e:
            logger.error(f"Error in action node: {str(e)}")
            state.error_message = f"Action processing failed: {str(e)}"
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
        Determine the next node based on action processing results.

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
            return state.metadata.get("next_node", "action_output")

        # Check if we need to route to prometheus
        if state.metadata.get("needs_prometheus"):
            return "prometheus"

        # Otherwise route to output
        return state.metadata.get("next_node", "action_output")


# Create singleton instance
action_node = ActionNode()
