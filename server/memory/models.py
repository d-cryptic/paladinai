"""
Memory models for the PaladinAI memory layer.
"""

from typing import Dict, List, Any, Optional
from pydantic import BaseModel, ConfigDict, field_serializer, field_validator
from datetime import datetime


class MemoryEntry(BaseModel):
    """A single memory entry with metadata."""
    model_config = ConfigDict()
    
    content: str
    memory_type: str  # Dynamic type instead of enum
    user_id: Optional[str] = None
    session_id: Optional[str] = None
    workflow_type: Optional[str] = None
    confidence: float = 1.0
    metadata: Dict[str, Any] = {}
    created_at: datetime = datetime.now()
    
    @field_validator('memory_type')
    @classmethod
    def validate_memory_type(cls, v: str) -> str:
        """Validate memory type is a non-empty string."""
        if not v or not v.strip():
            raise ValueError("Memory type cannot be empty")
        return v.strip().lower()
    
    @field_serializer('created_at')
    def serialize_created_at(self, dt: datetime) -> str:
        return dt.isoformat()


class RelationshipEntry(BaseModel):
    """A relationship entry for the knowledge graph."""
    model_config = ConfigDict()
    
    source_entity: str
    relationship_type: str  # Dynamic type instead of enum
    target_entity: str
    properties: Dict[str, Any] = {}
    confidence: float = 1.0
    created_at: datetime = datetime.now()
    
    @field_validator('relationship_type')
    @classmethod
    def validate_relationship_type(cls, v: str) -> str:
        """Validate relationship type is a non-empty string."""
        if not v or not v.strip():
            raise ValueError("Relationship type cannot be empty")
        return v.strip().upper()
    
    @field_serializer('created_at')
    def serialize_created_at(self, dt: datetime) -> str:
        return dt.isoformat()


class MemorySearchQuery(BaseModel):
    """Query model for searching memories."""
    query: str
    memory_types: Optional[List[str]] = None  # Dynamic types
    user_id: Optional[str] = None
    limit: int = 10
    confidence_threshold: float = 0.5
    
    @field_validator('memory_types')
    @classmethod
    def validate_memory_types(cls, v: Optional[List[str]]) -> Optional[List[str]]:
        """Validate memory types are non-empty strings."""
        if v is None:
            return None
        return [t.strip().lower() for t in v if t and t.strip()]


class MemoryInstructionRequest(BaseModel):
    """Request model for storing explicit instructions."""
    instruction: str
    user_id: Optional[str] = None
    context: Optional[Dict[str, Any]] = {}


class MemoryExtractionRequest(BaseModel):
    """Request model for extracting memories from interactions."""
    content: str
    user_input: str
    workflow_type: str
    user_id: Optional[str] = None
    session_id: Optional[str] = None
    context: Optional[Dict[str, Any]] = {}


class GraphRelationship(BaseModel):
    """Model for Neo4j graph relationships."""
    source: str
    relationship: str
    target: str
    properties: Dict[str, Any] = {}
    
    def to_cypher_create(self) -> str:
        """Convert to Cypher CREATE statement."""
        props_str = ""
        if self.properties:
            props_list = [f"{k}: '{v}'" for k, v in self.properties.items()]
            props_str = f" {{{', '.join(props_list)}}}"
        
        # Sanitize relationship name for Neo4j (replace spaces with underscores)
        sanitized_relationship = self.relationship.upper().replace(' ', '_').replace('-', '_')
        
        return f"""
        MERGE (s:Entity {{name: '{self.source}'}})
        MERGE (t:Entity {{name: '{self.target}'}})
        MERGE (s)-[r:{sanitized_relationship}{props_str}]->(t)
        """