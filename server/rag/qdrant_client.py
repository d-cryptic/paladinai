import os
from typing import List, Dict, Any, Optional
from qdrant_client import QdrantClient
from qdrant_client.models import (
    VectorParams,
    Distance,
    PointStruct,
    Filter,
    FieldCondition,
    MatchValue,
)
from loguru import logger
from dotenv import load_dotenv

load_dotenv()


class QdrantService:
    def __init__(self):
        self.host = os.getenv("QDRANT_HOST", "http://localhost")
        self.port = int(os.getenv("QDRANT_PORT", 6333))
        self.vector_size = int(os.getenv("QDRANT_VECTOR_SIZE", 1536))
        self.distance_metric = os.getenv("QDRANT_DISTANCE_METRIC", "Cosine")
        
        self.client = QdrantClient(
            host=self.host.replace("http://", "").replace("https://", ""),
            port=self.port,
        )
        
        self.documentation_collection = "paladin_documentation"
        logger.info(f"QdrantService initialized with host: {self.host}:{self.port}")
    
    async def create_documentation_collection(self):
        try:
            collections = self.client.get_collections().collections
            collection_names = [col.name for col in collections]
            
            if self.documentation_collection not in collection_names:
                self.client.create_collection(
                    collection_name=self.documentation_collection,
                    vectors_config=VectorParams(
                        size=self.vector_size,
                        distance=Distance[self.distance_metric.upper()],
                    ),
                )
                logger.info(f"Created collection: {self.documentation_collection}")
            else:
                logger.info(f"Collection {self.documentation_collection} already exists")
        except Exception as e:
            logger.error(f"Error creating collection: {e}")
            raise
    
    async def upsert_chunks(self, chunks: List[Dict[str, Any]]) -> bool:
        try:
            points = []
            for chunk in chunks:
                point = PointStruct(
                    id=chunk["id"],
                    vector=chunk["embedding"],
                    payload={
                        "content": chunk["content"],
                        "metadata": chunk["metadata"],
                    },
                )
                points.append(point)
            
            self.client.upsert(
                collection_name=self.documentation_collection,
                points=points,
            )
            logger.info(f"Upserted {len(points)} chunks to Qdrant")
            return True
        except Exception as e:
            logger.error(f"Error upserting chunks: {e}")
            return False
    
    async def search_similar_chunks(
        self,
        query_embedding: List[float],
        limit: int = 5,
        score_threshold: float = 0.7,
        filter_metadata: Optional[Dict[str, Any]] = None,
    ) -> List[Dict[str, Any]]:
        try:
            search_filter = None
            if filter_metadata and any(filter_metadata.values()):
                conditions = []
                for key, value in filter_metadata.items():
                    # Skip None or empty values
                    if value is not None and value != "":
                        conditions.append(
                            FieldCondition(
                                key=f"metadata.{key}",
                                match=MatchValue(value=str(value)),  # Ensure string type
                            )
                        )
                if conditions:  # Only create filter if we have valid conditions
                    search_filter = Filter(must=conditions)
            
            results = self.client.search(
                collection_name=self.documentation_collection,
                query_vector=query_embedding,
                limit=limit,
                score_threshold=score_threshold,
                with_payload=True,
                query_filter=search_filter,
            )
            
            return [
                {
                    "id": hit.id,
                    "score": hit.score,
                    "content": hit.payload.get("content", ""),
                    "metadata": hit.payload.get("metadata", {}),
                }
                for hit in results
            ]
        except Exception as e:
            logger.error(f"Error searching chunks: {e}")
            return []