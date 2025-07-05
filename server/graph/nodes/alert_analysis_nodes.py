"""Alert Analysis Workflow Nodes"""

import json
from typing import Dict, Any, List
from datetime import datetime, timedelta
import asyncio
from langchain_openai import ChatOpenAI
from langchain_core.messages import HumanMessage, SystemMessage
from langfuse import observe

from ..state import WorkflowState
from utils.json_parser import parse_llm_json
from pydantic import BaseModel
from prompts.alert_analysis.alert_context_prompt import (
    ALERT_CONTEXT_SYSTEM_PROMPT,
    ALERT_CONTEXT_USER_PROMPT
)
from prompts.alert_analysis.alert_analysis_prompt import (
    ALERT_ANALYSIS_SYSTEM_PROMPT,
    ALERT_ANALYSIS_USER_PROMPT,
    ALERT_ANALYSIS_TOOLS_PROMPT
)
from prompts.alert_analysis.alert_decision_prompt import (
    ALERT_DECISION_SYSTEM_PROMPT,
    ALERT_DECISION_USER_PROMPT
)
from prompts.alert_analysis.alert_result_prompt import (
    ALERT_RESULT_SYSTEM_PROMPT,
    ALERT_RESULT_USER_PROMPT
)
from prompts.alert_analysis.rag_search_prompt import (
    RAG_SEARCH_SYSTEM_PROMPT,
    RAG_SEARCH_USER_PROMPT
)
from tools import prometheus, loki, alertmanager
from rag.service import RAGService
from memory.service import MemoryService
from utils.data_reduction import reduce_data_for_context


def safe_json_dumps(obj, **kwargs):
    """Safely convert object to JSON string, handling Pydantic models."""
    def convert_obj(item):
        """Recursively convert Pydantic models to dicts."""
        if isinstance(item, BaseModel):
            return item.model_dump()
        elif isinstance(item, dict):
            return {k: convert_obj(v) for k, v in item.items()}
        elif isinstance(item, list):
            return [convert_obj(i) for i in item]
        elif hasattr(item, '__dict__') and not isinstance(item, (str, int, float, bool, type(None))):
            # Handle other objects with __dict__
            try:
                return convert_obj(item.__dict__)
            except:
                return str(item)
        else:
            return item
    
    converted = convert_obj(obj)
    return json.dumps(converted, default=str, **kwargs)


@observe(name="alert_context_node")
async def alert_context_node(state: WorkflowState) -> WorkflowState:
    """Extract comprehensive context from the alert."""
    print(f"\n[ALERT CONTEXT NODE] Starting analysis...")
    
    try:
        llm = ChatOpenAI(model="gpt-4o-mini", temperature=0.3)
        
        # Get alert data from state
        alert_data = state.user_context.get("alert_data", {})
        print(f"[ALERT CONTEXT NODE] Processing alert: {alert_data.get('labels', {}).get('alertname', 'Unknown')}")
        
        # Generate context analysis
        messages = [
            SystemMessage(content=ALERT_CONTEXT_SYSTEM_PROMPT),
            HumanMessage(content=ALERT_CONTEXT_USER_PROMPT.format(
                alert_data=safe_json_dumps(alert_data, indent=2)
            ))
        ]
        
        response = await llm.ainvoke(messages)
        print(f"[ALERT CONTEXT NODE] LLM response received, length: {len(response.content)}")
        
        try:
            alert_context = parse_llm_json(response.content)
            print(f"[ALERT CONTEXT NODE] Successfully parsed JSON context")
        except json.JSONDecodeError as e:
            print(f"[ALERT CONTEXT NODE] JSON parsing failed: {e}")
            print(f"[ALERT CONTEXT NODE] Raw content: {response.content[:200]}...")
            # Fallback to text parsing if JSON fails
            alert_context = {
                "summary": response.content[:200],
                "affected_components": [],
                "severity": "unknown",
                "key_metrics": [],
                "time_context": {},
                "investigation_focus": [],
                "related_context": {}
            }
        
        # Store context in state
        state.user_context["alert_context"] = alert_context
        state.user_context["analysis_state"] = {
            "status": "context_extracted",
            "iterations": 0,
            "findings": [],
            "tools_used": []
        }
        
        print(f"[ALERT CONTEXT NODE] Context extracted: {alert_context.get('summary', 'No summary')}")
        return state
        
    except Exception as e:
        print(f"[ALERT CONTEXT NODE] Error: {e}")
        print(f"[ALERT CONTEXT NODE] Error type: {type(e).__name__}")
        import traceback
        traceback.print_exc()
        raise


