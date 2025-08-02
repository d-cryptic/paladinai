"""
Analyzer prompts for workflow nodes.

This module contains prompts for analyzing user requests to determine
data requirements and processing needs for different workflow types.
"""

QUERY_ANALYZER_PROMPT = """Analyze this user query to determine what data sources are needed:

User Query: {user_input}

Determine if this query requires:
1. Metrics data from Prometheus (CPU, memory, response times, rates, etc.)
2. Log data from Loki (error logs, application logs, system logs, etc.)
3. Alert data from Alertmanager (active alerts, alert history, silences, etc.)

IMPORTANT: Only set needs_logs=true if the user EXPLICITLY asks for:
- Logs, log entries, log messages, log data
- Error messages, stack traces, debug output
- Application output, console output
- Specific log analysis or log searching

DO NOT fetch logs for:
- General metrics queries (CPU, memory, etc.)
- Alert queries
- Performance monitoring
- Status checks

Consider:
- EXPLICIT keywords like "logs", "log entries", "error messages", "stack traces" require log data
- Keywords like "CPU", "memory", "rate", "throughput", "latency" suggest ONLY metrics
- Keywords like "alerts", "alarms", "notifications", "silenced", "firing" suggest ONLY alert data
- Only fetch multiple sources if explicitly requested

Response format (JSON):
{{
    "needs_metrics": boolean,
    "needs_logs": boolean,
    "needs_alerts": boolean,
    "reasoning": "brief explanation"
}}
"""

ACTION_ANALYZER_PROMPT = """Analyze this action request to determine data requirements:

User Request: {user_input}

Determine:
1. Does this action require metrics data from Prometheus (CPU, memory, rates, latency, etc.)?
2. Does this action require log data from Loki (error logs, application logs, system logs, etc.)?
3. Does this action require alert data from Alertmanager (active alerts, alert history, silences)?
4. What type of action is this (data_analysis, report_generation, trend_analysis, etc.)?
5. What level of analysis is needed (basic, intermediate, comprehensive)?

IMPORTANT: Only set needs_logs=true if the user EXPLICITLY mentions:
- Analyzing logs, error logs, application logs
- Debugging issues that require log inspection
- Finding error messages or stack traces
- Log-based root cause analysis

DO NOT fetch logs for:
- General performance analysis (use metrics only)
- Alert management or correlation (use alerts only)
- Capacity planning (use metrics only)
- Basic status or health checks

Consider:
- Actions EXPLICITLY mentioning "logs", "error analysis", "debugging with logs" need log data
- Actions involving performance analysis, capacity planning need ONLY metrics
- Actions involving alert management, silence creation need ONLY alert data
- Only fetch multiple sources if the user explicitly requests correlation between them

Response format (JSON):
{{
    "needs_metrics": boolean,
    "needs_logs": boolean,
    "needs_alerts": boolean,
    "action_type": "string",
    "analysis_level": "basic|intermediate|comprehensive",
    "reasoning": "brief explanation",
    "data_requirements": {{
        "metrics": ["list of specific metrics if needed"],
        "logs": ["list of log types if needed"],
        "alerts": ["list of alert types if needed"]
    }}
}}
"""

INCIDENT_ANALYZER_PROMPT = """Analyze this incident report to determine investigation requirements:

Incident Description: {user_input}

Determine:
1. Does this incident investigation require metrics data from Prometheus (CPU, memory, rates, latency, etc.)?
2. Does this incident investigation require log data from Loki (error logs, application logs, system logs, etc.)?
3. Does this incident investigation require alert data from Alertmanager (alert history, correlations, silences)?
4. What type of incident is this (performance, availability, security, data_loss, etc.)?
5. What is the severity level (low, medium, high, critical)?
6. What should be the investigation focus areas?

IMPORTANT: Only set needs_logs=true if:
- The incident explicitly mentions errors, exceptions, or application failures
- The user specifically asks to check logs or error messages
- The incident type is error/failure/crash related
- Log analysis is essential for root cause (not just nice to have)

For incidents, be selective:
- Performance degradation incidents: Usually ONLY need metrics (CPU, memory, latency)
- Alert-based incidents: Usually need alerts and metrics, logs only if errors mentioned
- Error/crash incidents: Need logs, along with metrics and alerts
- Availability incidents: Usually metrics and alerts suffice unless errors are mentioned

Consider:
- Not all incidents require all three data sources
- Performance incidents primarily need metrics
- Alert investigations primarily need alert history
- Only fetch logs when error analysis or debugging is explicitly needed
- Timeline reconstruction can often be done with metrics and alerts alone

Response format (JSON):
{{
    "needs_metrics": boolean,
    "needs_logs": boolean,
    "needs_alerts": boolean,
    "incident_type": "string",
    "severity": "low|medium|high|critical",
    "investigation_focus": ["list of focus areas"],
    "urgency": "low|normal|high|critical",
    "reasoning": "brief explanation",
    "data_requirements": {{
        "metrics": ["list of specific metrics if needed"],
        "logs": ["list of log types if needed"],
        "alerts": ["list of alert types if needed"]
    }}
}}
"""

ANALYZER_SYSTEM_PROMPT = """You are an expert SRE analyzing user requests to determine data collection and processing requirements. 

IMPORTANT: Be conservative about data collection - only fetch what is EXPLICITLY requested or absolutely necessary. 
- For logs: Only when user explicitly mentions logs, errors, stack traces, or debugging
- For metrics: When user asks about performance, resources, rates, or measurements
- For alerts: When user asks about alerts, alarms, or notifications

Avoid fetching unnecessary data sources that will slow down the response.

Respond only with valid JSON as specified in the prompt."""


def get_analyzer_prompt(workflow_type: str, **kwargs) -> str:
    """
    Get the appropriate analyzer prompt based on workflow type.
    
    Args:
        workflow_type: The type of workflow (QUERY, ACTION, INCIDENT)
        **kwargs: Variables to format into the prompt
        
    Returns:
        Formatted prompt string
    """
    prompts = {
        "QUERY": QUERY_ANALYZER_PROMPT,
        "ACTION": ACTION_ANALYZER_PROMPT,
        "INCIDENT": INCIDENT_ANALYZER_PROMPT
    }
    
    prompt_template = prompts.get(workflow_type.upper())
    if not prompt_template:
        raise ValueError(f"Unknown workflow type: {workflow_type}")
    
    return prompt_template.format(**kwargs)