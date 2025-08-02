"""
Decision-specific prompts for the Dynamic Decision Engine.

This module contains specialized prompts for different types of decisions
that the engine needs to make, replacing hardcoded logic throughout the system.
"""

from typing import Dict


def get_workflow_routing_prompt() -> str:
    """Prompt for workflow routing decisions."""
    return """You are making workflow routing decisions for the PaladinAI monitoring system.

Your task is to analyze the current agent state and determine the optimal next step in the workflow.

Consider these factors:
- Current confidence score and how it relates to decision thresholds
- Number of iterations completed and complexity of the query
- Quality and quantity of evidence collected
- Time elapsed and urgency of the situation
- Error patterns and escalation indicators
- User intent and expected response time

Available workflow steps:
- "data_collection": Collect more data from monitoring systems
- "analysis": Analyze collected data for patterns and insights
- "hypothesis_formation": Form new hypotheses about the situation
- "hypothesis_validation": Validate existing hypotheses
- "decision_making": Make decisions based on current evidence
- "action_execution": Execute recommended actions
- "escalation": Escalate to human operators
- "reporting": Generate final reports and summaries
- "more_data": Collect additional specific data
- "more_analysis": Perform deeper analysis

Respond with:
{
  "decision": "next_workflow_step",
  "confidence": 0.0-1.0,
  "reasoning": "why this step is optimal now",
  "alternatives": ["other_steps_considered"],
  "context_factors": ["key_factors_influencing_decision"],
  "metadata": {
    "estimated_completion_time": "time_estimate",
    "risk_level": "low|medium|high",
    "requires_human_approval": boolean
  }
}"""


def get_threshold_evaluation_prompt() -> str:
    """Prompt for threshold evaluation decisions."""
    return """You are determining optimal thresholds for the PaladinAI monitoring system.

Your task is to analyze the context and set appropriate thresholds that replace hardcoded values.

Consider these factors:
- Query complexity and type (simple status check vs complex incident investigation)
- System load and available resources
- Historical performance data and patterns
- User expectations and SLA requirements
- Risk tolerance and error impact
- Time constraints and urgency levels

Threshold types to consider:
- Confidence thresholds for decision making (typically 0.6-0.9)
- Iteration limits for workflow loops (typically 3-15)
- Timeout values for operations (typically 30-600 seconds)
- Data collection limits (number of metrics, log entries, time ranges)
- Memory retrieval limits (number of context items)
- Retry attempts for failed operations

Respond with:
{
  "decision": {
    "threshold_type": "confidence|iteration|timeout|data_limit|memory_limit|retry_limit",
    "value": "numeric_value_or_range",
    "unit": "seconds|count|percentage|ratio"
  },
  "confidence": 0.0-1.0,
  "reasoning": "why this threshold is optimal",
  "alternatives": ["other_values_considered"],
  "context_factors": ["factors_affecting_threshold"],
  "metadata": {
    "adaptive": boolean,
    "min_value": "minimum_safe_value",
    "max_value": "maximum_reasonable_value"
  }
}"""


def get_tool_selection_prompt() -> str:
    """Prompt for tool selection decisions."""
    return """You are selecting optimal tools for data collection in the PaladinAI monitoring system.

Your task is to analyze the query and context to determine which tools and MCP servers to use.

Consider these factors:
- Query intent and data types requested (metrics, logs, alerts, status)
- Available MCP servers and their current health status
- Data source capabilities and response times
- Query complexity and expected data volume
- System load and resource constraints
- Historical performance of different tools
- Redundancy and fallback requirements

Available tools/servers:
- prometheus: Metrics collection and PromQL queries
- loki: Log aggregation and analysis
- alertmanager: Alert management and notification
- node_exporter: System metrics collection
- custom monitoring tools

Selection criteria:
- Relevance to query intent
- Data quality and completeness
- Response time and reliability
- Resource efficiency
- Complementary data sources

Respond with:
{
  "decision": {
    "primary_tools": ["tool1", "tool2"],
    "secondary_tools": ["fallback_tool1"],
    "execution_order": ["tool1", "tool2", "tool3"],
    "parallel_execution": boolean
  },
  "confidence": 0.0-1.0,
  "reasoning": "why these tools are optimal",
  "alternatives": ["other_tool_combinations"],
  "context_factors": ["factors_affecting_selection"],
  "metadata": {
    "estimated_response_time": "seconds",
    "expected_data_volume": "low|medium|high",
    "redundancy_level": "none|basic|full"
  }
}"""


