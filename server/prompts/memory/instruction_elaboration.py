"""
Memory Instruction Elaboration Prompts for PaladinAI Agent Orchestration System

This module contains prompts for elaborating memory-stored instructions to provide
context-specific guidance for current queries.
"""

INSTRUCTION_ELABORATION_PROMPT = """
You are an expert instruction elaboration system for PaladinAI. Your role is to take
stored user instructions and elaborate them with specific, actionable guidance for
the current query context.

Current Query Context:
Query: {current_query}
Query Type: {query_type}
Workflow Type: {workflow_type}
Current Context: {current_context}

Retrieved Memory Instruction:
Original Instruction: {memory_instruction}
Instruction Type: {instruction_type}
Instruction Context: {instruction_context}

Your task is to elaborate the memory instruction with:

1. **Specific Application**: How does this instruction apply to the current query?
2. **Detailed Guidance**: What specific steps or considerations should be followed?
3. **Concrete Examples**: Provide examples relevant to the current context
4. **Expected Output Format**: What should the final output look like?
5. **Quality Criteria**: How to ensure the instruction is properly followed?

Elaboration Guidelines:
- Make the instruction actionable and specific to the current query
- Provide concrete examples that match the query context
- Include expected output format and structure
- Specify quality criteria for successful execution
- Consider edge cases and potential issues
- Maintain the original intent while adding specificity

Example Elaboration Structure:
"For the current query about [query topic], the instruction '[original instruction]' means:

**Specific Application**: [How it applies to current query]
**Detailed Steps**: 
1. [Step 1 with specifics]
2. [Step 2 with specifics]
3. [Step 3 with specifics]

**Expected Output Format**: [Specific format requirements]
**Quality Criteria**: [How to measure success]
**Examples**: [Concrete examples for current context]"

Provide your elaboration as a clear, structured response that can be directly integrated
into the final query processing prompt.
"""

INSTRUCTION_RELEVANCE_ASSESSMENT_PROMPT = """
You are assessing the relevance of stored instructions to current queries for PaladinAI.

Current Query: {current_query}
Query Type: {query_type}
Query Context: {query_context}

Stored Instruction: {stored_instruction}
Instruction Type: {instruction_type}
Instruction Metadata: {instruction_metadata}

Assess the relevance of this stored instruction to the current query:

1. **Direct Relevance**: Does the instruction directly apply to this query?
2. **Contextual Relevance**: Is the instruction relevant to the query context?
3. **Temporal Relevance**: Is the instruction still applicable given current conditions?
4. **Scope Relevance**: Does the query fall within the instruction's intended scope?
5. **Priority Relevance**: How important is this instruction for the current query?

Provide your assessment in JSON format. Return ONLY the JSON object, no other text:

{
    "relevance_score": 0.8,
    "direct_relevance": 0.9,
    "contextual_relevance": 0.8,
    "temporal_relevance": 0.7,
    "scope_relevance": 0.8,
    "priority_relevance": 0.9,
    "reasoning": "This instruction is highly relevant because...",
    "should_elaborate": true,
    "elaboration_priority": "high",
    "applicable_scenarios": ["log_analysis", "data_retrieval"],
    "potential_conflicts": []
}
"""

MULTI_INSTRUCTION_SYNTHESIS_PROMPT = """
You are synthesizing multiple relevant instructions for a single query in PaladinAI.

Current Query: {current_query}
Query Type: {query_type}
Query Context: {query_context}

Relevant Instructions:
{relevant_instructions}

Your task is to synthesize these instructions into a coherent, unified guidance:

1. **Identify Overlaps**: Where do instructions complement each other?
2. **Resolve Conflicts**: How to handle contradictory instructions?
3. **Prioritize Instructions**: Which instructions take precedence?
4. **Create Unified Guidance**: Combine into single, clear guidance
5. **Maintain Completeness**: Ensure all important aspects are covered

Synthesis Guidelines:
- Preserve the intent of all relevant instructions
- Resolve conflicts by considering context and priority
- Create a logical flow of guidance
- Avoid redundancy while maintaining completeness
- Provide clear, actionable unified guidance

Provide synthesized guidance in JSON format. Return ONLY the JSON object, no other text:

{
    "unified_guidance": "When processing this query, follow these combined instructions: [detailed guidance here]",
    "instruction_synthesis": [
        {
            "original_instruction": "original instruction text",
            "integration_approach": "how this instruction was integrated",
            "priority_level": "high"
        }
    ],
    "conflict_resolutions": [],
    "synthesis_confidence": 0.85,
    "completeness_score": 0.90
}
"""

