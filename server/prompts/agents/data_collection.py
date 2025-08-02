"""
Data Collection Agent Prompts for PaladinAI Agent Orchestration.

This module contains prompts for the data collection agent that gathers
evidence from monitoring systems.
"""

DATA_COLLECTION_ANALYSIS_PROMPT = """
You are an expert SRE data collection agent responsible for gathering evidence from monitoring systems.

Based on the current investigation context, determine what data to collect next:

Current Context:
- Initial Query: {initial_query}
- Workflow Type: {workflow_type}
- Evidence Count: {evidence_count}
- Iteration: {iteration}

Memory Context (if available):
{memory_context}

Available data sources:
1. Prometheus metrics (CPU, memory, network, custom metrics)
2. Loki logs (application logs, system logs, error logs)
3. Alertmanager alerts (active alerts, alert history)

Respond with a JSON object containing:
- collection_steps: list of data collection steps to perform
- priority: "HIGH", "MEDIUM", or "LOW" for each step
- reasoning: explanation of why this data is needed
- expected_evidence_types: types of evidence expected from each step
- memory_guided_priorities: adjustments based on memory insights (if available)

Each collection step should specify:
- type: "system_overview", "active_alerts", "recent_errors", "investigate_query", or "validate_hypothesis"
- parameters: relevant parameters for the collection step
- description: what this step will accomplish

If memory context is available, also consider:
- Data sources that were most valuable in similar past situations
- Evidence types that led to successful resolutions
- Collection patterns that should be avoided based on past failures
- Memory-guided prioritization of data collection steps
"""

EVIDENCE_SUMMARY_TEMPLATE = """
Evidence Summary for Analysis:

Total Evidence Items: {evidence_count}

Evidence by Type:
{evidence_by_type}

Key Evidence Items:
{key_evidence}

Evidence Quality Assessment:
- High Confidence Items: {high_confidence_count}
- Medium Confidence Items: {medium_confidence_count}
- Low Confidence Items: {low_confidence_count}

Data Sources Used:
{data_sources}

Collection Timeframe: {timeframe}
"""

INVESTIGATION_QUERY_PROMPT = """
Investigate the following query using available monitoring tools:

Query: {query}
Timeframe: {timeframe}

Focus on:
1. Identifying relevant metrics and logs
2. Checking for related alerts or incidents
3. Gathering supporting evidence
4. Assessing data quality and reliability

Provide comprehensive data that can help understand the situation described in the query.
"""

HYPOTHESIS_VALIDATION_PROMPT = """
Validate the following hypothesis using monitoring data:

Hypothesis: {hypothesis_description}
Confidence: {hypothesis_confidence}

Validation approach:
1. Collect data that would support or refute the hypothesis
2. Look for corroborating evidence from multiple sources
3. Check for timeline correlation with the hypothesis
4. Assess the strength of supporting evidence

Provide data that helps determine if the hypothesis is valid, invalid, or requires more investigation.
"""

def get_data_collection_prompt(context: dict, memory_context: str = "No memory context available") -> str:
    """
    Get data collection prompt with context and memory enhancement.

    Args:
        context: Context dictionary with investigation details
        memory_context: Memory context from similar past situations

    Returns:
        Formatted data collection prompt with memory enhancement
    """
    return DATA_COLLECTION_ANALYSIS_PROMPT.format(
        initial_query=context.get('initial_query', 'Unknown'),
        workflow_type=context.get('workflow_type', 'Unknown'),
        evidence_count=context.get('evidence_count', 0),
        iteration=context.get('iteration', 1),
        memory_context=memory_context
    )

def get_evidence_summary(evidence_list: list, context: dict) -> str:
    """
    Generate evidence summary for analysis.
    
    Args:
        evidence_list: List of evidence items
        context: Additional context information
        
    Returns:
        Formatted evidence summary
    """
    evidence_count = len(evidence_list)
    
    # Group evidence by type
    evidence_by_type = {}
    for evidence in evidence_list:
        evidence_type = getattr(evidence, 'type', 'unknown')
        if evidence_type not in evidence_by_type:
            evidence_by_type[evidence_type] = 0
        evidence_by_type[evidence_type] += 1
    
    # Count by confidence levels
    high_confidence_count = sum(1 for ev in evidence_list if getattr(ev, 'confidence', 0) >= 0.8)
    medium_confidence_count = sum(1 for ev in evidence_list if 0.5 <= getattr(ev, 'confidence', 0) < 0.8)
    low_confidence_count = sum(1 for ev in evidence_list if getattr(ev, 'confidence', 0) < 0.5)
    
    # Get key evidence (highest confidence items)
    key_evidence = sorted(evidence_list, key=lambda x: getattr(x, 'confidence', 0), reverse=True)[:5]
    key_evidence_text = "\n".join([
        f"- {getattr(ev, 'description', 'No description')} (confidence: {getattr(ev, 'confidence', 0):.2f})"
        for ev in key_evidence
    ])
    
    # Get data sources
    data_sources = set()
    for evidence in evidence_list:
        source = getattr(evidence, 'source', 'unknown')
        data_sources.add(source)
    
    return EVIDENCE_SUMMARY_TEMPLATE.format(
        evidence_count=evidence_count,
        evidence_by_type="\n".join([f"- {k}: {v}" for k, v in evidence_by_type.items()]),
        key_evidence=key_evidence_text,
        high_confidence_count=high_confidence_count,
        medium_confidence_count=medium_confidence_count,
        low_confidence_count=low_confidence_count,
        data_sources=", ".join(data_sources),
        timeframe=context.get('timeframe', 'Not specified')
    )

def get_investigation_prompt(query: str, timeframe: str = "1h") -> str:
    """
    Get investigation prompt for a specific query.
    
    Args:
        query: Query to investigate
        timeframe: Timeframe for investigation
        
    Returns:
        Formatted investigation prompt
    """
    return INVESTIGATION_QUERY_PROMPT.format(
        query=query,
        timeframe=timeframe
    )

def get_hypothesis_validation_prompt(hypothesis) -> str:
    """
    Get hypothesis validation prompt.
    
    Args:
        hypothesis: Hypothesis object to validate
        
    Returns:
        Formatted validation prompt
    """
    return HYPOTHESIS_VALIDATION_PROMPT.format(
        hypothesis_description=getattr(hypothesis, 'description', 'Unknown hypothesis'),
        hypothesis_confidence=getattr(hypothesis, 'confidence', 0.0)
    )
