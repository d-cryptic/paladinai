"""
Prompts for Loki node operations.

This module contains prompts for LogQL query generation, tool selection,
and log data processing.
"""


def get_loki_query_generation_prompt(user_input: str, context: dict) -> str:
    """
    Generate prompt for creating LogQL queries.
    
    Args:
        user_input: The user's request
        context: Additional context about the workflow
        
    Returns:
        Prompt for LogQL query generation
    """
    return f"""Generate a LogQL query based on the user's request.

User Request: {user_input}
Workflow Context: {context.get('workflow_type', 'general')}

Generate an appropriate LogQL query that will retrieve the relevant log data.

IMPORTANT: To avoid empty logs, consider:
- Adding line filters to get non-empty logs: |~ ".+" (matches any non-empty line)
- Using specific filters for common log patterns
- For container logs, filter for actual log content: |~ "level=" or |~ "msg="

LogQL Query Language Reference:
1. Basic selector: {{job="myapp"}} - selects logs from a specific job
2. Label matching: {{job=~".+"}} - regex match for any job
3. Multiple labels: {{job="myapp", level="error"}} - AND condition
4. Line filters:
   - |~ "pattern" - line contains (regex)
   - !~ "pattern" - line does not contain
   - |= "exact" - line contains exact string
5. JSON parsing: | json - parse JSON logs (ONLY use if logs are in JSON format)
6. Label filters after parsing: | json | level="error" (requires JSON logs)
7. Logfmt parsing: | logfmt - parse logfmt formatted logs
8. Pattern parsing: | pattern "<pattern>" - extract fields using pattern
9. Line formatting: | line_format "{{{{.field}}}}" - format output
10. LogQL Metric queries (for LOG-based metrics, NOT system metrics):
   - rate({{job="app"}}[5m]) - rate of log lines over 5 minutes
   - sum by (level) (rate({{job="app"}}[5m])) - sum log rates by level
   - count_over_time({{job="app"}}[1h]) - count logs over time
   
   IMPORTANT: LogQL metrics are for analyzing LOG data (counting logs, log rates, etc.).
   For system metrics like CPU, memory, network I/O, use Prometheus, NOT Loki!

IMPORTANT RULES:
- When the user asks for "latest logs" or doesn't specify filters, use {{job=~".+"}} |~ ".+" to capture all non-empty logs
- DO NOT use "sort" in LogQL queries - sorting is handled by the direction parameter
- DO NOT use "limit" in LogQL queries - use the limit parameter instead
- DO NOT use | json unless you know the logs are in JSON format
- If you see parsing errors in results, try without | json or use | logfmt for logfmt-style logs
- Valid query examples:
  - {{job=~".+"}} |~ ".+" - get all non-empty logs (recommended)
  - {{job=~".+"}} |~ "error" - get logs containing "error"
  - {{job=~".+"}} |~ "level=" - get structured logs with level field
  - {{job=~".+"}} | logfmt - parse logfmt style logs
  - {{job=~".+"}} | json - parse JSON logs (only if logs are JSON)

Response format (JSON):
{{
    "query": "the LogQL query string",
    "description": "what this query does",
    "is_metric": boolean (true if this is a metric query),
    "labels": {{label_name: label_value}},
    "filters": ["list", "of", "filters"],
    "expected_content": "what kind of log content we expect to see"
}}
"""


def get_loki_analysis_prompt(user_input: str) -> str:
    """
    Generate prompt for analyzing Loki query requirements.
    
    Args:
        user_input: The user's request
        
    Returns:
        Prompt for requirements analysis
    """
    return f"""Analyze what type of Loki data is needed for this request.

User Request: {user_input}

IMPORTANT: Loki is for LOG data only. System metrics (CPU, memory, network) come from Prometheus!
Only identify LOG-related requirements from the request.

Determine:
1. Whether log data is needed (e.g., error logs, warn logs, application logs)
2. Whether LOG-based metrics are needed (e.g., log rates, error counts)
3. What time range would be appropriate for logs
4. What labels might be relevant for log filtering
5. The type of log analysis needed

NOTE: Ignore any requests for system metrics like CPU usage, memory usage, or network I/O.
Those are handled by Prometheus, not Loki!

Response format (JSON):
{{
    "needs_logs": boolean,
    "needs_metrics": boolean (only for LOG-based metrics),
    "time_range": "duration string (e.g., '1h', '24h', '7d')",
    "labels_needed": ["list", "of", "potential", "labels"],
    "analysis_type": "general|error_analysis|performance|audit|security",
    "ignored_metrics": ["list of system metrics that should go to Prometheus"]
}}
"""