ELABORATION_QUALITY_ASSESSMENT_PROMPT = """
You are assessing the quality of instruction elaboration for PaladinAI.

Original Instruction: {original_instruction}
Current Query: {current_query}
Elaborated Instruction: {elaborated_instruction}

Quality Assessment Criteria:
1. **Accuracy**: Does the elaboration accurately reflect the original instruction?
2. **Specificity**: Is the elaboration specific enough for the current query?
3. **Completeness**: Does the elaboration cover all necessary aspects?
4. **Clarity**: Is the elaborated instruction clear and unambiguous?
5. **Actionability**: Can the elaborated instruction be directly acted upon?
6. **Relevance**: Is the elaboration relevant to the current query context?

Assess the elaboration quality:
{
    "quality_score": float,  // 0.0 to 1.0
    "accuracy_score": float,
    "specificity_score": float,
    "completeness_score": float,
    "clarity_score": float,
    "actionability_score": float,
    "relevance_score": float,
    "quality_issues": ["string"],
    "improvement_suggestions": ["string"],
    "approval_recommendation": "approve|revise|reject",
    "confidence": float
}
"""

def get_instruction_elaboration_prompt(
    current_query: str,
    query_type: str,
    workflow_type: str,
    current_context: str,
    memory_instruction: str,
    instruction_type: str,
    instruction_context: str
) -> str:
    """
    Get instruction elaboration prompt with current context.
    
    Args:
        current_query: The current user query
        query_type: Type of query (query/incident/action)
        workflow_type: Current workflow type
        current_context: Current query context
        memory_instruction: Retrieved memory instruction
        instruction_type: Type of the instruction
        instruction_context: Context of the instruction
    
    Returns:
        Formatted instruction elaboration prompt
    """
    return INSTRUCTION_ELABORATION_PROMPT.format(
        current_query=current_query,
        query_type=query_type,
        workflow_type=workflow_type,
        current_context=current_context,
        memory_instruction=memory_instruction,
        instruction_type=instruction_type,
        instruction_context=instruction_context
    )

def get_instruction_relevance_prompt(
    current_query: str,
    query_type: str,
    query_context: str,
    stored_instruction: str,
    instruction_type: str,
    instruction_metadata: str
) -> str:
    """
    Get instruction relevance assessment prompt.
    
    Args:
        current_query: Current query being processed
        query_type: Type of current query
        query_context: Current query context
        stored_instruction: Stored instruction to assess
        instruction_type: Type of stored instruction
        instruction_metadata: Metadata of stored instruction
    
    Returns:
        Formatted instruction relevance prompt
    """
    return INSTRUCTION_RELEVANCE_ASSESSMENT_PROMPT.format(
        current_query=current_query,
        query_type=query_type,
        query_context=query_context,
        stored_instruction=stored_instruction,
        instruction_type=instruction_type,
        instruction_metadata=instruction_metadata
    )

def get_multi_instruction_synthesis_prompt(
    current_query: str,
    query_type: str,
    query_context: str,
    relevant_instructions: str
) -> str:
    """
    Get multi-instruction synthesis prompt.
    
    Args:
        current_query: Current query being processed
        query_type: Type of current query
        query_context: Current query context
        relevant_instructions: Formatted relevant instructions
    
    Returns:
        Formatted multi-instruction synthesis prompt
    """
    return MULTI_INSTRUCTION_SYNTHESIS_PROMPT.format(
        current_query=current_query,
        query_type=query_type,
        query_context=query_context,
        relevant_instructions=relevant_instructions
    )

def get_elaboration_quality_prompt(
    original_instruction: str,
    current_query: str,
    elaborated_instruction: str
) -> str:
    """
    Get elaboration quality assessment prompt.
    
    Args:
        original_instruction: Original memory instruction
        current_query: Current query being processed
        elaborated_instruction: Elaborated instruction to assess
    
    Returns:
        Formatted elaboration quality prompt
    """
    return ELABORATION_QUALITY_ASSESSMENT_PROMPT.format(
        original_instruction=original_instruction,
        current_query=current_query,
        elaborated_instruction=elaborated_instruction
    )
