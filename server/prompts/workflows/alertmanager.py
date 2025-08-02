"""
Prompts for Alertmanager node operations.

This module contains prompts for alert analysis, tool selection,
and alert data processing.
"""


def get_alertmanager_analysis_prompt(user_input: str) -> str:
    """
    Generate prompt for analyzing Alertmanager requirements.
    
    Args:
        user_input: The user's request
        
    Returns:
        Prompt for requirements analysis
    """
    return f"""Analyze what type of Alertmanager data is needed for this request.

User Request: {user_input}

Alertmanager provides:
- Active alerts (currently firing)
- Alert history
- Silenced alerts
- Alert groups and routing
- Alert severity and labels
- Alert annotations and descriptions

Determine:
1. Whether alert data is needed
2. What types of alerts to query (active, silenced, inhibited)
3. What time range would be appropriate
4. What filters might be relevant (severity, service, etc.)
5. Whether alert groups or routing information is needed

Response format (JSON):
{{
    "needs_alerts": boolean,
    "alert_types": ["active", "silenced", "inhibited", "history"],
    "severity_filter": ["critical", "warning", "info"] or null,
    "service_filter": ["service names"] or null,
    "time_range": "duration string (e.g., '1h', '24h', '7d')",
    "needs_groups": boolean,
    "analysis_type": "current_state|historical|trending|correlation"
}}
"""


def get_alertmanager_tool_decision_prompt(user_input: str, analysis: dict) -> str:
    """
    Generate prompt for deciding which Alertmanager tools to use.
    
    Args:
        user_input: The user's request
        analysis: Analysis results
        
    Returns:
        Prompt for tool selection
    """
    return f"""Based on the user's request, decide which Alertmanager tools to use.

User Request: {user_input}
Analysis: {analysis}

Available Alertmanager tools:

1. alertmanager.get_alerts: Get current alerts
   - active: filter for active alerts (boolean)
   - silenced: filter for silenced alerts (boolean)
   - inhibited: filter for inhibited alerts (boolean)
   - unprocessed: filter for unprocessed alerts (boolean)
   - filter: label filter string (e.g., 'severity="critical"')

2. alertmanager.get_silences: Get alert silences
   - filter: filter silences by label

3. alertmanager.create_silence: Create a new silence
   - matchers: list of label matchers
   - starts_at: when silence starts
   - ends_at: when silence ends
   - created_by: who created it
   - comment: reason for silence

4. alertmanager.get_alert_groups: Get alerts grouped by label
   - active: filter for active alerts
   - silenced: filter for silenced alerts
   - inhibited: filter for inhibited alerts
   - filter: label filter string
   - receiver: filter by receiver name

5. alertmanager.get_status: Get Alertmanager status
   - No parameters needed

6. alertmanager.get_receivers: Get configured receivers
   - No parameters needed

Tool Selection Guidelines:
- For current alert status: use get_alerts with active=true
- For alert history: use get_alerts with appropriate filters
- For understanding alert routing: use get_alert_groups
- For silence management: use get_silences
- For system health: use get_status

Response format (JSON):
{{
    "tool_calls": [
        {{
            "tool": "tool_name",
            "parameters": {{
                "param1": "value1",
                "param2": "value2"
            }},
            "purpose": "why this tool is being used"
        }}
    ]
}}
"""


def get_alertmanager_processing_prompt(alert_data: dict, user_input: str) -> str:
    """
    Generate prompt for processing collected alert data.
    
    Args:
        alert_data: Collected alert data
        user_input: Original user request
        
    Returns:
        Prompt for data processing
    """
    return f"""Analyze the collected alert data and provide insights based on the user's request.

User Request: {user_input}

Alert Data Summary:
- Total alerts: {len(alert_data.get('alerts', []))}
- Active alerts: {len([a for a in alert_data.get('alerts', []) if a.get('status', {}).get('state') == 'active'])}
- Silenced alerts: {len([a for a in alert_data.get('alerts', []) if a.get('status', {}).get('silencedBy')])}
- Alert groups: {len(alert_data.get('groups', []))}

Provide a comprehensive analysis including:
1. Current alert status summary
2. Critical alerts that need attention
3. Patterns in alerting (if any)
4. Recommendations for alert management

Focus on answering the user's specific request while highlighting critical findings.
"""