@observe(name="alert_analysis_mode_node")
async def alert_analysis_mode_node(state: WorkflowState) -> WorkflowState:
    """Iterative alert analysis with tool selection and execution."""
    print(f"\n[ALERT ANALYSIS NODE] Starting analysis iteration...")
    
    llm = ChatOpenAI(model="gpt-4o-mini", temperature=0.3)
    
    alert_context = state.user_context.get("alert_context", {})
    analysis_state = state.user_context.get("analysis_state", {})
    
    # Increment iteration counter
    analysis_state["iterations"] += 1
    
    # Prepare analysis prompt
    messages = [
        SystemMessage(content=ALERT_ANALYSIS_SYSTEM_PROMPT),
        HumanMessage(content=ALERT_ANALYSIS_USER_PROMPT.format(
            alert_context=safe_json_dumps(alert_context, indent=2),
            current_analysis=safe_json_dumps(analysis_state.get("findings", []), indent=2),
            previous_results=safe_json_dumps(analysis_state.get("last_results", {}), indent=2)
        ))
    ]
    
    response = await llm.ainvoke(messages)
    
    try:
        analysis_decision = parse_llm_json(response.content)
    except json.JSONDecodeError:
        analysis_decision = {
            "analysis_status": "needs_more_data",
            "findings": [],
            "next_tools": [],
            "confidence_level": 0,
            "missing_context": []
        }
    
    # If analysis is complete, update state and return
    if analysis_decision.get("analysis_status") == "analysis_complete":
        analysis_state["status"] = "complete"
        analysis_state["confidence_level"] = analysis_decision.get("confidence_level", 0)
        state.user_context["analysis_state"] = analysis_state
        return state
    
    # Execute tools based on decision
    tool_results = {}
    
    for tool_request in analysis_decision.get("next_tools", []):
        tool_name = tool_request.get("tool_name")
        
        if tool_name == "prometheus":
            results = await _execute_prometheus_queries(tool_request, alert_context)
            tool_results["prometheus"] = results
            
        elif tool_name == "loki":
            results = await _execute_loki_queries(tool_request, alert_context)
            tool_results["loki"] = results
            
        elif tool_name == "alertmanager":
            results = await _execute_alertmanager_queries(tool_request, alert_context)
            tool_results["alertmanager"] = results
            
        elif tool_name == "rag":
            # RAG will be handled by a separate node
            state.user_context["rag_needed"] = True
            state.user_context["rag_request"] = tool_request
            
        elif tool_name == "memory":
            # Memory will be handled by a separate node
            state.user_context["memory_needed"] = True
            state.user_context["memory_request"] = tool_request
    
    # Update analysis state with results
    analysis_state["last_results"] = tool_results
    analysis_state["findings"].extend(analysis_decision.get("findings", []))
    analysis_state["tools_used"].extend([t["tool_name"] for t in analysis_decision.get("next_tools", [])])
    
    # Check if we've hit iteration limit
    if analysis_state["iterations"] >= 5:
        print("[ALERT ANALYSIS NODE] Hit iteration limit, forcing completion")
        analysis_state["status"] = "complete"
    
    state.user_context["analysis_state"] = analysis_state
    state.user_context["tool_results"] = tool_results
    
    print(f"[ALERT ANALYSIS NODE] Iteration {analysis_state['iterations']} complete")
    return state


