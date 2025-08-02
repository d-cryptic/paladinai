"""
Data Completeness Evaluation Prompts for Recursive Data Refinement

This module provides OpenAI prompts for evaluating data completeness and determining
what additional data is needed to fulfill elaborated memory instructions.
"""


def get_data_completeness_evaluation_prompt(
    query: str,
    unified_guidance: str,
    current_data_summary: str,
    iteration: int,
    workflow_type: str = "query"
) -> str:
    """
    Create a comprehensive prompt for evaluating data completeness.
    
    Args:
        query: Original user query
        unified_guidance: Elaborated instructions from memory
        current_data_summary: Summary of currently collected data
        iteration: Current iteration number
        workflow_type: Type of workflow (query/incident/action)
        
    Returns:
        Formatted prompt for OpenAI evaluation
    """
    return f"""
You are an expert data completeness evaluator for monitoring and observability systems. Your task is to determine if the current data is sufficient to fulfill the user's query according to the elaborated instructions.

**CONTEXT:**
- User Query: {query}
- Workflow Type: {workflow_type}
- Current Iteration: {iteration}

**ELABORATED INSTRUCTIONS:**
{unified_guidance}

**CURRENT DATA STATUS:**
{current_data_summary}

**EVALUATION CRITERIA:**
1. Does the current data address all aspects mentioned in the elaborated instructions?
2. Are there specific metrics, logs, or alerts mentioned in the instructions that are missing?
3. Is the data granular enough to provide actionable insights?
4. Are there temporal or contextual gaps in the data?
5. Can the current data support confident decision-making?

**RESPONSE FORMAT:**
Provide your evaluation in the following JSON format:

{{
    "is_sufficient": boolean,
    "confidence_score": float (0.0-1.0),
    "missing_data_types": ["type1", "type2", ...],
    "recommended_actions": [
        {{
            "action_type": "fetch_metrics|fetch_logs|fetch_alerts",
            "specific_query": "specific PromQL/LogQL query or filter criteria",
            "priority": "high|medium|low",
            "reasoning": "why this specific data is needed",
            "expected_outcome": "what this data will help determine"
        }}
    ],
    "reasoning": "detailed explanation of your assessment",
    "data_gaps": [
        {{
            "gap_type": "temporal|contextual|metric_specific|log_specific|alert_specific",
            "description": "description of the gap",
            "impact": "how this gap affects the analysis"
        }}
    ],
    "completeness_percentage": float (0.0-100.0)
}}

**GUIDELINES:**
- Be conservative: if in doubt, request additional data
- Prioritize data that directly addresses the elaborated instructions
- Consider both current state and historical context when relevant
- Focus on actionable data collection recommendations
- Provide specific queries when possible (PromQL for metrics, LogQL for logs)
- Consider the iteration number - avoid infinite loops by being more lenient in later iterations
"""


def get_data_sufficiency_threshold_prompt(
    workflow_type: str,
    confidence_threshold: float,
    iteration: int,
    max_iterations: int
) -> str:
    """
    Create a prompt for determining data sufficiency thresholds.
    
    Args:
        workflow_type: Type of workflow
        confidence_threshold: Minimum confidence threshold
        iteration: Current iteration
        max_iterations: Maximum allowed iterations
        
    Returns:
        Prompt for threshold evaluation
    """
    return f"""
You are evaluating data sufficiency for a {workflow_type} workflow.

**CONTEXT:**
- Current Iteration: {iteration}/{max_iterations}
- Minimum Confidence Threshold: {confidence_threshold}
- Workflow Type: {workflow_type}

**THRESHOLD GUIDELINES:**

For QUERY workflows:
- High priority on speed and relevance
- Confidence threshold: 0.6-0.8
- Focus on direct answers to user questions

For INCIDENT workflows:
- High priority on comprehensive data
- Confidence threshold: 0.7-0.9
- Need sufficient data for root cause analysis

For ACTION workflows:
- High priority on actionable data
- Confidence threshold: 0.8-0.9
- Need data to support safe action execution

**ITERATION CONSIDERATIONS:**
- Iteration 1-2: Be thorough, collect comprehensive data
- Iteration 3-4: Balance thoroughness with efficiency
- Iteration 5+: Be more lenient to avoid infinite loops

Adjust your confidence scoring based on these guidelines.
"""


