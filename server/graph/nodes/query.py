"""
Query Node for PaladinAI LangGraph Workflow.

This module implements the query node that handles query-type requests
and routes metrics-related requests to the prometheus node.
"""

import json
import logging
from typing import Dict, Any, Optional
from langfuse import observe
from prompts import get_query_analysis_prompt

from ..state import WorkflowState, NodeResult, update_state_node
from llm.openai import openai
from prompts.data_collection.query_prompts import get_query_prompt

logger = logging.getLogger(__name__)


class QueryNode:
    """
    Node responsible for handling query-type workflow requests.
    
    This node processes user queries, determines if metrics data is needed,
    and routes to prometheus node when necessary for data collection.
    """
    
    def __init__(self):
        """Initialize the query node."""
        self.node_name = "query"

    def _serialize_prometheus_data(self, prometheus_data: Dict[str, Any]) -> Dict[str, Any]:
        """
        Serialize prometheus data containing Pydantic objects to JSON-compatible format.

        Args:
            prometheus_data: Prometheus data that may contain Pydantic objects

        Returns:
            JSON-serializable dictionary
        """
        try:
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

            return serialize_item(prometheus_data)

        except Exception as e:
            logger.warning(f"Failed to serialize prometheus data: {str(e)}")
            # Return a simplified version without problematic objects
            return {
                "serialization_error": str(e),
                "data_summary": "Prometheus data available but not serializable"
            }
    
    @observe(name="query_node")
    async def execute(self, state: WorkflowState) -> WorkflowState:
        """
        Execute query workflow processing.
        
        Args:
            state: Current workflow state containing user input and categorization
            
        Returns:
            Updated workflow state with query processing results
        """
        logger.info(f"Executing query node for input: {state.user_input[:100]}...")
        
        # Update state to indicate query node execution
        state = update_state_node(state, self.node_name)
        
        try:
            # Determine if this query requires metrics data
            needs_metrics = await self._analyze_metrics_requirement(state.user_input)
            
            if needs_metrics:
                logger.info("Query requires metrics data - routing to prometheus node")
                # Set flag to route to prometheus node
                state.metadata["needs_prometheus"] = True
                state.metadata["originating_node"] = "query"
                
                # Store query context for prometheus node
                state.metadata["query_context"] = {
                    "workflow_type": "query",
                    "user_input": state.user_input,
                    "processing_stage": "data_collection"
                }
                
                # Set next node to prometheus
                state.metadata["next_node"] = "prometheus"
                
            else:
                logger.info("Query can be handled without metrics data")
                # Process query directly without metrics
                result = await self._process_non_metrics_query(state.user_input)
                
                if result["success"]:
                    state.metadata["query_result"] = result["data"]
                    state.metadata["next_node"] = "query_output"
                else:
                    state.error_message = result.get("error", "Failed to process query")
                    state.metadata["next_node"] = "error_handler"
            
            # Store detailed execution info in metadata
            state.metadata[f"{self.node_name}_execution"] = {
                "timestamp": state.metadata.get("current_timestamp"),
                "status": "completed" if not state.error_message else "error",
                "needs_metrics": needs_metrics
            }
            
        except Exception as e:
            logger.error(f"Error in query node: {str(e)}")
            state.error_message = f"Query processing failed: {str(e)}"
            state.metadata["next_node"] = "error_handler"
            
            # Store error execution info in metadata
            state.metadata[f"{self.node_name}_execution"] = {
                "timestamp": state.metadata.get("current_timestamp"),
                "status": "error",
                "error": str(e)
            }
        
        return state
    
    async def _analyze_metrics_requirement(self, user_input: str) -> bool:
        """
        Analyze if the user query requires metrics data from Prometheus.
        
        Args:
            user_input: The user's query
            
        Returns:
            Boolean indicating if metrics data is needed
        """
        try:
            # Use OpenAI to determine if metrics are needed
            analysis_prompt = get_query_analysis_prompt(user_input)
            
            response = await openai.chat_completion(
                user_message=analysis_prompt,
                system_prompt="You are an expert SRE analyzing monitoring queries. Always include the word 'json' in your response when using JSON format.",
                temperature=0.1
            )

            if not response["success"]:
                raise Exception(response.get("error", "OpenAI request failed"))

            result = json.loads(response["content"])
            return result.get("needs_metrics", False)
            
        except Exception as e:
            logger.error(f"Error analyzing metrics requirement: {str(e)}")
            # Default to requiring metrics if analysis fails
            return True
    
    async def _process_non_metrics_query(self, user_input: str) -> Dict[str, Any]:
        """
        Process queries that don't require metrics data.
        
        Args:
            user_input: The user's query
            
        Returns:
            Dictionary with processing results
        """
        try:
            # Use query output formatting prompt for non-metrics queries
            prompt = get_query_prompt(
                "output_formatting",
                user_input=user_input,
                collected_data="No metrics data required for this query",
                data_sources="Direct processing"
            )
            
            response = await openai.chat_completion(
                user_message=prompt,
                system_prompt="You are an expert SRE providing direct answers to non-metrics queries. Always include the word 'json' in your response when using JSON format.",
                temperature=0.3
            )

            if not response["success"]:
                raise Exception(response.get("error", "OpenAI request failed"))

            result = json.loads(response["content"])
            
            return {
                "success": True,
                "data": result
            }
            
        except Exception as e:
            logger.error(f"Error processing non-metrics query: {str(e)}")
            return {
                "success": False,
                "error": str(e)
            }
    
    async def process_prometheus_result(self, state: WorkflowState, prometheus_data: Dict[str, Any]) -> WorkflowState:
        """
        Process results returned from prometheus node.

        Args:
            state: Current workflow state
            prometheus_data: Data returned from prometheus node

        Returns:
            Updated workflow state with processed results
        """
        logger.info("Processing prometheus results in query node")

        try:
            # Serialize prometheus data to handle Pydantic objects
            serialized_prometheus_data = self._serialize_prometheus_data(prometheus_data)

            # Use query output formatting prompt to format the final response
            prompt = get_query_prompt(
                "output_formatting",
                user_input=state.user_input,
                collected_data=json.dumps(serialized_prometheus_data, indent=2),
                data_sources="Prometheus"
            )

            response = await openai.chat_completion(
                user_message=prompt,
                system_prompt="You are an expert SRE formatting query responses with metrics data. Always include the word 'json' in your response when using JSON format.",
                temperature=0.3
            )

            if not response["success"]:
                raise Exception(response.get("error", "OpenAI request failed"))

            # Parse JSON response with better error handling
            try:
                result = json.loads(response["content"])
            except json.JSONDecodeError as e:
                logger.error(f"Failed to parse OpenAI response as JSON: {str(e)}")
                logger.error(f"Response content: {response['content'][:500]}...")  # Log first 500 chars
                # Return a fallback response
                result = {
                    "error": "Failed to parse OpenAI response",
                    "analysis_results": "Unable to process the query results due to JSON parsing error",
                    "data_summary": "Query processing failed",
                    "recommendations": ["Please try the request again", "Check system logs for details"]
                }

            # Store formatted result
            state.metadata["query_result"] = result
            state.metadata["next_node"] = "query_output"

            # Clear prometheus routing flags to prevent loops
            state.metadata["needs_prometheus"] = False
            state.metadata["prometheus_collection_complete"] = False

            # Store prometheus processing info in metadata
            state.metadata[f"{self.node_name}_prometheus_processing"] = {
                "timestamp": state.metadata.get("current_timestamp"),
                "status": "completed",
                "data_processed": True
            }

        except Exception as e:
            logger.error(f"Error processing prometheus results: {str(e)}")
            state.error_message = f"Failed to process prometheus results: {str(e)}"
            state.metadata["next_node"] = "error_handler"
            # Clear prometheus routing flags even on error
            state.metadata["needs_prometheus"] = False
            state.metadata["prometheus_collection_complete"] = False

        return state
    
    def get_next_node(self, state: WorkflowState) -> str:
        """
        Determine the next node based on query processing results.

        Args:
            state: Current workflow state

        Returns:
            Name of the next node to execute
        """
        if state.error_message:
            return "error_handler"

        # Check if prometheus processing is complete first
        if state.metadata.get("prometheus_collection_complete"):
            # Prometheus data has been collected and processed, route to output
            return state.metadata.get("next_node", "query_output")

        # Check if we need to route to prometheus
        if state.metadata.get("needs_prometheus"):
            return "prometheus"

        # Otherwise route to output
        return state.metadata.get("next_node", "query_output")


# Create singleton instance
query_node = QueryNode()