async def _execute_prometheus_queries(tool_request: Dict, alert_context: Dict) -> Dict:
    """Execute Prometheus queries based on tool request."""
    query_params = tool_request.get("query_params", {})
    
    # Determine time range from alert context
    time_context = alert_context.get("time_context", {})
    end_time = datetime.now()
    start_time = end_time - timedelta(hours=query_params.get("hours", 1))
    
    results = {}
    
    from tools.prometheus.models import PrometheusRangeQueryRequest, PrometheusQueryRequest
    
    # Build queries based on alert context if not provided
    queries_to_execute = []
    
    if query_params.get("query_type") == "metrics":
        metrics = query_params.get("metrics", [])
        if not metrics and alert_context.get("affected_components"):
            # Generate default queries based on alert
            alertname = alert_context.get("summary", "").lower()
            instance = alert_context.get("affected_components", [None])[0]
            
            if "cpu" in alertname and instance:
                queries_to_execute.append(f'rate(node_cpu_seconds_total{{instance="{instance}",mode!="idle"}}[5m])')
            elif "disk" in alertname and instance:
                queries_to_execute.append(f'100 - (node_filesystem_avail_bytes{{instance="{instance}"}} / node_filesystem_size_bytes{{instance="{instance}"}}) * 100')
            elif "memory" in alertname and instance:
                queries_to_execute.append(f'(1 - (node_memory_MemAvailable_bytes{{instance="{instance}"}} / node_memory_MemTotal_bytes{{instance="{instance}"}})) * 100')
            elif "process" in alertname and instance:
                queries_to_execute.append(f'node_procs_running{{instance="{instance}"}}')
            else:
                # Generic up query
                queries_to_execute.append(f'up{{instance="{instance}"}}')
        else:
            queries_to_execute = metrics
            
        # Execute queries
        for query in queries_to_execute:
            try:
                request = PrometheusRangeQueryRequest(
                    query=query,
                    start=str(int(start_time.timestamp())),
                    end=str(int(end_time.timestamp())),
                    step="1m"
                )
                result = await prometheus.query_range(request)
                results[query] = result
            except Exception as e:
                print(f"[PROMETHEUS] Query failed: {query} - Error: {e}")
                results[query] = {"error": str(e)}
    
    elif query_params.get("query_type") == "instant":
        queries = query_params.get("queries", [])
        if not queries and alert_context.get("affected_components"):
            # Default instant query
            instance = alert_context.get("affected_components", [None])[0]
            queries = [f'up{{instance="{instance}"}}'] if instance else ['up']
            
        for query in queries:
            try:
                request = PrometheusQueryRequest(query=query)
                result = await prometheus.query(request)
                results[query] = result
            except Exception as e:
                print(f"[PROMETHEUS] Query failed: {query} - Error: {e}")
                results[query] = {"error": str(e)}
    
    return reduce_data_for_context(results, max_chars=10000)


async def _execute_loki_queries(tool_request: Dict, alert_context: Dict) -> Dict:
    """Execute Loki queries based on tool request."""
    query_params = tool_request.get("query_params", {})
    
    # Build LogQL query from parameters
    base_query = query_params.get("base_query", "")
    filters = query_params.get("filters", [])
    
    # If no base query provided, build one from alert context
    if not base_query or base_query == "{}":
        affected_components = alert_context.get("affected_components", [])
        if affected_components:
            # Build query with instance label
            instance = affected_components[0]
            base_query = f'{{instance="{instance}"}}'
        else:
            # Use a safe default query
            base_query = '{job=~".+"}'  # Matches any job
    
    # Add filters if provided
    for filter_item in filters:
        if filter_item:  # Only add non-empty filters
            base_query += f' |~ "{filter_item}"'
    
    # Determine time range
    end_time = datetime.now()
    start_time = end_time - timedelta(minutes=query_params.get("minutes", 30))
    
    try:
        from tools.loki.models import LokiRangeQueryRequest
        
        request = LokiRangeQueryRequest(
            query=base_query,
            start=str(int(start_time.timestamp() * 1e9)),  # Convert to Unix nanoseconds string
            end=str(int(end_time.timestamp() * 1e9)),      # Convert to Unix nanoseconds string
            limit=query_params.get("limit", 1000)
        )
        result = await loki.query_range(request)
        return reduce_data_for_context(result, max_chars=15000)
    except Exception as e:
        print(f"[LOKI] Query failed: {base_query} - Error: {e}")
        return {"error": str(e), "query": base_query}


