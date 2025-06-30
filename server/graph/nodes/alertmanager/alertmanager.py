"""
Alertmanager Node for PaladinAI LangGraph Workflow.

This module implements the Alertmanager node that collects alert data
based on requests from query, action, or incident nodes.
"""

import logging
import json
from typing import Dict, Any, List
from langfuse import observe

from ...state import WorkflowState, update_state_node
from llm.openai import openai
from prompts.workflows.alertmanager import (
    get_alertmanager_analysis_prompt,
    get_alertmanager_tool_decision_prompt,
    get_alertmanager_processing_prompt,
    get_alertmanager_query_prompt,
    get_alertmanager_action_prompt,
    get_alertmanager_incident_prompt
)

logger = logging.getLogger(__name__)


class AlertmanagerNode:
    """
    Node responsible for collecting alert data from Alertmanager.
    
    This node receives requests from query/action/incident nodes,
    analyzes alert requirements, collects alert data,
    and routes back to the originating node.
    """
    
    def __init__(self):
        """Initialize the Alertmanager node."""
        self.node_name = "alertmanager"
    
    async def _analyze_alert_requirements(self, user_input: str) -> Dict[str, Any]:
        """
        Use AI to analyze what Alertmanager data is needed.
        
        Args:
            user_input: The user's request
            
        Returns:
            Analysis results including data requirements
        """
        prompt = get_alertmanager_analysis_prompt(user_input)
        
        response = await openai.chat_completion(
            user_message=prompt,
            system_prompt="You are an expert in alert management and monitoring. Analyze the user's request to determine alert data requirements.",
            temperature=0.1
        )
        
        if not response["success"]:
            raise Exception(f"Failed to analyze Alertmanager requirements: {response.get('error')}")
        
        try:
            return json.loads(response["content"])
        except json.JSONDecodeError:
            logger.error("Failed to parse Alertmanager analysis response")
            return {
                "needs_alerts": True,
                "alert_types": ["active"],
                "severity_filter": None,
                "service_filter": None,
                "time_range": "1h",
                "needs_groups": False,
                "analysis_type": "current_state"
            }
    
    async def _decide_tools(self, user_input: str, analysis: Dict[str, Any]) -> List[Dict[str, Any]]:
        """
        Decide which Alertmanager tools to use.
        
        Args:
            user_input: The user's request
            analysis: Analysis results
            
        Returns:
            List of tool calls to make
        """
        prompt = get_alertmanager_tool_decision_prompt(user_input, analysis)
        
        response = await openai.chat_completion(
            user_message=prompt,
            system_prompt="You are an expert in Alertmanager. Decide which tools to use based on the analysis.",
            temperature=0.1
        )
        
        if not response["success"]:
            raise Exception(f"Failed to decide tools: {response.get('error')}")
        
        try:
            tool_decision = json.loads(response["content"])
            return tool_decision.get("tool_calls", [])
        except json.JSONDecodeError:
            logger.error("Failed to parse tool decision response")
            # Fallback to basic alert query
            return [{
                "tool": "alertmanager.get_alerts",
                "parameters": {"active": True},
                "purpose": "Get current active alerts"
            }]
    
    async def _collect_alert_data(self, tool_calls: List[Dict[str, Any]]) -> Dict[str, Any]:
        """
        Collect data from Alertmanager using the specified tools.
        
        Args:
            tool_calls: List of tool calls to execute
            
        Returns:
            Collected alert data
        """
        # Import Alertmanager tools
        from tools.alertmanager import alertmanager
        
        collected_data: Dict[str, Any] = {
            "alerts": [],
            "silences": [],
            "groups": [],
            "status": {},
            "receivers": [],
            "metadata": {
                "tools_used": [],
                "collection_timestamp": None
            }
        }
        
        # Execute each tool call
        for tool_call in tool_calls:
            tool_name = tool_call.get("tool")
            params = tool_call.get("parameters", {})
            
            logger.info(f"Executing Alertmanager tool: {tool_name} with params: {params}")
            collected_data["metadata"]["tools_used"].append(tool_name)
            
            try:
                if tool_name == "alertmanager.get_alerts":
                    result = await alertmanager.get_alerts(
                        active=params.get("active"),
                        silenced=params.get("silenced"),
                        inhibited=params.get("inhibited"),
                        unprocessed=params.get("unprocessed"),
                        filter=params.get("filter")
                    )
                    if result.success and result.alerts:
                        collected_data["alerts"].extend(result.alerts)
                        logger.info(f"Collected {len(result.alerts)} alerts")
                
                elif tool_name == "alertmanager.get_silences":
                    result = await alertmanager.get_silences(
                        filter=params.get("filter")
                    )
                    if result.success and result.silences:
                        collected_data["silences"].extend(result.silences)
                        logger.info(f"Collected {len(result.silences)} silences")
                
                elif tool_name == "alertmanager.get_alert_groups":
                    result = await alertmanager.get_alert_groups(
                        active=params.get("active"),
                        silenced=params.get("silenced"),
                        inhibited=params.get("inhibited"),
                        filter=params.get("filter"),
                        receiver=params.get("receiver")
                    )
                    if result.success and result.groups:
                        collected_data["groups"].extend(result.groups)
                        logger.info(f"Collected {len(result.groups)} alert groups")
                
                elif tool_name == "alertmanager.get_status":
                    result = await alertmanager.get_status()
                    if result.success:
                        collected_data["status"] = result.dict()
                        logger.info("Collected Alertmanager status")
                
                elif tool_name == "alertmanager.get_receivers":
                    result = await alertmanager.get_receivers()
                    if result.success and result.receivers:
                        collected_data["receivers"] = result.receivers
                        logger.info(f"Collected {len(result.receivers)} receivers")
                
                else:
                    logger.warning(f"Unknown tool: {tool_name}")
                    
            except Exception as e:
                logger.error(f"Error executing {tool_name}: {str(e)}")
                collected_data["metadata"][f"{tool_name}_error"] = str(e)
        
        # Add metadata
        from datetime import datetime
        collected_data["metadata"]["collection_timestamp"] = datetime.now().isoformat()
        
        # Calculate summary statistics
        if collected_data["alerts"]:
            collected_data["metadata"]["alert_summary"] = {
                "total": len(collected_data["alerts"]),
                "active": len([a for a in collected_data["alerts"] if a.get("status", {}).get("state") == "active"]),
                "silenced": len([a for a in collected_data["alerts"] if a.get("status", {}).get("silencedBy")]),
                "critical": len([a for a in collected_data["alerts"] if a.get("labels", {}).get("severity") == "critical"]),
                "warning": len([a for a in collected_data["alerts"] if a.get("labels", {}).get("severity") == "warning"])
            }
        
        return collected_data
    
    async def _process_alert_data(self, alert_data: Dict[str, Any], state: WorkflowState) -> Dict[str, Any]:
        """
        Process collected alert data based on workflow type.
        
        Args:
            alert_data: Collected alert data
            state: Current workflow state
            
        Returns:
            Processed results
        """
        # Determine originating node
        originating_node = state.metadata.get("alertmanager_requested_by", "unknown")
        
        # Select appropriate processing prompt
        if originating_node == "query":
            prompt = get_alertmanager_query_prompt(alert_data, state.user_input)
        elif originating_node == "action":
            action_context = state.metadata.get("action_context", {})
            prompt = get_alertmanager_action_prompt(alert_data, state.user_input, action_context)
        elif originating_node == "incident":
            incident_context = state.metadata.get("incident_context", {})
            prompt = get_alertmanager_incident_prompt(alert_data, state.user_input, incident_context)
        else:
            # Generic processing
            prompt = get_alertmanager_processing_prompt(alert_data, state.user_input)
        
        response = await openai.chat_completion(
            user_message=prompt,
            system_prompt="You are an expert in alert analysis and incident management. Provide insights based on the alert data.",
            temperature=0.3
        )
        
        if not response["success"]:
            raise Exception(f"Failed to process alert data: {response.get('error')}")
        
        try:
            return json.loads(response["content"])
        except json.JSONDecodeError:
            # Return as text if not JSON
            return {"analysis": response["content"]}
    
    @observe(name="alertmanager_node")
    async def execute(self, state: WorkflowState) -> WorkflowState:
        """
        Execute Alertmanager data collection.
        
        Args:
            state: Current workflow state
            
        Returns:
            Updated workflow state with alert data
        """
        logger.info(f"Executing Alertmanager node for session: {state.session_id}")
        
        # Update state to reflect current node
        state = update_state_node(state, self.node_name)
        
        try:
            # Analyze what alert data is needed
            analysis = await self._analyze_alert_requirements(state.user_input)
            
            # Store analysis
            state.metadata["alertmanager_analysis"] = analysis
            
            # Decide which tools to use
            tool_calls = await self._decide_tools(state.user_input, analysis)
            
            # Collect alert data
            alert_data = await self._collect_alert_data(tool_calls)
            
            # Store raw alert data
            state.metadata["alertmanager_data"] = alert_data
            
            # Process the data
            processed_results = await self._process_alert_data(alert_data, state)
            
            # Store processed results
            state.metadata["alertmanager_results"] = processed_results
            
            # Route back to originating node
            originating_node = state.metadata.get("alertmanager_requested_by", "result")
            # Format the return node name to match the routing configuration
            if originating_node in ["query", "action", "incident"]:
                state.metadata["next_node"] = f"{originating_node}_alertmanager_return"
            else:
                state.metadata["next_node"] = originating_node
            state.metadata["alertmanager_collection_complete"] = True
            
            # Clear the flag
            state.metadata["needs_alertmanager"] = False
            
            logger.info(f"Alertmanager data collection complete, routing to {originating_node}")
            
        except Exception as e:
            logger.error(f"Error in Alertmanager node: {str(e)}")
            state.error_message = f"Failed to collect alert data: {str(e)}"
            state.metadata["next_node"] = "error_handler"
            state.metadata["needs_alertmanager"] = False
        
        return state
    
    def get_next_node(self, state: WorkflowState) -> str:
        """
        Determine the next node based on state.
        
        Args:
            state: Current workflow state
            
        Returns:
            Name of the next node
        """
        return state.metadata.get("next_node", "result")


# Create singleton instance
alertmanager_node = AlertmanagerNode()