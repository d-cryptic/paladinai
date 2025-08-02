"""
Data Collection Prompts for INCIDENT Workflows.

This module contains prompts specifically designed for INCIDENT workflow
data collection, evaluation, and processing.
"""

INCIDENT_DATA_COLLECTION_PROMPT = """
You are an expert SRE conducting incident response and data collection for root cause analysis.

Your task is to determine what comprehensive data needs to be collected to investigate and analyze the reported incident.

Incident Description: {user_input}

Available Data Sources:
- Prometheus: Metrics data for performance analysis and trend identification
- Loki: Log data for error analysis and event correlation
- Alertmanager: Alert history and current alert status

For INCIDENT workflows, focus on:
1. Comprehensive data collection from multiple sources
2. Historical data to establish timeline and trends
3. Error logs and exception patterns
4. Performance metrics before, during, and after incident
5. Alert correlation and escalation patterns

Analyze the incident and determine:
- Which data sources are critical for investigation
- What time range covers the incident timeline
- What specific metrics indicate the problem scope
- What log patterns might reveal root cause
- What alerts are related to this incident

Respond with JSON containing:
- required_tools: list of tools needed for comprehensive investigation
- time_range: incident timeline with pre/during/post periods
- investigation_focus: key areas to investigate
- data_correlation_points: data points that should be correlated
- completeness_criteria: what constitutes thorough incident data
"""

INCIDENT_DATA_EVALUATION_PROMPT = """
You are an expert SRE evaluating whether collected data is sufficient for comprehensive incident analysis.

Incident Description: {user_input}
Collected Data Summary: {collected_data}

For INCIDENT workflows, evaluate:
1. Do we have sufficient data to understand the incident scope?
2. Can we establish a clear timeline of events?
3. Are there enough data points for root cause analysis?
4. Do we have data from all relevant systems/components?
5. Can we assess impact and severity accurately?

Evaluation Criteria for INCIDENT workflows:
- Scope Coverage: Data from all affected systems
- Timeline Completeness: Data covering incident lifecycle
- Root Cause Indicators: Sufficient data for cause analysis
- Impact Assessment: Data to measure business/technical impact
- Correlation Potential: Data that can be cross-referenced

Respond with JSON containing:
- completeness_score: float (0.0 to 1.0)
- missing_requirements: specific gaps in incident data
- investigation_readiness: assessment of readiness for analysis
- data_correlation_opportunities: potential data correlations
- next_tools: additional tools needed for complete investigation
- is_complete: boolean indicating if investigation can proceed
- reasoning: detailed explanation of evaluation
"""

INCIDENT_OUTPUT_FORMATTING_PROMPT = """
You are an expert SRE conducting comprehensive incident analysis and creating detailed incident reports.

Incident Description: {user_input}
Collected Data: {collected_data}
Investigation Timeline: {timeline}

Your task is to create a comprehensive incident report following SRE best practices.

For INCIDENT workflows, your report must include:
1. Executive Summary: Clear, concise incident overview
2. Timeline: Chronological sequence of events
3. Impact Assessment: Business and technical impact analysis
4. Root Cause Analysis: Detailed investigation findings
5. Resolution Steps: Actions taken to resolve the incident
6. Recommendations: Prevention and improvement measures

Report Structure:
- Start with severity assessment and current status
- Provide detailed timeline with data timestamps (IST format)
- Include specific metrics and log evidence
- Analyze contributing factors and root causes
- Recommend immediate and long-term actions

Respond with JSON containing:
- incident_report: comprehensive incident summary
- severity: "critical", "high", "medium", or "low"
- impact_assessment: detailed impact analysis
- root_cause_analysis: investigation findings and hypotheses
- timeline: chronological sequence of events with timestamps
- recommendations: actionable recommendations for resolution/prevention
- confidence: float (0.0 to 1.0) indicating confidence in analysis
- escalation_needed: boolean indicating if escalation is required
- next_steps: immediate actions to take
"""

INCIDENT_INVESTIGATION_PROMPT = """
You are an expert SRE conducting systematic incident investigation using collected monitoring data.

Incident Description: {user_input}
Available Evidence: {collected_data}

Investigation Framework:
1. Establish incident timeline and scope
2. Identify affected systems and components
3. Analyze performance metrics and anomalies
4. Correlate logs and error patterns
5. Assess alert patterns and escalations
6. Form and test hypotheses about root cause

Investigation Questions to Address:
- When did the incident start and what triggered it?
- Which systems/services are affected and to what extent?
- What performance degradation or errors are observed?
- Are there patterns in logs that indicate specific failures?
- What alerts fired and in what sequence?
- What are the most likely root causes based on evidence?

Respond with JSON containing:
- timeline_analysis: detailed timeline with key events
- affected_systems: list of impacted systems/services
- performance_analysis: metrics analysis and anomaly detection
- log_analysis: key findings from log data
- alert_correlation: alert patterns and relationships
- hypothesis_formation: potential root causes with evidence
- investigation_confidence: confidence in findings
- additional_investigation_needed: areas requiring more data
"""

def get_incident_prompt(prompt_type: str, **kwargs) -> str:
    """
    Get a formatted incident workflow prompt.
    
    Args:
        prompt_type: Type of prompt needed
        **kwargs: Variables to format into the prompt
        
    Returns:
        Formatted prompt string
    """
    prompts = {
        "data_collection": INCIDENT_DATA_COLLECTION_PROMPT,
        "data_evaluation": INCIDENT_DATA_EVALUATION_PROMPT,
        "output_formatting": INCIDENT_OUTPUT_FORMATTING_PROMPT,
        "investigation": INCIDENT_INVESTIGATION_PROMPT
    }
    
    prompt_template = prompts.get(prompt_type)
    if not prompt_template:
        raise ValueError(f"Unknown prompt type: {prompt_type}")
    
    return prompt_template.format(**kwargs)

def get_incident_examples() -> str:
    """
    Get examples of INCIDENT workflow interactions.
    
    Returns:
        String containing incident examples
    """
    return """
INCIDENT Workflow Examples:

Example 1 - Performance Incident:
Incident: "High response times on web application"
Investigation: Collected metrics show 95th percentile response time increased from 200ms to 2.5s starting at 2024/06/28 13:45:00. Database connection pool exhaustion detected in logs. Alert correlation shows database alerts firing 2 minutes before web application alerts.

Example 2 - Service Outage:
Incident: "Users unable to access login service"
Investigation: Service completely unavailable since 2024/06/28 14:15:30. Prometheus shows 0/3 healthy instances. Loki logs indicate OOM kills on all instances. Memory usage spiked to 100% at incident start time.

Example 3 - Data Pipeline Failure:
Incident: "ETL job failing with timeout errors"
Investigation: Job execution time increased from 30 minutes to 4+ hours starting 2024/06/28 12:00:00. Source database shows lock contention. Network metrics indicate packet loss between pipeline and database servers.
"""
