"""
Result guidance and constants for workflow nodes.

This module contains guidance information and error messages
used by the result node for different workflow types.
"""

# Workflow guidance for different types
WORKFLOW_GUIDANCE = {
    "QUERY": {
        "description": "Quick status/boolean information request",
        "typical_response_time": "< 30 seconds",
        "data_sources": ["Prometheus metrics", "Alertmanager alerts"],
        "response_format": "Concise, factual, boolean or status-based"
    },
    "INCIDENT": {
        "description": "Problem investigation and root cause analysis",
        "typical_response_time": "2-10 minutes",
        "data_sources": ["Prometheus metrics", "Loki logs", "Alertmanager alerts"],
        "response_format": "Comprehensive analysis with root cause investigation"
    },
    "ACTION": {
        "description": "Data retrieval, analysis, and reporting",
        "typical_response_time": "1-5 minutes",
        "data_sources": ["Historical metrics", "Log aggregations", "Performance data"],
        "response_format": "Structured data with detailed analysis and reports"
    }
}

# Default guidance for unknown workflow types
DEFAULT_WORKFLOW_GUIDANCE = {
    "description": "Unknown workflow type",
    "typical_response_time": "Variable",
    "data_sources": ["To be determined"],
    "response_format": "To be determined"
}

# Error messages
NO_CATEGORIZATION_ERROR = "I am a very dumb intern. I don't know anything other than SRE, DevOps, and system reliability. Please ask questions related to SRE, incident, or related technical operations."

# Success messages
QUERY_SUCCESS_MESSAGE = "✅ Query processed successfully"
ACTION_SUCCESS_MESSAGE = "✅ Action completed successfully"
INCIDENT_SUCCESS_MESSAGE = "✅ Incident analysis completed"

# Fallback content template
CATEGORIZATION_FALLBACK_TEMPLATE = "✅ Categorized as {workflow_type} workflow\n📊 Confidence: {confidence:.1%}\n💡 {reasoning}\n🎯 Suggested approach: {suggested_approach}"


def get_workflow_guidance(workflow_type: str) -> dict:
    """
    Get guidance for the next steps based on workflow type.
    
    Args:
        workflow_type: The categorized workflow type
        
    Returns:
        Dictionary containing guidance and next steps
    """
    return WORKFLOW_GUIDANCE.get(workflow_type, DEFAULT_WORKFLOW_GUIDANCE)