async def _execute_alertmanager_queries(tool_request: Dict, alert_context: Dict) -> Dict:
    """Execute AlertManager queries based on tool request."""
    query_params = tool_request.get("query_params", {})
    
    results = {}
    
    # Get active alerts
    if query_params.get("active_alerts", True):
        alerts = await alertmanager.get_alerts(
            active=True,
            silenced=False,
            inhibited=False
        )
        results["active_alerts"] = alerts
    
    # Get alert groups
    if query_params.get("groups", True):
        groups = await alertmanager.get_alert_groups()
        results["alert_groups"] = groups
    
    return reduce_data_for_context(results, max_chars=8000)


@observe(name="rag_search_node")
async def rag_search_node(state: WorkflowState) -> WorkflowState:
    """Search documentation using RAG."""
    print(f"\n[RAG SEARCH NODE] Searching documentation...")
    
    if not state.user_context.get("rag_needed", False):
        return state
    
    llm = ChatOpenAI(model="gpt-4o-mini", temperature=0.3)
    rag_service = RAGService()
    await rag_service.initialize()
    
    # Generate search queries
    alert_context = state.user_context.get("alert_context", {})
    rag_request = state.user_context.get("rag_request", {})
    
    messages = [
        SystemMessage(content=RAG_SEARCH_SYSTEM_PROMPT),
        HumanMessage(content=RAG_SEARCH_USER_PROMPT.format(
            alert_context=safe_json_dumps(alert_context, indent=2),
            investigation_focus=safe_json_dumps(rag_request.get("query_params", {}), indent=2)
        ))
    ]
    
    response = await llm.ainvoke(messages)
    
    try:
        search_config = parse_llm_json(response.content)
    except json.JSONDecodeError:
        search_config = {
            "search_queries": [{"query": alert_context.get("summary", "alert investigation"), "intent": "general", "priority": "high"}],
            "filters": {}
        }
    
    # Execute searches
    all_results = []
    
    for search_item in search_config.get("search_queries", []):
        if search_item.get("priority") in ["high", "medium"]:
            results = await rag_service.search_documents(
                query=search_item["query"],
                limit=5,
                filter_metadata=search_config.get("filters", {})
            )
            
            for result in results:
                result["search_intent"] = search_item["intent"]
                all_results.append(result)
    
    # Store results
    state.user_context["rag_results"] = all_results
    state.user_context["rag_needed"] = False
    
    print(f"[RAG SEARCH NODE] Found {len(all_results)} relevant documents")
    return state