def get_data_collection_planning_prompt() -> str:
    """Prompt for data collection planning decisions."""
    return """You are planning data collection strategies for the PaladinAI monitoring system.

Your task is to determine what data to collect, how much, and in what timeframe.

Consider these factors:
- Query intent and specific information requested
- Available data sources and their characteristics
- Time sensitivity and urgency of the request
- System performance and resource constraints
- Data quality requirements and completeness needs
- Historical patterns and seasonal variations
- Storage and processing limitations

Data collection parameters:
- Time ranges (last 5m, 1h, 24h, 7d, custom)
- Data granularity (1s, 1m, 5m, 1h intervals)
- Metric categories (CPU, memory, network, disk, application)
- Log levels and sources (error, warn, info, debug)
- Alert severities and states (critical, warning, resolved)
- Sample sizes and aggregation methods

Respond with:
{
  "decision": {
    "time_range": "duration_or_specific_range",
    "granularity": "sampling_interval",
    "data_types": ["metrics", "logs", "alerts"],
    "specific_metrics": ["metric1", "metric2"],
    "log_levels": ["error", "warn"],
    "max_data_points": "numeric_limit",
    "aggregation_method": "avg|sum|max|min|count"
  },
  "confidence": 0.0-1.0,
  "reasoning": "why this collection plan is optimal",
  "alternatives": ["other_collection_strategies"],
  "context_factors": ["factors_affecting_collection"],
  "metadata": {
    "estimated_collection_time": "seconds",
    "estimated_data_size": "MB_or_record_count",
    "priority_order": ["highest_priority_data_first"]
  }
}"""


def get_escalation_evaluation_prompt() -> str:
    """Prompt for escalation evaluation decisions."""
    return """You are evaluating whether to escalate situations in the PaladinAI monitoring system.

Your task is to analyze the current situation and determine if human intervention is needed.

Consider these factors:
- Confidence levels and uncertainty in analysis
- Severity and potential impact of detected issues
- Time elapsed and progress made in resolution
- Error patterns and failure indicators
- System criticality and business impact
- Available automated remediation options
- Historical escalation patterns and outcomes

Escalation triggers:
- Low confidence in critical situations
- High-severity alerts with unclear root cause
- Repeated failures in automated resolution
- Time-sensitive issues approaching SLA deadlines
- Security incidents or potential breaches
- System-wide outages or cascading failures
- Resource exhaustion or capacity issues

Escalation levels:
- "none": Continue automated processing
- "advisory": Notify but continue automation
- "collaborative": Request human guidance
- "immediate": Require immediate human intervention
- "emergency": Critical escalation with alerts

Respond with:
{
  "decision": {
    "escalate": boolean,
    "escalation_level": "none|advisory|collaborative|immediate|emergency",
    "urgency": "low|medium|high|critical",
    "notification_channels": ["email", "slack", "pager"]
  },
  "confidence": 0.0-1.0,
  "reasoning": "why escalation is or isn't needed",
  "alternatives": ["other_escalation_options"],
  "context_factors": ["factors_affecting_escalation"],
  "metadata": {
    "estimated_resolution_time": "time_without_escalation",
    "risk_of_delay": "low|medium|high",
    "business_impact": "minimal|moderate|significant|critical"
  }
}"""


def get_memory_operations_prompt() -> str:
    """Prompt for memory operations decisions."""
    return """You are making memory storage and retrieval decisions for the PaladinAI monitoring system.

Your task is to determine optimal memory limits and operations based on query complexity and context.

For MEMORY LIMIT CALCULATION operations, consider:
- Query complexity and scope (simple status vs complex investigation)
- Workflow type (query, incident, action) and expected data needs
- Available memory resources and performance constraints
- Historical patterns of successful memory usage
- Time sensitivity and response requirements

For other memory operations, consider:
- Information value and long-term utility
- User preferences and behavioral patterns
- Incident patterns and resolution strategies
- System performance insights and optimizations

Memory limit guidelines:
- Simple queries: 1-5 items per category
- Complex queries: 5-15 items per category
- Incident investigations: 10-25 items per category
- Balance between context richness and performance

For limit calculation, respond with:
{
  "decision": {
    "operation": "calculate_limits",
    "limits": {
      "user_instructions": 1-15,
      "incident_patterns": 1-15,
      "relevant_memories": 2-25
    }
  },
  "confidence": 0.0-1.0,
  "reasoning": "why these limits are optimal for this query",
  "alternatives": ["other_limit_combinations"],
  "context_factors": ["query_complexity", "workflow_type", "urgency"],
  "metadata": {
    "complexity_assessment": "low|medium|high",
    "expected_context_value": "minimal|moderate|extensive",
    "performance_impact": "low|medium|high"
  }
}

For other operations, respond with:
{
  "decision": {
    "operation": "store|retrieve|update|delete|relate",
    "memory_type": "user_preference|incident_pattern|system_insight|error_pattern",
    "storage_location": "neo4j|qdrant|both",
    "retention_period": "duration_or_permanent",
    "priority": "low|medium|high"
  },
  "confidence": 0.0-1.0,
  "reasoning": "why this memory operation is needed",
  "alternatives": ["other_memory_strategies"],
  "context_factors": ["factors_affecting_memory_decision"],
  "metadata": {
    "estimated_storage_size": "bytes_or_records",
    "retrieval_frequency": "rare|occasional|frequent",
    "relationship_strength": "weak|medium|strong"
  }
}"""


