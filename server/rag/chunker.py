import os
import uuid
from typing import List, Dict, Any, Optional, Tuple
import tiktoken
from langchain.text_splitter import RecursiveCharacterTextSplitter
from langchain_openai import OpenAIEmbeddings
from loguru import logger
from .models import DocumentChunk, ChunkMetadata, DocumentType


class DocumentChunker:
    def __init__(self):
        self.openai_api_key = os.getenv("OPENAI_API_KEY")
        if not self.openai_api_key:
            raise ValueError("OPENAI_API_KEY not found in environment variables")
        
        # Initialize embeddings model
        self.embeddings = OpenAIEmbeddings(
            model="text-embedding-ada-002",
            openai_api_key=self.openai_api_key
        )
        
        # Initialize tokenizer for accurate token counting
        self.tokenizer = tiktoken.encoding_for_model("text-embedding-ada-002")
        
        # Configure chunking parameters
        self.chunk_size = 500  # tokens
        self.chunk_overlap = 50  # tokens
        self.separators = ["\n\n", "\n", ". ", " ", ""]
        
        logger.info("DocumentChunker initialized with chunk_size: {}, overlap: {}", 
                   self.chunk_size, self.chunk_overlap)
    
    def _count_tokens(self, text: str) -> int:
        """Count tokens in text using tiktoken."""
        return len(self.tokenizer.encode(text))
    
    async def create_chunks(
        self,
        text: str,
        source: str,
        document_type: DocumentType,
        page_numbers: Optional[List[int]] = None
    ) -> List[DocumentChunk]:
        """
        Create chunks from document text using recursive character text splitter.
        """
        try:
            # Create text splitter with token-based length function
            text_splitter = RecursiveCharacterTextSplitter(
                chunk_size=self.chunk_size,
                chunk_overlap=self.chunk_overlap,
                length_function=self._count_tokens,
                separators=self.separators,
                is_separator_regex=False,
            )
            
            # Split text into chunks
            if page_numbers and document_type == DocumentType.PDF:
                # For PDFs, split by pages first then chunk each page
                chunks = []
                pages = text.split("[Page ")
                
                for i, page_content in enumerate(pages[1:], 1):  # Skip first empty split
                    page_num = int(page_content.split("]")[0])
                    page_text = page_content.split("]", 1)[1] if "]" in page_content else page_content
                    
                    page_chunks = text_splitter.split_text(page_text)
                    for chunk_text in page_chunks:
                        chunks.append({
                            "text": chunk_text,
                            "page_number": page_num
                        })
            else:
                # For markdown or other documents
                chunk_texts = text_splitter.split_text(text)
                chunks = [{"text": text, "page_number": None} for text in chunk_texts]
            
            # Create document chunks with metadata and embeddings
            document_chunks = []
            total_chunks = len(chunks)
            
            # Process chunks in batches for embedding generation
            batch_size = 20
            for i in range(0, total_chunks, batch_size):
                batch = chunks[i:i + batch_size]
                batch_texts = [chunk["text"] for chunk in batch]
                
                # Generate embeddings for batch
                batch_embeddings = await self.embeddings.aembed_documents(batch_texts)
                
                # Create DocumentChunk objects
                for j, (chunk_data, embedding) in enumerate(zip(batch, batch_embeddings)):
                    chunk_index = i + j
                    chunk_id = str(uuid.uuid4())
                    
                    metadata = ChunkMetadata(
                        source=source,
                        page_number=chunk_data.get("page_number"),
                        chunk_index=chunk_index,
                        total_chunks=total_chunks,
                        document_type=document_type
                    )
                    
                    document_chunk = DocumentChunk(
                        id=chunk_id,
                        content=chunk_data["text"],
                        metadata=metadata,
                        embedding=embedding
                    )
                    
                    document_chunks.append(document_chunk)
            
            logger.info(f"Created {len(document_chunks)} chunks from document: {source}")
            return document_chunks
            
        except Exception as e:
            logger.error(f"Error creating chunks: {e}")
            raise ValueError(f"Failed to create chunks: {str(e)}")
    
    async def create_semantic_chunks(
        self,
        text: str,
        source: str,
        document_type: DocumentType,
        similarity_threshold: float = 0.8
    ) -> List[DocumentChunk]:
        """
        Create semantic chunks based on sentence similarity.
        This is an advanced chunking method that groups sentences with similar embeddings.
        """
        # For now, we'll use the recursive character splitter as the primary method
        # Semantic chunking can be implemented as an enhancement
        logger.info("Using recursive character text splitter for chunking")
        return await self.create_chunks(text, source, document_type)