@observe(name="memory_aggregation_node")
async def memory_aggregation_node(state: WorkflowState) -> WorkflowState:
    """Aggregate memories from multiple sources."""
    print(f"\n[MEMORY AGGREGATION NODE] Fetching historical context...")
    
    if not state.user_context.get("memory_needed", False):
        return state
    
    memory_service = MemoryService()
    alert_context = state.user_context.get("alert_context", {})
    memory_request = state.user_context.get("memory_request", {})
    
    # Build search queries based on alert context
    search_queries = []
    
    # Add affected components as search terms
    for component in alert_context.get("affected_components", []):
        search_queries.append(f"incident {component}")
        search_queries.append(f"alert {component}")
        search_queries.append(f"fix {component}")
    
    # Add key metrics as search terms
    for metric in alert_context.get("key_metrics", []):
        search_queries.append(metric)
    
    # Add investigation focus
    for focus in alert_context.get("investigation_focus", []):
        search_queries.append(focus)
    
    # Aggregate memories from all sources
    all_memories = {
        "mem0": [],
        "qdrant": [],
        "neo4j": []
    }
    
    # Search in parallel
    async def search_memories(query: str):
        results = await memory_service.search_all_memories(
            query=query,
            limit=3,
            memory_types=["instruction", "extracted", "incident", "resolution"]
        )
        return results
    
    # Execute searches
    tasks = [search_memories(query) for query in search_queries[:5]]  # Limit to top 5 queries
    results = await asyncio.gather(*tasks)
    
    # Aggregate results
    for result_set in results:
        for memory in result_set.get("memories", []):
            source = memory.get("source", "unknown")
            if source in all_memories:
                all_memories[source].append(memory)
    
    # Deduplicate and rank by relevance
    for source in all_memories:
        # Simple deduplication by memory_id
        seen_ids = set()
        unique_memories = []
        for memory in all_memories[source]:
            if memory.get("memory_id") not in seen_ids:
                seen_ids.add(memory.get("memory_id"))
                unique_memories.append(memory)
        all_memories[source] = unique_memories[:10]  # Keep top 10 per source
    
    # Store results
    state.user_context["memory_results"] = all_memories
    state.user_context["memory_needed"] = False
    
    total_memories = sum(len(memories) for memories in all_memories.values())
    print(f"[MEMORY AGGREGATION NODE] Found {total_memories} relevant memories")
    return state


@observe(name="alert_decision_node")
async def alert_decision_node(state: WorkflowState) -> WorkflowState:
    """Decide if analysis is sufficient or needs more data."""
    print(f"\n[ALERT DECISION NODE] Evaluating analysis completeness...")
    
    llm = ChatOpenAI(model="gpt-4o-mini", temperature=0.2)
    
    # Gather all analysis data
    alert_data = state.user_context.get("alert_data", {})
    alert_context = state.user_context.get("alert_context", {})
    analysis_state = state.user_context.get("analysis_state", {})
    tool_results = state.user_context.get("tool_results", {})
    rag_results = state.user_context.get("rag_results", [])
    memory_results = state.user_context.get("memory_results", {})
    
    # Check if we've hit iteration limits or have minimal data
    iterations = analysis_state.get("iterations", 0)
    has_data = bool(tool_results or rag_results or memory_results)
    
    # Force completion if we've exhausted iterations or have no data sources
    if iterations >= 5 or (iterations >= 3 and not has_data):
        print(f"[ALERT DECISION NODE] Forcing completion - iterations: {iterations}, has_data: {has_data}")
        decision = {
            "decision": "proceed_to_results",
            "confidence_score": 60 if has_data else 40,
            "reasoning": "Analysis completed with available data" if has_data else "Limited data available, proceeding with basic analysis",
            "gaps_identified": ["Limited external data sources available"] if not has_data else [],
            "key_findings": analysis_state.get("findings", []),
            "root_cause_identified": False,
            "impact_assessment": "Unable to fully assess impact due to limited data"
        }
    else:
        # Prepare decision prompt
        messages = [
            SystemMessage(content=ALERT_DECISION_SYSTEM_PROMPT),
            HumanMessage(content=ALERT_DECISION_USER_PROMPT.format(
                original_alert=safe_json_dumps(alert_data, indent=2),
                alert_context=safe_json_dumps(alert_context, indent=2),
                analysis_results=safe_json_dumps(analysis_state.get("findings", []), indent=2),
                metrics_data=safe_json_dumps(tool_results.get("prometheus", {}), indent=2)[:5000],
                logs_data=safe_json_dumps(tool_results.get("loki", {}), indent=2)[:5000],
                alert_history=safe_json_dumps(tool_results.get("alertmanager", {}), indent=2)[:3000],
                rag_results=safe_json_dumps(rag_results, indent=2)[:3000],
                memory_results=safe_json_dumps({k: len(v) for k, v in memory_results.items()}, indent=2)
            ))
        ]
        
        response = await llm.ainvoke(messages)
        
        try:
            # Log the raw response for debugging
            print(f"[ALERT DECISION] Raw LLM response: {response.content[:500]}...")
            
            decision = parse_llm_json(response.content)
        except json.JSONDecodeError as e:
            print(f"[ALERT DECISION] JSON parsing error: {e}")
            print(f"[ALERT DECISION] Failed to parse: {response.content[:200]}...")
            decision = {
                "decision": "proceed_to_results",
                "confidence_score": 70,
                "reasoning": "Unable to parse decision, proceeding with available data",
                "gaps_identified": [],
                "key_findings": [],
                "root_cause_identified": False
            }
    
    # Store decision
    state.user_context["analysis_decision"] = decision
    
    # Update analysis state based on decision
    if decision.get("decision") == "need_more_analysis" and iterations < 5:
        # Reset for another iteration
        state.user_context["analysis_state"]["status"] = "needs_more_data"
        
        # Add specific data request if provided
        if decision.get("additional_data_needed"):
            state.user_context["analysis_state"]["last_results"] = {
                "decision_feedback": decision.get("additional_data_needed")
            }
    else:
        # Mark as complete
        state.user_context["analysis_state"]["status"] = "complete"
        state.user_context["key_findings"] = decision.get("key_findings", [])
        state.user_context["root_cause"] = decision.get("reasoning", "")
    
    print(f"[ALERT DECISION NODE] Decision: {decision.get('decision')} (Confidence: {decision.get('confidence_score')}%)")
    return state