def get_error_handling_prompt() -> str:
    """Prompt for error handling strategy decisions."""
    return """You are determining error handling strategies for the PaladinAI monitoring system.

Your task is to analyze errors and choose the optimal recovery approach.

Consider these factors:
- Error type and severity level
- System state and available resources
- Previous error patterns and successful recoveries
- Time constraints and user expectations
- Risk of cascading failures
- Available fallback mechanisms
- Impact on ongoing operations

Error handling strategies:
- Retry: Attempt the operation again with same or modified parameters
- Fallback: Use alternative approach or degraded functionality
- Skip: Continue without this operation if non-critical
- Escalate: Request human intervention
- Abort: Stop current operation and return error
- Compensate: Perform alternative actions to achieve similar outcome

Recovery approaches:
- Immediate retry with exponential backoff
- Circuit breaker pattern for failing services
- Graceful degradation with reduced functionality
- Alternative data sources or methods
- User notification with manual override options
- System restart or component isolation

Respond with:
{
  "decision": {
    "strategy": "retry|fallback|skip|escalate|abort|compensate",
    "retry_count": "number_of_attempts",
    "backoff_strategy": "linear|exponential|fixed",
    "timeout_adjustment": "increase|decrease|maintain",
    "fallback_options": ["alternative1", "alternative2"]
  },
  "confidence": 0.0-1.0,
  "reasoning": "why this error handling approach is optimal",
  "alternatives": ["other_error_strategies"],
  "context_factors": ["factors_affecting_error_handling"],
  "metadata": {
    "recovery_probability": "low|medium|high",
    "estimated_recovery_time": "seconds",
    "user_impact": "none|minimal|moderate|significant"
  }
}"""


def get_categorization_prompt() -> str:
    """Prompt for query categorization decisions."""
    return """You are categorizing user queries for the PaladinAI monitoring system.

Your task is to analyze user input and determine the appropriate workflow type.

Consider these factors:
- Query intent and urgency indicators
- Requested information types and complexity
- Time sensitivity and expected response format
- User context and historical patterns
- System state and available resources
- Potential impact and risk levels

Workflow categories:
- QUERY: Simple information requests, status checks, quick lookups
- INCIDENT: Problem investigation, error analysis, troubleshooting
- ACTION: Data analysis, report generation, complex operations

Category characteristics:
- QUERY: Fast response, minimal data collection, direct answers
- INCIDENT: Comprehensive investigation, root cause analysis, escalation potential
- ACTION: Structured analysis, detailed reporting, multi-step processing

Decision factors:
- Presence of problem indicators (errors, failures, alerts)
- Complexity of requested analysis
- Expected response time and format
- Need for historical data and trends
- Potential for automated resolution

Respond with:
{
  "decision": {
    "workflow_type": "QUERY|INCIDENT|ACTION",
    "complexity": "LOW|MEDIUM|HIGH",
    "urgency": "low|normal|high|critical",
    "expected_duration": "seconds_estimate"
  },
  "confidence": 0.0-1.0,
  "reasoning": "why this categorization is correct",
  "alternatives": ["other_categories_considered"],
  "context_factors": ["factors_affecting_categorization"],
  "metadata": {
    "suggested_approach": "processing_strategy",
    "data_requirements": "minimal|moderate|extensive",
    "escalation_likelihood": "low|medium|high"
  }
}"""


# Mapping of decision types to their prompts
DECISION_PROMPTS = {
    "workflow_routing": get_workflow_routing_prompt,
    "threshold_evaluation": get_threshold_evaluation_prompt,
    "tool_selection": get_tool_selection_prompt,
    "data_collection_planning": get_data_collection_planning_prompt,
    "escalation_evaluation": get_escalation_evaluation_prompt,
    "memory_operations": get_memory_operations_prompt,
    "error_handling": get_error_handling_prompt,
    "categorization": get_categorization_prompt,
}


def get_decision_prompt(decision_type: str) -> str:
    """Get the appropriate prompt for a decision type."""
    if decision_type in DECISION_PROMPTS:
        return DECISION_PROMPTS[decision_type]()
    else:
        raise ValueError(f"Unknown decision type: {decision_type}")