def get_data_collection_prioritization_prompt(
    missing_data_types: list,
    query_context: str,
    available_tools: list
) -> str:
    """
    Create a prompt for prioritizing data collection actions.
    
    Args:
        missing_data_types: List of missing data types
        query_context: Context of the original query
        available_tools: List of available data collection tools
        
    Returns:
        Prompt for action prioritization
    """
    return f"""
You are prioritizing data collection actions for monitoring system analysis.

**MISSING DATA TYPES:**
{', '.join(missing_data_types)}

**QUERY CONTEXT:**
{query_context}

**AVAILABLE TOOLS:**
{', '.join(available_tools)}

**PRIORITIZATION CRITERIA:**
1. **High Priority**: Data directly answering the user's question
2. **Medium Priority**: Contextual data that supports analysis
3. **Low Priority**: Nice-to-have data for comprehensive understanding

**RESPONSE FORMAT:**
Provide prioritized actions in JSON format:

{{
    "prioritized_actions": [
        {{
            "action_type": "fetch_metrics|fetch_logs|fetch_alerts",
            "priority": "high|medium|low",
            "specific_query": "detailed query specification",
            "reasoning": "why this action is prioritized at this level",
            "estimated_value": float (0.0-1.0),
            "dependencies": ["list of other actions this depends on"]
        }}
    ],
    "execution_order": ["action1", "action2", "action3"],
    "parallel_execution": [["action1", "action2"], ["action3"]],
    "reasoning": "overall prioritization strategy"
}}

Focus on actions that provide the highest value for answering the user's query.
"""


def get_data_quality_assessment_prompt(
    collected_data: dict,
    expected_data_characteristics: str
) -> str:
    """
    Create a prompt for assessing the quality of collected data.
    
    Args:
        collected_data: Dictionary of collected data
        expected_data_characteristics: Expected characteristics of good data
        
    Returns:
        Prompt for data quality assessment
    """
    return f"""
You are assessing the quality of collected monitoring data.

**COLLECTED DATA SUMMARY:**
{collected_data}

**EXPECTED DATA CHARACTERISTICS:**
{expected_data_characteristics}

**QUALITY ASSESSMENT CRITERIA:**
1. **Completeness**: Are all expected data points present?
2. **Accuracy**: Does the data appear consistent and reasonable?
3. **Timeliness**: Is the data recent enough to be relevant?
4. **Granularity**: Is the data detailed enough for analysis?
5. **Consistency**: Are there any contradictions in the data?

**RESPONSE FORMAT:**
{{
    "overall_quality_score": float (0.0-1.0),
    "quality_dimensions": {{
        "completeness": float (0.0-1.0),
        "accuracy": float (0.0-1.0),
        "timeliness": float (0.0-1.0),
        "granularity": float (0.0-1.0),
        "consistency": float (0.0-1.0)
    }},
    "quality_issues": [
        {{
            "issue_type": "missing_data|inconsistent_data|stale_data|insufficient_detail",
            "description": "description of the issue",
            "severity": "high|medium|low",
            "recommendation": "how to address this issue"
        }}
    ],
    "data_reliability": "high|medium|low",
    "recommendations": [
        "specific recommendations for improving data quality"
    ]
}}

Provide actionable feedback on data quality and suggestions for improvement.
"""


def get_iteration_termination_prompt(
    iteration_history: list,
    max_iterations: int,
    current_confidence: float,
    threshold: float
) -> str:
    """
    Create a prompt for determining when to terminate iterations.
    
    Args:
        iteration_history: History of previous iterations
        max_iterations: Maximum allowed iterations
        current_confidence: Current confidence score
        threshold: Confidence threshold
        
    Returns:
        Prompt for termination decision
    """
    return f"""
You are determining whether to continue or terminate the data refinement process.

**ITERATION HISTORY:**
{iteration_history}

**CURRENT STATUS:**
- Current Confidence: {current_confidence}
- Confidence Threshold: {threshold}
- Max Iterations: {max_iterations}

**TERMINATION CRITERIA:**
1. Confidence threshold met
2. Diminishing returns in data collection
3. Maximum iterations reached
4. No more actionable data collection opportunities

**RESPONSE FORMAT:**
{{
    "should_terminate": boolean,
    "termination_reason": "threshold_met|diminishing_returns|max_iterations|no_more_actions",
    "confidence_trend": "improving|stable|declining",
    "final_recommendation": "continue|terminate_with_current_data|escalate",
    "reasoning": "detailed explanation of the decision"
}}

Consider the trend in confidence scores and the value of additional iterations.
"""
