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


def serialize_loki_data(loki_data: Dict[str, Any]) -> Dict[str, Any]:
    """
    Serialize loki data to ensure JSON-compatibility.

    Args:
        loki_data: Loki data that may contain complex objects

    Returns:
        JSON-serializable dictionary
    """
    try:
        # Loki data is typically already JSON-compatible, but we'll ensure it
        def serialize_item(item):
            """Recursively serialize items."""
            if hasattr(item, 'model_dump'):  # Handle Pydantic models if any
                return item.model_dump()
            elif hasattr(item, '__dict__'):  # Handle objects with __dict__
                return {k: serialize_item(v) for k, v in item.__dict__.items() if not k.startswith('_')}
            elif isinstance(item, dict):
                return {k: serialize_item(v) for k, v in item.items()}
            elif isinstance(item, list):
                return [serialize_item(i) for i in item]
            elif isinstance(item, (str, int, float, bool, type(None))):
                return item
            else:
                return str(item)  # Convert unknown types to string

        return serialize_item(loki_data)

    except Exception as e:
        logger.warning(f"Failed to serialize loki data: {str(e)}")
        # Return a simplified version without problematic objects
        return {
            "serialization_error": str(e),
            "data_summary": "Loki data available but not fully serializable"
        }