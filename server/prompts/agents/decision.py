"""
Decision Making Agent Prompts for PaladinAI Agent Orchestration.

This module contains prompts for the decision-making agent that determines
appropriate actions based on analysis results.
"""

DECISION_MAKING_PROMPT = """
You are an expert SRE making decisions about system incidents and monitoring.

Apply the following decision framework:
- >85% Confidence: Proceed with invasive fixes (restarts, rollbacks)
- 70-85% Confidence: Implement non-invasive mitigations first
- 50-70% Confidence: Gather more evidence, prepare contingencies
- <50% Confidence: Focus on data collection, avoid disruptive actions

Current confidence threshold for action: {min_confidence_for_action}
High confidence threshold: {high_confidence_threshold}

Respond with JSON containing:
- recommendations: list of action recommendations
- escalate: boolean
- escalation_reason: string (if escalating)
- reasoning: explanation of decisions
"""

DECISION_CONTEXT_PROMPT = """
Make decisions based on the current investigation state:

Current State:
- Confidence Score: {confidence_score:.2f}
- Confidence Level: {confidence_level}
- Evidence Count: {evidence_count}
- Hypotheses Count: {hypotheses_count}
- Workflow Type: {workflow_type}
- Initial Query: {initial_query}

Key Findings:
{findings}

Memory Context (if available):
{memory_context}

Decision Framework:
- Confidence threshold for action: {min_confidence_for_action}
- High confidence threshold: {high_confidence_threshold}

Please make decisions and provide recommendations based on this information.

Consider:
1. Whether the confidence level justifies taking action
2. What type of actions are appropriate given the evidence
3. Whether escalation is needed based on severity or uncertainty
4. Risk assessment for any recommended actions
5. Rollback plans for invasive actions
6. Memory-guided insights from similar past situations (if available)
7. Historical success/failure patterns for similar decisions
8. Memory-based risk assessment and mitigation strategies
"""

def get_decision_prompt(min_confidence_for_action: float, high_confidence_threshold: float) -> str:
    """
    Get decision making prompt with confidence thresholds.

    Args:
        min_confidence_for_action: Minimum confidence required for action
        high_confidence_threshold: Threshold for high confidence actions

    Returns:
        Formatted decision making prompt
    """
    return DECISION_MAKING_PROMPT.format(
        min_confidence_for_action=min_confidence_for_action,
        high_confidence_threshold=high_confidence_threshold
    )

def get_decision_context_prompt(state_info: dict, memory_context: str = "No memory context available") -> str:
    """
    Get decision making prompt with current state context and memory enhancement.

    Args:
        state_info: Dictionary containing current state information
        memory_context: Memory context from similar past situations

    Returns:
        Formatted decision context prompt with memory enhancement
    """
    return DECISION_CONTEXT_PROMPT.format(
        confidence_score=state_info.get('confidence_score', 0.0),
        confidence_level=state_info.get('confidence_level', 'UNKNOWN'),
        evidence_count=state_info.get('evidence_count', 0),
        hypotheses_count=state_info.get('hypotheses_count', 0),
        workflow_type=state_info.get('workflow_type', 'UNKNOWN'),
        initial_query=state_info.get('initial_query', 'Unknown'),
        findings=state_info.get('findings', 'No findings available'),
        min_confidence_for_action=state_info.get('min_confidence_for_action', 0.7),
        high_confidence_threshold=state_info.get('high_confidence_threshold', 0.85),
        memory_context=memory_context
    )

