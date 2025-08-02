"""
Centralized Prompt Manager for PaladinAI LangGraph Workflows.

This module provides centralized access to all workflow-specific prompts
with dynamic loading and formatting capabilities.
"""

import logging
from typing import Dict, Any, Optional
from enum import Enum

from .data_collection.query_prompts import get_query_prompt, get_query_examples
from .data_collection.incident_prompts import get_incident_prompt, get_incident_examples
from .data_collection.action_prompts import get_action_prompt, get_action_examples
from .workflows.categorization import (
    WORKFLOW_CATEGORIZATION_PROMPT,
    get_workflow_prompt,
    get_categorization_examples
)

# Import WorkflowType from state module to avoid conflicts
try:
    from graph.state.enums import WorkflowType
except ImportError:
    # Fallback definition if import fails
    class WorkflowType(str, Enum):
        QUERY = "QUERY"
        INCIDENT = "INCIDENT"
        ACTION = "ACTION"

logger = logging.getLogger(__name__)


class PromptType(str, Enum):
    """Types of prompts available."""
    CATEGORIZATION = "categorization"
    DATA_COLLECTION = "data_collection"
    DATA_EVALUATION = "data_evaluation"
    OUTPUT_FORMATTING = "output_formatting"
    TOOL_SELECTION = "tool_selection"
    ANALYSIS = "analysis"
    REPORTING = "reporting"
    INVESTIGATION = "investigation"


