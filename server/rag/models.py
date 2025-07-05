from pydantic import BaseModel, Field
from typing import List, Dict, Any, Optional
from enum import Enum


class DocumentType(str, Enum):
    PDF = "pdf"
    MARKDOWN = "md"


class ChunkMetadata(BaseModel):
    source: str
    page_number: Optional[int] = None
    chunk_index: int
    total_chunks: int
    document_type: DocumentType


class DocumentChunk(BaseModel):
    id: str
    content: str
    metadata: ChunkMetadata
    embedding: Optional[List[float]] = None


class ProcessingStatus(str, Enum):
    PENDING = "pending"
    PROCESSING = "processing"
    COMPLETED = "completed"
    FAILED = "failed"


class DocumentProcessingResponse(BaseModel):
    document_id: str
    status: ProcessingStatus
    message: str
    chunks_created: Optional[int] = None
    collection_name: str