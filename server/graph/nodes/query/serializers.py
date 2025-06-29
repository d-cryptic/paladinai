"""
Serializers for query node - handles data serialization logic.
"""

import logging
from typing import Dict, Any

logger = logging.getLogger(__name__)


def serialize_prometheus_data(prometheus_data: Dict[str, Any]) -> Dict[str, Any]:
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