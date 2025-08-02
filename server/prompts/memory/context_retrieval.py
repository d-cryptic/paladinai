"""
Memory Context Retrieval Prompts for PaladinAI Agent Orchestration System

This module contains prompts for retrieving and utilizing memory context
in workflow execution and decision making.
"""

MEMORY_CONTEXT_RETRIEVAL_PROMPT = """
You are an expert memory retrieval system for PaladinAI. Your role is to analyze
the current query and retrieve relevant context from previous interactions.

Given the current query and available memory context, identify:
1. Similar past queries and their outcomes
2. Relevant patterns from previous incidents
3. Historical context that might inform the current situation
4. Previous successful resolution strategies
5. Known failure patterns to avoid

Current Query: {current_query}
Query Type: {query_type}
Session Context: {session_context}

Available Memory Context:
{memory_context}

Provide a structured analysis of how the memory context relates to the current query:
- Relevance score (0.0 to 1.0) for each memory item
- Key insights from similar past situations
- Recommended approaches based on historical success
- Potential pitfalls based on past failures
- Context that should influence current decision making

Format your response as JSON with the following structure:
{
    "relevant_memories": [
        {
            "memory_id": "string",
            "relevance_score": float,
            "key_insights": "string",
            "recommended_actions": ["string"]
        }
    ],
    "historical_patterns": {
        "successful_strategies": ["string"],
        "common_failures": ["string"],
        "key_correlations": ["string"]
    },
    "context_summary": "string",
    "confidence": float
}
"""

MEMORY_SIMILARITY_SEARCH_PROMPT = """
You are analyzing memory similarity for the PaladinAI system. Given a current
situation and a set of historical memories, determine the similarity and relevance.

Current Situation:
Query: {current_query}
Context: {current_context}
Workflow Type: {workflow_type}

Historical Memory:
Content: {memory_content}
Type: {memory_type}
Metadata: {memory_metadata}

Analyze the similarity between the current situation and the historical memory:

1. Semantic Similarity: How similar are the concepts and intent?
2. Contextual Relevance: How relevant is the historical context to current needs?
3. Temporal Relevance: How applicable is the historical solution to current conditions?
4. Pattern Matching: Are there recurring patterns that apply?
5. Outcome Relevance: How useful would the historical outcome be for current situation?

Provide a similarity analysis with:
- Overall similarity score (0.0 to 1.0)
- Breakdown by similarity type
- Specific reasons for similarity/dissimilarity
- Actionable insights from the historical memory
- Confidence in the similarity assessment

Respond in JSON format:
{
    "overall_similarity": float,
    "similarity_breakdown": {
        "semantic": float,
        "contextual": float,
        "temporal": float,
        "pattern": float,
        "outcome": float
    },
    "reasoning": "string",
    "actionable_insights": ["string"],
    "confidence": float
}
"""

MEMORY_CONTEXT_INTEGRATION_PROMPT = """
You are integrating memory context into current workflow execution for PaladinAI.

Current Workflow State:
Query: {current_query}
Workflow Type: {workflow_type}
Current Step: {current_step}
Evidence Collected: {evidence_count} items
Confidence Score: {confidence_score}

Relevant Memory Context:
{memory_context}

Your task is to integrate the memory context with the current workflow state to:

1. Enhance current understanding with historical insights
2. Identify potential next steps based on similar past workflows
3. Adjust confidence levels based on historical patterns
4. Recommend memory-guided actions
5. Flag potential issues based on past failures

Provide memory-enhanced recommendations:
- How memory context should influence current analysis
- Suggested next steps based on historical success patterns
- Confidence adjustments based on memory patterns
- Warnings about potential issues from past failures
- Additional evidence to collect based on memory insights

Format as JSON:
{
    "memory_enhanced_insights": ["string"],
    "recommended_next_steps": ["string"],
    "confidence_adjustment": {
        "current": float,
        "memory_adjusted": float,
        "reasoning": "string"
    },
    "warnings": ["string"],
    "additional_evidence_needed": ["string"],
    "memory_guided_actions": ["string"]
}
"""

def get_memory_context_prompt(
    current_query: str,
    query_type: str,
    session_context: str,
    memory_context: str
) -> str:
    """
    Get memory context retrieval prompt with current situation details.
    
    Args:
        current_query: The current user query
        query_type: Type of query (query/incident/action)
        session_context: Current session context
        memory_context: Available memory context
    
    Returns:
        Formatted memory context retrieval prompt
    """
    return MEMORY_CONTEXT_RETRIEVAL_PROMPT.format(
        current_query=current_query,
        query_type=query_type,
        session_context=session_context,
        memory_context=memory_context
    )

def get_memory_similarity_prompt(
    current_query: str,
    current_context: str,
    workflow_type: str,
    memory_content: str,
    memory_type: str,
    memory_metadata: str
) -> str:
    """
    Get memory similarity search prompt with comparison details.
    
    Args:
        current_query: Current query being processed
        current_context: Current workflow context
        workflow_type: Type of current workflow
        memory_content: Content of historical memory
        memory_type: Type of historical memory
        memory_metadata: Metadata of historical memory
    
    Returns:
        Formatted memory similarity search prompt
    """
    return MEMORY_SIMILARITY_SEARCH_PROMPT.format(
        current_query=current_query,
        current_context=current_context,
        workflow_type=workflow_type,
        memory_content=memory_content,
        memory_type=memory_type,
        memory_metadata=memory_metadata
    )

def get_memory_integration_prompt(
    current_query: str,
    workflow_type: str,
    current_step: str,
    evidence_count: int,
    confidence_score: float,
    memory_context: str
) -> str:
    """
    Get memory context integration prompt for workflow enhancement.
    
    Args:
        current_query: Current query being processed
        workflow_type: Type of current workflow
        current_step: Current workflow step
        evidence_count: Number of evidence items collected
        confidence_score: Current confidence score
        memory_context: Relevant memory context
    
    Returns:
        Formatted memory integration prompt
    """
    return MEMORY_CONTEXT_INTEGRATION_PROMPT.format(
        current_query=current_query,
        workflow_type=workflow_type,
        current_step=current_step,
        evidence_count=evidence_count,
        confidence_score=confidence_score,
        memory_context=memory_context
    )
