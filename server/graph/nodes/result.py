"""
Result Node for PaladinAI LangGraph Workflow.

This module implements the result node that formats and returns
the final workflow results to the user.
"""

import logging
import json
from datetime import datetime
from typing import Dict, Any, Optional
from langfuse import observe

from ..state import WorkflowState, update_state_node, finalize_state
from llm.openai import openai
from prompts.workflows.result_formatting import get_formatting_prompt, FORMATTING_SYSTEM_PROMPT
from prompts.workflows.result_guidance import (
    get_workflow_guidance, 
    NO_CATEGORIZATION_ERROR,
    QUERY_SUCCESS_MESSAGE,
    ACTION_SUCCESS_MESSAGE,
    INCIDENT_SUCCESS_MESSAGE,
    CATEGORIZATION_FALLBACK_TEMPLATE
)

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
            
            # Validate if memory instructions were followed
            if state.memory_instructions:
                validation_result = await self._validate_instruction_compliance(state, result)
                if validation_result:
                    result["instruction_compliance"] = validation_result
                    logger.info(f"Memory instruction compliance: {validation_result}")
            
            # Format the entire state data using OpenAI
            formatted_content = await self._format_with_openai(state, result)
            if formatted_content:
                result["formatted_markdown"] = formatted_content
                logger.info(f"Successfully formatted markdown content: {len(formatted_content)} chars")
            else:
                logger.warning("Failed to format markdown content, falling back to standard response")
            
            # Memory extraction removed - memories are only saved explicitly by user
            # This ensures no automatic memory storage during workflow execution
            
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
                "error": NO_CATEGORIZATION_ERROR,
                "session_id": state.session_id,
                "execution_path": state.execution_path
            }
        
        # Get workflow type for result formatting
        workflow_type_value = state.categorization.workflow_type
        if hasattr(workflow_type_value, 'value'):
            workflow_type_value = workflow_type_value.value

        complexity_value = state.categorization.estimated_complexity
        if hasattr(complexity_value, 'value'):
            complexity_value = complexity_value.value

        # Check for workflow-specific results
        query_result = state.metadata.get("query_result")
        action_result = state.metadata.get("action_result")
        incident_result = state.metadata.get("incident_result")

        result = {
            "success": True,
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

        # Add workflow-specific results
        if query_result:
            result["query_result"] = query_result
            result["content"] = QUERY_SUCCESS_MESSAGE
        elif action_result:
            result["action_result"] = action_result
            result["content"] = ACTION_SUCCESS_MESSAGE
        elif incident_result:
            result["incident_result"] = incident_result
            result["content"] = INCIDENT_SUCCESS_MESSAGE
        else:
            # Fallback to categorization-only result
            result["content"] = CATEGORIZATION_FALLBACK_TEMPLATE.format(
                workflow_type=workflow_type_value,
                confidence=state.categorization.confidence,
                reasoning=state.categorization.reasoning,
                suggested_approach=state.categorization.suggested_approach
            )
        
        # Add workflow-specific guidance
        workflow_type_for_guidance = state.categorization.workflow_type
        if hasattr(workflow_type_for_guidance, 'value'):
            workflow_type_for_guidance = workflow_type_for_guidance.value
        result["next_steps"] = get_workflow_guidance(workflow_type_for_guidance)
        
        return result
    
    
    async def _format_with_openai(self, state: WorkflowState, result: Dict[str, Any]) -> Optional[str]:
        """
        Format the complete workflow data using OpenAI into a clean markdown response.
        
        Args:
            state: Current workflow state
            result: Formatted result dictionary
            
        Returns:
            Markdown-formatted string of the results
        """
        try:
            # Get workflow type
            if state.categorization:
                workflow_type = state.categorization.workflow_type
                if hasattr(workflow_type, 'value'):
                    workflow_type = workflow_type.value
            else:
                workflow_type = "UNKNOWN"
            
            # Prepare data for formatting
            format_data = {
                "user_request": state.user_input,
                "workflow_type": workflow_type,
                "execution_time_ms": state.metadata.get("total_execution_time_ms", 0),
                "execution_path": state.execution_path
            }
            
            # Add workflow-specific data
            if workflow_type == "QUERY":
                format_data["query_result"] = state.metadata.get("query_result", {})
            elif workflow_type == "ACTION":
                format_data["action_result"] = state.metadata.get("action_result", {})
                format_data["prometheus_data"] = state.metadata.get("prometheus_data", {})
                format_data["loki_data"] = state.metadata.get("loki_data", {})
                format_data["alertmanager_data"] = state.metadata.get("alertmanager_data", {})
            elif workflow_type == "INCIDENT":
                format_data["incident_result"] = state.metadata.get("incident_result", {})
                format_data["prometheus_data"] = state.metadata.get("prometheus_data", {})
                format_data["loki_data"] = state.metadata.get("loki_data", {})
                format_data["alertmanager_data"] = state.metadata.get("alertmanager_data", {})
                format_data["incident_analysis"] = state.metadata.get("incident_analysis", {})
            
            # Create the prompt based on workflow type
            if workflow_type == "QUERY":
                prompt = get_formatting_prompt(
                    "QUERY",
                    user_request=format_data['user_request'],
                    query_result=json.dumps(format_data.get('query_result', {}), indent=2)
                )
            elif workflow_type == "ACTION":
                # Extract log entries if available
                loki_logs = format_data.get('loki_data', {}).get('logs', [])
                log_entries = []
                for log in loki_logs[:10]:  # Get up to 10 logs
                    log_entries.append({
                        'timestamp': log.get('timestamp', ''),
                        'line': log.get('line', '').strip()
                    })
                
                # Extract alert entries if available
                alert_data = format_data.get('alertmanager_data', {})
                alerts = alert_data.get('alerts', [])
                alert_entries = []
                for alert in alerts[:10]:  # Get up to 10 alerts
                    alert_entries.append({
                        'alertname': alert.get('labels', {}).get('alertname', 'Unknown'),
                        'severity': alert.get('labels', {}).get('severity', 'unknown'),
                        'state': alert.get('status', {}).get('state', 'unknown'),
                        'summary': alert.get('annotations', {}).get('summary', '')
                    })
                
                prompt = get_formatting_prompt(
                    "ACTION",
                    user_request=format_data['user_request'],
                    action_result=json.dumps(format_data.get('action_result', {}), indent=2),
                    metrics_data=json.dumps(format_data.get('prometheus_data', {}).get('processed_metrics', {}), indent=2),
                    logs_data=json.dumps(log_entries, indent=2),
                    total_logs=len(loki_logs),
                    alerts_data=json.dumps(alert_entries, indent=2),
                    total_alerts=len(alerts)
                )
            elif workflow_type == "INCIDENT":
                prompt = get_formatting_prompt(
                    "INCIDENT",
                    user_request=format_data['user_request'],
                    incident_analysis=json.dumps(format_data.get('incident_result', {}), indent=2),
                    metrics_data=json.dumps(format_data.get('prometheus_data', {}), indent=2),
                    logs_data=json.dumps(format_data.get('loki_data', {}), indent=2),
                    alerts_data=json.dumps(format_data.get('alertmanager_data', {}), indent=2)
                )
            else:
                # Fallback formatting
                prompt = get_formatting_prompt(
                    "FALLBACK",
                    format_data=json.dumps(format_data, indent=2)
                )
            
            # Call OpenAI to format the response using markdown formatting
            response = await openai.markdown_formatting(
                user_message=prompt,
                system_prompt=FORMATTING_SYSTEM_PROMPT,
                temperature=0.3
            )

            
            if response["success"] and response.get("content"):
                content = response["content"]
                # Strip the content and check if it's actually meaningful
                if content and len(content.strip()) > 10:
                    return content
                else:
                    logger.warning(f"OpenAI returned empty or invalid content: '{content[:100]}'")
                    return None
            else:
                logger.warning(f"OpenAI formatting failed: {response.get('error', 'Unknown error')}")
                return None
                
        except Exception as e:
            logger.error(f"Error formatting with OpenAI: {str(e)}")
            return None
    
    async def _validate_instruction_compliance(self, state: WorkflowState, result: Dict[str, Any]) -> Dict[str, Any]:
        """
        Validate if memory instructions were followed in the result.
        
        Args:
            state: Current workflow state with memory instructions
            result: The final result to validate
            
        Returns:
            Compliance validation results
        """
        try:
            if not state.memory_instructions:
                return {}
            
            compliance_results = {
                "instructions_provided": len(state.memory_instructions),
                "instructions_followed": [],
                "instructions_missed": [],
                "compliance_score": 0.0
            }
            
            # Check each instruction
            for instruction in state.memory_instructions:
                instruction_lower = instruction.lower()
                
                # Check if the instruction was about fetching additional data
                if "fetch" in instruction_lower or "also" in instruction_lower:
                    # Look for evidence in the result
                    result_str = json.dumps(result).lower()
                    
                    # Check for common instruction patterns
                    followed = False
                    if "memory" in instruction_lower and "memory" in result_str:
                        followed = True
                    elif "disk" in instruction_lower and "disk" in result_str:
                        followed = True
                    elif "network" in instruction_lower and "network" in result_str:
                        followed = True
                    elif "logs" in instruction_lower and ("logs" in result_str or "loki" in result_str):
                        followed = True
                    
                    if followed:
                        compliance_results["instructions_followed"].append(instruction)
                    else:
                        compliance_results["instructions_missed"].append(instruction)
                else:
                    # For other types of instructions, assume followed if result exists
                    if result.get("data") or result.get("analysis"):
                        compliance_results["instructions_followed"].append(instruction)
                    else:
                        compliance_results["instructions_missed"].append(instruction)
            
            # Calculate compliance score
            total = len(state.memory_instructions)
            followed = len(compliance_results["instructions_followed"])
            compliance_results["compliance_score"] = followed / total if total > 0 else 1.0
            
            return compliance_results
            
        except Exception as e:
            logger.error(f"Error validating instruction compliance: {str(e)}")
            return {}
    
    def get_next_node(self, state: WorkflowState) -> str:
        """
        Determine the next node (terminal node).

        Args:
            state: Current workflow state

        Returns:
            Empty string as this is the final node
        """
        return ""  # Result node is terminal


# Create singleton instance
result_node = ResultNode()
