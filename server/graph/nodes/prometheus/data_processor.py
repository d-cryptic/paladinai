"""
Data processing for Prometheus node.

This module handles processing and formatting of collected metrics data
for different workflow types.
"""

import json
import logging
from typing import Dict, Any
from datetime import datetime
from langfuse import observe

from prompts.workflows.processing import get_action_query_processing_prompt, get_incident_processing_prompt
from llm.openai import openai
from .utils import serialize_raw_data

logger = logging.getLogger(__name__)


class DataProcessor:
    """Handles processing and formatting of collected metrics data."""

    def __init__(self):
        """Initialize the data processor."""
        pass

    @observe(name="prometheus_data_processing")
    async def process_collected_data(
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
            serialized_raw_data = serialize_raw_data(raw_data)

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
                "raw_data": serialize_raw_data(raw_data)  # Serialize Pydantic objects for JSON compatibility
            })

            return processed_result
            
        except Exception as e:
            logger.error(f"Error processing collected data: {str(e)}")
            return {
                "error": f"Data processing failed: {str(e)}",
                "raw_data": serialize_raw_data(raw_data),  # Ensure serializable
                "timestamp": datetime.now().isoformat()
            }


# Create singleton instance
data_processor = DataProcessor()
