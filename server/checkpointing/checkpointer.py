"""
MongoDB Checkpointer for LangGraph workflow persistence.

This module provides MongoDB-based checkpointing functionality for
saving and restoring workflow state across executions.
"""

import logging
from typing import Optional, Dict, Any
from contextlib import asynccontextmanager
from datetime import datetime, timezone

from langgraph.checkpoint.mongodb import AsyncMongoDBSaver
from motor.motor_asyncio import AsyncIOMotorClient
from pymongo.errors import ConnectionFailure, OperationFailure

from .config import checkpoint_config

logger = logging.getLogger(__name__)


class PaladinCheckpointer:
    """
    MongoDB-based checkpointer for PaladinAI workflows.
    
    This class manages checkpoint storage and retrieval using MongoDB,
    enabling workflow persistence, fault tolerance, and state recovery.
    """
    
    def __init__(self):
        """Initialize the checkpointer with MongoDB configuration."""
        self.config = checkpoint_config
        self.mongodb_uri = self.config.mongodb_uri
        self.database_name = self.config.database_name
        self.collection_name = self.config.collection_name
        self.enabled = self.config.enabled
        self.ttl_days = self.config.checkpoint_ttl_days
        
        self._client: Optional[AsyncIOMotorClient] = None
        self._checkpointer: Optional[AsyncMongoDBSaver] = None
        self._checkpointer_context = None
        
        logger.info(f"MongoDB checkpointing {'enabled' if self.enabled else 'disabled'}")
    
    async def initialize(self) -> None:
        """
        Initialize MongoDB connection and checkpointer.
        
        Raises:
            ConnectionFailure: If unable to connect to MongoDB
        """
        if not self.enabled:
            logger.info("Checkpointing is disabled, skipping initialization")
            return
        
        try:
            # Log the connection attempt
            safe_uri = self.mongodb_uri.split('@')[-1] if '@' in self.mongodb_uri else self.mongodb_uri
            logger.info(f"Attempting to connect to MongoDB at: {safe_uri}")
            
            # Create MongoDB client directly first to test connection
            # Use directConnection=True to bypass replica set hostname issues
            self._client = AsyncIOMotorClient(
                self.mongodb_uri,
                directConnection=True,
                serverSelectionTimeoutMS=30000
            )
            
            # Test the connection
            await self._client.admin.command('ping')
            logger.info("Successfully connected to MongoDB")
            
            # Create checkpointer with the client
            self._checkpointer = AsyncMongoDBSaver(
                client=self._client,
                db_name=self.database_name
            )
            
            logger.info(f"AsyncMongoDBSaver initialized for database '{self.database_name}'")
            
            # Create indexes for better performance
            await self._create_indexes()
            
        except ConnectionFailure as e:
            logger.error(f"Failed to connect to MongoDB: {str(e)}")
            self.enabled = False
            raise
        except Exception as e:
            logger.error(f"Failed to initialize checkpointer: {str(e)}")
            self.enabled = False
            raise
    
    async def _ensure_database_exists(self) -> None:
        """Ensure the database and collection exist in MongoDB."""
        try:
            # List existing databases
            db_names = await self._client.list_database_names()
            logger.debug(f"Existing databases: {db_names}")
            
            # Access the database (creates it if it doesn't exist)
            db = self._client[self.database_name]
            
            # Check if collection exists
            collection_names = await db.list_collection_names()
            logger.debug(f"Existing collections in {self.database_name}: {collection_names}")
            
            if self.collection_name not in collection_names:
                # Create the collection explicitly
                await db.create_collection(self.collection_name)
                logger.info(f"Created collection '{self.collection_name}' in database '{self.database_name}'")
            else:
                logger.info(f"Collection '{self.collection_name}' already exists in database '{self.database_name}'")
                
            # Insert a test document to ensure database is created
            test_collection = db["_checkpoint_init"]
            await test_collection.insert_one({"initialized": True, "timestamp": datetime.now(timezone.utc)})
            await test_collection.delete_many({})  # Clean up test collection
            
        except Exception as e:
            logger.error(f"Failed to ensure database exists: {str(e)}")
            raise
    
    async def _create_indexes(self) -> None:
        """Create MongoDB indexes for optimal checkpoint queries."""
        try:
            # Access the client from the checkpointer
            if hasattr(self._checkpointer, 'client'):
                client = self._checkpointer.client
            elif hasattr(self._checkpointer, '_client'):
                client = self._checkpointer._client
            else:
                logger.warning("Could not access MongoDB client from checkpointer for index creation")
                return
                
            db = client[self.database_name]
            collection = db[self.collection_name]
            
            # Create compound index for thread_id and timestamp
            await collection.create_index([
                ("thread_id", 1),
                ("timestamp", -1)
            ])
            
            # Create index for checkpoint_ns
            await collection.create_index("checkpoint_ns")
            
            # Create TTL index for automatic cleanup
            await collection.create_index(
                "timestamp",
                expireAfterSeconds=self.ttl_days * 24 * 60 * 60
            )
            
            logger.info("Created MongoDB indexes for checkpointing")
            
        except Exception as e:
            logger.warning(f"Failed to create indexes: {str(e)}")
    
    @property
    def checkpointer(self) -> Optional[AsyncMongoDBSaver]:
        """
        Get the checkpointer instance.
        
        Returns:
            AsyncMongoDBSaver instance if enabled, None otherwise
        """
        return self._checkpointer if self.enabled else None
    
    def get_config(self, thread_id: str, checkpoint_ns: str = "") -> Dict[str, Any]:
        """
        Create a checkpoint configuration.
        
        Args:
            thread_id: Unique identifier for the conversation thread
            checkpoint_ns: Namespace for checkpoint organization
            
        Returns:
            Configuration dictionary for checkpointing
        """
        return {
            "configurable": {
                "thread_id": thread_id,
                "checkpoint_ns": checkpoint_ns
            }
        }
    
    async def save_checkpoint(
        self,
        thread_id: str,
        checkpoint: Dict[str, Any],
        metadata: Optional[Dict[str, Any]] = None,
        checkpoint_ns: str = ""
    ) -> None:
        """
        Save a checkpoint to MongoDB.
        
        Args:
            thread_id: Thread identifier
            checkpoint: Checkpoint data to save
            metadata: Optional metadata to store with checkpoint
            checkpoint_ns: Namespace for organization
        """
        if not self.enabled or not self._checkpointer:
            return
        
        try:
            config = self.get_config(thread_id, checkpoint_ns)
            await self._checkpointer.aput(
                config=config,
                checkpoint=checkpoint,
                metadata=metadata or {},
                new_versions={}
            )
            logger.debug(f"Saved checkpoint for thread {thread_id}")
            
        except Exception as e:
            logger.error(f"Failed to save checkpoint: {str(e)}")
            # Don't fail the workflow if checkpointing fails
    
    async def load_checkpoint(
        self,
        thread_id: str,
        checkpoint_ns: str = ""
    ) -> Optional[Dict[str, Any]]:
        """
        Load the latest checkpoint for a thread.
        
        Args:
            thread_id: Thread identifier
            checkpoint_ns: Namespace for organization
            
        Returns:
            Checkpoint data if found, None otherwise
        """
        if not self.enabled or not self._checkpointer:
            return None
        
        try:
            config = self.get_config(thread_id, checkpoint_ns)
            checkpoint = await self._checkpointer.aget(config)
            
            if checkpoint:
                logger.debug(f"Loaded checkpoint for thread {thread_id}")
            
            return checkpoint
            
        except Exception as e:
            logger.error(f"Failed to load checkpoint: {str(e)}")
            return None
    
    async def list_checkpoints(
        self,
        thread_id: Optional[str] = None,
        checkpoint_ns: str = "",
        limit: int = 10
    ) -> list[Dict[str, Any]]:
        """
        List checkpoints for a thread or all threads.
        
        Args:
            thread_id: Optional thread identifier to filter by
            checkpoint_ns: Namespace to filter by
            limit: Maximum number of checkpoints to return
            
        Returns:
            List of checkpoint metadata
        """
        if not self.enabled or not self._checkpointer:
            return []
        
        try:
            if thread_id:
                config = self.get_config(thread_id, checkpoint_ns)
            else:
                config = {"configurable": {"checkpoint_ns": checkpoint_ns}}
            
            checkpoints = []
            async for checkpoint in self._checkpointer.alist(config, limit=limit):
                checkpoints.append(checkpoint)
            
            return checkpoints
            
        except Exception as e:
            logger.error(f"Failed to list checkpoints: {str(e)}")
            return []
    
    async def delete_checkpoint(
        self,
        thread_id: str,
        checkpoint_ns: str = ""
    ) -> bool:
        """
        Delete all checkpoints for a thread.
        
        Args:
            thread_id: Thread identifier
            checkpoint_ns: Namespace for organization
            
        Returns:
            True if deletion successful, False otherwise
        """
        if not self.enabled or not self._checkpointer:
            return False
        
        try:
            # Access the client from the checkpointer
            if hasattr(self._checkpointer, 'client'):
                client = self._checkpointer.client
            elif hasattr(self._checkpointer, '_client'):
                client = self._checkpointer._client
            else:
                logger.warning("Could not access MongoDB client from checkpointer for deletion")
                return False
                
            db = client[self.database_name]
            collection = db[self.collection_name]
            
            # Delete all checkpoints for the thread
            result = await collection.delete_many({
                "thread_id": thread_id,
                "checkpoint_ns": checkpoint_ns
            })
            
            logger.info(f"Deleted {result.deleted_count} checkpoints for thread {thread_id}")
            return result.deleted_count > 0
            
        except Exception as e:
            logger.error(f"Failed to delete checkpoints: {str(e)}")
            return False
    
    async def cleanup_old_checkpoints(self, days: int = 30) -> int:
        """
        Clean up checkpoints older than specified days.
        
        Args:
            days: Number of days to keep checkpoints
            
        Returns:
            Number of checkpoints deleted
        """
        if not self.enabled or not self._checkpointer:
            return 0
        
        try:
            from datetime import datetime, timedelta, timezone
            
            # Access the client from the checkpointer
            if hasattr(self._checkpointer, 'client'):
                client = self._checkpointer.client
            elif hasattr(self._checkpointer, '_client'):
                client = self._checkpointer._client
            else:
                logger.warning("Could not access MongoDB client from checkpointer for cleanup")
                return 0
                
            db = client[self.database_name]
            collection = db[self.collection_name]
            
            # Calculate cutoff date
            cutoff_date = datetime.now(timezone.utc) - timedelta(days=days)
            
            # Delete old checkpoints
            result = await collection.delete_many({
                "timestamp": {"$lt": cutoff_date}
            })
            
            logger.info(f"Cleaned up {result.deleted_count} old checkpoints")
            return result.deleted_count
            
        except Exception as e:
            logger.error(f"Failed to clean up old checkpoints: {str(e)}")
            return 0
    
    async def close(self) -> None:
        """Close MongoDB connection."""
        if self._client:
            self._client.close()
            logger.info("Closed MongoDB checkpointer connection")
    
    @asynccontextmanager
    async def session(self):
        """
        Context manager for checkpointer session.
        
        Ensures proper initialization and cleanup of MongoDB connection.
        """
        try:
            await self.initialize()
            yield self
        finally:
            await self.close()


# Global checkpointer instance
_checkpointer: Optional[PaladinCheckpointer] = None


async def get_checkpointer() -> PaladinCheckpointer:
    """
    Get or create the global checkpointer instance.
    
    Returns:
        PaladinCheckpointer instance
    """
    global _checkpointer
    
    if _checkpointer is None:
        _checkpointer = PaladinCheckpointer()
        await _checkpointer.initialize()
    
    return _checkpointer


async def close_checkpointer() -> None:
    """Close the global checkpointer instance."""
    global _checkpointer
    
    if _checkpointer:
        await _checkpointer.close()
        _checkpointer = None