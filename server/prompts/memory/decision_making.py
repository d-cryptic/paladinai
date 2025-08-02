"""
Memory-Guided Decision Making Prompts for PaladinAI Agent Orchestration System

This module contains prompts for making decisions based on memory context
and historical patterns.
"""

MEMORY_GUIDED_DECISION_PROMPT = """
You are a memory-guided decision making system for PaladinAI. Your role is to
make informed decisions based on current evidence and historical memory context.

Current Situation:
Query: {current_query}
Workflow Type: {workflow_type}
Evidence Summary: {evidence_summary}
Current Confidence: {confidence_score}

Historical Memory Context:
{memory_context}

Memory-Guided Analysis:
{memory_analysis}

Based on the current situation and memory context, make decisions about:

1. **Next Actions**: What should be done next based on similar past situations?
2. **Risk Assessment**: What risks should be considered based on historical patterns?
3. **Confidence Adjustment**: How should confidence be adjusted based on memory patterns?
4. **Escalation Decisions**: Should this be escalated based on past similar incidents?
5. **Resource Allocation**: What resources should be prioritized based on memory insights?

Decision Framework:
- Analyze current evidence in context of historical patterns
- Consider success/failure patterns from similar past situations
- Evaluate risk factors based on memory insights
- Recommend actions with confidence levels
- Provide reasoning based on memory correlation

Respond with structured decisions in JSON format:
{
    "recommended_actions": [
        {
            "action": "string",
            "priority": "high|medium|low",
            "confidence": float,
            "memory_basis": "string",
            "expected_outcome": "string"
        }
    ],
    "risk_assessment": {
        "identified_risks": ["string"],
        "risk_levels": ["high|medium|low"],
        "mitigation_strategies": ["string"],
        "historical_precedents": ["string"]
    },
    "confidence_adjustment": {
        "original_confidence": float,
        "memory_adjusted_confidence": float,
        "adjustment_reasoning": "string"
    },
    "escalation_recommendation": {
        "should_escalate": boolean,
        "escalation_reason": "string",
        "escalation_urgency": "immediate|high|medium|low",
        "historical_basis": "string"
    },
    "memory_insights": ["string"],
    "decision_confidence": float
}
"""

MEMORY_PATTERN_ANALYSIS_PROMPT = """
You are analyzing patterns in memory data for PaladinAI decision making.

Current Context:
Query: {current_query}
Workflow Type: {workflow_type}

Memory Data for Pattern Analysis:
{memory_data}

Analyze the memory data to identify:

1. **Recurring Patterns**: What patterns appear across multiple memories?
2. **Success Indicators**: What factors correlate with successful outcomes?
3. **Failure Indicators**: What factors correlate with failures or escalations?
4. **Temporal Patterns**: Are there time-based patterns in the data?
5. **Contextual Patterns**: How does context influence outcomes?

Pattern Analysis Framework:
- Identify common themes across memories
- Correlate input conditions with outcomes
- Analyze success/failure factors
- Identify predictive indicators
- Extract actionable insights

Provide pattern analysis in JSON format:
{
    "recurring_patterns": [
        {
            "pattern_description": "string",
            "frequency": int,
            "confidence": float,
            "examples": ["string"]
        }
    ],
    "success_indicators": [
        {
            "indicator": "string",
            "correlation_strength": float,
            "description": "string"
        }
    ],
    "failure_indicators": [
        {
            "indicator": "string",
            "correlation_strength": float,
            "description": "string"
        }
    ],
    "temporal_patterns": [
        {
            "pattern": "string",
            "time_correlation": "string",
            "significance": float
        }
    ],
    "actionable_insights": ["string"],
    "pattern_confidence": float
}
"""

