"""
Utility functions for checkpointing operations.

This module provides helper functions for checkpoint management,
serialization, and validation.
"""

import sys
import json
import logging
from typing import Dict, Any, Optional
from datetime import datetime, timedelta, timezone

logger = logging.getLogger(__name__)


def get_checkpoint_size(checkpoint: Dict[str, Any]) -> int:
    """
    Calculate the size of a checkpoint in bytes.
    
    Args:
        checkpoint: The checkpoint data
        
    Returns:
        Size in bytes
    """
    try:
        # Serialize to JSON to get accurate size
        serialized = json.dumps(checkpoint, default=str)
        return sys.getsizeof(serialized)
    except Exception as e:
        logger.error(f"Failed to calculate checkpoint size: {e}")
        return 0


def is_checkpoint_expired(checkpoint: Dict[str, Any], ttl_days: int) -> bool:
    """
    Check if a checkpoint has expired based on TTL.
    
    Args:
        checkpoint: The checkpoint data
        ttl_days: Time to live in days
        
    Returns:
        True if expired, False otherwise
    """
    try:
        timestamp = checkpoint.get("timestamp")
        if not timestamp:
            return False
            
        if isinstance(timestamp, str):
            checkpoint_time = datetime.fromisoformat(timestamp)
        else:
            checkpoint_time = timestamp
            
        expiry_time = checkpoint_time + timedelta(days=ttl_days)
        return datetime.now(timezone.utc) > expiry_time
        
    except Exception as e:
        logger.error(f"Failed to check checkpoint expiry: {e}")
        return False


def format_checkpoint_info(checkpoint: Dict[str, Any]) -> Dict[str, Any]:
    """
    Format checkpoint information for display.
    
    Args:
        checkpoint: Raw checkpoint data
        
    Returns:
        Formatted checkpoint information
    """
    try:
        info = {
            "thread_id": checkpoint.get("thread_id", "unknown"),
            "checkpoint_id": checkpoint.get("checkpoint_id", ""),
            "checkpoint_ns": checkpoint.get("checkpoint_ns", ""),
            "timestamp": checkpoint.get("timestamp", ""),
            "size_bytes": get_checkpoint_size(checkpoint),
            "metadata": checkpoint.get("metadata", {})
        }
        
        # Add human-readable size
        size_mb = info["size_bytes"] / (1024 * 1024)
        info["size_mb"] = round(size_mb, 2)
        
        return info
        
    except Exception as e:
        logger.error(f"Failed to format checkpoint info: {e}")
        return {
            "error": str(e),
            "thread_id": checkpoint.get("thread_id", "unknown")
        }


def validate_session_id(session_id: str) -> bool:
    """
    Validate a session ID format.
    
    Args:
        session_id: The session ID to validate
        
    Returns:
        True if valid, False otherwise
    """
    if not session_id:
        return False
        
    # Session ID should be alphanumeric with underscores
    # and between 1 and 128 characters
    if not 1 <= len(session_id) <= 128:
        return False
        
    # Allow alphanumeric, underscores, and hyphens
    allowed_chars = set("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-")
    return all(c in allowed_chars for c in session_id)


def generate_checkpoint_namespace(workflow_type: str, node_name: Optional[str] = None) -> str:
    """
    Generate a checkpoint namespace based on workflow type and node.
    
    Args:
        workflow_type: Type of workflow (e.g., "QUERY", "ACTION")
        node_name: Optional node name for node-specific checkpoints
        
    Returns:
        Formatted namespace string
    """
    namespace = f"workflow_{workflow_type.lower()}"
    if node_name:
        namespace = f"{namespace}_{node_name}"
    return namespace