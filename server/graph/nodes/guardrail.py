"""
Guardrail Node for PaladinAI LangGraph Workflow.

This module implements the guardrail node that validates user input
to determine if it requires SRE workflow processing or can be handled
with a static response.
"""

import json
import logging
from typing import Dict, Any, Optional, List
from langfuse import observe

from ..state import WorkflowState, update_state_node, finalize_state, set_error
from llm.openai import openai
from prompts.system.guardrail import SYSTEM_PROMPT_GUARDRAIL
from memory import get_memory_service

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
                
                # Fetch and apply memory instructions
                state = await self._enhance_with_memory_instructions(state)
                
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
    
    async def _enhance_with_memory_instructions(self, state: WorkflowState) -> WorkflowState:
        """
        Enhance user input with relevant instructions from memory.
        
        Args:
            state: Current workflow state
            
        Returns:
            Updated workflow state with enhanced input
        """
        try:
            # Get memory service
            memory_service = get_memory_service()
            
            # Search for relevant memories, especially instructions
            from memory.models import MemorySearchQuery
            search_query = MemorySearchQuery(
                query=state.user_input,
                memory_types=["instruction"],  # Focus on instruction type memories
                limit=10,
                confidence_threshold=0.6
            )
            
            search_results = await memory_service.search_memories(search_query)
            
            if not search_results.get("success") or not search_results.get("memories"):
                logger.debug("No memory instructions found for enhancement")
                return state
            
            # Extract relevant instructions
            instructions = []
            for memory in search_results["memories"]:
                if memory.get("memory_type") == "instruction":
                    content = memory.get("memory", "")
                    if content and self._is_instruction_relevant(content, state.user_input):
                        instructions.append(content)
                        logger.info(f"Found relevant instruction: {content}")
            
            # If we have instructions, enhance the input
            if instructions:
                # Store original input if not already stored
                if not state.enhanced_input:
                    state.enhanced_input = state.user_input
                
                # Add instructions to the input
                instruction_text = " Additionally, follow these instructions: " + "; ".join(instructions)
                state.enhanced_input = state.user_input + instruction_text
                state.memory_instructions = instructions
                
                # Update metadata
                state.metadata["memory_enhanced"] = True
                state.metadata["instruction_count"] = len(instructions)
                
                logger.info(f"Enhanced input with {len(instructions)} memory instructions")
            else:
                # No relevant instructions, use original input
                state.enhanced_input = state.user_input
                state.metadata["memory_enhanced"] = False
                
        except Exception as e:
            logger.error(f"Error enhancing input with memory: {str(e)}", exc_info=True)
            # On error, use original input
            state.enhanced_input = state.user_input
            
        return state
    
    def _is_instruction_relevant(self, instruction: str, user_input: str) -> bool:
        """
        Check if an instruction is relevant to the current user input.
        
        Args:
            instruction: Memory instruction content
            user_input: Current user input
            
        Returns:
            True if instruction is relevant
        """
        # Simple keyword matching for now
        instruction_lower = instruction.lower()
        input_lower = user_input.lower()
        
        # Check for common monitoring keywords
        relevant_keywords = ["cpu", "memory", "disk", "network", "metrics", "logs", "alerts", 
                           "prometheus", "loki", "alertmanager", "performance", "usage"]
        
        # Check if instruction mentions any keyword that's also in user input
        for keyword in relevant_keywords:
            if keyword in instruction_lower and keyword in input_lower:
                return True
        
        # Check for specific patterns
        if "when asked for" in instruction_lower or "always fetch" in instruction_lower:
            # Check if the condition matches current input
            for keyword in relevant_keywords:
                if keyword in instruction_lower and keyword in input_lower:
                    return True
        
        return False
    
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