def get_loki_tool_decision_prompt(user_input: str, query_info: dict, available_tools: list) -> str:
    """
    Generate prompt for deciding which Loki tools to use and creating their queries.
    
    Args:
        user_input: The user's request
        query_info: Generated query information
        available_tools: List of available Loki tools
        
    Returns:
        Prompt for tool selection and query generation
    """
    return f"""Based on the user's request, decide which Loki tools to use AND generate the appropriate LogQL queries.

User Request: {user_input}
Suggested Base Query: {query_info.get('query', 'N/A')}
Query Type: {'Metric' if query_info.get('is_metric') else 'Log'}
Available Tools: {available_tools}

Tool descriptions and parameters:
1. loki.query: Execute instant log query at current time
   - query: LogQL query string (required)
   - limit: max number of entries (default: 100)
   - direction: "forward" or "backward" (default: "backward")

2. loki.query_range: Execute log query over a time range
   - query: LogQL query string (required)
   - start: start time (e.g., "now-1h", "2024-01-01T00:00:00Z")
   - end: end time (default: "now")
   - limit: max entries per stream (default: 100)
   - direction: "forward" or "backward"

3. loki.get_labels: Get all available label names
   - start: start time for label discovery
   - end: end time for label discovery

4. loki.get_label_values: Get all values for a specific label
   - label: label name to get values for (required)
   - start: start time
   - end: end time

5. loki.get_series: Get log series/streams matching selector
   - match: label selector (e.g., {{job="myapp"}})
   - start: start time
   - end: end time

6. loki.query_metrics: Execute LogQL metric queries (for LOG-based metrics ONLY)
   - query: LogQL metric query (required) - e.g., rate({{job="app"}}[5m])
   - start: start time
   - end: end time
   - step: query resolution step
   
   WARNING: This is for LOG-based metrics (log rates, counts, etc.), NOT system metrics!
   For CPU, memory, network metrics, those should be handled by Prometheus, not Loki!

IMPORTANT Query Generation Rules:
1. For "latest logs" or general log requests, use {{job=~".+"}} to get all available logs
2. Time parameters MUST use relative format: "now", "now-1h", "now-24h", "now-7d"
   - "latest" or "recent" → start: "now-5m", end: "now"
   - "last hour" → start: "now-1h", end: "now"
   - "last 24 hours" → start: "now-24h", end: "now"
   - "last week" → start: "now-7d", end: "now"
3. For error searches, add line filters like |~ "(?i)error"
4. For specific services, use label selectors like {{service="service-name"}}
5. Always specify appropriate time ranges based on user request
6. DO NOT include "sort" or "limit" in the LogQL query itself
7. Use simple queries without invalid LogQL syntax
8. DO NOT use | json unless you're certain logs are JSON format
9. Start with simple queries like {{job=~".+"}} for general log fetching
10. Valid examples: {{job=~".+"}}, {{job=~".+"}} |~ "error", {{job=~".+"}} | logfmt

CRITICAL: DO NOT use Loki for system metrics like:
- CPU usage, memory usage, disk usage, network I/O
- Response times, latency, throughput
- System performance metrics
These are Prometheus metrics, NOT Loki log data!

For the user's request, ONLY handle the LOG-related parts (e.g., "warn logs").
Ignore requests for CPU, memory, or network metrics - those go to Prometheus!

Response format (JSON):
{{
    "tool_calls": [
        {{
            "tool": "tool_name",
            "parameters": {{
                "query": "generated LogQL query",
                "start": "time expression",
                "limit": 100
            }}
        }}
    ]
}}

Generate the complete query and parameters for each tool based on the user's specific request."""


