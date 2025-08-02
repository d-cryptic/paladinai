"""
Memory-Enhanced Analysis Prompts for PaladinAI Agent Orchestration System

This module contains prompts for enhancing analysis with memory context
and historical insights.
"""

MEMORY_ENHANCED_ANALYSIS_PROMPT = """
You are conducting memory-enhanced analysis for PaladinAI. Your role is to
analyze current evidence in the context of historical memory and patterns.

Current Analysis Context:
Query: {current_query}
Workflow Type: {workflow_type}
Current Evidence: {current_evidence}
Initial Analysis: {initial_analysis}

Historical Memory Context:
{memory_context}

Memory-Enhanced Analysis Framework:
1. **Evidence Correlation**: How does current evidence correlate with historical patterns?
2. **Pattern Recognition**: What patterns from memory apply to current situation?
3. **Confidence Enhancement**: How does memory context affect confidence in analysis?
4. **Gap Identification**: What evidence gaps can be filled using memory insights?
5. **Hypothesis Refinement**: How can hypotheses be refined using memory patterns?

Enhance your analysis by:
- Comparing current evidence with similar historical cases
- Identifying patterns that strengthen or weaken current hypotheses
- Using memory context to fill evidence gaps
- Adjusting confidence based on historical precedents
- Generating new insights from memory correlation

Provide memory-enhanced analysis in JSON format:
{
    "enhanced_findings": {
        "memory_correlated_insights": ["string"],
        "pattern_matches": ["string"],
        "historical_precedents": ["string"],
        "confidence_factors": ["string"]
    },
    "evidence_correlation": [
        {
            "current_evidence": "string",
            "historical_match": "string",
            "correlation_strength": float,
            "implications": "string"
        }
    ],
    "confidence_enhancement": {
        "original_confidence": float,
        "memory_enhanced_confidence": float,
        "enhancement_factors": ["string"],
        "confidence_reasoning": "string"
    },
    "refined_hypotheses": [
        {
            "hypothesis": "string",
            "memory_support": "string",
            "confidence": float,
            "historical_basis": "string"
        }
    ],
    "identified_gaps": [
        {
            "gap_description": "string",
            "memory_guidance": "string",
            "collection_priority": "high|medium|low"
        }
    ],
    "memory_insights": ["string"],
    "analysis_confidence": float
}
"""

MEMORY_CORRELATION_PROMPT = """
You are analyzing correlations between current data and historical memory for PaladinAI.

Current Data:
Query: {current_query}
Evidence: {current_evidence}
Metrics: {current_metrics}
Context: {current_context}

Historical Memory Data:
{historical_data}

Correlation Analysis Framework:
1. **Direct Correlations**: What direct matches exist between current and historical data?
2. **Pattern Correlations**: What pattern-level similarities exist?
3. **Contextual Correlations**: How do contexts correlate across time?
4. **Outcome Correlations**: How do similar inputs correlate with outcomes?
5. **Temporal Correlations**: Are there time-based correlation patterns?

Analyze correlations to identify:
- Strong positive correlations (similar patterns, similar outcomes)
- Strong negative correlations (similar patterns, different outcomes)
- Weak correlations that might still provide insights
- Missing correlations that indicate unique situations
- Correlation confidence levels

Provide correlation analysis in JSON format:
{
    "direct_correlations": [
        {
            "current_element": "string",
            "historical_match": "string",
            "correlation_strength": float,
            "correlation_type": "exact|similar|pattern",
            "significance": "high|medium|low"
        }
    ],
    "pattern_correlations": [
        {
            "pattern_description": "string",
            "current_manifestation": "string",
            "historical_manifestations": ["string"],
            "pattern_strength": float
        }
    ],
    "outcome_predictions": [
        {
            "predicted_outcome": "string",
            "confidence": float,
            "historical_basis": "string",
            "correlation_factors": ["string"]
        }
    ],
    "anomaly_detection": [
        {
            "anomaly_description": "string",
            "deviation_from_memory": "string",
            "significance": "high|medium|low"
        }
    ],
    "correlation_insights": ["string"],
    "overall_correlation_strength": float
}
"""

