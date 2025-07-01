"""
Memory Extractor for PaladinAI workflows.

This module provides utilities for extracting and storing memories
from workflow interactions automatically.
"""

import logging
from typing import Dict, Any, List, Optional
from datetime import datetime

from .models import MemoryExtractionRequest
from llm.openai import openai

logger = logging.getLogger(__name__)


class MemoryExtractor:
    """
    Utility class for extracting memories from workflow executions.
    
    This class analyzes workflow states and interactions to identify
    valuable information that should be stored as long-term memory.
    """
    
    def __init__(self):
        """Initialize the memory extractor."""
        self.logger = logging.getLogger(f"{__name__}.{self.__class__.__name__}")
    
    async def extract_from_workflow_state(
        self,
        state: Dict[str, Any],
        user_input: str,
        workflow_type: str,
        user_id: Optional[str] = None,
        session_id: Optional[str] = None
    ) -> Dict[str, Any]:
        """
        Extract memories from a complete workflow state.
        
        Args:
            state: Complete workflow state
            user_input: Original user input
            workflow_type: Type of workflow (QUERY, ACTION, INCIDENT)
            user_id: Optional user identifier
            session_id: Optional session identifier
            
        Returns:
            Memory extraction results
        """
        try:
            # Determine if extraction is needed
            if not await self._should_extract_memories(state, user_input, workflow_type):
                return {"success": True, "memories_stored": 0, "reason": "no_extraction_needed"}
            
            # Build content for extraction
            content = self._build_extraction_content(state)
            
            # Create extraction request
            extraction_request = MemoryExtractionRequest(
                content=content,
                user_input=user_input,
                workflow_type=workflow_type,
                user_id=user_id,
                session_id=session_id,
                context={
                    "workflow_nodes": list(state.get("metadata", {}).keys()),
                    "has_prometheus_data": bool(state.get("metadata", {}).get("prometheus_data")),
                    "has_loki_data": bool(state.get("metadata", {}).get("loki_data")),
                    "has_alert_data": bool(state.get("metadata", {}).get("alertmanager_data")),
                    "extraction_timestamp": datetime.now().isoformat()
                }
            )
            
            # Extract and store memories
            from .service import get_memory_service
            result = await get_memory_service().extract_and_store_memories(extraction_request)
            
            self.logger.info(f"Memory extraction completed: {result.get('memories_stored', 0)} memories stored")
            
            return result
            
        except Exception as e:
            self.logger.error(f"Failed to extract memories from workflow state: {str(e)}")
            return {"success": False, "error": str(e)}
    
    async def extract_from_error(
        self,
        error_message: str,
        user_input: str,
        workflow_type: str,
        context: Optional[Dict[str, Any]] = None,
        user_id: Optional[str] = None
    ) -> Dict[str, Any]:
        """
        Extract memories from workflow errors for learning.
        
        Args:
            error_message: Error that occurred
            user_input: Original user input
            workflow_type: Type of workflow
            context: Additional context about the error
            user_id: Optional user identifier
            
        Returns:
            Memory extraction results
        """
        try:
            # Build error context
            error_content = f"""
            Error occurred during {workflow_type} workflow:
            User Input: {user_input}
            Error: {error_message}
            Context: {context or {}}
            """
            
            # Create extraction request
            extraction_request = MemoryExtractionRequest(
                content=error_content,
                user_input=user_input,
                workflow_type=workflow_type,
                user_id=user_id,
                context={
                    "error_type": "workflow_error",
                    "error_message": error_message,
                    "error_context": context or {},
                    "extraction_timestamp": datetime.now().isoformat()
                }
            )
            
            # Extract and store error memories
            from .service import get_memory_service
            result = await get_memory_service().extract_and_store_memories(extraction_request)
            
            self.logger.info(f"Error memory extraction completed: {result.get('memories_stored', 0)} memories stored")
            
            return result
            
        except Exception as e:
            self.logger.error(f"Failed to extract memories from error: {str(e)}")
            return {"success": False, "error": str(e)}
    
    async def get_contextual_memories_for_workflow(
        self,
        user_input: str,
        workflow_type: str,
        limit: int = 5
    ) -> List[Dict[str, Any]]:
        """
        Get relevant memories for a workflow based on user input.
        
        Args:
            user_input: User's input/query
            workflow_type: Type of workflow
            limit: Maximum number of memories to return
            
        Returns:
            List of relevant memories
        """
        try:
            # Get contextual memories
            from .service import get_memory_service
            memories = await get_memory_service().get_contextual_memories(
                context=user_input,
                workflow_type=workflow_type,
                limit=limit
            )
            
            self.logger.debug(f"Retrieved {len(memories)} contextual memories for {workflow_type}")
            
            return memories
            
        except Exception as e:
            self.logger.error(f"Failed to get contextual memories: {str(e)}")
            return []
    
    async def _should_extract_memories(
        self,
        state: Dict[str, Any],
        user_input: str,
        workflow_type: str
    ) -> bool:
        """
        Determine if memories should be extracted from this workflow state.
        
        Args:
            state: Workflow state
            user_input: User input
            workflow_type: Workflow type
            
        Returns:
            True if memories should be extracted
        """
        # Check if there's substantial data to extract from
        metadata = state.get("metadata", {})
        
        # Extract if we have data from multiple sources
        data_sources = 0
        if metadata.get("prometheus_data"):
            data_sources += 1
        if metadata.get("loki_data"):
            data_sources += 1
        if metadata.get("alertmanager_data"):
            data_sources += 1
        
        # Extract if we have analysis results
        has_analysis = bool(
            metadata.get("prometheus_results") or
            metadata.get("loki_results") or
            metadata.get("alertmanager_results")
        )
        
        # Extract for incidents or complex actions
        is_complex_workflow = workflow_type in ["INCIDENT", "ACTION"] or data_sources > 1
        
        # Use AI to make final decision
        if has_analysis or is_complex_workflow:
            return await self._ai_should_extract(state, user_input, workflow_type)
        
        return False
    
    async def _ai_should_extract(
        self,
        state: Dict[str, Any],
        user_input: str,
        workflow_type: str
    ) -> bool:
        """Use AI to determine if memories should be extracted."""
        prompt = f"""
        Analyze this workflow execution and determine if valuable memories should be extracted and stored.
        
        User Input: {user_input}
        Workflow Type: {workflow_type}
        
        State Summary:
        - Has Prometheus data: {bool(state.get("metadata", {}).get("prometheus_data"))}
        - Has Loki data: {bool(state.get("metadata", {}).get("loki_data"))}
        - Has Alert data: {bool(state.get("metadata", {}).get("alertmanager_data"))}
        - Has analysis results: {bool(state.get("metadata", {}).get("prometheus_results") or state.get("metadata", {}).get("loki_results"))}
        
        Should memories be extracted? Consider:
        1. Is this a complex troubleshooting scenario?
        2. Are there operational patterns worth remembering?
        3. Would this information help with similar future requests?
        4. Is there enough substantial data to warrant extraction?
        
        Respond with only "true" or "false".
        """
        
        try:
            response = await openai.chat_completion(
                user_message=prompt,
                system_prompt="You are an expert at determining when operational memories should be stored for future reference.",
                temperature=0.1
            )
            
            if response["success"]:
                return response["content"].strip().lower() == "true"
            
        except Exception as e:
            self.logger.error(f"AI memory extraction decision failed: {str(e)}")
        
        # Default to extraction for complex workflows
        return workflow_type in ["INCIDENT", "ACTION"]
    
    def _build_extraction_content(self, state: Dict[str, Any]) -> str:
        """Build content string for memory extraction."""
        metadata = state.get("metadata", {})
        content_parts = []
        
        # Add final result if available
        if state.get("final_result"):
            content_parts.append(f"Final Result: {state['final_result']}")
        
        # Add analysis results
        if metadata.get("prometheus_results"):
            content_parts.append(f"Prometheus Analysis: {metadata['prometheus_results']}")
        
        if metadata.get("loki_results"):
            content_parts.append(f"Loki Analysis: {metadata['loki_results']}")
        
        if metadata.get("alertmanager_results"):
            content_parts.append(f"Alertmanager Analysis: {metadata['alertmanager_results']}")
        
        # Add data summaries
        if metadata.get("prometheus_data"):
            prom_data = metadata["prometheus_data"]
            content_parts.append(f"Prometheus Data: {len(prom_data.get('metrics', []))} metrics collected")
        
        if metadata.get("loki_data"):
            loki_data = metadata["loki_data"]
            content_parts.append(f"Loki Data: {len(loki_data.get('logs', []))} logs collected")
        
        if metadata.get("alertmanager_data"):
            alert_data = metadata["alertmanager_data"]
            content_parts.append(f"Alert Data: {len(alert_data.get('alerts', []))} alerts collected")
        
        return "\n\n".join(content_parts)


# Global memory extractor instance
memory_extractor = MemoryExtractor()