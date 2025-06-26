"""
Pydantic Models for PaladinAI LangGraph Workflow State Management.

This module defines the core data models used throughout the workflow
system for state management, results, and configuration.
"""

from typing import Dict, List, Optional, Any
from pydantic import BaseModel, Field
from datetime import datetime

from .enums import WorkflowType, ComplexityLevel, NodeStatus, ExecutionPhase


class CategorizationResult(BaseModel):
    """
    Result of workflow categorization analysis.
    
    This model contains the output from the AI categorization process,
    including the determined workflow type and associated metadata.
    """
    workflow_type: WorkflowType = Field(description="Categorized workflow type")
    confidence: float = Field(ge=0.0, le=1.0, description="Confidence score for categorization")
    reasoning: str = Field(description="Explanation of categorization decision")
    suggested_approach: str = Field(description="Recommended approach for handling request")
    estimated_complexity: ComplexityLevel = Field(description="Estimated complexity level")
    
    # Additional categorization metadata
    keywords_detected: List[str] = Field(default_factory=list, description="Key terms that influenced categorization")
    alternative_types: List[Dict[str, float]] = Field(default_factory=list, description="Alternative workflow types with confidence scores")
    processing_hints: Dict[str, Any] = Field(default_factory=dict, description="Hints for downstream processing")
    
    class Config:
        """Pydantic configuration."""
        use_enum_values = True


class NodeResult(BaseModel):
    """
    Result returned by individual workflow nodes.
    
    This model standardizes the output format from all workflow nodes
    to ensure consistent data flow and error handling.
    """
    success: bool = Field(description="Whether node execution was successful")
    status: NodeStatus = Field(default=NodeStatus.COMPLETED, description="Node execution status")
    data: Optional[Dict[str, Any]] = Field(default=None, description="Node output data")
    error: Optional[str] = Field(default=None, description="Error message if node failed")
    next_node: Optional[str] = Field(default=None, description="Next node to execute")
    
    # Execution metadata
    execution_time_ms: Optional[int] = Field(default=None, description="Node execution time in milliseconds")
    retry_count: int = Field(default=0, description="Number of retries attempted")
    metadata: Dict[str, Any] = Field(default_factory=dict, description="Additional node metadata")
    
    class Config:
        """Pydantic configuration."""
        use_enum_values = True


