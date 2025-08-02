"""
Result formatting prompts for workflow responses.

This module contains prompts for formatting workflow results into
clean, user-friendly markdown using OpenAI.
"""

QUERY_FORMATTING_PROMPT = """
Format this query response into clean, user-friendly markdown:

User Request: {user_request}
Query Result: {query_result}

Requirements:
- Start with a clear answer to the user's question
- If it's a yes/no question, state YES or NO clearly
- Include relevant details or explanations
- Keep it concise and direct
- Use markdown formatting (headers, lists, bold, etc.) appropriately
"""

ACTION_FORMATTING_PROMPT = """
Format this action workflow response into clean, user-friendly markdown:

User Request: {user_request}
Action Result: {action_result}
Metrics Data: {metrics_data}
Logs Data: {logs_data}
Total Logs Retrieved: {total_logs}
Alerts Data: {alerts_data}
Total Alerts: {total_alerts}

Requirements:
- Provide exactly what the user requested (metrics values, logs, alerts, or combination)
- If user asked for metrics, show the actual numerical values prominently
- If user asked for logs, show the actual log entries in a formatted code block
- If user asked for alerts, show alert names, severity, and summaries
- If user asked for multiple types, show each clearly separated
- Add a brief summary or insight
- Use markdown formatting with clear sections
- Keep technical details but make it readable
- For logs, show them in a formatted way with timestamps
- For alerts, show them in a table or structured format
"""

INCIDENT_FORMATTING_PROMPT = """
Format this incident analysis into a comprehensive markdown report:

User Request: {user_request}
Incident Analysis: {incident_analysis}
Metrics Data: {metrics_data}
Logs Data: {logs_data}
Alerts Data: {alerts_data}

Requirements:
- Start with an executive summary
- Include timeline of events (including when alerts fired)
- Show root cause analysis
- Display relevant metrics, logs, and alerts that support the analysis
- Show which alerts were triggered and when
- Provide clear recommendations with priorities
- Include all critical timestamps and error patterns
- Use markdown formatting with clear sections and subsections
- Make it comprehensive but well-organized
- Include alert correlation if multiple alerts are related
"""

FALLBACK_FORMATTING_PROMPT = """
Format this workflow response into clean markdown:

Data: {format_data}

Requirements:
- Present the information clearly
- Use appropriate markdown formatting
- Make it user-friendly
"""

FORMATTING_SYSTEM_PROMPT = """You are an expert technical writer formatting SRE workflow results into clear, professional markdown. Focus on delivering exactly what the user requested in a well-organized format."""


def get_formatting_prompt(workflow_type: str, **kwargs) -> str:
    """
    Get the appropriate formatting prompt based on workflow type.
    
    Args:
        workflow_type: The type of workflow (QUERY, ACTION, INCIDENT)
        **kwargs: Variables to format into the prompt
        
    Returns:
        Formatted prompt string
    """
    prompts = {
        "QUERY": QUERY_FORMATTING_PROMPT,
        "ACTION": ACTION_FORMATTING_PROMPT,
        "INCIDENT": INCIDENT_FORMATTING_PROMPT,
        "FALLBACK": FALLBACK_FORMATTING_PROMPT
    }
    
    # Provide defaults for optional alert parameters
    if workflow_type.upper() == "ACTION" and "alerts_data" not in kwargs:
        kwargs["alerts_data"] = "[]"
        kwargs["total_alerts"] = 0
    elif workflow_type.upper() == "INCIDENT" and "alerts_data" not in kwargs:
        kwargs["alerts_data"] = "{}"
    
    prompt_template = prompts.get(workflow_type.upper(), FALLBACK_FORMATTING_PROMPT)
    return prompt_template.format(**kwargs)