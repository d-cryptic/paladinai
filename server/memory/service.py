"""
Memory Service for PaladinAI using mem0 with Qdrant and Neo4j.

This service handles memory storage, retrieval, and relationship management
for intelligent context awareness and learning.
"""

import logging
import os
import json
from typing import Dict, List, Any, Optional
from datetime import datetime

from mem0 import Memory
from qdrant_client import QdrantClient
from neo4j import GraphDatabase
from pydantic import BaseModel

from .models import (
    MemoryEntry, RelationshipEntry,
    MemorySearchQuery, MemoryInstructionRequest, MemoryExtractionRequest,
    GraphRelationship
)
from llm.openai import openai
from prompts.memory.extraction import (
    get_memory_extraction_prompt, get_relationship_extraction_prompt,
    get_instruction_processing_prompt, MEMORY_ANALYSIS_SYSTEM_PROMPT,
    RELATIONSHIP_ANALYSIS_SYSTEM_PROMPT, INSTRUCTION_PROCESSING_SYSTEM_PROMPT
)

logger = logging.getLogger(__name__)


class MemoryService:
    """
    Core memory service integrating mem0, Qdrant, and Neo4j.
    
    This service provides:
    - Memory storage and retrieval using mem0
    - Vector similarity search via Qdrant
    - Knowledge graph relationships via Neo4j
    - Intelligent memory extraction using OpenAI
    """
    
    def __init__(self):
        """Initialize the memory service with all backends."""
        self._initialize_backends()
        self._setup_logging()
    
    def _initialize_backends(self):
        """Initialize mem0, Qdrant, and Neo4j connections."""
        try:
            # Get Qdrant configuration for mem0
            qdrant_host = os.getenv("QDRANT_HOST", "localhost")
            qdrant_port = int(os.getenv("QDRANT_PORT", "6333"))
            
            # Handle environment variable substitution
            if "${HOST}" in qdrant_host:
                host_ip = os.getenv("HOST_IP", "localhost")
                qdrant_host = f"http://{host_ip}"
            
            # For now, use default mem0 initialization
            # TODO: Configure mem0 to use cloud Qdrant once we understand the proper config format
            self.mem0 = Memory()
            
            # But we'll bypass mem0 for storage and use direct Qdrant for our needs
            
            # Store additional config from .env for our custom operations
            self.mem0_config = {
                "collection_name": os.getenv("QDRANT_COLLECTION_NAME", "paladinai_memory"),
                "vector_size": int(os.getenv("QDRANT_VECTOR_SIZE", "1536")),
                "distance_metric": os.getenv("QDRANT_DISTANCE_METRIC", "Cosine"),
                "embedding_model": os.getenv("MEM0_EMBEDDING_MODEL", "text-embedding-3-small"),
                "similarity_threshold": float(os.getenv("MEM0_SIMILARITY_THRESHOLD", "0.7")),
                "max_memory_items": int(os.getenv("MEM0_MAX_MEMORY_ITEMS", "10000"))
            }
            
            # Direct Qdrant client for advanced operations using existing .env values
            qdrant_host = os.getenv("QDRANT_HOST", "localhost")
            qdrant_port = int(os.getenv("QDRANT_PORT", "6333"))
            
            # Handle environment variable substitution if needed
            if "${HOST_IP}" in qdrant_host:
                host_ip = os.getenv("HOST_IP", "localhost")
                qdrant_host = qdrant_host.replace("${HOST_IP}", host_ip)
            
            # Handle case where QDRANT_HOST includes protocol
            if qdrant_host.startswith("http://") or qdrant_host.startswith("https://"):
                self.qdrant_client = QdrantClient(url=f"{qdrant_host}:{qdrant_port}")
            else:
                self.qdrant_client = QdrantClient(
                    host=qdrant_host,
                    port=qdrant_port
                )
            
            # Direct Neo4j driver using existing .env values
            neo4j_host = os.getenv("NEO4J_HOST", "localhost")
            neo4j_bolt_port = os.getenv("NEO4J_BOLT_PORT", "7687")
            
            # Handle environment variable substitution if needed
            if "${HOST_IP}" in neo4j_host:
                host_ip = os.getenv("HOST_IP", "localhost")
                neo4j_host = neo4j_host.replace("${HOST_IP}", host_ip)
            
            # Remove protocol if present
            neo4j_host = neo4j_host.replace("http://", "").replace("https://", "")
            neo4j_url = f"bolt://{neo4j_host}:{neo4j_bolt_port}"
            
            self.neo4j_driver = GraphDatabase.driver(
                neo4j_url,
                auth=(
                    os.getenv("NEO4J_USERNAME", "neo4j"),
                    os.getenv("NEO4J_PASSWORD", "paladinai_neo4j_pass")
                )
            )
            
            logger.info("Memory service backends initialized successfully")
            
        except Exception as e:
            logger.error(f"Failed to initialize memory backends: {str(e)}")
            raise
    
    def _setup_logging(self):
        """Setup logging for memory operations."""
        self.logger = logging.getLogger(f"{__name__}.{self.__class__.__name__}")
    
    async def store_instruction(self, request: MemoryInstructionRequest) -> Dict[str, Any]:
        """
        Store an explicit instruction as memory.
        
        Args:
            request: Instruction storage request
            
        Returns:
            Storage result with memory ID and metadata
        """
        try:
            self.logger.debug(f"store_instruction called with request: {request}")
            
            # Create memory entry
            memory_entry = MemoryEntry(
                content=request.instruction,
                memory_type="instruction",  # Using dynamic string type
                user_id=request.user_id,
                metadata=request.context or {}
            )
            
            self.logger.debug(f"Created memory_entry: {memory_entry}")
            
            # Store in mem0 - force it to store by formatting as a fact
            self.logger.debug("About to call mem0.add")
            
            # Try a different approach - store directly
            import uuid
            memory_id = str(uuid.uuid4())
            
            # Always use direct Qdrant storage for reliability
            # mem0 uses its own local instance which we don't want
            self.logger.info("Storing memory directly in cloud Qdrant")
            await self._store_memory_in_qdrant(memory_entry, memory_id)
            
            self.logger.debug(f"Extracted memory_id: {memory_id}")
            
            # Extract relationships using OpenAI
            self.logger.debug("About to extract relationships")
            relationships = await self._extract_relationships(request.instruction)
            self.logger.debug(f"Extracted {len(relationships)} relationships")
            
            # Store relationships in Neo4j
            for relationship in relationships:
                await self._store_relationship(relationship)
            
            self.logger.info(f"Stored instruction memory with {len(relationships)} relationships")
            
            return {
                "success": True,
                "memory_id": memory_id,
                "relationships_count": len(relationships),
                "memory_type": memory_entry.memory_type  # Now it's already a string
            }
            
        except Exception as e:
            self.logger.error(f"Failed to store instruction: {str(e)}", exc_info=True)
            return {
                "success": False,
                "error": str(e)
            }
    
    async def extract_and_store_memories(self, request: MemoryExtractionRequest) -> Dict[str, Any]:
        """
        Extract and store memories from workflow interactions.
        
        Args:
            request: Memory extraction request
            
        Returns:
            Extraction and storage results
        """
        try:
            # Extract important memories using OpenAI
            memories = await self._extract_memories_from_interaction(request)
            
            stored_memories = []
            total_relationships = 0
            
            for memory_data in memories:
                # Create memory entry
                memory_entry = MemoryEntry(
                    content=memory_data["content"],
                    memory_type=memory_data["type"],  # Dynamic string type
                    user_id=request.user_id,
                    session_id=request.session_id,
                    workflow_type=request.workflow_type,
                    confidence=memory_data.get("confidence", 0.8),
                    metadata={
                        "extracted_from": "workflow_interaction",
                        "original_input": request.user_input,
                        "entities": memory_data.get("entities", []),  # Store entities
                        **request.context
                    }
                )
                
                # Store directly in cloud Qdrant
                import uuid
                memory_id = str(uuid.uuid4())
                
                self.logger.info("Storing extracted memory directly in cloud Qdrant")
                await self._store_memory_in_qdrant(memory_entry, memory_id)
                
                # Extract and store relationships
                relationships = await self._extract_relationships(memory_data["content"])
                for relationship in relationships:
                    await self._store_relationship(relationship)
                
                # Also create entities in Neo4j for the entities mentioned in this memory
                entities = memory_data.get("entities", [])
                for entity in entities:
                    # Create a self-referential relationship to ensure entity exists
                    entity_creation = GraphRelationship(
                        source=entity.lower(),
                        relationship="IS",
                        target=entity.lower(),
                        properties={"type": "entity", "created_from": "memory_extraction"}
                    )
                    await self._store_relationship(entity_creation)
                
                stored_memories.append({
                    "memory_id": memory_id,
                    "type": memory_entry.memory_type,  # Now it's already a string
                    "content": memory_data["content"],
                    "relationships_count": len(relationships)
                })
                total_relationships += len(relationships)
            
            self.logger.info(f"Extracted and stored {len(stored_memories)} memories with {total_relationships} relationships")
            
            return {
                "success": True,
                "memories_stored": len(stored_memories),
                "total_relationships": total_relationships,
                "memories": stored_memories
            }
            
        except Exception as e:
            self.logger.error(f"Failed to extract and store memories: {str(e)}")
            return {
                "success": False,
                "error": str(e)
            }
    
    async def search_memories(self, query: MemorySearchQuery) -> Dict[str, Any]:
        """
        Search memories using vector similarity and filters.
        
        Args:
            query: Search query parameters
            
        Returns:
            Search results with relevant memories
        """
        try:
            # Build search filters
            filters = {}
            if query.user_id:
                filters["user_id"] = query.user_id
            if query.memory_types:
                filters["memory_type"] = query.memory_types  # Already a list of strings
            
            # Always use direct Qdrant search since mem0 uses local instance
            self.logger.debug(f"Searching memories with query: {query.query}, filters: {filters}")
            self.logger.info("Using direct cloud Qdrant search")
            results = await self._search_qdrant_directly(query)
            
            # Handle different response formats
            if isinstance(results, dict):
                # If results is a dict with a 'results' key
                if "results" in results:
                    results_list = results["results"]
                else:
                    # If it's a dict without 'results', wrap it in a list
                    results_list = [results]
            elif isinstance(results, list):
                results_list = results
            else:
                self.logger.warning(f"Unexpected search results format: {type(results)}")
                results_list = []
            
            # Filter by confidence threshold
            filtered_results = []
            for result in results_list:
                if isinstance(result, dict):
                    score = result.get("score", 0)
                    if score >= query.confidence_threshold:
                        filtered_results.append(result)
                elif isinstance(result, str):
                    # If result is just a string, wrap it
                    filtered_results.append({
                        "memory": result,
                        "score": 1.0
                    })
            
            # Get related entities from Neo4j for each memory
            enriched_results = []
            for result in filtered_results:
                memory_content = result.get("memory", "") if isinstance(result, dict) else str(result)
                
                # Extract potential entity names from the memory content
                # Look for common entities like cpu, memory, metrics, etc.
                entities_to_check = []
                content_lower = memory_content.lower()
                
                # Common monitoring entities
                common_entities = ["cpu", "memory", "disk", "network", "metrics", "prometheus", 
                                 "loki", "alertmanager", "logs", "alerts", "performance"]
                
                for entity in common_entities:
                    if entity in content_lower:
                        entities_to_check.append(entity)
                
                # Also check metadata for entities if available
                if isinstance(result, dict) and "metadata" in result:
                    metadata = result.get("metadata", {})
                    if "entities" in metadata:
                        entities_to_check.extend(metadata["entities"])
                
                # Get related entities for each found entity
                all_related_entities = []
                seen_entities = set()
                
                for entity in entities_to_check:
                    related = await self._get_related_entities(entity)
                    for rel in related:
                        rel_entity = rel.get("entity")
                        if rel_entity and rel_entity not in seen_entities:
                            seen_entities.add(rel_entity)
                            all_related_entities.append(rel)
                
                if isinstance(result, dict):
                    enriched_results.append({
                        **result,
                        "related_entities": all_related_entities
                    })
                else:
                    enriched_results.append({
                        "memory": str(result),
                        "score": 1.0,
                        "related_entities": all_related_entities
                    })
            
            return {
                "success": True,
                "total_results": len(enriched_results),
                "memories": enriched_results,
                "query": query.query
            }
            
        except Exception as e:
            self.logger.error(f"Failed to search memories: {str(e)}", exc_info=True)
            return {
                "success": False,
                "error": str(e)
            }
    
    async def search_all_memories(self, query: str, limit: int = 10, memory_types: Optional[List[str]] = None) -> Dict[str, Any]:
        """
        Search all memory sources (Mem0, Qdrant, Neo4j) for relevant memories.
        
        Args:
            query: Search query string
            limit: Maximum number of results per source
            memory_types: Filter by memory types
            
        Returns:
            Combined search results from all sources
        """
        try:
            # Create search query object
            search_query = MemorySearchQuery(
                query=query,
                limit=limit,
                memory_types=memory_types or ["instruction", "extracted", "incident", "resolution"],
                confidence_threshold=0.6
            )
            
            # Search main memories
            main_results = await self.search_memories(search_query)
            
            # Also search Neo4j for graph relationships
            neo4j_results = []
            if self.neo4j_driver:
                # Search for entities that match the query
                with self.neo4j_driver.session() as session:
                    result = session.run(
                        """
                        MATCH (n)
                        WHERE toLower(n.name) CONTAINS toLower($query) OR 
                              toLower(n.type) CONTAINS toLower($query) OR
                              toLower(n.description) CONTAINS toLower($query)
                        OPTIONAL MATCH (n)-[r]-(related)
                        RETURN n, collect({
                            relationship: type(r),
                            related_entity: related.name,
                            related_type: related.type
                        }) as relationships
                        LIMIT $limit
                        """,
                        query=query,
                        limit=limit
                    )
                    
                    for record in result:
                        node = record["n"]
                        relationships = record["relationships"]
                        neo4j_results.append({
                            "entity": node.get("name"),
                            "type": node.get("type"),
                            "description": node.get("description"),
                            "relationships": relationships,
                            "source": "neo4j"
                        })
            
            # Combine results
            all_memories = main_results.get("memories", [])
            
            # Add Neo4j results as a special memory type
            for neo4j_result in neo4j_results:
                all_memories.append({
                    "memory": f"Entity: {neo4j_result['entity']} ({neo4j_result['type']})",
                    "metadata": neo4j_result,
                    "score": 0.8,  # Fixed score for graph results
                    "memory_type": "graph_entity",
                    "source": "neo4j"
                })
            
            return {
                "success": True,
                "memories": all_memories,
                "total_results": len(all_memories)
            }
            
        except Exception as e:
            self.logger.error(f"Error in search_all_memories: {e}")
            return {
                "success": False,
                "error": str(e),
                "memories": []
            }
    
    async def get_contextual_memories(self, context: str, workflow_type: str, limit: int = 5) -> List[Dict[str, Any]]:
        """
        Get contextually relevant memories for a given situation.
        
        Args:
            context: Current context or situation
            workflow_type: Type of workflow (QUERY, ACTION, INCIDENT)
            limit: Maximum number of memories to return
            
        Returns:
            List of contextually relevant memories
        """
        try:
            # Search for relevant memories
            search_query = MemorySearchQuery(
                query=context,
                limit=limit,
                confidence_threshold=0.6
            )
            
            search_results = await self.search_memories(search_query)
            
            if not search_results["success"]:
                return []
            
            # Filter by workflow type relevance
            relevant_memories = []
            for memory in search_results["memories"]:
                metadata = memory.get("metadata", {})
                memory_workflow = metadata.get("workflow_type")
                
                # Include if same workflow type or general memories
                if not memory_workflow or memory_workflow == workflow_type:
                    relevant_memories.append(memory)
            
            return relevant_memories[:limit]
            
        except Exception as e:
            self.logger.error(f"Failed to get contextual memories: {str(e)}")
            return []
    
    async def _extract_memories_from_interaction(self, request: MemoryExtractionRequest) -> List[Dict[str, Any]]:
        """Extract important memories from workflow interaction using OpenAI."""
        try:
            prompt = get_memory_extraction_prompt(
                user_input=request.user_input,
                workflow_type=request.workflow_type,
                content=request.content,
                context=request.context
            )
            
            response = await openai.chat_completion(
                user_message=prompt,
                system_prompt=MEMORY_ANALYSIS_SYSTEM_PROMPT,
                temperature=0.1
            )
            
            self.logger.debug(f"OpenAI response for memory extraction: {response}")
            
            if not response["success"]:
                self.logger.error(f"Failed to extract memories: {response.get('error')}")
                return []
            
            # Handle case where response["content"] might be a string that needs parsing
            content = response.get("content", "[]")
            self.logger.debug(f"Content type: {type(content)}, Content: {content}")
            
            if isinstance(content, str):
                cleaned_content = content.strip()
                try:
                    # Clean up the content - remove any markdown code blocks
                    if cleaned_content.startswith("```json"):
                        cleaned_content = cleaned_content[7:]  # Remove ```json
                    if cleaned_content.startswith("```"):
                        cleaned_content = cleaned_content[3:]  # Remove ```
                    if cleaned_content.endswith("```"):
                        cleaned_content = cleaned_content[:-3]  # Remove trailing ```
                    cleaned_content = cleaned_content.strip()
                    
                    # Try to parse as JSON
                    parsed = json.loads(cleaned_content)
                    if isinstance(parsed, list):
                        return parsed
                    elif isinstance(parsed, dict):
                        # Handle case where OpenAI returns {memories: [...]}
                        if "memories" in parsed and isinstance(parsed["memories"], list):
                            self.logger.info("Extracted memories from dict format")
                            return parsed["memories"]
                        else:
                            self.logger.error(f"Expected list or dict with 'memories' key but got: {parsed}")
                            return []
                    else:
                        self.logger.error(f"Expected list but got {type(parsed)}: {parsed}")
                        return []
                except json.JSONDecodeError as e:
                    self.logger.error(f"Failed to parse content as JSON: {e}")
                    self.logger.error(f"Raw content: {content}")
                    self.logger.error(f"Cleaned content: {cleaned_content}")
                    return []
            elif isinstance(content, list):
                return content
            else:
                self.logger.error(f"Unexpected content type: {type(content)}")
                return []
                
        except json.JSONDecodeError as e:
            self.logger.error(f"Failed to parse memory extraction response: {e}")
            return []
        except Exception as e:
            self.logger.error(f"Error in _extract_memories_from_interaction: {e}", exc_info=True)
            return []
    
    async def _extract_relationships(self, content: str) -> List[GraphRelationship]:
        """Extract relationships from content using OpenAI."""
        try:
            prompt = get_relationship_extraction_prompt(content)
            
            response = await openai.chat_completion(
                user_message=prompt,
                system_prompt=RELATIONSHIP_ANALYSIS_SYSTEM_PROMPT,
                temperature=0.1
            )
            
            self.logger.debug(f"OpenAI response for relationship extraction: {response}")
            
            if not response["success"]:
                self.logger.error(f"Failed to extract relationships: {response.get('error')}")
                return []
            
            # Handle different response formats
            content_data = response.get("content", "[]")
            if isinstance(content_data, str):
                parsed_data = json.loads(content_data)
                # Check if the response has a "relationships" key
                if isinstance(parsed_data, dict) and "relationships" in parsed_data:
                    relationships_data = parsed_data["relationships"]
                else:
                    relationships_data = parsed_data
            elif isinstance(content_data, list):
                relationships_data = content_data
            else:
                self.logger.error(f"Unexpected content type for relationships: {type(content_data)}")
                return []
            
            relationships = []
            
            for rel_data in relationships_data:
                if isinstance(rel_data, dict) and all(key in rel_data for key in ["source", "relationship", "target"]):
                    relationship = GraphRelationship(
                        source=rel_data["source"],
                        relationship=rel_data["relationship"],
                        target=rel_data["target"],
                        properties=rel_data.get("properties", {})
                    )
                    relationships.append(relationship)
                else:
                    self.logger.warning(f"Invalid relationship data format: {rel_data}")
            
            return relationships
        except json.JSONDecodeError as e:
            self.logger.error(f"Failed to parse relationships JSON: {str(e)}")
            return []
        except KeyError as e:
            self.logger.error(f"Missing key in relationships data: {str(e)}")
            return []
        except Exception as e:
            self.logger.error(f"Error in _extract_relationships: {str(e)}", exc_info=True)
            return []
    
    async def _store_relationship(self, relationship: GraphRelationship):
        """Store a relationship in Neo4j."""
        try:
            cypher_query = relationship.to_cypher_create()
            
            with self.neo4j_driver.session() as session:
                session.run(cypher_query)
            
            self.logger.debug(f"Stored relationship: {relationship.source} -[{relationship.relationship}]-> {relationship.target}")
            
        except Exception as e:
            self.logger.error(f"Failed to store relationship: {str(e)}")
    
    async def _get_related_entities(self, entity: str) -> List[Dict[str, Any]]:
        """Get related entities from Neo4j for a given entity."""
        try:
            cypher_query = """
            MATCH (e:Entity {name: $entity})-[r]->(related:Entity)
            RETURN related.name as entity, type(r) as relationship, r as properties
            LIMIT 10
            """
            
            with self.neo4j_driver.session() as session:
                result = session.run(cypher_query, entity=entity)
                
                related_entities = []
                for record in result:
                    related_entities.append({
                        "entity": record["entity"],
                        "relationship": record["relationship"],
                        "properties": dict(record["properties"]) if record["properties"] else {}
                    })
                
                return related_entities
                
        except Exception as e:
            self.logger.error(f"Failed to get related entities: {str(e)}")
            return []
    
    async def _search_qdrant_directly(self, query: MemorySearchQuery) -> Dict[str, Any]:
        """Search Qdrant directly when mem0 search fails or returns no results."""
        try:
            from openai import AsyncOpenAI
            openai_client = AsyncOpenAI(api_key=os.getenv("OPENAI_API_KEY"))
            
            # Create embedding for the query
            embedding_response = await openai_client.embeddings.create(
                model=self.mem0_config["embedding_model"],
                input=query.query,
                dimensions=self.mem0_config["vector_size"]
            )
            
            query_embedding = embedding_response.data[0].embedding
            
            # Always use our cloud Qdrant client
            qdrant_client = self.qdrant_client
            collection_name = self.mem0_config["collection_name"]
            
            try:
                # Check if collection exists
                try:
                    collection_info = qdrant_client.get_collection(collection_name)
                    self.logger.debug(f"Collection {collection_name} exists with {collection_info.points_count} points")
                except Exception as e:
                    self.logger.warning(f"Collection {collection_name} might not exist: {str(e)}")
                    # Try to create collection if it doesn't exist
                    await self._create_qdrant_collection_if_not_exists()
                
                # Lower threshold for better recall on partial matches
                adjusted_threshold = max(0.2, query.confidence_threshold * 0.6)
                
                # Use query_points instead of deprecated search method
                query_response = qdrant_client.query_points(
                    collection_name=collection_name,
                    query=query_embedding,
                    limit=query.limit * 3,  # Get more results for filtering
                    score_threshold=adjusted_threshold,
                    with_payload=True,  # Ensure we get the payload data
                    with_vectors=False  # We don't need vectors in response
                )
                
                # Format results to match mem0 format
                results = []
                
                # Extract keywords from query for additional filtering
                query_keywords = set(query.query.lower().split())
                
                # Handle the response - query_points returns a list of points
                search_result = query_response.points if hasattr(query_response, 'points') else query_response
                
                for hit in search_result:
                    payload = hit.payload or {}
                    # mem0 stores the memory text in 'text' field
                    memory_text = payload.get("memory") or payload.get("text", "")
                    
                    # Boost score if keywords match
                    memory_lower = memory_text.lower()
                    keyword_matches = sum(1 for keyword in query_keywords if keyword in memory_lower)
                    keyword_boost = keyword_matches * 0.1
                    
                    # Adjust score with keyword boost
                    adjusted_score = min(1.0, hit.score + keyword_boost)
                    
                    results.append({
                        "id": str(hit.id),
                        "memory": memory_text,
                        "score": adjusted_score,
                        "metadata": payload.get("metadata", {}),
                        "memory_type": payload.get("memory_type"),
                        "user_id": payload.get("user_id") or payload.get("user", {}).get("id"),
                        "created_at": payload.get("created_at") or payload.get("created_at_fmt")
                    })
                
                # Remove duplicates based on memory content
                seen_memories = set()
                unique_results = []
                for result in results:
                    memory_text = result["memory"]
                    if memory_text not in seen_memories:
                        seen_memories.add(memory_text)
                        unique_results.append(result)
                
                # Sort by score and limit to requested number
                results = sorted(unique_results, key=lambda x: x["score"], reverse=True)[:query.limit]
                
                # If no results, try a keyword-based fallback search
                if not results and query_keywords:
                    self.logger.info("No vector search results, trying keyword fallback")
                    results = await self._keyword_fallback_search(query, qdrant_client, collection_name)
                
                return {"results": results}
                
            except Exception as e:
                self.logger.warning(f"Qdrant collection might not exist: {e}")
                return {"results": []}
                
        except Exception as e:
            self.logger.error(f"Direct Qdrant search failed: {e}", exc_info=True)
            return {"results": []}
    
    async def _keyword_fallback_search(self, query: MemorySearchQuery, qdrant_client, collection_name: str) -> List[Dict[str, Any]]:
        """Fallback search using keyword matching when vector search fails."""
        try:
            # Get all points from collection (with pagination if needed)
            all_points = []
            offset = None
            
            while True:
                result = qdrant_client.scroll(
                    collection_name=collection_name,
                    limit=100,
                    offset=offset
                )
                
                points, next_offset = result
                all_points.extend(points)
                
                if next_offset is None:
                    break
                offset = next_offset
            
            # Filter by keywords
            query_keywords = set(query.query.lower().split())
            matching_results = []
            
            for point in all_points:
                payload = point.payload or {}
                memory_text = payload.get("memory") or payload.get("text", "")
                memory_lower = memory_text.lower()
                
                # Check if any keyword matches
                keyword_matches = sum(1 for keyword in query_keywords if keyword in memory_lower)
                
                if keyword_matches > 0:
                    # Calculate a simple relevance score
                    relevance_score = keyword_matches / len(query_keywords)
                    
                    matching_results.append({
                        "id": str(point.id),
                        "memory": memory_text,
                        "score": relevance_score,
                        "metadata": payload.get("metadata", {}),
                        "memory_type": payload.get("memory_type"),
                        "user_id": payload.get("user_id"),
                        "created_at": payload.get("created_at")
                    })
            
            # Sort by relevance and limit
            matching_results = sorted(matching_results, key=lambda x: x["score"], reverse=True)[:query.limit]
            
            return matching_results
            
        except Exception as e:
            self.logger.error(f"Keyword fallback search failed: {e}")
            return []
    
    async def _store_memory_in_qdrant(self, memory_entry: MemoryEntry, memory_id: str):
        """Store memory directly in Qdrant when mem0 fails."""
        try:
            from qdrant_client.models import PointStruct, VectorParams, Distance
            import numpy as np
            
            # Get embeddings using OpenAI
            # Import the OpenAI client directly
            from openai import AsyncOpenAI
            openai_client = AsyncOpenAI(api_key=os.getenv("OPENAI_API_KEY"))
            
            # Create embedding for the memory content
            embedding_response = await openai_client.embeddings.create(
                model=self.mem0_config["embedding_model"],
                input=memory_entry.content,
                dimensions=self.mem0_config["vector_size"]
            )
            
            embedding = embedding_response.data[0].embedding
            
            # Always use our cloud Qdrant client
            qdrant_client = self.qdrant_client
            collection_name = self.mem0_config["collection_name"]
            
            # Ensure collection exists
            try:
                qdrant_client.get_collection(collection_name)
            except Exception:
                # Create collection if it doesn't exist
                self.logger.info(f"Creating collection {collection_name}")
                qdrant_client.create_collection(
                    collection_name=collection_name,
                    vectors_config=VectorParams(
                        size=self.mem0_config["vector_size"],
                        distance=Distance.COSINE
                    )
                )
            
            # Store in Qdrant
            point = PointStruct(
                id=memory_id,
                vector=embedding,
                payload={
                    "memory": memory_entry.content,
                    "memory_type": memory_entry.memory_type,  # Now it's already a string
                    "user_id": memory_entry.user_id or "system",
                    "created_at": memory_entry.created_at.isoformat(),
                    "metadata": memory_entry.metadata
                }
            )
            
            qdrant_client.upsert(
                collection_name=collection_name,
                points=[point]
            )
            
            self.logger.info(f"Successfully stored memory {memory_id} directly in Qdrant")
            
        except Exception as e:
            self.logger.error(f"Failed to store memory in Qdrant: {str(e)}", exc_info=True)
    
    async def _create_qdrant_collection_if_not_exists(self):
        """Create Qdrant collection if it doesn't exist."""
        try:
            from qdrant_client.models import VectorParams, Distance
            
            collection_name = self.mem0_config["collection_name"]
            vector_size = self.mem0_config["vector_size"]
            
            # Check if collection exists
            try:
                self.qdrant_client.get_collection(collection_name)
                self.logger.info(f"Collection {collection_name} already exists")
                return
            except Exception:
                # Collection doesn't exist, create it
                self.logger.info(f"Creating collection {collection_name}")
                
                self.qdrant_client.create_collection(
                    collection_name=collection_name,
                    vectors_config=VectorParams(
                        size=vector_size,
                        distance=Distance.COSINE
                    )
                )
                
                self.logger.info(f"Successfully created collection {collection_name}")
                
        except Exception as e:
            self.logger.error(f"Failed to create Qdrant collection: {str(e)}")
    
    def close(self):
        """Close all database connections."""
        try:
            if hasattr(self, 'neo4j_driver'):
                self.neo4j_driver.close()
            if hasattr(self, 'qdrant_client'):
                self.qdrant_client.close()
            self.logger.info("Memory service connections closed")
        except Exception as e:
            self.logger.error(f"Error closing memory service: {str(e)}")


# Global memory service instance (lazy initialization)
memory_service = None

def get_memory_service() -> MemoryService:
    """Get the global memory service instance with lazy initialization."""
    global memory_service
    if memory_service is None:
        memory_service = MemoryService()
    return memory_service