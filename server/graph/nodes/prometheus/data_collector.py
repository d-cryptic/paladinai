"""
Data collection for Prometheus node.

This module handles AI-driven metrics data collection using OpenAI
to decide which Prometheus tools to use and execute them.
"""

import json
import logging
from typing import Dict, Any
from datetime import datetime
from langfuse import observe

from prompts.workflows.tool_decision import get_tool_decision_prompt
from llm.openai import openai
from tools.prometheus import prometheus, PrometheusQueryRequest, PrometheusRangeQueryRequest
from .utils import generate_prometheus_timestamps

logger = logging.getLogger(__name__)


class DataCollector:
    """Handles AI-driven metrics data collection from Prometheus."""

    def __init__(self):
        """Initialize the data collector."""
        pass

    @observe(name="prometheus_data_collection")
    async def collect_metrics_data_ai_driven(
        self, 
        user_input: str, 
        context: Dict[str, Any], 
        workflow_type: str
    ) -> Dict[str, Any]:
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
            timestamps = generate_prometheus_timestamps(1)  # 1 hour lookback

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
                    result = await self._execute_tool_call(tool_name, parameters)

                    # Store the result
                    if result.success:
                        # Extract the appropriate data based on response type
                        result_data = self._extract_result_data(result)

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

    async def _execute_tool_call(self, tool_name: str, parameters: Dict[str, Any]):
        """Execute a specific Prometheus tool call."""
        if tool_name == "prometheus.query":
            request = PrometheusQueryRequest(query=parameters.get("query"))
            return await prometheus.query(request)

        elif tool_name == "prometheus.query_range":
            # Generate default timestamps if not provided
            default_timestamps = generate_prometheus_timestamps(1)

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
            return await prometheus.query_range(request)

        elif tool_name == "prometheus.get_metadata":
            metric = parameters.get("metric")
            return await prometheus.get_metadata(metric)

        elif tool_name == "prometheus.get_labels":
            start = parameters.get("start")
            end = parameters.get("end")
            return await prometheus.get_labels(start, end)

        elif tool_name == "prometheus.get_label_values":
            label_name = parameters.get("label_name", "")
            start = parameters.get("start")
            end = parameters.get("end")
            return await prometheus.get_label_values(label_name, start, end)

        elif tool_name == "prometheus.get_targets":
            state_filter = parameters.get("state")
            return await prometheus.get_targets(state_filter)

        else:
            logger.warning(f"Unknown tool: {tool_name}")
            # Return a mock result object with error
            class MockResult:
                success = False
                error = f"Unknown tool: {tool_name}"
            return MockResult()

    def _extract_result_data(self, result) -> Any:
        """Extract appropriate data from tool result."""
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
        
        return result_data


# Create singleton instance
data_collector = DataCollector()
