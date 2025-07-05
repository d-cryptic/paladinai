from fastapi import APIRouter, UploadFile, File, HTTPException, Depends
from fastapi.responses import JSONResponse
from typing import Optional, Dict, Any
from loguru import logger

from rag.service import RAGService
from rag.models import DocumentProcessingResponse, ProcessingStatus

router = APIRouter(prefix="/api/v1/documents", tags=["documents"])

# Global variable to store RAG service instance
_rag_service = None


async def get_rag_service() -> RAGService:
    """Dependency to get RAG service instance."""
    global _rag_service
    if _rag_service is None:
        _rag_service = RAGService()
        await _rag_service.initialize()
    return _rag_service


@router.post("/upload", response_model=DocumentProcessingResponse)
async def upload_document(
    file: UploadFile = File(..., description="PDF or Markdown file to upload"),
    rag_service: RAGService = Depends(get_rag_service)
):
    """
    Upload and process a document (PDF or Markdown) through the RAG pipeline.
    
    The document will be:
    1. Parsed to extract text content
    2. Split into optimized chunks using recursive character text splitter
    3. Embedded using OpenAI embeddings
    4. Stored in Qdrant vector database
    
    Args:
        file: The document file to upload (PDF or Markdown)
    
    Returns:
        DocumentProcessingResponse with processing status and details
    """
    try:
        # Validate file size (max 50MB)
        max_size = 50 * 1024 * 1024  # 50MB
        file_size = 0
        
        # Read file in chunks to check size
        file_content = await file.read()
        file_size = len(file_content)
        
        if file_size > max_size:
            raise HTTPException(
                status_code=413,
                detail=f"File too large. Maximum size is 50MB, received {file_size / 1024 / 1024:.2f}MB"
            )
        
        # Reset file position
        file.file.seek(0)
        
        logger.info(f"Processing document: {file.filename} ({file_size / 1024:.2f}KB)")
        
        # Process document through RAG pipeline
        response = await rag_service.process_document(file)
        
        if response.status == ProcessingStatus.FAILED:
            raise HTTPException(status_code=400, detail=response.message)
        
        return response
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Unexpected error processing document: {e}")
        raise HTTPException(
            status_code=500,
            detail=f"Internal server error while processing document: {str(e)}"
        )


@router.post("/search")
async def search_documents(
    query: str,
    limit: Optional[int] = 5,
    source_filter: Optional[str] = None,
    document_type_filter: Optional[str] = None,
    rag_service: RAGService = Depends(get_rag_service)
):
    """
    Search for relevant document chunks based on a query.
    
    Args:
        query: The search query
        limit: Maximum number of results to return (default: 5)
        source_filter: Filter by document source/filename
        document_type_filter: Filter by document type (pdf or md)
    
    Returns:
        List of relevant document chunks with similarity scores
    """
    try:
        # Build filter metadata
        filter_metadata = {}
        if source_filter:
            filter_metadata["source"] = source_filter
        if document_type_filter:
            filter_metadata["document_type"] = document_type_filter
        
        results = await rag_service.search_documents(
            query=query,
            limit=limit,
            filter_metadata=filter_metadata if filter_metadata else None
        )
        
        return {
            "query": query,
            "results": results,
            "count": len(results)
        }
        
    except Exception as e:
        logger.error(f"Error searching documents: {e}")
        raise HTTPException(
            status_code=500,
            detail=f"Error searching documents: {str(e)}"
        )


@router.get("/health")
async def health_check(rag_service: RAGService = Depends(get_rag_service)):
    """Check if the document service is healthy."""
    try:
        # Check if Qdrant is accessible
        collections = rag_service.qdrant.client.get_collections()
        
        return {
            "status": "healthy",
            "qdrant_connected": True,
            "collection": rag_service.qdrant.documentation_collection,
            "collections_count": len(collections.collections)
        }
    except Exception as e:
        logger.error(f"Health check failed: {e}")
        return JSONResponse(
            status_code=503,
            content={
                "status": "unhealthy",
                "error": str(e)
            }
        )