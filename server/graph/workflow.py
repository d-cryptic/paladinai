"""
LangGraph Workflow Orchestrator for PaladinAI.

This module implements the main workflow orchestrator using LangGraph
for categorizing user input and routing to appropriate handlers.
"""

import logging
import os
from typing import Dict, Any, Optional
from datetime import datetime

from langgraph.graph import StateGraph, END
from langfuse import observe

from .state import WorkflowState, GraphConfig, create_initial_state
from .nodes import (
    start_node, guardrail_node, categorization_node,
    query_node, action_node, incident_node, prometheus_node,
    result_node, error_handler_node
)

logger = logging.getLogger(__name__)


class PaladinWorkflow:
    """
    Main workflow orchestrator for PaladinAI using LangGraph.
    
    This class manages the workflow execution from user input
    through categorization to final result delivery.
    """
    
    def __init__(self, config: Optional[GraphConfig] = None):
        """
        Initialize the workflow orchestrator.

        Args:
            config: Optional configuration for the workflow
        """
        self.config = config or self._load_default_config()
        self.graph = self._build_graph()

        logger.info("PaladinWorkflow initialized successfully")
    
    def _load_default_config(self) -> GraphConfig:
        """
        Load default configuration from environment variables.

        Returns:
            Default GraphConfig object
        """
        return GraphConfig(
            max_execution_time=int(os.getenv("MAX_EXECUTION_TIME", "300")),
            retry_attempts=int(os.getenv("RETRY_ATTEMPTS", "3")),
            enable_tracing=os.getenv("ENABLE_TRACING", "true").lower() == "true",
            langfuse_public_key=os.getenv("LANGFUSE_PUBLIC_KEY"),
            langfuse_secret_key=os.getenv("LANGFUSE_SECRET_KEY"),
            langfuse_host=os.getenv("LANGFUSE_HOST")
        )
    
    def _build_graph(self):
        """Build the LangGraph workflow graph."""
        # Create state graph
        workflow = StateGraph(WorkflowState)

        # Add nodes
        workflow.add_node("start", self._start_node_wrapper)
        workflow.add_node("guardrail", self._guardrail_node_wrapper)
        workflow.add_node("categorize", self._categorization_node_wrapper)
        workflow.add_node("query", self._query_node_wrapper)
        workflow.add_node("action", self._action_node_wrapper)
        workflow.add_node("incident", self._incident_node_wrapper)
        workflow.add_node("prometheus", self._prometheus_node_wrapper)
        workflow.add_node("result", self._result_node_wrapper)
        workflow.add_node("error_handler", self._error_handler_node_wrapper)

        # Define edges
        workflow.set_entry_point("start")

        # Conditional edges based on node results
        workflow.add_conditional_edges(
            "start",
            self._route_from_start,
            {
                "guardrail": "guardrail",
                "error_handler": "error_handler"
            }
        )

        workflow.add_conditional_edges(
            "guardrail",
            self._route_from_guardrail,
            {
                "categorize": "categorize",
                "result": "result",
                "error_handler": "error_handler"
            }
        )

        workflow.add_conditional_edges(
            "categorize",
            self._route_from_categorization,
            {
                "query": "query",
                "action": "action",
                "incident": "incident",
                "result": "result",
                "error_handler": "error_handler"
            }
        )

        # Add routing from workflow nodes to prometheus and back
        workflow.add_conditional_edges(
            "query",
            self._route_from_query,
            {
                "prometheus": "prometheus",
                "query_output": "result",
                "error_handler": "error_handler"
            }
        )

        workflow.add_conditional_edges(
            "action",
            self._route_from_action,
            {
                "prometheus": "prometheus",
                "action_output": "result",
                "error_handler": "error_handler"
            }
        )

        workflow.add_conditional_edges(
            "incident",
            self._route_from_incident,
            {
                "prometheus": "prometheus",
                "incident_output": "result",
                "error_handler": "error_handler"
            }
        )

        workflow.add_conditional_edges(
            "prometheus",
            self._route_from_prometheus,
            {
                "query_prometheus_return": "query",
                "action_prometheus_return": "action",
                "incident_prometheus_return": "incident",
                "error_handler": "error_handler"
            }
        )

        # Terminal nodes
        workflow.add_edge("result", END)
        workflow.add_edge("error_handler", END)

        # Compile graph without checkpointing for now
        compiled_graph = workflow.compile()

        logger.info("LangGraph workflow compiled successfully")
        return compiled_graph
    
    async def _start_node_wrapper(self, state: WorkflowState) -> WorkflowState:
        """Wrapper for start node execution."""
        return await start_node.execute(state)

    async def _guardrail_node_wrapper(self, state: WorkflowState) -> WorkflowState:
        """Wrapper for guardrail node execution."""
        return await guardrail_node.execute(state)

    async def _categorization_node_wrapper(self, state: WorkflowState) -> WorkflowState:
        """Wrapper for categorization node execution."""
        return await categorization_node.execute(state)
    
    async def _result_node_wrapper(self, state: WorkflowState) -> WorkflowState:
        """Wrapper for result node execution."""
        return await result_node.execute(state)
    
    async def _error_handler_node_wrapper(self, state: WorkflowState) -> WorkflowState:
        """Wrapper for error handler node execution."""
        return await error_handler_node.execute(state)

    async def _query_node_wrapper(self, state: WorkflowState) -> WorkflowState:
        """Wrapper for query node execution."""
        # Check if returning from prometheus
        if state.metadata.get("prometheus_collection_complete"):
            prometheus_data = state.metadata.get("prometheus_data", {})
            return await query_node.process_prometheus_result(state, prometheus_data)
        else:
            return await query_node.execute(state)

    async def _action_node_wrapper(self, state: WorkflowState) -> WorkflowState:
        """Wrapper for action node execution."""
        # Check if returning from prometheus
        if state.metadata.get("prometheus_collection_complete"):
            prometheus_data = state.metadata.get("prometheus_data", {})
            return await action_node.process_prometheus_result(state, prometheus_data)
        else:
            return await action_node.execute(state)

    async def _incident_node_wrapper(self, state: WorkflowState) -> WorkflowState:
        """Wrapper for incident node execution."""
        # Check if returning from prometheus
        if state.metadata.get("prometheus_collection_complete"):
            prometheus_data = state.metadata.get("prometheus_data", {})
            return await incident_node.process_prometheus_result(state, prometheus_data)
        else:
            return await incident_node.execute(state)

    async def _prometheus_node_wrapper(self, state: WorkflowState) -> WorkflowState:
        """Wrapper for prometheus node execution."""
        return await prometheus_node.execute(state)
    
    def _route_from_start(self, state: WorkflowState) -> str:
        """Route from start node based on state."""
        return start_node.get_next_node(state)

    def _route_from_guardrail(self, state: WorkflowState) -> str:
        """Route from guardrail node based on state."""
        return guardrail_node.get_next_node(state)

    def _route_from_categorization(self, state: WorkflowState) -> str:
        """Route from categorization node based on state."""
        return categorization_node.get_next_node(state)

    def _route_from_query(self, state: WorkflowState) -> str:
        """Route from query node based on state."""
        return query_node.get_next_node(state)

    def _route_from_action(self, state: WorkflowState) -> str:
        """Route from action node based on state."""
        return action_node.get_next_node(state)

    def _route_from_incident(self, state: WorkflowState) -> str:
        """Route from incident node based on state."""
        return incident_node.get_next_node(state)

    def _route_from_prometheus(self, state: WorkflowState) -> str:
        """Route from prometheus node based on state."""
        return prometheus_node.get_next_node(state)
    
    @observe(name="workflow_execution")
    async def execute(
        self,
        user_input: str,
        session_id: Optional[str] = None
    ) -> Dict[str, Any]:
        """
        Execute the workflow for a given user input.
        
        Args:
            user_input: The user's input/query to process
            session_id: Optional session identifier for tracking
            config: Optional execution configuration
            
        Returns:
            Dictionary containing the workflow results
        """
        logger.info(f"Executing workflow for input: {user_input[:100]}...")
        
        try:
            # Create initial state
            initial_state = create_initial_state(user_input, session_id)
            
            # Execute workflow
            final_state = await self.graph.ainvoke(initial_state)
            
            # Return final result - access from AddableValuesDict
            if hasattr(final_state, 'final_result') and final_state.final_result:
                return final_state.final_result
            elif 'final_result' in final_state and final_state['final_result']:
                return final_state['final_result']
            else:
                # Fallback if no final result was set
                execution_path = getattr(final_state, 'execution_path', final_state.get('execution_path', []))
                return {
                    "success": False,
                    "error": "Workflow completed but no result was generated",
                    "session_id": session_id,
                    "execution_path": execution_path
                }
                
        except Exception as e:
            error_msg = f"Workflow execution failed: {str(e)}"
            logger.error(error_msg, exc_info=True)
            
            return {
                "success": False,
                "error": error_msg,
                "session_id": session_id,
                "timestamp": datetime.now().isoformat()
            }
    
    @observe(name="workflow_stream")
    async def stream(
        self,
        user_input: str,
        session_id: Optional[str] = None
    ):
        """
        Stream workflow execution for real-time updates.
        
        Args:
            user_input: The user's input/query to process
            session_id: Optional session identifier for tracking
            config: Optional execution configuration
            
        Yields:
            Workflow state updates as they occur
        """
        logger.info(f"Streaming workflow for input: {user_input[:100]}...")
        
        try:
            # Create initial state
            initial_state = create_initial_state(user_input, session_id)
            
            # Stream workflow execution
            async for state in self.graph.astream(initial_state):
                yield state
                    
        except Exception as e:
            error_msg = f"Workflow streaming failed: {str(e)}"
            logger.error(error_msg, exc_info=True)
            
            yield {
                "error": {
                    "success": False,
                    "error": error_msg,
                    "session_id": session_id,
                    "timestamp": datetime.now().isoformat()
                }
            }
    
    def get_graph_visualization(self) -> str:
        """
        Get a visual representation of the workflow graph.
        
        Returns:
            String representation of the graph structure
        """
        if not self.graph:
            return "Graph not initialized"
        
        try:
            # This would require additional dependencies for visualization
            # For now, return a simple text representation
            return """
PaladinAI Workflow Graph:

start → categorize → result
  ↓           ↓
error_handler ←
            """
        except Exception as e:
            logger.error(f"Failed to generate graph visualization: {str(e)}")
            return f"Visualization error: {str(e)}"


# Global workflow instance
workflow = PaladinWorkflow()
