"""
Categorization Node for PaladinAI LangGraph Workflow.

This module implements the categorization node that analyzes user input
and categorizes it into Query, Incident, or Action workflow types.
"""

import json
import logging
from typing import Dict, Any
from langfuse import observe

from ..state import WorkflowState, CategorizationResult, NodeResult, update_state_node
from llm.openai import openai
from prompts.workflows.categorization import WORKFLOW_CATEGORIZATION_PROMPT

logger = logging.getLogger(__name__)


class CategorizationNode:
    """
    Node responsible for categorizing user input into workflow types.
    
    This node uses OpenAI to analyze user input and determine whether
    it should be handled as a Query, Incident, or Action workflow.
    """
    
    def __init__(self):
        """Initialize the categorization node."""
        self.node_name = "categorization"
    
    @observe(name="categorization_node")
    async def execute(self, state: WorkflowState) -> WorkflowState:
        """
        Execute categorization analysis on user input.
        
        Args:
            state: Current workflow state containing user input
            
        Returns:
            Updated workflow state with categorization results
        """
        logger.info(f"Executing categorization node for input: {state.user_input[:100]}...")
        
        # Update state to reflect current node
        state = update_state_node(state, self.node_name)
        
        try:
            # Perform categorization using OpenAI
            # Use enhanced input if available, otherwise use original
            input_to_categorize = state.enhanced_input or state.user_input
            result = await self._categorize_input(input_to_categorize)
            
            if result.success:
                # Parse categorization result
                categorization = self._parse_categorization_result(result.data)
                state.categorization = categorization
                
                logger.info(
                    f"Categorization successful: {categorization.workflow_type} "
                    f"(confidence: {categorization.confidence:.2f})"
                )
                
                # Add categorization metadata
                complexity_value = categorization.estimated_complexity
                if hasattr(complexity_value, 'value'):
                    complexity_value = complexity_value.value

                state.metadata.update({
                    "categorization_confidence": categorization.confidence,
                    "categorization_reasoning": categorization.reasoning,
                    "estimated_complexity": complexity_value
                })
                
            else:
                # Handle categorization failure
                error_msg = f"Categorization failed: {result.error}"
                logger.error(error_msg)
                state.error_message = error_msg
                
        except Exception as e:
            error_msg = f"Unexpected error in categorization node: {str(e)}"
            logger.error(error_msg, exc_info=True)
            state.error_message = error_msg
        
        return state
    
    @observe(name="categorize_input")
    async def _categorize_input(self, user_input: str) -> NodeResult:
        """
        Use OpenAI to categorize the user input.
        
        Args:
            user_input: The user's input to categorize
            
        Returns:
            NodeResult containing categorization data or error
        """
        try:
            # Prepare the prompt with user input
            categorization_prompt = f"{WORKFLOW_CATEGORIZATION_PROMPT}\n\nUser Input: {user_input}"
            
            # Make OpenAI API call
            response = await openai.chat_completion(
                user_message=user_input,
                system_prompt=categorization_prompt,
                temperature=0.1,  # Low temperature for consistent categorization
                max_tokens=500    # Sufficient for categorization response
            )
            
            if response["success"]:
                # Parse JSON response
                try:
                    categorization_data = json.loads(response["content"])
                    return NodeResult(
                        success=True,
                        data=categorization_data,
                        metadata={
                            "model": response["model"],
                            "usage": response["usage"],
                            "finish_reason": response["finish_reason"]
                        }
                    )
                except json.JSONDecodeError as e:
                    return NodeResult(
                        success=False,
                        error=f"Failed to parse categorization JSON: {str(e)}",
                        data={"raw_response": response["content"]}
                    )
            else:
                return NodeResult(
                    success=False,
                    error=f"OpenAI API call failed: {response['error']}"
                )
                
        except Exception as e:
            return NodeResult(
                success=False,
                error=f"Unexpected error during categorization: {str(e)}"
            )
    
    def _parse_categorization_result(self, data: Dict[str, Any]) -> CategorizationResult:
        """
        Parse and validate categorization result from OpenAI response.
        
        Args:
            data: Raw categorization data from OpenAI
            
        Returns:
            Validated CategorizationResult object
            
        Raises:
            ValueError: If categorization data is invalid
        """
        try:
            # Validate required fields
            required_fields = ["workflow_type", "confidence", "reasoning", "suggested_approach", "estimated_complexity"]
            for field in required_fields:
                if field not in data:
                    raise ValueError(f"Missing required field: {field}")
            
            # Create and validate CategorizationResult
            categorization = CategorizationResult(
                workflow_type=data["workflow_type"],
                confidence=float(data["confidence"]),
                reasoning=data["reasoning"],
                suggested_approach=data["suggested_approach"],
                estimated_complexity=data["estimated_complexity"]
            )
            
            return categorization
            
        except Exception as e:
            raise ValueError(f"Invalid categorization data: {str(e)}")
    
    def get_next_node(self, state: WorkflowState) -> str:
        """
        Determine the next node based on categorization results.

        Args:
            state: Current workflow state with categorization

        Returns:
            Name of the next node to execute
        """
        if state.error_message:
            return "error_handler"

        if not state.categorization:
            return "error_handler"

        # Route to appropriate workflow node based on categorization
        workflow_type = state.categorization.workflow_type

        # Handle both enum and string values (due to use_enum_values = True)
        workflow_type_value = workflow_type.value if hasattr(workflow_type, 'value') else workflow_type

        if workflow_type_value == "QUERY":
            return "query"
        elif workflow_type_value == "ACTION":
            return "action"
        elif workflow_type_value == "INCIDENT":
            return "incident"
        else:
            # Fallback to result node for unknown types
            logger.warning(f"Unknown workflow type: {workflow_type_value}, routing to result")
            return "result"


# Create singleton instance
categorization_node = CategorizationNode()
