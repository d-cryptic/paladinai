"""
Reporting Agent Prompts for PaladinAI Agent Orchestration.

This module contains prompts for the reporting agent that generates
comprehensive reports and summaries.
"""

REPORTING_PROMPT = """
You are an expert SRE generating a comprehensive incident response report.

Create a detailed, professional report that includes:
1. Executive Summary with specific metric values
2. Timeline of Investigation
3. Evidence Analysis with actual numbers and units
4. Findings and Root Cause with quantitative data
5. Actions Taken
6. Recommendations
7. Lessons Learned

CRITICAL REQUIREMENTS:
- Include specific metric values with units (e.g., "CPU: 85.2%", "Memory: 3.1GB", "Response time: 250ms")
- Use actual numbers from monitoring data
- Avoid generic phrases like "Analysis completed" or "Data processed"
- Quantify impact and performance metrics

Use clear, technical language appropriate for both technical and management audiences.
"""

COMPREHENSIVE_REPORT_PROMPT = """
Generate a comprehensive report for this workflow:

Workflow Details:
- ID: {workflow_id}
- Type: {workflow_type}
- Initial Query: {initial_query}
- Duration: {duration:.2f} seconds
- Iterations: {iterations}

Final State:
- Confidence Score: {confidence_score:.2f}
- Evidence Count: {evidence_count}
- Hypotheses: {hypotheses_count}
- Recommendations: {recommendations_count}
- Actions Taken: {actions_taken_count}

Key Findings:
{findings}

Tools Used: {tools_used}

Please generate a comprehensive, professional report that summarizes the investigation,
findings, and recommendations. Format it in clear markdown with appropriate sections.

CRITICAL REQUIREMENTS:
- Include specific metric values with units throughout the report
- Use actual numbers from monitoring data (e.g., "CPU usage reached 95.3%")
- Quantify performance impacts and system metrics
- Avoid generic phrases like "Analysis completed" or "Data processed"
- Show before/after metrics when applicable

The report should be suitable for both technical teams and management, providing
different levels of detail as appropriate with quantitative data.
"""

QUERY_RESPONSE_TEMPLATE = """
# Query Response

**Query:** {query}
**Status:** {status}
**Confidence:** {confidence:.2f}

## Answer
{answer}

## Specific Metrics
{specific_metrics}

## Supporting Data
{supporting_data}

## Additional Notes
{notes}

Note: All metric values should include units (%, GB, ms, count, etc.)
"""

INCIDENT_REPORT_TEMPLATE = """
# Incident Response Report

**Incident ID:** {workflow_id}
**Query:** {initial_query}
**Status:** {status}
**Severity:** {severity}
**Duration:** {duration:.2f} seconds

## Executive Summary
{executive_summary}

## Technical Analysis
{technical_analysis}

## Evidence Collected
{evidence_summary}

## Root Cause Analysis
{root_cause}

## Recommendations
{recommendations}

## Actions Taken
{actions_taken}

## Next Steps
{next_steps}
"""

ACTION_REPORT_TEMPLATE = """
# Data Analysis Report

**Request:** {initial_query}
**Analysis Type:** {analysis_type}
**Data Sources:** {data_sources}
**Timeframe:** {timeframe}

## Summary
{summary}

## Data Analysis
{data_analysis}

## Key Findings
{key_findings}

## Trends and Patterns
{trends}

## Recommendations
{recommendations}

## Raw Data Summary
{raw_data_summary}
"""

def get_comprehensive_report_prompt(workflow_details: dict) -> str:
    """
    Get comprehensive report prompt with workflow details.

    Args:
        workflow_details: Dictionary containing workflow information

    Returns:
        Formatted comprehensive report prompt
    """
    return COMPREHENSIVE_REPORT_PROMPT.format(
        workflow_id=workflow_details.get('workflow_id', 'Unknown'),
        workflow_type=workflow_details.get('workflow_type', 'Unknown'),
        initial_query=workflow_details.get('initial_query', 'Unknown'),
        duration=workflow_details.get('duration', 0.0),
        iterations=workflow_details.get('iterations', 0),
        confidence_score=workflow_details.get('confidence_score', 0.0),
        evidence_count=workflow_details.get('evidence_count', 0),
        hypotheses_count=workflow_details.get('hypotheses_count', 0),
        recommendations_count=workflow_details.get('recommendations_count', 0),
        actions_taken_count=workflow_details.get('actions_taken_count', 0),
        findings=workflow_details.get('findings', 'No findings available'),
        tools_used=workflow_details.get('tools_used', 'None')
    )

def get_workflow_report_template(workflow_type: str) -> str:
    """
    Get appropriate report template based on workflow type.

    Args:
        workflow_type: Type of workflow ("QUERY", "INCIDENT", or "ACTION")

    Returns:
        Appropriate report template
    """
    templates = {
        "QUERY": QUERY_RESPONSE_TEMPLATE,
        "INCIDENT": INCIDENT_REPORT_TEMPLATE,
        "ACTION": ACTION_REPORT_TEMPLATE
    }

    return templates.get(workflow_type.upper(), INCIDENT_REPORT_TEMPLATE)