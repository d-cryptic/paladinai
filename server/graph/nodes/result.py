"""
Result Node for PaladinAI LangGraph Workflow.

This module implements the result node that formats and returns
the final workflow results to the user.
"""

import logging
from datetime import datetime
from typing import Dict, Any
from langfuse import observe

from ..state import WorkflowState, update_state_node, finalize_state

logger = logging.getLogger(__name__)


class ResultNode:
    """
    Final node that formats and returns workflow results.
    
    This node prepares the final response based on the categorization
    and any processing that occurred during the workflow.
    """
    
    def __init__(self):
        """Initialize the result node."""
        self.node_name = "result"
    
    @observe(name="result_node")
    async def execute(self, state: WorkflowState) -> WorkflowState:
        """
        Execute result formatting and finalization.
        
        Args:
            state: Current workflow state with processing results
            
        Returns:
            Finalized workflow state with formatted results
        """
        logger.info(f"Formatting results for session: {state.session_id}")
        
        # Update state to reflect current node
        state = update_state_node(state, self.node_name)
        
        try:
            # Calculate execution time
            start_time = datetime.fromisoformat(state.metadata.get("workflow_started_at", datetime.now().isoformat()))
            execution_time_ms = int((datetime.now() - start_time).total_seconds() * 1000)
            
            # Format final result
            result = self._format_result(state)
            
            # Finalize state
            state = finalize_state(state, result, execution_time_ms)
            
            logger.info(
                f"Workflow completed successfully in {execution_time_ms}ms "
                f"for session: {state.session_id}"
            )
            
        except Exception as e:
            error_msg = f"Unexpected error in result node: {str(e)}"
            logger.error(error_msg, exc_info=True)
            state.error_message = error_msg
        
        return state
    
    def _format_result(self, state: WorkflowState) -> Dict[str, Any]:
        """
        Format the final workflow result.
        
        Args:
            state: Current workflow state
            
        Returns:
            Formatted result dictionary
        """
        if state.error_message:
            return {
                "success": False,
                "error": state.error_message,
                "session_id": state.session_id,
                "execution_path": state.execution_path
            }
        
        if not state.categorization:
            return {
                "success": False,
                "error": "I am a very dumb intern. I don't know anything other than SRE, DevOps, and system reliability. Please ask questions related to SRE, incident, or related technical operations.",
                "session_id": state.session_id,
                "execution_path": state.execution_path
            }
        
        # Format successful categorization result
        workflow_type_value = state.categorization.workflow_type
        if hasattr(workflow_type_value, 'value'):
            workflow_type_value = workflow_type_value.value

        complexity_value = state.categorization.estimated_complexity
        if hasattr(complexity_value, 'value'):
            complexity_value = complexity_value.value

        result = {
            "success": True,
            "content": f"✅ Categorized as {workflow_type_value} workflow\n📊 Confidence: {state.categorization.confidence:.1%}\n💡 {state.categorization.reasoning}\n🎯 Suggested approach: {state.categorization.suggested_approach}",
            "session_id": state.session_id,
            "user_input": state.user_input,
            "categorization": {
                "workflow_type": workflow_type_value,
                "confidence": state.categorization.confidence,
                "reasoning": state.categorization.reasoning,
                "suggested_approach": state.categorization.suggested_approach,
                "estimated_complexity": complexity_value
            },
            "execution_metadata": {
                "execution_path": state.execution_path,
                "timestamp": state.timestamp.isoformat(),
                "input_length": len(state.user_input)
            }
        }
        
        # Add workflow-specific guidance
        workflow_type_for_guidance = state.categorization.workflow_type
        if hasattr(workflow_type_for_guidance, 'value'):
            workflow_type_for_guidance = workflow_type_for_guidance.value
        result["next_steps"] = self._get_workflow_guidance(workflow_type_for_guidance)
        
        return result
    
    def _get_workflow_guidance(self, workflow_type: str) -> Dict[str, Any]:
        """
        Get guidance for the next steps based on workflow type.
        
        Args:
            workflow_type: The categorized workflow type
            
        Returns:
            Dictionary containing guidance and next steps
        """
        guidance = {
            "QUERY": {
                "description": "Quick status/boolean information request",
                "typical_response_time": "< 30 seconds",
                "data_sources": ["Prometheus metrics", "Alertmanager alerts"],
                "response_format": "Concise, factual, boolean or status-based"
            },
            "INCIDENT": {
                "description": "Problem investigation and root cause analysis",
                "typical_response_time": "2-10 minutes",
                "data_sources": ["Prometheus metrics", "Loki logs", "Alertmanager alerts"],
                "response_format": "Comprehensive analysis with root cause investigation"
            },
            "ACTION": {
                "description": "Data retrieval, analysis, and reporting",
                "typical_response_time": "1-5 minutes",
                "data_sources": ["Historical metrics", "Log aggregations", "Performance data"],
                "response_format": "Structured data with detailed analysis and reports"
            }
        }
        
        return guidance.get(workflow_type, {
            "description": "Unknown workflow type",
            "typical_response_time": "Variable",
            "data_sources": ["To be determined"],
            "response_format": "To be determined"
        })
    
    def get_next_node(self, state: WorkflowState) -> str:
        """
        Determine the next node (should be None for result node).
        
        Args:
            state: Current workflow state
            
        Returns:
            None as this is the final node
        """
        return None  # Result node is terminal


# Create singleton instance
result_node = ResultNode()
