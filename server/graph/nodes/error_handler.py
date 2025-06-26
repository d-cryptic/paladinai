"""
Error Handler Node for PaladinAI LangGraph Workflow.

This module implements the error handler node that manages
workflow errors and provides appropriate error responses.
"""

import logging
from datetime import datetime
from typing import Dict, Any
from langfuse import observe

from ..state import WorkflowState, update_state_node, finalize_state

logger = logging.getLogger(__name__)


class ErrorHandlerNode:
    """
    Node responsible for handling workflow errors.
    
    This node processes errors that occur during workflow execution
    and formats appropriate error responses for the user.
    """
    
    def __init__(self):
        """Initialize the error handler node."""
        self.node_name = "error_handler"
    
    @observe(name="error_handler_node")
    async def execute(self, state: WorkflowState) -> WorkflowState:
        """
        Execute error handling and response formatting.
        
        Args:
            state: Current workflow state with error information
            
        Returns:
            Finalized workflow state with error response
        """
        logger.error(f"Handling workflow error for session: {state.session_id}")
        logger.error(f"Error message: {state.error_message}")
        
        # Update state to reflect current node
        state = update_state_node(state, self.node_name)
        
        try:
            # Calculate execution time
            start_time = datetime.fromisoformat(state.metadata.get("workflow_started_at", datetime.now().isoformat()))
            execution_time_ms = int((datetime.now() - start_time).total_seconds() * 1000)
            
            # Format error response
            error_result = self._format_error_response(state)
            
            # Finalize state with error result
            state = finalize_state(state, error_result, execution_time_ms)
            
            logger.info(
                f"Error handling completed in {execution_time_ms}ms "
                f"for session: {state.session_id}"
            )
            
        except Exception as e:
            # Handle errors in error handling (meta-error)
            meta_error_msg = f"Error in error handler: {str(e)}"
            logger.critical(meta_error_msg, exc_info=True)
            
            # Create minimal error response
            state.final_result = {
                "success": False,
                "error": "Critical system error occurred",
                "session_id": state.session_id,
                "details": meta_error_msg
            }
        
        return state
    
    def _format_error_response(self, state: WorkflowState) -> Dict[str, Any]:
        """
        Format a comprehensive error response.
        
        Args:
            state: Current workflow state with error
            
        Returns:
            Formatted error response dictionary
        """
        # Categorize error type
        error_category = self._categorize_error(state.error_message)
        
        # Create base error response
        error_response = {
            "success": False,
            "error": state.error_message,
            "error_category": error_category,
            "session_id": state.session_id,
            "timestamp": datetime.now().isoformat(),
            "execution_metadata": {
                "execution_path": state.execution_path,
                "current_node": state.current_node,
                "user_input": state.user_input[:200] + "..." if len(state.user_input) > 200 else state.user_input
            }
        }
        
        # Add error-specific guidance
        error_response["guidance"] = self._get_error_guidance(error_category)
        
        # Add retry information if applicable
        if error_category in ["api_error", "temporary_error"]:
            error_response["retry_recommended"] = True
            error_response["retry_delay_seconds"] = 5
        else:
            error_response["retry_recommended"] = False
        
        # Add partial results if available
        if state.categorization:
            workflow_type_value = state.categorization.workflow_type
            if hasattr(workflow_type_value, 'value'):
                workflow_type_value = workflow_type_value.value

            error_response["partial_results"] = {
                "categorization": {
                    "workflow_type": workflow_type_value,
                    "confidence": state.categorization.confidence,
                    "reasoning": state.categorization.reasoning
                }
            }
        
        return error_response
    
    def _categorize_error(self, error_message: str) -> str:
        """
        Categorize the error based on the error message.
        
        Args:
            error_message: The error message to categorize
            
        Returns:
            Error category string
        """
        if not error_message:
            return "unknown_error"
        
        error_lower = error_message.lower()
        
        # API-related errors
        if any(keyword in error_lower for keyword in ["api", "openai", "rate limit", "quota"]):
            return "api_error"
        
        # Input validation errors
        if any(keyword in error_lower for keyword in ["empty", "invalid", "missing", "required"]):
            return "input_error"
        
        # JSON parsing errors
        if any(keyword in error_lower for keyword in ["json", "parse", "decode"]):
            return "parsing_error"
        
        # Network/connection errors
        if any(keyword in error_lower for keyword in ["connection", "network", "timeout", "unreachable"]):
            return "network_error"
        
        # Temporary/transient errors
        if any(keyword in error_lower for keyword in ["temporary", "retry", "unavailable"]):
            return "temporary_error"
        
        # System errors
        if any(keyword in error_lower for keyword in ["system", "internal", "unexpected"]):
            return "system_error"
        
        return "unknown_error"
    
    def _get_error_guidance(self, error_category: str) -> Dict[str, Any]:
        """
        Get user guidance based on error category.
        
        Args:
            error_category: The categorized error type
            
        Returns:
            Dictionary containing user guidance
        """
        guidance_map = {
            "api_error": {
                "message": "There was an issue with the AI service. Please try again in a moment.",
                "user_action": "Wait a few seconds and retry your request",
                "technical_note": "API rate limits or service unavailability"
            },
            "input_error": {
                "message": "There was an issue with your input. Please check and try again.",
                "user_action": "Ensure your input is not empty and contains a clear question or request",
                "technical_note": "Input validation failed"
            },
            "parsing_error": {
                "message": "There was an issue processing the AI response. Please try again.",
                "user_action": "Retry your request - this is usually a temporary issue",
                "technical_note": "JSON parsing or response format error"
            },
            "network_error": {
                "message": "There was a network connectivity issue. Please try again.",
                "user_action": "Check your connection and retry in a moment",
                "technical_note": "Network connectivity or timeout error"
            },
            "temporary_error": {
                "message": "A temporary issue occurred. Please try again shortly.",
                "user_action": "Wait a moment and retry your request",
                "technical_note": "Transient system error"
            },
            "system_error": {
                "message": "An internal system error occurred. Please contact support if this persists.",
                "user_action": "Try again, or contact support if the issue continues",
                "technical_note": "Internal system error requiring investigation"
            },
            "unknown_error": {
                "message": "An unexpected error occurred. Please try again or contact support.",
                "user_action": "Retry your request or contact support for assistance",
                "technical_note": "Unclassified error requiring investigation"
            }
        }
        
        return guidance_map.get(error_category, guidance_map["unknown_error"])
    
    def get_next_node(self, state: WorkflowState) -> str:
        """
        Determine the next node (should be None for error handler).
        
        Args:
            state: Current workflow state
            
        Returns:
            None as this is a terminal node
        """
        return None  # Error handler is terminal


# Create singleton instance
error_handler_node = ErrorHandlerNode()
