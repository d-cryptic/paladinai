"""
Utility functions for Prometheus node operations.

This module contains helper functions for timestamp generation,
data serialization, and other common operations.
"""

import json
import logging
from typing import Dict, Any
from datetime import datetime, timedelta

logger = logging.getLogger(__name__)


def generate_prometheus_timestamps(duration_hours: int = 1) -> Dict[str, str]:
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


def serialize_raw_data(raw_data: Dict[str, Any]) -> Dict[str, Any]:
    """
    Serialize raw data containing Pydantic objects to JSON-compatible format.

    Args:
        raw_data: Raw data that may contain Pydantic objects

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