class WorkflowState(BaseModel):
    """
    Main state object for LangGraph workflow execution.
    
    This state is passed between nodes and maintains the complete
    context of the workflow execution, including input, processing
    state, results, and metadata.
    """
    # Input data
    user_input: str = Field(description="Original user input/query")
    session_id: Optional[str] = Field(default=None, description="Session identifier for tracking")
    timestamp: datetime = Field(default_factory=datetime.now, description="Workflow start timestamp")
    
    # Categorization results
    categorization: Optional[CategorizationResult] = Field(default=None, description="Workflow categorization result")
    
    # Processing state
    current_node: str = Field(default="start", description="Current node in workflow execution")
    current_phase: ExecutionPhase = Field(default=ExecutionPhase.INITIALIZATION, description="Current execution phase")
    execution_path: List[str] = Field(default_factory=list, description="Path of executed nodes")
    
    # Node results tracking
    node_results: Dict[str, NodeResult] = Field(default_factory=dict, description="Results from individual nodes")
    
    # Results and output
    final_result: Optional[Dict[str, Any]] = Field(default=None, description="Final workflow result")
    error_message: Optional[str] = Field(default=None, description="Error message if workflow fails")
    
    # Tracing and monitoring
    trace_id: Optional[str] = Field(default=None, description="Langfuse trace identifier")
    execution_time_ms: Optional[int] = Field(default=None, description="Total execution time in milliseconds")
    
    # Performance metrics
    total_api_calls: int = Field(default=0, description="Total number of API calls made")
    total_tokens_used: int = Field(default=0, description="Total tokens consumed")
    
    # Additional context and metadata
    metadata: Dict[str, Any] = Field(default_factory=dict, description="Additional metadata and context")
    user_context: Dict[str, Any] = Field(default_factory=dict, description="User-specific context information")
    
    class Config:
        """Pydantic configuration."""
        arbitrary_types_allowed = True
        use_enum_values = True
        json_encoders = {
            datetime: lambda v: v.isoformat()
        }
    
    def add_node_result(self, node_name: str, result: NodeResult) -> None:
        """
        Add a node result to the state.
        
        Args:
            node_name: Name of the node
            result: Result from the node execution
        """
        self.node_results[node_name] = result
        
        # Update performance metrics
        if result.execution_time_ms:
            if self.execution_time_ms is None:
                self.execution_time_ms = 0
            self.execution_time_ms += result.execution_time_ms
        
        # Update API call count if available in metadata
        if "api_calls" in result.metadata:
            self.total_api_calls += result.metadata["api_calls"]
        
        # Update token usage if available in metadata
        if "tokens_used" in result.metadata:
            self.total_tokens_used += result.metadata["tokens_used"]
    
    def get_node_result(self, node_name: str) -> Optional[NodeResult]:
        """
        Get the result for a specific node.
        
        Args:
            node_name: Name of the node
            
        Returns:
            NodeResult if available, None otherwise
        """
        return self.node_results.get(node_name)
    
    def has_errors(self) -> bool:
        """
        Check if the workflow has encountered any errors.
        
        Returns:
            True if there are errors in the workflow
        """
        if self.error_message:
            return True
        
        return any(
            result.status == NodeStatus.FAILED 
            for result in self.node_results.values()
        )
    
    def get_execution_summary(self) -> Dict[str, Any]:
        """
        Get a summary of the workflow execution.
        
        Returns:
            Dictionary containing execution summary
        """
        return {
            "session_id": self.session_id,
            "execution_path": self.execution_path,
            "current_phase": self.current_phase,
            "total_nodes": len(self.node_results),
            "successful_nodes": sum(1 for r in self.node_results.values() if r.success),
            "failed_nodes": sum(1 for r in self.node_results.values() if not r.success),
            "total_execution_time_ms": self.execution_time_ms,
            "total_api_calls": self.total_api_calls,
            "total_tokens_used": self.total_tokens_used,
            "has_errors": self.has_errors(),
            "categorization_type": self.categorization.workflow_type if self.categorization else None,
            "categorization_confidence": self.categorization.confidence if self.categorization else None
        }


class GraphConfig(BaseModel):
    """
    Configuration for LangGraph workflow execution.
    
    This model defines the configuration parameters that control
    workflow behavior, performance, and integrations.
    """
    # Execution settings
    max_execution_time: int = Field(default=300, description="Maximum execution time in seconds")
    retry_attempts: int = Field(default=3, description="Number of retry attempts for failed nodes")
    enable_tracing: bool = Field(default=True, description="Enable Langfuse tracing")
    
    # Performance settings
    max_concurrent_nodes: int = Field(default=1, description="Maximum number of nodes to execute concurrently")
    node_timeout: int = Field(default=60, description="Timeout for individual node execution in seconds")
    
    # Langfuse configuration
    langfuse_public_key: Optional[str] = Field(default=None, description="Langfuse public key")
    langfuse_secret_key: Optional[str] = Field(default=None, description="Langfuse secret key")
    langfuse_host: Optional[str] = Field(default=None, description="Langfuse host URL")
    
    # OpenAI configuration
    openai_model: str = Field(default="gpt-4o", description="OpenAI model to use")
    openai_temperature: float = Field(default=0.1, description="Temperature for OpenAI calls")
    openai_max_tokens: int = Field(default=4000, description="Maximum tokens for OpenAI responses")
    
    # Workflow behavior
    enable_streaming: bool = Field(default=True, description="Enable workflow streaming")
    save_intermediate_results: bool = Field(default=True, description="Save results from intermediate nodes")
    
    class Config:
        """Pydantic configuration."""
        validate_assignment = True
