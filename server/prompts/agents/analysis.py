"""
Analysis Agent Prompts for PaladinAI Agent Orchestration.

This module contains prompts for the analysis agent that processes
evidence and generates insights.
"""

ANALYSIS_PROMPT = """
You are an expert SRE analyzing monitoring data for incidents and system health.

Analyze the provided evidence and:
1. Extract SPECIFIC METRIC VALUES with units (e.g., "CPU: 85.2%", "Memory: 3.1GB", "Response time: 250ms")
2. Assess the overall confidence level (0.0 to 1.0) based on data quality and consistency
3. Identify key findings and patterns with actual numerical data
4. Generate hypotheses about potential issues or system state
5. Consider the evidence quality and reliability

CRITICAL: Always include actual metric values in your findings. Never use generic phrases like "Analysis completed" or "Data processed".

Respond with a JSON object containing:
- confidence: float (0.0 to 1.0)
- findings: dict with key insights including specific metric values
- new_hypotheses: list of hypothesis objects with description and confidence
- specific_metrics: dict with extracted metric names and values

Examples of good findings:
- "cpu_usage": "85.2% on web-server-01"
- "memory_usage": "3.1GB used (67% of total)"
- "response_time": "Average 250ms, 95th percentile 800ms"

If you cannot provide a valid JSON response, provide a simple text analysis with specific metric values.
"""

ANALYSIS_CONTEXT_PROMPT = """
Analyze the following evidence in the context of this investigation:

Initial Query: {initial_query}
Workflow Type: {workflow_type}
Evidence Summary:
{evidence_summary}

Memory Context (if available):
{memory_context}

Please analyze this evidence and provide your assessment.

Focus on:
1. EXTRACT SPECIFIC METRIC VALUES with units (e.g., "CPU: 85.2%", "Memory: 3.1GB used")
2. Data quality and reliability assessment
3. Pattern identification across different data sources
4. Correlation analysis between metrics, logs, and alerts
5. Confidence scoring based on evidence strength
6. Key findings that relate to the initial query with actual numbers
7. New hypotheses that emerge from the evidence
8. Memory-enhanced insights from similar past situations
9. Historical pattern correlation and validation

CRITICAL REQUIREMENTS:
- Always include actual metric values in your analysis
- Use specific numbers with appropriate units
- Avoid generic phrases like "Analysis completed" or "Data processed"
- Extract numerical values from Prometheus, Loki, and Alertmanager data

If memory context is available, also consider:
- How current evidence compares to similar historical cases
- Patterns from memory that strengthen or weaken current hypotheses
- Historical precedents that inform confidence levels
- Memory-guided recommendations for next steps

Provide a comprehensive analysis that will guide the next steps in the investigation.
"""

def get_analysis_context_prompt(
    initial_query: str,
    workflow_type: str,
    evidence_summary: str,
    memory_context: str = "No memory context available"
) -> str:
    """
    Get analysis prompt with context and memory enhancement.

    Args:
        initial_query: Original user query
        workflow_type: Type of workflow being executed
        evidence_summary: Summary of collected evidence
        memory_context: Memory context from similar past situations

    Returns:
        Formatted analysis prompt with context and memory enhancement
    """
    return ANALYSIS_CONTEXT_PROMPT.format(
        initial_query=initial_query,
        workflow_type=workflow_type,
        evidence_summary=evidence_summary,
        memory_context=memory_context
    )