class PromptManager:
    """
    Centralized manager for all workflow prompts.
    
    This class provides a unified interface for accessing and formatting
    prompts across different workflow types and processing stages.
    """
    
    def __init__(self):
        """Initialize the prompt manager."""
        self.prompt_cache: Dict[str, str] = {}
        self.workflow_examples = {
            WorkflowType.QUERY: get_query_examples(),
            WorkflowType.INCIDENT: get_incident_examples(),
            WorkflowType.ACTION: get_action_examples()
        }
    
    def get_prompt(
        self, 
        workflow_type: WorkflowType, 
        prompt_type: PromptType, 
        **kwargs
    ) -> str:
        """
        Get a formatted prompt for a specific workflow and prompt type.
        
        Args:
            workflow_type: Type of workflow (QUERY, INCIDENT, ACTION)
            prompt_type: Type of prompt needed
            **kwargs: Variables to format into the prompt
            
        Returns:
            Formatted prompt string
            
        Raises:
            ValueError: If workflow or prompt type is not supported
        """
        cache_key = f"{workflow_type}_{prompt_type}"
        
        try:
            if prompt_type == PromptType.CATEGORIZATION:
                return self._get_categorization_prompt(**kwargs)
            
            elif workflow_type == WorkflowType.QUERY:
                return get_query_prompt(prompt_type.value, **kwargs)
            
            elif workflow_type == WorkflowType.INCIDENT:
                return get_incident_prompt(prompt_type.value, **kwargs)
            
            elif workflow_type == WorkflowType.ACTION:
                return get_action_prompt(prompt_type.value, **kwargs)
            
            else:
                raise ValueError(f"Unsupported workflow type: {workflow_type}")
                
        except Exception as e:
            logger.error(f"Error getting prompt {cache_key}: {str(e)}")
            raise ValueError(f"Failed to get prompt: {str(e)}")
    
    def _get_categorization_prompt(self, **kwargs) -> str:
        """Get the workflow categorization prompt."""
        return WORKFLOW_CATEGORIZATION_PROMPT.format(**kwargs) if kwargs else WORKFLOW_CATEGORIZATION_PROMPT
    
    def get_workflow_specific_prompt(self, workflow_type: WorkflowType) -> str:
        """
        Get the workflow-specific processing prompt.
        
        Args:
            workflow_type: Type of workflow
            
        Returns:
            Workflow-specific prompt string
        """
        return get_workflow_prompt(workflow_type.value)
    
    def get_examples(self, workflow_type: WorkflowType) -> str:
        """
        Get examples for a specific workflow type.
        
        Args:
            workflow_type: Type of workflow
            
        Returns:
            Examples string for the workflow type
        """
        if workflow_type == WorkflowType.QUERY:
            return get_query_examples()
        elif workflow_type == WorkflowType.INCIDENT:
            return get_incident_examples()
        elif workflow_type == WorkflowType.ACTION:
            return get_action_examples()
        else:
            return get_categorization_examples()
    
    def get_data_collection_prompt(
        self, 
        workflow_type: WorkflowType, 
        user_input: str,
        available_tools: Optional[Dict[str, Any]] = None
    ) -> str:
        """
        Get a data collection prompt for a specific workflow.
        
        Args:
            workflow_type: Type of workflow
            user_input: User's input/request
            available_tools: Available tools information
            
        Returns:
            Formatted data collection prompt
        """
        return self.get_prompt(
            workflow_type, 
            PromptType.DATA_COLLECTION,
            user_input=user_input,
            available_tools=available_tools or {}
        )
    
    def get_evaluation_prompt(
        self, 
        workflow_type: WorkflowType, 
        user_input: str,
        collected_data: Dict[str, Any]
    ) -> str:
        """
        Get a data evaluation prompt for a specific workflow.
        
        Args:
            workflow_type: Type of workflow
            user_input: User's input/request
            collected_data: Summary of collected data
            
        Returns:
            Formatted evaluation prompt
        """
        return self.get_prompt(
            workflow_type,
            PromptType.DATA_EVALUATION,
            user_input=user_input,
            collected_data=collected_data
        )
    
    def get_output_formatting_prompt(
        self, 
        workflow_type: WorkflowType, 
        user_input: str,
        collected_data: Dict[str, Any],
        **kwargs
    ) -> str:
        """
        Get an output formatting prompt for a specific workflow.
        
        Args:
            workflow_type: Type of workflow
            user_input: User's input/request
            collected_data: Collected data for formatting
            **kwargs: Additional formatting parameters
            
        Returns:
            Formatted output prompt
        """
        return self.get_prompt(
            workflow_type,
            PromptType.OUTPUT_FORMATTING,
            user_input=user_input,
            collected_data=collected_data,
            **kwargs
        )
    
    def get_tool_selection_prompt(
        self, 
        workflow_type: WorkflowType, 
        user_input: str,
        available_tools: Dict[str, Any]
    ) -> str:
        """
        Get a tool selection prompt for a specific workflow.
        
        Args:
            workflow_type: Type of workflow
            user_input: User's input/request
            available_tools: Available tools information
            
        Returns:
            Formatted tool selection prompt
        """
        try:
            return self.get_prompt(
                workflow_type,
                PromptType.TOOL_SELECTION,
                user_input=user_input,
                available_tools=available_tools
            )
        except ValueError:
            # Fallback to generic tool selection if workflow-specific not available
            return self._get_generic_tool_selection_prompt(user_input, available_tools)
    
    def _get_generic_tool_selection_prompt(
        self, 
        user_input: str, 
        available_tools: Dict[str, Any]
    ) -> str:
        """Get a generic tool selection prompt."""
        return f"""
You are an expert SRE selecting appropriate monitoring tools for a user request.

User Request: {user_input}
Available Tools: {available_tools}

Select the most appropriate tools based on:
1. Relevance to the user's request
2. Data quality and reliability
3. Expected response time
4. Tool capabilities and limitations

Respond with JSON containing:
- selected_tools: list of tools to use
- tool_parameters: parameters for each tool
- reasoning: explanation for selection
"""
    
    def get_analysis_prompt(
        self, 
        workflow_type: WorkflowType, 
        user_input: str,
        collected_data: Dict[str, Any],
        analysis_scope: Optional[str] = None
    ) -> str:
        """
        Get an analysis prompt for ACTION workflows.
        
        Args:
            workflow_type: Type of workflow (should be ACTION)
            user_input: User's input/request
            collected_data: Data to analyze
            analysis_scope: Scope of analysis
            
        Returns:
            Formatted analysis prompt
        """
        if workflow_type != WorkflowType.ACTION:
            raise ValueError("Analysis prompts are only available for ACTION workflows")
        
        return self.get_prompt(
            workflow_type,
            PromptType.ANALYSIS,
            user_input=user_input,
            collected_data=collected_data,
            analysis_scope=analysis_scope or "comprehensive"
        )
    
    def get_investigation_prompt(
        self, 
        workflow_type: WorkflowType, 
        user_input: str,
        collected_data: Dict[str, Any]
    ) -> str:
        """
        Get an investigation prompt for INCIDENT workflows.
        
        Args:
            workflow_type: Type of workflow (should be INCIDENT)
            user_input: User's input/request
            collected_data: Evidence for investigation
            
        Returns:
            Formatted investigation prompt
        """
        if workflow_type != WorkflowType.INCIDENT:
            raise ValueError("Investigation prompts are only available for INCIDENT workflows")
        
        return self.get_prompt(
            workflow_type,
            PromptType.INVESTIGATION,
            user_input=user_input,
            collected_data=collected_data
        )
    
    def validate_prompt_parameters(
        self, 
        workflow_type: WorkflowType, 
        prompt_type: PromptType, 
        **kwargs
    ) -> bool:
        """
        Validate that required parameters are provided for a prompt.
        
        Args:
            workflow_type: Type of workflow
            prompt_type: Type of prompt
            **kwargs: Parameters to validate
            
        Returns:
            True if parameters are valid
        """
        required_params = {
            PromptType.DATA_COLLECTION: ["user_input"],
            PromptType.DATA_EVALUATION: ["user_input", "collected_data"],
            PromptType.OUTPUT_FORMATTING: ["user_input", "collected_data"],
            PromptType.TOOL_SELECTION: ["user_input", "available_tools"],
            PromptType.ANALYSIS: ["user_input", "collected_data"],
            PromptType.INVESTIGATION: ["user_input", "collected_data"]
        }
        
        required = required_params.get(prompt_type, [])
        missing = [param for param in required if param not in kwargs]
        
        if missing:
            logger.warning(f"Missing required parameters for {prompt_type}: {missing}")
            return False
        
        return True
    
    def get_prompt_summary(self) -> Dict[str, Any]:
        """
        Get a summary of available prompts.
        
        Returns:
            Dictionary with prompt availability summary
        """
        return {
            "workflow_types": [wt.value for wt in WorkflowType],
            "prompt_types": [pt.value for pt in PromptType],
            "examples_available": list(self.workflow_examples.keys()),
            "cache_size": len(self.prompt_cache)
        }


# Global instance
prompt_manager = PromptManager()
