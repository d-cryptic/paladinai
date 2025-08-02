"""
Processor prompts for workflow nodes.

This module contains system prompts used by processor functions
in query, action, and incident nodes for data processing.
"""

# Query processor prompts
QUERY_METRICS_SYSTEM_PROMPT = """You are an expert SRE formatting query responses with metrics data. Always include the word 'json' in your response when using JSON format."""

QUERY_COMBINED_SYSTEM_PROMPT = """You are an expert SRE formatting query responses with metrics and log data. Always include the word 'json' in your response when using JSON format."""

QUERY_NON_METRICS_SYSTEM_PROMPT = """You are an expert SRE providing direct answers to non-metrics queries. Always include the word 'json' in your response when using JSON format."""

# Action processor prompts
ACTION_NON_METRICS_SYSTEM_PROMPT = """You are an expert SRE handling non-metrics action requests. Always include the word 'json' in your response when using JSON format."""

ACTION_RESPONSE_TYPE_SYSTEM_PROMPT = """You are an expert SRE determining appropriate response types. Always include the word 'json' in your response when using JSON format."""

ACTION_METRICS_SYSTEM_PROMPT = """You are an expert SRE processing action results with metrics data. Always include the word 'json' in your response when using JSON format."""

# Action Loki processing prompts based on data availability
ACTION_COMBINED_DATA_SYSTEM_PROMPT = """You are an expert SRE analyzing both logs and metrics data. Correlate the data types to provide comprehensive insights. Extract actual metric values and connect them with log events. Always include the word 'json' in your response when using JSON format."""

ACTION_LOGS_ONLY_SYSTEM_PROMPT = """You are an expert SRE analyzing log data. Focus on extracting meaningful insights from the logs. Always include the word 'json' in your response when using JSON format."""

ACTION_METRICS_ONLY_SYSTEM_PROMPT = """You are an expert SRE processing metrics data. Extract and return the actual numerical values. Always include the word 'json' in your response when using JSON format."""

ACTION_GENERAL_SYSTEM_PROMPT = """You are an expert SRE processing monitoring data. Analyze all available data comprehensively. Always include the word 'json' in your response when using JSON format."""

# Incident processor prompts
INCIDENT_INVESTIGATION_SYSTEM_PROMPT = """You are an expert SRE conducting incident investigation with metrics data. Always include the word 'json' in your response when using JSON format."""

INCIDENT_REPORT_SYSTEM_PROMPT = """You are an expert SRE creating comprehensive incident reports. Always include the word 'json' in your response when using JSON format."""

INCIDENT_COMBINED_SYSTEM_PROMPT = """You are an expert SRE conducting incident investigation with metrics and log data. Always include the word 'json' in your response when using JSON format."""


def get_processor_system_prompt(node_type: str, context: str = "default", **kwargs) -> str:
    """
    Get the appropriate system prompt for processor functions.
    
    Args:
        node_type: The type of node (QUERY, ACTION, INCIDENT)
        context: The specific context within the processor
        **kwargs: Additional context for selecting the right prompt
        
    Returns:
        Appropriate system prompt string
    """
    prompts = {
        "QUERY": {
            "metrics": QUERY_METRICS_SYSTEM_PROMPT,
            "combined": QUERY_COMBINED_SYSTEM_PROMPT,
            "non_metrics": QUERY_NON_METRICS_SYSTEM_PROMPT,
            "default": QUERY_METRICS_SYSTEM_PROMPT
        },
        "ACTION": {
            "non_metrics": ACTION_NON_METRICS_SYSTEM_PROMPT,
            "response_type": ACTION_RESPONSE_TYPE_SYSTEM_PROMPT,
            "metrics": ACTION_METRICS_SYSTEM_PROMPT,
            "combined_data": ACTION_COMBINED_DATA_SYSTEM_PROMPT,
            "logs_only": ACTION_LOGS_ONLY_SYSTEM_PROMPT,
            "metrics_only": ACTION_METRICS_ONLY_SYSTEM_PROMPT,
            "general": ACTION_GENERAL_SYSTEM_PROMPT,
            "default": ACTION_METRICS_SYSTEM_PROMPT
        },
        "INCIDENT": {
            "investigation": INCIDENT_INVESTIGATION_SYSTEM_PROMPT,
            "report": INCIDENT_REPORT_SYSTEM_PROMPT,
            "combined": INCIDENT_COMBINED_SYSTEM_PROMPT,
            "default": INCIDENT_INVESTIGATION_SYSTEM_PROMPT
        }
    }
    
    node_prompts = prompts.get(node_type.upper(), {})
    if not node_prompts:
        raise ValueError(f"Unknown node type: {node_type}")
    
    return node_prompts.get(context, node_prompts.get("default", ""))