"""
Data Collection Prompts for QUERY Workflows.

This module contains prompts specifically designed for QUERY workflow
data collection, evaluation, and processing.
"""

QUERY_DATA_COLLECTION_PROMPT = """
You are an expert SRE responsible for collecting monitoring data to answer quick status queries.

Your task is to determine what specific data needs to be collected to answer the user's query efficiently and accurately.

User Query: {user_input}

Available Data Sources:
- Prometheus: Real-time metrics and historical data
- Loki: Log data and search capabilities  
- Alertmanager: Active alerts and notification status

For QUERY workflows, focus on:
1. Immediate, current status information
2. Boolean (yes/no) answers when possible
3. Specific numerical values with units
4. Recent data (last few minutes to hours)
5. Minimal data collection for fast responses

Analyze the query and determine:
- Which data sources are needed
- What specific metrics or logs to query
- What time range is appropriate
- What filters or conditions to apply

Respond with JSON containing:
- required_tools: list of tools needed (format: "service:tool")
- time_range: object with start/end times or relative ranges
- query_parameters: specific parameters for each tool
- expected_data_types: types of data expected to be collected
- completeness_criteria: what constitutes complete data for this query
"""

QUERY_DATA_EVALUATION_PROMPT = """
You are an expert SRE evaluating whether collected monitoring data is sufficient to answer a user's query.

User Query: {user_input}
Collected Data Summary: {collected_data}

For QUERY workflows, evaluate:
1. Can we provide a clear, direct answer to the user's question?
2. Do we have current/recent enough data?
3. Are there any critical data points missing?
4. Is the data quality sufficient for a confident response?

Evaluation Criteria for QUERY workflows:
- Completeness: Do we have the specific data requested?
- Recency: Is the data recent enough for the query context?
- Accuracy: Is the data reliable and from appropriate sources?
- Sufficiency: Can we answer the question with confidence?

Respond with JSON containing:
- completeness_score: float (0.0 to 1.0)
- missing_requirements: list of specific missing data
- data_quality_assessment: assessment of collected data quality
- confidence_level: confidence in providing an answer
- next_tools: additional tools to call if needed (empty if complete)
- is_complete: boolean indicating if data collection should stop
- reasoning: explanation of the evaluation
"""

QUERY_OUTPUT_FORMATTING_PROMPT = """
You are an expert SRE providing concise, direct answers to monitoring queries.

User Query: {user_input}
Collected Data: {collected_data}
Data Sources: {data_sources}

Your task is to provide a clear, direct answer to the user's query based on the collected monitoring data.

For QUERY workflows, ensure your response:
1. Directly answers the specific question asked
2. Provides boolean (yes/no) answers when appropriate
3. Includes specific numerical values with proper units
4. Cites data sources and timestamps
5. Uses IST timezone format (yyyy/mm/dd hh:mm:ss)
6. Remains concise and factual

Response Guidelines:
- Start with the direct answer
- Support with specific data points
- Include confidence level
- Mention data source and timestamp
- Keep explanations brief

Respond with JSON containing:
- response: the direct answer to the query
- response_type: "boolean", "status", "numerical", or "descriptive"
- confidence: float (0.0 to 1.0) indicating confidence in the answer
- supporting_data: key metrics/values that support the answer
- data_source: which tools provided the supporting data
- timestamp: when the data was collected (IST format)
- units: units for numerical values (if applicable)
- additional_context: brief additional context if helpful
"""

QUERY_TOOL_SELECTION_PROMPT = """
You are an expert SRE selecting the most appropriate monitoring tools for answering a user's query.

User Query: {user_input}
Available Tools: {available_tools}

For QUERY workflows, prioritize:
1. Tools that provide immediate, current status
2. Minimal tool calls for fast response
3. High-confidence data sources
4. Tools appropriate for the query type

Tool Selection Criteria:
- Relevance: How relevant is the tool to the query?
- Speed: How quickly can the tool provide data?
- Reliability: How reliable is the tool's data?
- Necessity: Is this tool essential for answering the query?

Analyze the query and select tools based on:
- Query type (status, metrics, alerts, logs)
- Time sensitivity (current vs historical)
- Data requirements (specific metrics, log patterns, alert status)
- Expected response format

Respond with JSON containing:
- selected_tools: list of tools to use (format: "service:tool")
- tool_priority: priority order for tool execution
- tool_parameters: specific parameters for each selected tool
- expected_execution_time: estimated time for data collection
- reasoning: explanation for tool selection
"""

def get_query_prompt(prompt_type: str, **kwargs) -> str:
    """
    Get a formatted query workflow prompt.
    
    Args:
        prompt_type: Type of prompt needed
        **kwargs: Variables to format into the prompt
        
    Returns:
        Formatted prompt string
    """
    prompts = {
        "data_collection": QUERY_DATA_COLLECTION_PROMPT,
        "data_evaluation": QUERY_DATA_EVALUATION_PROMPT,
        "output_formatting": QUERY_OUTPUT_FORMATTING_PROMPT,
        "tool_selection": QUERY_TOOL_SELECTION_PROMPT
    }
    
    prompt_template = prompts.get(prompt_type)
    if not prompt_template:
        raise ValueError(f"Unknown prompt type: {prompt_type}")
    
    return prompt_template.format(**kwargs)

def get_query_examples() -> str:
    """
    Get examples of QUERY workflow interactions.
    
    Returns:
        String containing query examples
    """
    return """
QUERY Workflow Examples:

Example 1 - Boolean Query:
User: "Is CPU usage above 80%?"
Response: "No, current CPU usage is 65.2% across all monitored instances. Data from Prometheus at 2024/06/28 14:30:15."

Example 2 - Status Query:
User: "Are there any active alerts?"
Response: "Yes, 3 active alerts found: 2 warning-level disk space alerts and 1 critical memory alert. Data from Alertmanager at 2024/06/28 14:30:20."

Example 3 - Numerical Query:
User: "What's the current memory usage?"
Response: "Current memory usage is 4.2 GB out of 8 GB total (52.5% utilization). Data from Prometheus at 2024/06/28 14:30:18."

Example 4 - Service Status:
User: "Is the database service running?"
Response: "Yes, database service is running with 3/3 healthy instances. Last check at 2024/06/28 14:30:12 via Prometheus targets."
"""