def get_alertmanager_query_prompt(alert_data: dict, user_input: str) -> str:
    """
    Generate prompt for query-specific alert analysis.
    
    Args:
        alert_data: Collected alert data
        user_input: User request
        
    Returns:
        Prompt for query analysis
    """
    return f"""Analyze alerts to answer the user's query.

User Request: {user_input}

Alert Data:
- Active alerts: {len([a for a in alert_data.get('alerts', []) if a.get('status', {}).get('state') == 'active'])}
- Critical alerts: {len([a for a in alert_data.get('alerts', []) if a.get('labels', {}).get('severity') == 'critical'])}

Provide a direct answer to the query based on alert data.
Focus on:
1. Yes/no answers if applicable
2. Current alert state
3. Any critical issues
4. Brief summary of relevant alerts

Response format (JSON):
{{
    "answer": "direct answer to the query",
    "alert_summary": "brief summary of relevant alerts",
    "critical_alerts": ["list of critical alert names"],
    "recommendations": ["any immediate actions needed"]
}}
"""


def get_alertmanager_action_prompt(alert_data: dict, user_input: str, action_context: dict) -> str:
    """
    Generate prompt for action-oriented alert analysis.
    
    Args:
        alert_data: Collected alert data
        user_input: User request
        action_context: Action workflow context
        
    Returns:
        Prompt for action analysis
    """
    return f"""Analyze alerts to support action planning and execution.

User Request: {user_input}
Action Type: {action_context.get('action_type', 'unknown')}

Alert Analysis:
- Total active alerts: {len([a for a in alert_data.get('alerts', []) if a.get('status', {}).get('state') == 'active'])}
- Alerts by severity: {alert_data.get('severity_breakdown', {})}
- Affected services: {alert_data.get('affected_services', [])}

Analyze the alerts to:
1. Identify alerts relevant to the requested action
2. Determine alert priorities
3. Find correlated alerts
4. Suggest actions based on alert patterns

Response format (JSON):
{{
    "relevant_alerts": [
        {{
            "name": "alert name",
            "severity": "severity level",
            "service": "affected service",
            "description": "what the alert means",
            "action_needed": "what should be done"
        }}
    ],
    "alert_patterns": ["identified patterns"],
    "recommended_actions": [
        {{
            "action": "specific action to take",
            "reason": "why based on alerts",
            "priority": "high|medium|low",
            "related_alerts": ["alert names"]
        }}
    ],
    "silence_recommendations": ["alerts that might need silencing during action"]
}}
"""


def get_alertmanager_incident_prompt(alert_data: dict, user_input: str, incident_context: dict) -> str:
    """
    Generate prompt for incident investigation using alerts.
    
    Args:
        alert_data: Collected alert data
        user_input: User request
        incident_context: Incident workflow context
        
    Returns:
        Prompt for incident analysis
    """
    return f"""Perform forensic analysis of alerts for incident investigation.

Incident Description: {user_input}
Severity: {incident_context.get('severity', 'unknown')}
Incident Type: {incident_context.get('incident_type', 'unknown')}

Alert Data:
- Total alerts: {len(alert_data.get('alerts', []))}
- Time range: {alert_data.get('time_range', 'N/A')}
- Critical alerts: {len([a for a in alert_data.get('alerts', []) if a.get('labels', {}).get('severity') == 'critical'])}

Perform a detailed incident analysis using alerts:
1. Identify the alert timeline leading to the incident
2. Find correlated alerts
3. Determine which alerts fired first (potential root cause)
4. Identify affected services from alerts
5. Look for alert patterns indicating the issue
6. Check for any missed or delayed alerts

Response format (JSON):
{{
    "alert_timeline": [
        {{
            "timestamp": "when alert fired",
            "alert_name": "name of the alert",
            "severity": "alert severity",
            "service": "affected service",
            "significance": "why this alert matters for the incident"
        }}
    ],
    "root_cause_alerts": ["alerts that likely indicate root cause"],
    "correlated_alerts": [
        {{
            "primary_alert": "main alert",
            "related_alerts": ["list of related alerts"],
            "correlation_reason": "why they're related"
        }}
    ],
    "affected_services": ["services identified from alerts"],
    "alert_patterns": ["patterns that indicate the incident type"],
    "missing_alerts": ["expected alerts that didn't fire"],
    "recommendations": ["immediate actions based on alerts"]
}}
"""