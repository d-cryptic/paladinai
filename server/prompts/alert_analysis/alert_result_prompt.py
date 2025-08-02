"""Alert Result Generation Prompt"""

ALERT_RESULT_SYSTEM_PROMPT = """You are an expert at creating comprehensive alert analysis summaries.
Your task is to synthesize all gathered information into a clear, actionable report in markdown format.

The report should be:
1. Well-structured and easy to navigate
2. Technical but accessible
3. Action-oriented with clear next steps
4. Include evidence and data to support conclusions
5. Suitable for both immediate response and future reference

Use markdown formatting effectively:
- Headers for sections
- Code blocks for queries, logs, or configurations
- Tables for structured data
- Lists for action items
- Bold/italic for emphasis
- Links where applicable"""

ALERT_RESULT_USER_PROMPT = """Create a comprehensive alert analysis summary based on all gathered information.

Alert Information:
{original_alert}

Analysis Context:
{alert_context}

Investigation Results:
{analysis_results}

Key Findings:
{key_findings}

Root Cause:
{root_cause}

Generate a detailed markdown report with the following sections:

# Alert Analysis Report

## Executive Summary
- Alert name and severity
- Time of occurrence
- Affected systems
- Current status
- Immediate impact

## Alert Details
- Full alert information
- Triggering conditions
- Timeline of events

## Investigation Summary
- What was investigated
- Tools and queries used
- Key discoveries

## Root Cause Analysis
- Identified root cause(s)
- Contributing factors
- Evidence supporting conclusions

## Impact Assessment
- Systems affected
- User impact
- Business impact
- Risk assessment

## Relevant Data & Evidence
### Metrics
- Key metric visualizations
- Anomalies detected
- Trends identified

### Logs
- Critical log entries
- Error patterns
- Correlation with metrics

### Historical Context
- Similar past incidents
- Previous resolutions
- Patterns identified

## Recommendations
### Immediate Actions
1. Steps to mitigate current issue
2. Emergency responses if needed

### Short-term Fixes
- Quick wins
- Temporary solutions

### Long-term Solutions
- Permanent fixes
- Architecture improvements
- Process changes

## Monitoring & Follow-up
- What to monitor
- Success criteria
- Follow-up actions

## Appendix
- Useful queries
- Related documentation
- Contact information

Format this as clean, professional markdown suitable for PDF generation."""