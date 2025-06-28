"""
Guardrail Node for PaladinAI LangGraph Workflow.

This module implements the guardrail node that validates user input
to determine if it requires SRE workflow processing or can be handled
with a static response.
"""

import json
import logging
from typing import Dict, Any, Optional
from langfuse import observe

from ..state import WorkflowState, update_state_node, finalize_state, set_error
from llm.openai import openai
from prompts.system.guardrail import SYSTEM_PROMPT_GUARDRAIL

logger = logging.getLogger(__name__)


class GuardrailNode:
    """
    Node responsible for validating user input before workflow processing.
    
    This node uses OpenAI to analyze user input and determine whether
    it should proceed to categorization or return a static response.
    """
    
    def __init__(self):
        """Initialize the guardrail node."""
        self.node_name = "guardrail"
    
    @observe(name="guardrail_node")
    async def execute(self, state: WorkflowState) -> WorkflowState:
        """
        Execute guardrail validation on user input.
        
        Args:
            state: Current workflow state containing user input
            
        Returns:
            Updated workflow state with validation results
        """
        logger.info(f"Executing guardrail validation for session: {state.session_id}")
        
        # Update state to reflect current node
        state = update_state_node(state, self.node_name)
        
        try:
            # Validate user input using OpenAI
            validation_result = await self._validate_user_input(state.user_input)
            
            # Update API call tracking
            state.total_api_calls += 1
            
            # Process validation result
            if validation_result.get("status") == "unable_to_answer":
                # Input is not SRE-related, provide static response
                static_response = self._create_static_response(validation_result, state)
                state = finalize_state(state, static_response, 0)
                
                logger.info(
                    f"Guardrail blocked non-SRE query for session: {state.session_id}"
                )
            else:
                # Input is SRE-related, proceed to categorization
                state.metadata["guardrail_validation"] = "passed"
                state.metadata["guardrail_reasoning"] = validation_result.get("reasoning", "SRE-related query")
                
                logger.info(
                    f"Guardrail approved SRE query for session: {state.session_id}"
                )
            
        except Exception as e:
            error_msg = f"Error in guardrail validation: {str(e)}"
            logger.error(error_msg, exc_info=True)
            state = set_error(state, error_msg, self.node_name)
        
        return state
    
    async def _validate_user_input(self, user_input: str) -> Dict[str, Any]:
        """
        Validate user input using OpenAI with guardrail prompt.
        
        Args:
            user_input: User's input to validate
            
        Returns:
            Dictionary containing validation result
        """
        try:
            # Create validation prompt
            validation_prompt = f"""
{SYSTEM_PROMPT_GUARDRAIL}

User Input: "{user_input}"

Analyze the above user input and determine if it's related to SRE, DevOps, system reliability, 
incident response, or related technical operations. 

If it's NOT related to SRE/DevOps/Operations, respond with the exact JSON format specified in the guardrail prompt.
If it IS related to SRE/DevOps/Operations, respond with: {{"status": "proceed", "reasoning": "Brief explanation of why this is SRE-related"}}

Response:"""

            # Call OpenAI API
            response = await openai.chat_completion(
                user_message=user_input,
                system_prompt=validation_prompt,
                max_tokens=500,
                temperature=0.1
            )
            
            # Extract and parse response
            response_content = response.get("content", "").strip()
            
            # Try to parse as JSON
            try:
                validation_result = json.loads(response_content)
                return validation_result
            except json.JSONDecodeError:
                # If not valid JSON, assume it's SRE-related
                logger.warning(f"Could not parse guardrail response as JSON: {response_content}")
                return {
                    "status": "proceed",
                    "reasoning": "Could not parse validation response, proceeding with caution"
                }
                
        except Exception as e:
            logger.error(f"Error calling OpenAI for guardrail validation: {str(e)}")
            # On API error, proceed to categorization to avoid blocking valid queries
            return {
                "status": "proceed",
                "reasoning": f"API error during validation: {str(e)}"
            }
    
    def _create_static_response(
        self, 
        validation_result: Dict[str, Any], 
        state: WorkflowState
    ) -> Dict[str, Any]:
        """
        Create static response for non-SRE queries.
        
        Args:
            validation_result: Result from guardrail validation
            state: Current workflow state
            
        Returns:
            Formatted static response
        """
        reason = validation_result.get(
            "reason", 
            "Query is outside the scope of site reliability engineering and incident response."
        )
        
        return {
            "success": True,
            "content": f"🚫 {reason}\n\n💡 I'm specialized in SRE, DevOps, and system reliability topics. Please ask about:\n• System monitoring and observability\n• Incident response and troubleshooting\n• Infrastructure and deployment issues\n• Performance optimization\n• Reliability engineering practices",
            "session_id": state.session_id,
            "user_input": state.user_input,
            "guardrail_status": "blocked",
            "execution_metadata": {
                "execution_path": state.execution_path,
                "timestamp": state.timestamp.isoformat(),
                "input_length": len(state.user_input),
                "blocked_reason": reason
            }
        }
    
    def get_next_node(self, state: WorkflowState) -> str:
        """
        Determine the next node based on guardrail validation results.
        
        Args:
            state: Current workflow state
            
        Returns:
            Name of the next node to execute
        """
        if state.error_message:
            return "error_handler"
        
        # If we have a final result, it means we provided a static response
        if state.final_result:
            return "result"
        
        # Otherwise, proceed to categorization
        return "categorize"


# Create singleton instance
guardrail_node = GuardrailNode()
