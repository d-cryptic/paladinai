"""
Loki Node for PaladinAI LangGraph Workflow.

This module implements the Loki node that collects log data from Loki
based on requests from query, action, or incident nodes.
"""

import logging
import json
from typing import Dict, Any
from langfuse import observe

from ...state import WorkflowState, update_state_node
from llm.openai import openai
from prompts.workflows.loki import (
    get_loki_query_generation_prompt,
    get_loki_analysis_prompt,
    get_loki_tool_decision_prompt,
    get_loki_processing_prompt,
    get_loki_action_analysis_prompt,
    get_loki_incident_analysis_prompt
)

logger = logging.getLogger(__name__)


class LokiNode:
    """
    Node responsible for collecting log data from Loki.
    
    This node receives requests from query/action/incident nodes,
    generates appropriate LogQL queries using AI, collects log data,
    and routes back to the originating node.
    """
    
    def __init__(self):
        """Initialize the Loki node."""
        self.node_name = "loki"
    
    async def _analyze_loki_requirements(self, user_input: str) -> Dict[str, Any]:
        """
        Use AI to analyze what Loki data is needed.
        
        Args:
            user_input: The user's request
            
        Returns:
            Analysis results including data requirements
        """
        prompt = get_loki_analysis_prompt(user_input)
        
        response = await openai.chat_completion(
            user_message=prompt,
            system_prompt="You are an expert in log analysis and Loki queries. Analyze the user's request to determine log data requirements.",
            temperature=0.1
        )
        
        if not response["success"]:
            raise Exception(f"Failed to analyze Loki requirements: {response.get('error')}")
        
        try:
            return json.loads(response["content"])
        except json.JSONDecodeError:
            logger.error("Failed to parse Loki analysis response")
            return {
                "needs_logs": True,
                "needs_metrics": False,
                "time_range": "1h",
                "labels_needed": [],
                "analysis_type": "general"
            }
    
    async def _generate_logql_query(self, user_input: str, context: Dict[str, Any]) -> Dict[str, Any]:
        """
        Use AI to generate LogQL query.
        
        Args:
            user_input: The user's request
            context: Workflow context
            
        Returns:
            Generated query information
        """
        prompt = get_loki_query_generation_prompt(user_input, context)
        
        response = await openai.chat_completion(
            user_message=prompt,
            system_prompt="You are an expert in LogQL and Loki queries. Generate appropriate LogQL queries based on user requests.",
            temperature=0.1
        )
        
        if not response["success"]:
            raise Exception(f"Failed to generate LogQL query: {response.get('error')}")
        
        try:
            return json.loads(response["content"])
        except json.JSONDecodeError:
            logger.error("Failed to parse query generation response")
            # Fallback to a basic query that filters empty logs
            return {
                "query": '{job=~".+"} |~ ".+"',
                "description": "Get all non-empty logs",
                "is_metric": False,
                "labels": {},
                "filters": ["non-empty"]
            }
    
    async def _decide_tools_and_collect(self, user_input: str, query_info: Dict[str, Any], analysis: Dict[str, Any]) -> Dict[str, Any]:
        """
        Use AI to decide which tools to use and collect data.
        
        Args:
            user_input: The user's request
            query_info: Generated query information
            analysis: Requirements analysis
            
        Returns:
            Collected data from Loki
        """
        # Import Loki tools
        from tools.loki import loki, LokiQueryRequest, LokiRangeQueryRequest
        import time
        from datetime import datetime, timedelta
        
        def convert_time_to_ns(time_param: str) -> str:
            """Convert time parameter to Unix nanoseconds."""
            now = datetime.now()
            
            if time_param == "now":
                return str(int(now.timestamp() * 1e9))
            elif time_param.startswith("now-"):
                # Parse duration like "now-1h", "now-24h", etc.
                duration_str = time_param[4:]  # Remove "now-"
                if duration_str.endswith('h'):
                    hours = int(duration_str[:-1])
                    target_time = now - timedelta(hours=hours)
                elif duration_str.endswith('m'):
                    minutes = int(duration_str[:-1])
                    target_time = now - timedelta(minutes=minutes)
                elif duration_str.endswith('d'):
                    days = int(duration_str[:-1])
                    target_time = now - timedelta(days=days)
                else:
                    # Default to 1 hour ago
                    target_time = now - timedelta(hours=1)
                return str(int(target_time.timestamp() * 1e9))
            else:
                # Assume it's already a timestamp
                return time_param
        
        # Get available tools
        available_tools = [
            "loki.query",
            "loki.query_range",
            "loki.get_labels",
            "loki.get_label_values",
            "loki.get_series",
            "loki.query_metrics"
        ]
        
        # Use AI to decide which tools to use
        prompt = get_loki_tool_decision_prompt(user_input, query_info, available_tools)
        
        response = await openai.chat_completion(
            user_message=prompt,
            system_prompt="You are an expert in Loki and LogQL. Decide which tools to use and generate appropriate queries with parameters.",
            temperature=0.1
        )
        
        if not response["success"]:
            raise Exception(f"Failed to decide tools: {response.get('error')}")
        
        try:
            tool_decision = json.loads(response["content"])
        except json.JSONDecodeError:
            logger.error("Failed to parse tool decision response")
            # Fallback to basic query_range
            tool_decision = {
                "tool_calls": [
                    {
                        "tool": "loki.query_range",
                        "parameters": {
                            "query": query_info.get("query", '{job=~".+"}'),
                            "start": "now-1h",
                            "end": "now",
                            "limit": 100
                        }
                    }
                ]
            }
        
        # Collect all results
        collected_data: Dict[str, Any] = {
            "logs": [],
            "metrics": [],
            "labels": {},
            "series": [],
            "metadata": {
                "query_used": query_info.get("query"),
                "tools_used": [],
                "time_range": analysis.get("time_range", "1h")
            }
        }
        
        # Execute each tool call
        for tool_call in tool_decision.get("tool_calls", []):
            tool_name = tool_call.get("tool")
            params = tool_call.get("parameters", {})
            
            logger.info(f"Executing Loki tool: {tool_name} with params: {params}")
            
            try:
                if tool_name == "loki.query":
                    # Handle time parameter if present
                    time_param = params.get("time")
                    if time_param and time_param == "now":
                        time_param = str(int(datetime.now().timestamp() * 1e9))
                    
                    request = LokiQueryRequest(
                        query=params.get("query"),
                        time=time_param,
                        limit=params.get("limit", 100),
                        direction=params.get("direction", "backward")
                    )
                    result = await loki.query(request)
                    
                elif tool_name == "loki.query_range":
                    # Convert relative time to Unix nanoseconds
                    start_param = params.get("start", "now-1h")
                    end_param = params.get("end", "now")
                    
                    start_ns = convert_time_to_ns(start_param)
                    end_ns = convert_time_to_ns(end_param)
                    
                    logger.info(f"Converted time range: start={start_param} -> {start_ns}, end={end_param} -> {end_ns}")
                    
                    request = LokiRangeQueryRequest(
                        query=params.get("query"),
                        start=start_ns,
                        end=end_ns,
                        limit=params.get("limit", 100),
                        direction=params.get("direction", "backward")
                    )
                    result = await loki.query_range(request)
                    
                elif tool_name == "loki.get_labels":
                    # Convert time parameters if provided
                    start = params.get("start")
                    end = params.get("end")
                    if start:
                        start = convert_time_to_ns(start)
                    if end:
                        end = convert_time_to_ns(end)
                    
                    result = await loki.get_labels(
                        start=start,
                        end=end
                    )
                    if result.success and result.labels:
                        collected_data["labels"]["available_labels"] = result.labels
                    
                elif tool_name == "loki.get_label_values":
                    # Convert time parameters if provided
                    start = params.get("start")
                    end = params.get("end")
                    if start:
                        start = convert_time_to_ns(start)
                    if end:
                        end = convert_time_to_ns(end)
                    
                    result = await loki.get_label_values(
                        label_name=params.get("label"),
                        start=start,
                        end=end
                    )
                    if result.success and result.values:
                        label_name = params.get("label")
                        collected_data["labels"][label_name] = result.values
                    
                elif tool_name == "loki.query_metrics":
                    # Convert relative time to Unix nanoseconds (same as query_range)
                    start_param = params.get("start", "now-1h")
                    end_param = params.get("end", "now")
                    
                    start_ns = convert_time_to_ns(start_param)
                    end_ns = convert_time_to_ns(end_param)
                    
                    logger.info(f"query_metrics: Converted time range: start={start_param} -> {start_ns}, end={end_param} -> {end_ns}")
                    
                    request = LokiRangeQueryRequest(
                        query=params.get("query"),
                        start=start_ns,
                        end=end_ns,
                        step=params.get("step")
                    )
                    result = await loki.query_range(request)
                else:
                    logger.warning(f"Unknown tool: {tool_name}")
                    continue
                
                # Process successful results
                if hasattr(result, 'success') and result.success:
                    logger.info(f"Tool {tool_name} executed successfully")
                    
                    # Handle different result types
                    if tool_name in ["loki.query", "loki.query_range"]:
                        # First check if we have a data attribute with the Loki response
                        if hasattr(result, 'data') and result.data:
                            data = result.data
                            if 'result' in data:
                                streams = data['result']
                                logger.info(f"Found {len(streams)} streams in Loki response")
                                
                                for stream in streams:
                                    stream_labels = stream.get('stream', {})
                                    values = stream.get('values', [])
                                        
                                    empty_count = sum(1 for v in values if isinstance(v, list) and len(v) >= 2 and not v[1].strip())
                                    logger.info(f"Processing stream with {len(values)} log entries ({empty_count} empty)")
                                    if empty_count == len(values):
                                        logger.warning(f"All logs in this stream are empty. Stream labels: {stream_labels}")
                                        
                                    for value in values:
                                        if value[1] and value[1].strip():
                                            collected_data["logs"].append({
                                                "timestamp": value[0],
                                                "line": value[1],
                                                "labels": stream_labels
                                            })
                        
                        # Check if result is in the model's result attribute
                        elif hasattr(result, 'result') and result.result:
                            logger.info(f"Processing result attribute")
                            # The result might be the raw Loki response
                            for item in result.result:
                                if 'stream' in item and 'values' in item:
                                    stream_labels = item.get('stream', {})
                                    values = item.get('values', [])
                                    for value in values:
                                        if value[1] and value[1].strip():
                                            collected_data["logs"].append({
                                                "timestamp": value[0],
                                                "line": value[1],
                                                "labels": stream_labels
                                            })
                        else:
                            logger.warning(f"No data found in result for {tool_name}")
                    
                    elif tool_name == "loki.query_metrics" and hasattr(result, 'result') and result.result:
                        collected_data["metrics"].extend(result.result)
                    
                    collected_data["metadata"]["tools_used"].append(tool_name)
                else:
                    logger.error(f"Tool {tool_name} failed: {getattr(result, 'error', 'Unknown error')}")
                    
            except Exception as e:
                logger.error(f"Error executing {tool_name}: {str(e)}")
                continue
        
        # Log collection summary
        logger.info(f"Collected {len(collected_data['logs'])} non-empty logs and {len(collected_data['metrics'])} metrics")
        if len(collected_data['logs']) == 0:
            logger.warning("No non-empty logs found. Consider adjusting the query or time range.")
        
        return {
            "success": True,
            "data": collected_data
        }
    
    async def _process_logs_for_workflow(self, logs_data: Dict[str, Any], state: WorkflowState, originating_node: str) -> Dict[str, Any]:
        """
        Process logs based on the originating workflow node.
        
        Args:
            logs_data: Collected log data
            state: Current workflow state
            originating_node: The node that requested the logs
            
        Returns:
            Processed log data
        """
        # Get context based on originating node
        if originating_node == "query":
            # Simple processing for query node
            prompt = get_loki_processing_prompt(logs_data, state.user_input)
            system_prompt = "You are an expert log analyst. Provide insights from the collected logs."
            
        elif originating_node == "action":
            # Action-oriented analysis
            action_context = state.metadata.get("action_context", {})
            prompt = get_loki_action_analysis_prompt(logs_data, state.user_input, action_context)
            system_prompt = "You are an expert SRE analyzing logs for action planning. Provide actionable insights."
            
        elif originating_node == "incident":
            # Incident investigation
            incident_context = state.metadata.get("incident_context", {})
            prompt = get_loki_incident_analysis_prompt(logs_data, state.user_input, incident_context)
            system_prompt = "You are an expert incident investigator. Perform forensic analysis of the logs."
            
        else:
            # Fallback to general processing
            prompt = get_loki_processing_prompt(logs_data, state.user_input)
            system_prompt = "You are an expert log analyst. Provide insights from the collected logs."
        
        response = await openai.chat_completion(
            user_message=prompt,
            system_prompt=system_prompt,
            temperature=0.3
        )
        
        if not response["success"]:
            logger.error(f"Failed to process logs: {response.get('error')}")
            return logs_data  # Return raw data if processing fails
        
        try:
            processed_result = json.loads(response["content"])
            # Merge with original data
            logs_data.update(processed_result)
            return logs_data
        except json.JSONDecodeError:
            logger.error("Failed to parse processing response")
            return logs_data
    
    @observe(name="loki_node")
    async def execute(self, state: WorkflowState) -> WorkflowState:
        """
        Execute Loki data collection using AI-driven decisions.
        
        Args:
            state: Current workflow state containing context from originating node
            
        Returns:
            Updated workflow state with collected log data
        """
        logger.info(f"Executing Loki node for collecting log data")
        
        # Update state to indicate Loki node execution
        state = update_state_node(state, self.node_name)
        
        try:
            # Get the originating node context
            originating_node = state.metadata.get("originating_node")
            logger.info(f"Loki node called from: {originating_node}")
            
            # Get context based on originating node
            if originating_node == "query":
                context = state.metadata.get("query_context", {})
            elif originating_node == "action":
                context = state.metadata.get("action_context", {})
            elif originating_node == "incident":
                context = state.metadata.get("incident_context", {})
            else:
                context = {}
            
            workflow_type = context.get("workflow_type", originating_node)
            
            # Step 1: Analyze requirements using AI
            logger.info("Analyzing Loki data requirements")
            analysis = await self._analyze_loki_requirements(state.user_input)
            
            # Step 2: Generate LogQL query using AI
            logger.info("Generating LogQL query using AI")
            query_info = await self._generate_logql_query(
                user_input=state.user_input,
                context={"workflow_type": workflow_type, "analysis": analysis}
            )
            
            # Step 3: Decide tools and collect data using AI
            logger.info("Collecting log data from Loki")
            collected_data = await self._decide_tools_and_collect(
                user_input=state.user_input,
                query_info=query_info,
                analysis=analysis
            )
            
            if not collected_data.get("success"):
                raise Exception(f"Data collection failed: {collected_data.get('error')}")
            
            # Step 4: Process collected data based on workflow type
            logger.info("Processing collected log data")
            logs_data = collected_data.get("data", {})
            
            # Log what we collected
            logger.info(f"Collected data summary: {len(logs_data.get('logs', []))} logs, "
                       f"{len(logs_data.get('metrics', []))} metrics, "
                       f"labels: {logs_data.get('labels', {})}")
            
            processed_data = await self._process_logs_for_workflow(
                logs_data=logs_data,
                state=state,
                originating_node=originating_node or "unknown"
            )
            
            # Store the processed data in metadata for the originating node
            state.metadata["loki_data"] = processed_data
            state.metadata["loki_collection_complete"] = True
            
            # Determine next node based on originating node
            if originating_node == "query":
                state.metadata["next_node"] = "query_loki_return"
            elif originating_node == "action":
                state.metadata["next_node"] = "action_loki_return"
            elif originating_node == "incident":
                state.metadata["next_node"] = "incident_loki_return"
            else:
                state.metadata["next_node"] = "error_handler"
                state.error_message = f"Unknown originating node: {originating_node}"
            
            logger.info(f"Loki data collection successful, returning to {originating_node}")
            
            # Store execution info with collected data
            logs_count = len(processed_data.get("logs", []))
            state.metadata[f"{self.node_name}_execution"] = {
                "timestamp": state.metadata.get("current_timestamp"),
                "status": "completed",
                "originating_node": originating_node,
                "logs_collected": logs_count,
                "query_used": query_info.get("query", ""),
                "tools_used": collected_data.get("data", {}).get("metadata", {}).get("tools_used", [])
            }
            
        except Exception as e:
            logger.error(f"Error in Loki node: {str(e)}")
            state.error_message = f"Loki data collection failed: {str(e)}"
            state.metadata["next_node"] = "error_handler"
            
            # Store error execution info
            state.metadata[f"{self.node_name}_execution"] = {
                "timestamp": state.metadata.get("current_timestamp"),
                "status": "error",
                "error": str(e),
                "originating_node": state.metadata.get("originating_node", "unknown")
            }
        
        return state
    
    def get_next_node(self, state: WorkflowState) -> str:
        """
        Determine the next node based on Loki processing results.
        
        Args:
            state: Current workflow state
            
        Returns:
            Name of the next node to execute
        """
        if state.error_message:
            return "error_handler"
        
        # Route based on the determined next node
        return state.metadata.get("next_node", "error_handler")


# Create singleton instance
loki_node = LokiNode()