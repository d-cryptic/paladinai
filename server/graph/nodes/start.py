"""
Start Node for PaladinAI LangGraph Workflow.

This module implements the start node that initializes the workflow
and prepares the state for processing.
"""

import logging
from datetime import datetime
from typing import Optional
from langfuse import observe

from ..state import WorkflowState, update_state_node

logger = logging.getLogger(__name__)


class StartNode:
    """
    Entry point node for the workflow.
    
    This node handles initial setup, validation, and preparation
    of the workflow state before processing begins.
    """
    
    def __init__(self):
        """Initialize the start node."""
        self.node_name = "start"
    
    @observe(name="start_node")
    async def execute(self, state: WorkflowState) -> WorkflowState:
        """
        Execute start node initialization.
        
        Args:
            state: Initial workflow state
            
        Returns:
            Updated workflow state ready for processing
        """
        logger.info(f"Starting workflow for input: {state.user_input[:100]}...")
        
        # Update state to reflect current node
        state = update_state_node(state, self.node_name)
        
        try:
            # Validate user input
            if not state.user_input or not state.user_input.strip():
                state.error_message = "Empty or invalid user input provided"
                logger.error("Empty user input provided")
                return state
            
            # Initialize metadata
            state.metadata.update({
                "workflow_started_at": datetime.now().isoformat(),
                "input_length": len(state.user_input),
                "input_preview": state.user_input[:200] + "..." if len(state.user_input) > 200 else state.user_input
            })
            
            # Generate session ID if not provided
            if not state.session_id:
                state.session_id = self._generate_session_id()
                logger.info(f"Generated session ID: {state.session_id}")
            
            logger.info(f"Workflow initialized successfully for session: {state.session_id}")
            
        except Exception as e:
            error_msg = f"Unexpected error in start node: {str(e)}"
            logger.error(error_msg, exc_info=True)
            state.error_message = error_msg
        
        return state
    
    def _generate_session_id(self) -> str:
        """
        Generate a unique session identifier.
        
        Returns:
            Unique session ID string
        """
        import uuid
        return f"session_{uuid.uuid4().hex[:12]}"
    
    def get_next_node(self, state: WorkflowState) -> str:
        """
        Determine the next node based on start node results.
        
        Args:
            state: Current workflow state
            
        Returns:
            Name of the next node to execute
        """
        if state.error_message:
            return "error_handler"
        
        # Always proceed to categorization after successful start
        return "categorize"


# Create singleton instance
start_node = StartNode()