MEMORY_TREND_ANALYSIS_PROMPT = """
You are analyzing trends and patterns in memory data for PaladinAI insights.

Current Situation:
Query: {current_query}
Timeframe: {timeframe}
Context: {current_context}

Memory Trend Data:
{memory_trend_data}

Trend Analysis Framework:
1. **Temporal Trends**: How have similar situations evolved over time?
2. **Frequency Trends**: Are similar issues becoming more or less frequent?
3. **Severity Trends**: Are similar issues becoming more or less severe?
4. **Resolution Trends**: How have resolution strategies evolved?
5. **Success Rate Trends**: Are success rates improving or declining?

Analyze trends to identify:
- Improving patterns (getting better over time)
- Degrading patterns (getting worse over time)
- Cyclical patterns (recurring with time periods)
- Emerging patterns (new trends appearing)
- Stable patterns (consistent over time)

Provide trend analysis in JSON format:
{
    "temporal_trends": [
        {
            "trend_description": "string",
            "trend_direction": "improving|degrading|stable|cyclical|emerging",
            "trend_strength": float,
            "time_period": "string",
            "implications": "string"
        }
    ],
    "frequency_analysis": {
        "current_frequency": "string",
        "historical_frequency": "string",
        "frequency_trend": "increasing|decreasing|stable",
        "trend_significance": float
    },
    "severity_analysis": {
        "current_severity": "string",
        "historical_severity": "string",
        "severity_trend": "increasing|decreasing|stable",
        "trend_significance": float
    },
    "resolution_evolution": [
        {
            "resolution_approach": "string",
            "effectiveness_trend": "improving|degrading|stable",
            "adoption_trend": "increasing|decreasing|stable",
            "recommendations": "string"
        }
    ],
    "predictive_insights": [
        {
            "prediction": "string",
            "confidence": float,
            "trend_basis": "string",
            "timeframe": "string"
        }
    ],
    "trend_confidence": float
}
"""

def get_memory_enhanced_analysis_prompt(
    current_query: str,
    workflow_type: str,
    current_evidence: str,
    initial_analysis: str,
    memory_context: str
) -> str:
    """
    Get memory-enhanced analysis prompt with current context.
    
    Args:
        current_query: Current query being processed
        workflow_type: Type of current workflow
        current_evidence: Current evidence summary
        initial_analysis: Initial analysis results
        memory_context: Relevant memory context
    
    Returns:
        Formatted memory-enhanced analysis prompt
    """
    return MEMORY_ENHANCED_ANALYSIS_PROMPT.format(
        current_query=current_query,
        workflow_type=workflow_type,
        current_evidence=current_evidence,
        initial_analysis=initial_analysis,
        memory_context=memory_context
    )

def get_memory_correlation_prompt(
    current_query: str,
    current_evidence: str,
    current_metrics: str,
    current_context: str,
    historical_data: str
) -> str:
    """
    Get memory correlation analysis prompt.
    
    Args:
        current_query: Current query being processed
        current_evidence: Current evidence data
        current_metrics: Current metrics data
        current_context: Current context information
        historical_data: Historical memory data
    
    Returns:
        Formatted memory correlation prompt
    """
    return MEMORY_CORRELATION_PROMPT.format(
        current_query=current_query,
        current_evidence=current_evidence,
        current_metrics=current_metrics,
        current_context=current_context,
        historical_data=historical_data
    )

def get_memory_trend_analysis_prompt(
    current_query: str,
    timeframe: str,
    current_context: str,
    memory_trend_data: str
) -> str:
    """
    Get memory trend analysis prompt.
    
    Args:
        current_query: Current query being processed
        timeframe: Analysis timeframe
        current_context: Current context information
        memory_trend_data: Memory data for trend analysis
    
    Returns:
        Formatted memory trend analysis prompt
    """
    return MEMORY_TREND_ANALYSIS_PROMPT.format(
        current_query=current_query,
        timeframe=timeframe,
        current_context=current_context,
        memory_trend_data=memory_trend_data
    )
