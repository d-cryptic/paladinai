"""
PromQL query generation for Prometheus node.

This module handles the generation of PromQL queries using both
pattern matching for common queries and AI-driven query generation.
"""

import json
import logging
from typing import Dict, Any, Optional, List
from langfuse import observe

from prompts.workflows.planning import get_planning_prompt
from prompts.workflows.tool_decision import get_tool_decision_prompt
from llm.openai import openai
from .utils import generate_prometheus_timestamps

logger = logging.getLogger(__name__)


class QueryGenerator:
    """Handles PromQL query generation and pattern matching."""

    def __init__(self):
        """Initialize the query generator."""
        pass

    async def generate_promql_queries(
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
                timestamps = generate_prometheus_timestamps(1)  # 1 hour lookback
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
            timestamps = generate_prometheus_timestamps(1)  # 1 hour lookback

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


# Create singleton instance
query_generator = QueryGenerator()
