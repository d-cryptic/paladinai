"""
Prometheus Node for PaladinAI LangGraph Workflow.

This module implements the prometheus node that handles metrics data collection
using tools from server/tools/prometheus/ and processes data according to user requirements.
"""

import json
import logging
from typing import Dict, Any, Optional, List
from datetime import datetime, timedelta
from langfuse import observe
from prompts.data_collection.incident_prompts import get_incident_prompt
from prompts.workflows.planning import get_planning_prompt
from prompts.workflows.processing import get_action_query_processing_prompt, get_incident_processing_prompt
from prompts.workflows.tool_decision import get_tool_decision_prompt

from ...state import WorkflowState, update_state_node
from llm.openai import openai
from tools.prometheus import prometheus, PrometheusQueryRequest, PrometheusRangeQueryRequest

logger = logging.getLogger(__name__)


class PrometheusNode:
    """
    Node responsible for collecting and processing metrics data from Prometheus.
    
    This node receives requests from query/action/incident nodes, collects relevant
    metrics data, and returns processed results back to the originating node.
    """
    
    def __init__(self):
        """Initialize the prometheus node."""
        self.node_name = "prometheus"
    
    @observe(name="prometheus_node")
    async def execute(self, state: WorkflowState) -> WorkflowState:
        """
        Execute prometheus data collection and processing with simplified single-pass approach.

        Args:
            state: Current workflow state containing context from originating node

        Returns:
            Updated workflow state with prometheus data and routing back to originating node
        """
        logger.info(f"Executing prometheus node for {state.metadata.get('originating_node', 'unknown')} workflow")

        # Update state to indicate prometheus node execution
        state = update_state_node(state, self.node_name)

        try:
            # Get context from originating node
            originating_node = state.metadata.get("originating_node", "unknown")
            context_key = f"{originating_node}_context"
            context = state.metadata.get(context_key, {})

            # Initial data collection
            collected_data = await self._collect_metrics_data_ai_driven(
                state.user_input,
                context,
                str(originating_node)
            )
            

            if not collected_data.get("success", False):
                state.error_message = f"Failed to collect metrics data: {collected_data.get('error')}"
                state.metadata["next_node"] = "error_handler"
                return state

            # Simplified data collection - single pass without validation loop
            successful_metrics = [m for m in collected_data["data"].get("metrics", []) if "error" not in m]
            logger.info(f"Collected {len(successful_metrics)} successful metrics out of {len(collected_data['data'].get('metrics', []))} total")

            # Process and format the collected data
            processed_data = await self._process_collected_data(
                collected_data["data"],
                state.user_input,
                context,
                str(originating_node)
            )

            # Store processed data for originating node
            state.metadata["prometheus_data"] = processed_data
            state.metadata["prometheus_collection_complete"] = True

            # Route back to originating node for final processing
            state.metadata["next_node"] = f"{originating_node}_prometheus_return"

            # Store detailed execution info in metadata
            state.metadata[f"{self.node_name}_execution"] = {
                "timestamp": state.metadata.get("current_timestamp"),
                "status": "completed",
                "metrics_collected": len(processed_data.get("metrics", [])),
                "originating_node": originating_node,
                "data_points": processed_data.get("total_data_points", 0),
                "validation_iterations": 0  # No validation loop anymore
            }

        except Exception as e:
            logger.error(f"Error in prometheus node: {str(e)}")
            state.error_message = f"Prometheus data collection failed: {str(e)}"
            state.metadata["next_node"] = "error_handler"

            # Store error execution info in metadata
            state.metadata[f"{self.node_name}_execution"] = {
                "timestamp": state.metadata.get("current_timestamp"),
                "status": "error",
                "error": str(e)
            }

        return state

    def _generate_prometheus_timestamps(self, duration_hours: int = 1) -> Dict[str, str]:
        """
        Generate proper Prometheus timestamps in Unix epoch format.

        Args:
            duration_hours: How many hours back to look (default: 1 hour)

        Returns:
            Dictionary with start, end timestamps and step
        """
        now = datetime.now()
        start_time = now - timedelta(hours=duration_hours)

        # Convert to Unix timestamps (seconds since epoch)
        end_timestamp = str(int(now.timestamp()))
        start_timestamp = str(int(start_time.timestamp()))

        # Calculate appropriate step based on duration
        if duration_hours <= 1:
            step = "1m"  # 1 minute steps for 1 hour or less
        elif duration_hours <= 6:
            step = "5m"  # 5 minute steps for up to 6 hours
        elif duration_hours <= 24:
            step = "15m"  # 15 minute steps for up to 24 hours
        else:
            step = "1h"  # 1 hour steps for longer periods

        return {
            "start": start_timestamp,
            "end": end_timestamp,
            "step": step
        } 

    def _serialize_raw_data(self, raw_data: Dict[str, Any]) -> Dict[str, Any]:
        """
        Serialize raw data containing Pydantic objects to JSON-compatible format.

        Args:
            raw_data: Raw data that may contain Pydantic objects

        Returns:
            JSON-serializable dictionary
        """
        try:
            import json
            from pydantic import BaseModel

            def serialize_item(item):
                """Recursively serialize items, handling Pydantic models."""
                if isinstance(item, BaseModel):
                    return item.model_dump()  # Convert Pydantic model to dict
                elif isinstance(item, dict):
                    return {k: serialize_item(v) for k, v in item.items()}
                elif isinstance(item, list):
                    return [serialize_item(i) for i in item]
                else:
                    return item

            serialized_data = serialize_item(raw_data)

            # Test JSON serialization to ensure it works
            json.dumps(serialized_data)

            return serialized_data

        except Exception as e:
            logger.warning(f"Failed to serialize raw data: {str(e)}")
            # Return a simplified version without problematic objects
            return {
                "metrics_count": len(raw_data.get("metrics", [])),
                "collection_timestamp": raw_data.get("collection_timestamp"),
                "total_tool_calls": raw_data.get("total_tool_calls", 0),
                "successful_calls": raw_data.get("successful_calls", 0),
                "serialization_error": str(e)
            }

    async def _generate_promql_queries(
        self,
        user_input: str,
        context: Dict[str, Any],
        workflow_type: str
    ) -> Dict[str, Any]:
        """
        Generate specific PromQL queries based on user input and context.
        
        Args:
            user_input: The user's original request
            context: Context from the originating node
            workflow_type: Type of workflow (query/action/incident)
            
        Returns:
            Dictionary containing the generated PromQL queries and execution plan
        """
        try:
            # First try pattern matching for common queries
            pattern_queries = await self._try_pattern_matching(user_input)

            if pattern_queries:
                logger.info(f"Found {len(pattern_queries)} pattern-matched queries for request")
                # Generate proper timestamps for pattern queries
                timestamps = self._generate_prometheus_timestamps(1)  # 1 hour lookback
                return {
                    "success": True,
                    "plan": {
                        "queries": [q["query"] for q in pattern_queries],
                        "query_types": [q["type"] for q in pattern_queries],
                        "query_descriptions": [q["description"] for q in pattern_queries],
                        "time_ranges": [timestamps for _ in pattern_queries],
                        "collection_strategy": "targeted",
                        "reasoning": "Matched common monitoring patterns",
                        "source": "pattern_matching"
                    }
                }

            # Fall back to OpenAI for complex queries
            logger.info("No pattern matches found, using OpenAI to generate PromQL queries")

            # Generate proper timestamps for the prompt
            timestamps = self._generate_prometheus_timestamps(1)  # 1 hour lookback

            planning_prompt = get_planning_prompt(
                user_input=user_input,
                context=context,
                workflow_type=workflow_type,
                timestamps=timestamps
            )
            
            response = await openai.chat_completion(
                user_message=planning_prompt,
                system_prompt="You are an expert SRE and PromQL specialist. Generate ready-to-execute PromQL queries for Prometheus metrics collection. Always include the word 'json' in your response when using JSON format.",
                temperature=0.1
            )

            if not response["success"]:
                raise Exception(response.get("error", "OpenAI request failed"))

            plan = json.loads(response["content"])
            
            return {
                "success": True,
                "plan": plan
            }
            
        except Exception as e:
            logger.error(f"Error creating metrics collection plan: {str(e)}")
            return {
                "success": False,
                "error": str(e)
            }

    def _get_common_promql_patterns(self) -> Dict[str, Dict[str, str]]:
        """
        Get common PromQL query patterns for typical monitoring requests.

        Returns:
            Dictionary of common patterns with their PromQL queries
        """
        return {
            "cpu_usage": {
                "query": "100 - (avg(rate(node_cpu_seconds_total{mode=\"idle\"}[5m])) * 100)",
                "description": "Current CPU usage percentage",
                "type": "instant"
            },
            "memory_usage": {
                "query": "(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100",
                "description": "Current memory usage percentage",
                "type": "instant"
            },
            "disk_usage": {
                "query": "(1 - (node_filesystem_avail_bytes{fstype!=\"tmpfs\"} / node_filesystem_size_bytes{fstype!=\"tmpfs\"})) * 100",
                "description": "Current disk usage percentage",
                "type": "instant"
            },
            "network_traffic_in": {
                "query": "rate(node_network_receive_bytes_total[5m])",
                "description": "Network traffic incoming rate",
                "type": "instant"
            },
            "network_traffic_out": {
                "query": "rate(node_network_transmit_bytes_total[5m])",
                "description": "Network traffic outgoing rate",
                "type": "instant"
            },
            "service_uptime": {
                "query": "up",
                "description": "Service availability status",
                "type": "instant"
            },
            "http_requests_rate": {
                "query": "rate(http_requests_total[5m])",
                "description": "HTTP requests per second",
                "type": "instant"
            },
            "http_response_time_p95": {
                "query": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))",
                "description": "95th percentile HTTP response time",
                "type": "instant"
            },
            "load_average": {
                "query": "node_load1",
                "description": "1-minute load average",
                "type": "instant"
            }
        }

    async def _try_pattern_matching(self, user_input: str) -> Optional[List[Dict[str, str]]]:
        """
        Try to match user input to common PromQL patterns before using OpenAI.

        Args:
            user_input: User's request

        Returns:
            List of matched queries or None if no patterns match
        """
        user_lower = user_input.lower()
        patterns = self._get_common_promql_patterns()
        matched_queries = []

        # Simple keyword matching for common requests
        if any(word in user_lower for word in ["cpu", "processor"]):
            matched_queries.append(patterns["cpu_usage"])

        if any(word in user_lower for word in ["memory", "ram"]):
            matched_queries.append(patterns["memory_usage"])

        if any(word in user_lower for word in ["disk", "storage", "filesystem"]):
            matched_queries.append(patterns["disk_usage"])

        if any(word in user_lower for word in ["network", "traffic", "bandwidth"]):
            matched_queries.extend([patterns["network_traffic_in"], patterns["network_traffic_out"]])

        if any(word in user_lower for word in ["uptime", "availability", "up", "down", "service"]):
            matched_queries.append(patterns["service_uptime"])

        if any(word in user_lower for word in ["http", "requests", "response time", "latency"]):
            matched_queries.extend([patterns["http_requests_rate"], patterns["http_response_time_p95"]])

        if any(word in user_lower for word in ["load", "load average"]):
            matched_queries.append(patterns["load_average"])

        return matched_queries if matched_queries else None
    
    async def _collect_metrics_data_ai_driven(self, user_input: str, context: Dict[str, Any], workflow_type: str) -> Dict[str, Any]:
        """
        Use OpenAI to decide which Prometheus tools to use and execute them.

        Args:
            user_input: User's original request
            context: Context from originating workflow node
            workflow_type: Type of workflow (query/action/incident)

        Returns:
            Dictionary containing collected metrics data
        """
        try:
            # Generate proper timestamps for examples
            timestamps = self._generate_prometheus_timestamps(1)  # 1 hour lookback

            # Let OpenAI decide which tools to use and how to use them
            tool_decision_prompt = get_tool_decision_prompt(
                user_input=user_input,
                context=context,
                workflow_type=workflow_type,
                timestamps=timestamps
            )

            response = await openai.chat_completion(
                user_message=tool_decision_prompt,
                system_prompt="You are an expert SRE deciding which Prometheus tools to use. Always include the word 'json' in your response when using JSON format.",
                temperature=0.1
            )

            if not response["success"]:
                raise Exception(response.get("error", "OpenAI request failed"))

            tool_decisions = json.loads(response["content"])
            tool_calls = tool_decisions.get("tool_calls", [])

            logger.info(f"OpenAI decided to make {len(tool_calls)} tool calls")

            # Execute the tool calls as decided by OpenAI
            collected_metrics = []

            for i, tool_call in enumerate(tool_calls):
                tool_name = tool_call.get("tool")
                parameters = tool_call.get("parameters", {})
                purpose = tool_call.get("purpose", "Unknown purpose")

                logger.info(f"Executing tool call {i+1}: {tool_name} - {purpose}")

                try:
                    if tool_name == "prometheus.query":
                        request = PrometheusQueryRequest(query=parameters.get("query"))
                        result = await prometheus.query(request)

                    elif tool_name == "prometheus.query_range":
                        # Generate default timestamps if not provided
                        default_timestamps = self._generate_prometheus_timestamps(1)

                        # Ensure all parameters are strings
                        start_param = parameters.get("start", default_timestamps["start"])
                        end_param = parameters.get("end", default_timestamps["end"])
                        step_param = parameters.get("step", default_timestamps["step"])

                        request = PrometheusRangeQueryRequest(
                            query=parameters.get("query"),
                            start=str(start_param),  # Ensure string conversion
                            end=str(end_param),      # Ensure string conversion
                            step=str(step_param)     # Ensure string conversion
                        )
                        result = await prometheus.query_range(request)

                    elif tool_name == "prometheus.get_metadata":
                        metric = parameters.get("metric")
                        result = await prometheus.get_metadata(metric)

                    elif tool_name == "prometheus.get_labels":
                        start = parameters.get("start")
                        end = parameters.get("end")
                        result = await prometheus.get_labels(start, end)

                    elif tool_name == "prometheus.get_label_values":
                        label_name = parameters.get("label_name")
                        start = parameters.get("start")
                        end = parameters.get("end")
                        result = await prometheus.get_label_values(label_name, start, end)

                    elif tool_name == "prometheus.get_targets":
                        state_filter = parameters.get("state")
                        result = await prometheus.get_targets(state_filter)

                    else:
                        logger.warning(f"Unknown tool: {tool_name}")
                        collected_metrics.append({
                            "tool": tool_name,
                            "purpose": purpose,
                            "error": f"Unknown tool: {tool_name}",
                            "timestamp": datetime.now().isoformat()
                        })
                        continue

                    # Store the result
                    if result.success:
                        # Extract the appropriate data based on response type
                        result_data = None
                        if hasattr(result, 'data') and result.data is not None:
                            result_data = result.data
                        elif hasattr(result, 'values') and result.values is not None:
                            result_data = result.values
                        elif hasattr(result, 'labels') and result.labels is not None:
                            result_data = result.labels
                        elif hasattr(result, 'metadata') and result.metadata is not None:
                            result_data = result.metadata
                        elif hasattr(result, 'active_targets') or hasattr(result, 'dropped_targets'):
                            result_data = {
                                "active_targets": getattr(result, 'active_targets', []),
                                "dropped_targets": getattr(result, 'dropped_targets', [])
                            }
                        else:
                            # Fallback: convert the entire result to dict, excluding success/error fields
                            result_data = {k: v for k, v in result.dict().items()
                                         if k not in ['success', 'error'] and v is not None}

                        collected_metrics.append({
                            "tool": tool_name,
                            "parameters": parameters,
                            "purpose": purpose,
                            "result": result_data,
                            "timestamp": datetime.now().isoformat()
                        })
                        logger.info(f"Successfully executed {tool_name}")
                    else:
                        logger.warning(f"Tool call failed: {tool_name} - {result.error}")
                        collected_metrics.append({
                            "tool": tool_name,
                            "parameters": parameters,
                            "purpose": purpose,
                            "error": result.error,
                            "timestamp": datetime.now().isoformat()
                        })

                except Exception as tool_error:
                    logger.error(f"Error executing tool {tool_name}: {str(tool_error)}")
                    collected_metrics.append({
                        "tool": tool_name,
                        "parameters": parameters,
                        "purpose": purpose,
                        "error": str(tool_error),
                        "timestamp": datetime.now().isoformat()
                    })

            return {
                "success": True,
                "data": {
                    "metrics": collected_metrics,
                    "tool_decisions": tool_decisions,
                    "collection_timestamp": datetime.now().isoformat(),
                    "total_tool_calls": len(tool_calls),
                    "successful_calls": len([m for m in collected_metrics if "error" not in m])
                }
            }

        except Exception as e:
            logger.error(f"Error in AI-driven metrics collection: {str(e)}")
            return {
                "success": False,
                "error": str(e)
            }
    
    async def _process_collected_data(
        self, 
        raw_data: Dict[str, Any], 
        user_input: str,
        context: Dict[str, Any],
        workflow_type: str
    ) -> Dict[str, Any]:
        """
        Process and format collected metrics data for the originating workflow.
        
        Args:
            raw_data: Raw data collected from Prometheus
            user_input: Original user request
            context: Context from originating node
            workflow_type: Type of workflow (query/action/incident)
            
        Returns:
            Processed and formatted data
        """
        try:
            # Serialize raw data to handle Pydantic objects before JSON conversion
            serialized_raw_data = self._serialize_raw_data(raw_data)

            # Different processing based on workflow type
            if workflow_type.upper() == "INCIDENT":
                # Detailed analysis for incident workflows
                processing_prompt = get_incident_processing_prompt(
                    user_input,
                    serialized_raw_data,
                    context
                )
            else:
                # Simple data extraction for query/action workflows
                processing_prompt = get_action_query_processing_prompt(
                    user_input,
                    serialized_raw_data,
                    context
                )
            
            response = await openai.chat_completion(
                user_message=processing_prompt,
                system_prompt="You are an expert SRE processing Prometheus metrics data. Always include the word 'json' in your response when using JSON format.",
                temperature=0.3
            )

            if not response["success"]:
                raise Exception(response.get("error", "OpenAI request failed"))

            # Parse JSON response with better error handling
            try:
                processed_result = json.loads(response["content"])
            except json.JSONDecodeError as e:
                logger.error(f"Failed to parse OpenAI response as JSON: {str(e)}")
                logger.error(f"Response content: {response['content'][:500]}...")  # Log first 500 chars
                # Return a fallback response
                processed_result = {
                    "error": "Failed to parse OpenAI response",
                    "analysis_results": "Unable to process the collected data due to JSON parsing error",
                    "data_summary": "Data collection completed but processing failed",
                    "recommendations": ["Please try the request again", "Check system logs for details"]
                }

            # Add metadata about the collection
            processed_result.update({
                "collection_metadata": {
                    "workflow_type": workflow_type,
                    "queries_executed": raw_data.get("total_queries", 0),
                    "successful_queries": raw_data.get("successful_queries", 0),
                    "collection_timestamp": raw_data.get("collection_timestamp"),
                    "data_source": "Prometheus"
                },
                "raw_data": self._serialize_raw_data(raw_data)  # Serialize Pydantic objects for JSON compatibility
            })

            return processed_result
            
        except Exception as e:
            logger.error(f"Error processing collected data: {str(e)}")
            return {
                "error": f"Data processing failed: {str(e)}",
                "raw_data": self._serialize_raw_data(raw_data),  # Ensure serializable
                "timestamp": datetime.now().isoformat()
            }
    
    def get_next_node(self, state: WorkflowState) -> str:
        """
        Determine the next node based on prometheus processing results.
        
        Args:
            state: Current workflow state
            
        Returns:
            Name of the next node to execute (back to originating node)
        """
        if state.error_message:
            return "error_handler"
        
        # Route back to originating node for final processing
        originating_node = state.metadata.get("originating_node")
        if originating_node:
            return f"{originating_node}_prometheus_return"
        
        # Fallback to result node if no originating node specified
        return "result"


# Create singleton instance
prometheus_node = PrometheusNode()
