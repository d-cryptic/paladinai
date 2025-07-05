"""Alert Analysis Workflow using LangGraph"""

import os
from typing import Dict, Any, Optional
from langgraph.graph import StateGraph, END
from langfuse import observe

from .state import WorkflowState, create_initial_state
from .nodes.alert_analysis_nodes import (
    alert_context_node,
    alert_analysis_mode_node,
    rag_search_node,
    memory_aggregation_node,
    alert_decision_node,
    alert_result_node
)
from checkpointing import get_checkpointer


class AlertAnalysisWorkflow:
    """Alert Analysis Workflow orchestrator."""
    
    def __init__(self):
        self.graph = self._build_graph()
        self.checkpointer = None
        self.compiled_graph = None
    
    async def initialize(self):
        """Initialize the workflow with checkpointing."""
        try:
            # Temporarily disable checkpointing for alert workflow
            # self.checkpointer = await get_checkpointer()
            # self.compiled_graph = self.graph.compile(checkpointer=self.checkpointer)
            self.compiled_graph = self.graph.compile()
            print("[ALERT WORKFLOW] Initialized without checkpointing")
        except Exception as e:
            print(f"[ALERT WORKFLOW] Failed to initialize: {e}")
            raise
    
    def _build_graph(self) -> StateGraph:
        """Build the alert analysis workflow graph."""
        workflow = StateGraph(WorkflowState)
        
        # Add nodes
        workflow.add_node("alert_context", alert_context_node)
        workflow.add_node("alert_analysis", alert_analysis_mode_node)
        workflow.add_node("rag_search", rag_search_node)
        workflow.add_node("memory_aggregation", memory_aggregation_node)
        workflow.add_node("alert_decision", alert_decision_node)
        workflow.add_node("alert_result", alert_result_node)
        
        # Set entry point
        workflow.set_entry_point("alert_context")
        
        # Add edges
        workflow.add_edge("alert_context", "alert_analysis")
        
        # Conditional routing from alert_analysis
        def route_from_analysis(state: WorkflowState) -> str:
            if state.user_context.get("rag_needed", False):
                return "rag_search"
            elif state.user_context.get("memory_needed", False):
                return "memory_aggregation"
            elif state.user_context.get("analysis_state", {}).get("status") == "complete":
                return "alert_decision"
            else:
                return "alert_decision"
        
        workflow.add_conditional_edges(
            "alert_analysis",
            route_from_analysis,
            {
                "rag_search": "rag_search",
                "memory_aggregation": "memory_aggregation",
                "alert_decision": "alert_decision"
            }
        )
        
        # Route from RAG search
        def route_from_rag(state: WorkflowState) -> str:
            if state.user_context.get("memory_needed", False):
                return "memory_aggregation"
            else:
                return "alert_analysis"
        
        workflow.add_conditional_edges(
            "rag_search",
            route_from_rag,
            {
                "memory_aggregation": "memory_aggregation",
                "alert_analysis": "alert_analysis"
            }
        )
        
        # Route from memory aggregation back to analysis
        workflow.add_edge("memory_aggregation", "alert_analysis")
        
        # Route from alert decision
        def route_from_decision(state: WorkflowState) -> str:
            decision = state.user_context.get("analysis_decision", {})
            if decision.get("decision") == "need_more_analysis":
                return "alert_analysis"
            else:
                return "alert_result"
        
        workflow.add_conditional_edges(
            "alert_decision",
            route_from_decision,
            {
                "alert_analysis": "alert_analysis",
                "alert_result": "alert_result"
            }
        )
        
        # End after result generation
        workflow.add_edge("alert_result", END)
        
        return workflow
    
    @observe(name="alert_workflow_invoke")
    async def invoke(self, alert_data: Dict[str, Any], config: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """Invoke the alert analysis workflow."""
        try:
            if not self.compiled_graph:
                await self.initialize()
            
            # Create initial state
            initial_state = create_initial_state(
                user_input="Alert analysis requested",
                user_context={
                    "alert_data": alert_data,
                    "workflow_type": "alert_analysis"
                }
            )
            
            print(f"[ALERT WORKFLOW] Starting workflow with alert: {alert_data.get('labels', {}).get('alertname', 'Unknown')}")
            
            # Run the workflow with increased recursion limit
            run_config = config or {}
            # Increase recursion limit to handle iterative analysis
            run_config["recursion_limit"] = 50
            
            result = await self.compiled_graph.ainvoke(initial_state, run_config)
        except Exception as e:
            print(f"[ALERT WORKFLOW] Error during workflow execution: {e}")
            print(f"[ALERT WORKFLOW] Error type: {type(e).__name__}")
            import traceback
            traceback.print_exc()
            raise
        
        # Result is a dict-like object from LangGraph
        user_context = result.get("user_context", {})
        final_result = result.get("final_result", {})
        
        return {
            "success": True,
            "alert_report": user_context.get("alert_report", {}),
            "markdown_content": final_result.get("answer", "") if final_result else "",
            "analysis_metadata": {
                "iterations": user_context.get("analysis_state", {}).get("iterations", 0),
                "tools_used": user_context.get("analysis_state", {}).get("tools_used", []),
                "confidence_score": user_context.get("analysis_decision", {}).get("confidence_score", 0),
                "findings_count": len(user_context.get("key_findings", []))
            }
        }
    
    async def stream(self, alert_data: Dict[str, Any], config: Optional[Dict[str, Any]] = None):
        """Stream the alert analysis workflow execution."""
        if not self.compiled_graph:
            await self.initialize()
        
        # Create initial state
        initial_state = create_initial_state(
            user_input="Alert analysis requested",
            user_context={
                "alert_data": alert_data,
                "workflow_type": "alert_analysis"
            }
        )
        
        # Stream the workflow
        if config:
            async for event in self.compiled_graph.astream(initial_state, config):
                yield event
        else:
            async for event in self.compiled_graph.astream(initial_state):
                yield event


# Create singleton instance
alert_workflow = AlertAnalysisWorkflow()