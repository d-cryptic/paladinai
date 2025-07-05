import io
import uuid
from typing import Dict, Any, List
from fastapi import UploadFile
from loguru import logger

from .models import (
    DocumentType,
    DocumentProcessingResponse,
    ProcessingStatus,
    DocumentChunk
)
from .document_parser import DocumentParser
from .chunker import DocumentChunker
from .qdrant_client import QdrantService


class RAGService:
    def __init__(self):
        self.parser = DocumentParser()
        self.chunker = DocumentChunker()
        self.qdrant = QdrantService()
        logger.info("RAGService initialized")
    
    async def initialize(self):
        """Initialize the RAG service and create Qdrant collection."""
        await self.qdrant.create_documentation_collection()
    
    async def process_document(self, file: UploadFile) -> DocumentProcessingResponse:
        """
        Process uploaded document through the RAG pipeline.
        """
        document_id = str(uuid.uuid4())
        
        try:
            # Validate file type
            file_extension = file.filename.split('.')[-1].lower()
            if file_extension not in ['pdf', 'md']:
                return DocumentProcessingResponse(
                    document_id=document_id,
                    status=ProcessingStatus.FAILED,
                    message=f"Unsupported file type: {file_extension}. Only PDF and Markdown files are supported.",
                    collection_name=self.qdrant.documentation_collection
                )
            
            document_type = DocumentType.PDF if file_extension == 'pdf' else DocumentType.MARKDOWN
            
            # Read file content
            file_content = await file.read()
            
            # Parse document
            logger.info(f"Parsing {document_type.value} document: {file.filename}")
            if document_type == DocumentType.PDF:
                text, page_numbers = await self.parser.parse_pdf(file_content)
            else:
                text = await self.parser.parse_markdown(file_content)
                page_numbers = None
            
            # Clean text
            text = self.parser.clean_text(text)
            
            if not text:
                return DocumentProcessingResponse(
                    document_id=document_id,
                    status=ProcessingStatus.FAILED,
                    message="No text content found in the document.",
                    collection_name=self.qdrant.documentation_collection
                )
            
            # Create chunks
            logger.info(f"Creating chunks for document: {file.filename}")
            chunks = await self.chunker.create_chunks(
                text=text,
                source=file.filename,
                document_type=document_type,
                page_numbers=page_numbers
            )
            
            if not chunks:
                return DocumentProcessingResponse(
                    document_id=document_id,
                    status=ProcessingStatus.FAILED,
                    message="Failed to create chunks from the document.",
                    collection_name=self.qdrant.documentation_collection
                )
            
            # Prepare chunks for Qdrant
            qdrant_chunks = [
                {
                    "id": chunk.id,
                    "embedding": chunk.embedding,
                    "content": chunk.content,
                    "metadata": chunk.metadata.model_dump()
                }
                for chunk in chunks
            ]
            
            # Store in Qdrant
            logger.info(f"Storing {len(chunks)} chunks in Qdrant")
            success = await self.qdrant.upsert_chunks(qdrant_chunks)
            
            if success:
                return DocumentProcessingResponse(
                    document_id=document_id,
                    status=ProcessingStatus.COMPLETED,
                    message=f"Successfully processed document: {file.filename}",
                    chunks_created=len(chunks),
                    collection_name=self.qdrant.documentation_collection
                )
            else:
                return DocumentProcessingResponse(
                    document_id=document_id,
                    status=ProcessingStatus.FAILED,
                    message="Failed to store chunks in Qdrant.",
                    collection_name=self.qdrant.documentation_collection
                )
                
        except Exception as e:
            logger.error(f"Error processing document: {e}")
            return DocumentProcessingResponse(
                document_id=document_id,
                status=ProcessingStatus.FAILED,
                message=f"Error processing document: {str(e)}",
                collection_name=self.qdrant.documentation_collection
            )
    
    async def search_documents(
        self,
        query: str,
        limit: int = 5,
        filter_metadata: Dict[str, Any] = None
    ) -> List[Dict[str, Any]]:
        """
        Search for relevant document chunks based on query.
        """
        try:
            # Generate query embedding
            query_embedding = await self.chunker.embeddings.aembed_query(query)
            
            # Search in Qdrant
            results = await self.qdrant.search_similar_chunks(
                query_embedding=query_embedding,
                limit=limit,
                filter_metadata=filter_metadata
            )
            
            return results
        except Exception as e:
            logger.error(f"Error searching documents: {e}")
            return []