def get_loki_processing_prompt(log_data: dict, user_input: str) -> str:
    """
    Generate prompt for processing collected log data.
    
    Args:
        log_data: Collected log data
        user_input: Original user request
        
    Returns:
        Prompt for data processing
    """
    return f"""Analyze the collected log data and provide insights based on the user's request.

User Request: {user_input}

Log Data Summary:
- Total logs: {len(log_data.get('logs', []))}
- Time range: {log_data.get('metadata', {}).get('time_range', 'N/A')}
- Available labels: {log_data.get('labels', {})}

Sample Logs (first 10):
{str(log_data.get('logs', [])[:10])}

Provide a comprehensive analysis including:
1. Summary of findings
2. Key patterns or issues identified
3. Relevant statistics
4. Recommendations based on the logs

Focus on answering the user's specific request while highlighting any important findings.
"""


def get_loki_action_analysis_prompt(log_data: dict, user_input: str, action_context: dict) -> str:
    """
    Generate prompt for action-oriented log analysis.
    
    Args:
        log_data: Collected log data
        user_input: User request
        action_context: Action workflow context
        
    Returns:
        Prompt for action analysis
    """
    return f"""Analyze logs to support action planning and execution.

User Request: {user_input}
Action Type: {action_context.get('action_type', 'unknown')}
Data Requirements: {action_context.get('data_requirements', [])}

Log Data Summary:
- Total logs: {len(log_data.get('logs', []))}
- Error logs: {sum(1 for log in log_data.get('logs', []) if 'error' in str(log).lower())}
- Services: {log_data.get('metadata', {}).get('services', [])}

Analyze the logs to:
1. Identify issues that need action
2. Find patterns relevant to the requested action
3. Determine affected components
4. Suggest specific actions based on log evidence

Response format (JSON):
{{
    "key_findings": ["list of important discoveries"],
    "affected_components": ["list of affected services/components"],
    "error_patterns": ["identified error patterns"],
    "recommended_actions": [
        {{
            "action": "specific action to take",
            "reason": "why this action is needed",
            "priority": "high|medium|low"
        }}
    ],
    "relevant_patterns": ["patterns to watch for"]
}}
"""


def get_loki_incident_analysis_prompt(log_data: dict, user_input: str, incident_context: dict) -> str:
    """
    Generate prompt for incident investigation using logs.
    
    Args:
        log_data: Collected log data
        user_input: User request
        incident_context: Incident workflow context
        
    Returns:
        Prompt for incident analysis
    """
    return f"""Perform forensic analysis of logs for incident investigation.

Incident Description: {user_input}
Severity: {incident_context.get('severity', 'unknown')}
Incident Type: {incident_context.get('incident_type', 'unknown')}
Investigation Focus: {incident_context.get('investigation_focus', [])}

Log Data:
- Total logs: {len(log_data.get('logs', []))}
- Time span: {log_data.get('metadata', {}).get('time_range', 'N/A')}
- Error count: {sum(1 for log in log_data.get('logs', []) if 'error' in str(log).lower())}

Perform a detailed incident analysis:
1. Identify the timeline of events
2. Find error patterns and anomalies
3. Determine affected services and components
4. Look for root cause indicators
5. Extract critical log entries
6. Identify any security concerns

Response format (JSON):
{{
    "incident_timeline": [
        {{
            "timestamp": "when it occurred",
            "event": "what happened",
            "severity": "impact level",
            "service": "affected service"
        }}
    ],
    "error_patterns": ["identified error patterns"],
    "affected_services": ["list of affected services"],
    "root_cause_indicators": ["potential root causes"],
    "security_concerns": ["any security issues found"],
    "critical_patterns": ["patterns that indicate critical issues"],
    "recommendations": ["immediate actions to take"]
}}
"""