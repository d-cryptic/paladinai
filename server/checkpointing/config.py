"""
Configuration settings for MongoDB checkpointing.

This module centralizes all checkpointing-related configuration
loaded from environment variables.
"""

import os
from typing import Optional
from pydantic import BaseModel, Field


class CheckpointConfig(BaseModel):
    """Configuration model for MongoDB checkpointing."""
    
    # MongoDB connection settings
    mongodb_uri: str = Field(
        default="mongodb://localhost:27017/paladinai_checkpoints",
        description="MongoDB connection URI"
    )
    database_name: str = Field(
        default="paladinai_checkpoints",
        description="MongoDB database name for checkpoints"
    )
    collection_name: str = Field(
        default="langgraph_checkpoints",
        description="MongoDB collection name for checkpoints (Note: LangGraph uses 'checkpoints_aio' and 'checkpoint_writes_aio')"
    )
    
    # Feature flags
    enabled: bool = Field(
        default=True,
        description="Whether checkpointing is enabled"
    )
    
    # TTL settings
    checkpoint_ttl_days: int = Field(
        default=30,
        description="Number of days to keep checkpoints before auto-cleanup"
    )
    
    # Performance settings
    max_checkpoint_size_mb: int = Field(
        default=16,
        description="Maximum size of a single checkpoint in MB"
    )
    
    @classmethod
    def from_env(cls) -> "CheckpointConfig":
        """Load configuration from environment variables."""
        # Get MongoDB URI and expand environment variables
        mongodb_uri = os.getenv("MONGODB_URI", cls.model_fields["mongodb_uri"].default)
        
        # Manually expand HOST_IP if present in the URI
        host_ip = os.getenv("HOST_IP", "localhost")
        if "${HOST_IP}" in mongodb_uri:
            mongodb_uri = mongodb_uri.replace("${HOST_IP}", host_ip)
        
        # # Remove replica set parameter if not needed (for single MongoDB instance)
        # if os.getenv("MONGODB_DISABLE_REPLICA_SET", "false").lower() == "true":
        #     # Remove replicaSet parameter from URI
        #     if "?replicaSet=" in mongodb_uri:
        #         mongodb_uri = mongodb_uri.split("?replicaSet=")[0]
        #     elif "&replicaSet=" in mongodb_uri:
        #         mongodb_uri = mongodb_uri.replace("&replicaSet=rs0", "")
       
        return cls(
            mongodb_uri=mongodb_uri,
            database_name=os.getenv("MONGODB_DATABASE", cls.model_fields["database_name"].default),
            collection_name=os.getenv("MONGODB_COLLECTION", cls.model_fields["collection_name"].default),
            enabled=os.getenv("MONGODB_CHECKPOINT_ENABLED", "true").lower() == "true",
            checkpoint_ttl_days=int(os.getenv("CHECKPOINT_TTL_DAYS", "30")),
            max_checkpoint_size_mb=int(os.getenv("MAX_CHECKPOINT_SIZE_MB", "16"))
        )


# Global configuration instance
checkpoint_config = CheckpointConfig.from_env()