@observe(name="alert_result_node")
async def alert_result_node(state: WorkflowState) -> WorkflowState:
    """Generate final alert analysis report."""
    print(f"\n[ALERT RESULT NODE] Generating final report...")
    
    llm = ChatOpenAI(model="gpt-4o-mini", temperature=0.3, max_tokens=4000)
    
    # Gather all information for the report
    alert_data = state.user_context.get("alert_data", {})
    alert_context = state.user_context.get("alert_context", {})
    analysis_state = state.user_context.get("analysis_state", {})
    tool_results = state.user_context.get("tool_results", {})
    rag_results = state.user_context.get("rag_results", [])
    memory_results = state.user_context.get("memory_results", {})
    decision = state.user_context.get("analysis_decision", {})
    
    # Prepare comprehensive data for report generation
    messages = [
        SystemMessage(content=ALERT_RESULT_SYSTEM_PROMPT),
        HumanMessage(content=ALERT_RESULT_USER_PROMPT.format(
            original_alert=safe_json_dumps(alert_data, indent=2),
            alert_context=safe_json_dumps(alert_context, indent=2),
            analysis_results=safe_json_dumps({
                "findings": analysis_state.get("findings", []),
                "tools_used": analysis_state.get("tools_used", []),
                "iterations": analysis_state.get("iterations", 0),
                "confidence_level": analysis_state.get("confidence_level", 0)
            }, indent=2),
            key_findings=safe_json_dumps(state.user_context.get("key_findings", []), indent=2),
            root_cause=state.user_context.get("root_cause", "Not definitively identified")
        ))
    ]
    
    response = await llm.ainvoke(messages)
    
    # Store the markdown report
    markdown_report = response.content
    state.final_result = {
        "answer": markdown_report,
        "type": "alert_analysis"
    }
    state.user_context["alert_report"] = {
        "markdown": markdown_report,
        "generated_at": datetime.now().isoformat(),
        "alert_id": alert_data.get("alert_id", "unknown"),
        "severity": alert_context.get("severity", "unknown"),
        "confidence_score": decision.get("confidence_score", 0)
    }
    
    # Mark workflow as complete
    state.user_context["workflow_status"] = "completed"
    
    print(f"[ALERT RESULT NODE] Report generated ({len(markdown_report)} characters)")
    return state