MEMORY_STRATEGY_RECOMMENDATION_PROMPT = """
You are providing strategic recommendations based on memory analysis for PaladinAI.

Current Situation:
Query: {current_query}
Workflow Type: {workflow_type}
Current Evidence: {evidence_summary}

Memory-Based Insights:
Pattern Analysis: {pattern_analysis}
Historical Outcomes: {historical_outcomes}
Success Factors: {success_factors}

Based on memory analysis, recommend strategies for:

1. **Investigation Strategy**: How should the investigation proceed?
2. **Data Collection Strategy**: What data should be prioritized?
3. **Analysis Strategy**: How should analysis be approached?
4. **Resolution Strategy**: What resolution approaches are most likely to succeed?
5. **Prevention Strategy**: How can similar issues be prevented in the future?

Strategy Recommendations Framework:
- Leverage successful patterns from memory
- Avoid known failure patterns
- Optimize based on historical effectiveness
- Consider context-specific factors
- Provide confidence-based recommendations

Respond with strategic recommendations in JSON format:
{
    "investigation_strategy": {
        "approach": "string",
        "priority_areas": ["string"],
        "expected_timeline": "string",
        "confidence": float,
        "memory_basis": "string"
    },
    "data_collection_strategy": {
        "priority_sources": ["string"],
        "collection_order": ["string"],
        "stop_conditions": ["string"],
        "memory_guidance": "string"
    },
    "analysis_strategy": {
        "analysis_approach": "string",
        "focus_areas": ["string"],
        "analysis_depth": "shallow|medium|deep",
        "historical_effectiveness": float
    },
    "resolution_strategy": {
        "recommended_approach": "string",
        "alternative_approaches": ["string"],
        "success_probability": float,
        "historical_precedents": ["string"]
    },
    "prevention_strategy": {
        "preventive_measures": ["string"],
        "monitoring_recommendations": ["string"],
        "long_term_improvements": ["string"]
    },
    "overall_confidence": float
}
"""

def get_memory_guided_decision_prompt(
    current_query: str,
    workflow_type: str,
    evidence_summary: str,
    confidence_score: float,
    memory_context: str,
    memory_analysis: str
) -> str:
    """
    Get memory-guided decision making prompt with current context.
    
    Args:
        current_query: Current query being processed
        workflow_type: Type of current workflow
        evidence_summary: Summary of collected evidence
        confidence_score: Current confidence score
        memory_context: Relevant memory context
        memory_analysis: Analysis of memory patterns
    
    Returns:
        Formatted memory-guided decision prompt
    """
    return MEMORY_GUIDED_DECISION_PROMPT.format(
        current_query=current_query,
        workflow_type=workflow_type,
        evidence_summary=evidence_summary,
        confidence_score=confidence_score,
        memory_context=memory_context,
        memory_analysis=memory_analysis
    )

def get_memory_pattern_analysis_prompt(
    current_query: str,
    workflow_type: str,
    memory_data: str
) -> str:
    """
    Get memory pattern analysis prompt.
    
    Args:
        current_query: Current query being processed
        workflow_type: Type of current workflow
        memory_data: Memory data for pattern analysis
    
    Returns:
        Formatted memory pattern analysis prompt
    """
    return MEMORY_PATTERN_ANALYSIS_PROMPT.format(
        current_query=current_query,
        workflow_type=workflow_type,
        memory_data=memory_data
    )

def get_memory_strategy_recommendation_prompt(
    current_query: str,
    workflow_type: str,
    evidence_summary: str,
    pattern_analysis: str,
    historical_outcomes: str,
    success_factors: str
) -> str:
    """
    Get memory-based strategy recommendation prompt.
    
    Args:
        current_query: Current query being processed
        workflow_type: Type of current workflow
        evidence_summary: Summary of current evidence
        pattern_analysis: Analysis of memory patterns
        historical_outcomes: Historical outcome data
        success_factors: Identified success factors
    
    Returns:
        Formatted memory strategy recommendation prompt
    """
    return MEMORY_STRATEGY_RECOMMENDATION_PROMPT.format(
        current_query=current_query,
        workflow_type=workflow_type,
        evidence_summary=evidence_summary,
        pattern_analysis=pattern_analysis,
        historical_outcomes=historical_outcomes,
        success_factors=success_factors
    )
