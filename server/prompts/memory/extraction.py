"""
Memory extraction prompts for PaladinAI.

This module contains prompts for extracting valuable memories
from workflow interactions using OpenAI.
"""

def get_memory_extraction_prompt(
    user_input: str,
    workflow_type: str,
    content: str,
    context: dict = None
) -> str:
    """
    Generate prompt for extracting memories from workflow interactions.
    
    Args:
        user_input: Original user input
        workflow_type: Type of workflow (QUERY, ACTION, INCIDENT)
        content: Workflow content/response
        context: Additional context information
        
    Returns:
        Formatted prompt for memory extraction
    """
    context_info = ""
    if context:
        context_info = f"""
Additional Context:
- Workflow nodes executed: {', '.join(context.get('workflow_nodes', []))}
- Has Prometheus data: {context.get('has_prometheus_data', False)}
- Has Loki data: {context.get('has_loki_data', False)}
- Has Alert data: {context.get('has_alert_data', False)}
"""
    
    return f"""
Analyze the following {workflow_type} workflow interaction and extract important memories that should be stored for future reference.

User Input: {user_input}
Workflow Type: {workflow_type}
{context_info}

Content/Response: {content}

Extract memories that represent valuable insights, patterns, and knowledge from this interaction. 
The system should automatically determine the most appropriate memory type based on the content.

Examples of memory types (but not limited to these):
- **Operational Patterns**: Relationships between monitoring data and actions
- **User Preferences**: How users prefer to receive information
- **System Knowledge**: Learned facts about the monitored systems
- **Troubleshooting Relationships**: Causal relationships for problem-solving
- **Configuration Details**: System configurations and settings
- **Historical Context**: Past incidents and their resolutions
- **Performance Baselines**: Normal operating parameters
- **Alert Correlations**: Relationships between different alerts
- **Service Dependencies**: How services depend on each other
- **Error Patterns**: Common error scenarios and their causes

You can create new memory types as needed based on the content. Be creative and specific with memory types to capture the nuance of what's being learned.

Return a valid JSON array of memories. You can return either:

Option 1 - Direct array:
```json
[
    {{
        "content": "specific, actionable memory content",
        "type": "descriptive_memory_type (lowercase, use underscores for spaces)",
        "confidence": 0.8,
        "entities": ["entity1", "entity2"],
        "relationship": "describes the specific relationship or pattern"
    }}
]
```

Option 2 - Object with memories key:
```json
{{
    "memories": [
        {{
            "content": "specific, actionable memory content",
            "type": "descriptive_memory_type (lowercase, use underscores for spaces)",
            "confidence": 0.8,
            "entities": ["entity1", "entity2"],
            "relationship": "describes the specific relationship or pattern"
        }}
    ]
}}
```

IMPORTANT: Return ONLY valid JSON, no other text or explanation before or after.

Examples of dynamic memory types you might create:
- "deployment_pattern" for deployment-related insights
- "security_configuration" for security settings
- "data_retention_policy" for data management rules
- "incident_resolution_step" for troubleshooting procedures
- "api_usage_pattern" for API behavior observations
- "resource_threshold" for performance limits
- "team_escalation_path" for organizational knowledge

**Important Guidelines:**
- Only extract memories that would be valuable for future similar situations
- Each memory should be specific and actionable
- Confidence should be between 0.5 and 1.0
- Include relevant entities (services, metrics, log sources, etc.)
- Focus on operational knowledge that improves monitoring effectiveness

If no valuable memories can be extracted, return an empty array: []
"""


def get_relationship_extraction_prompt(content: str) -> str:
    """
    Generate prompt for extracting entity relationships from content.
    
    Args:
        content: Content to extract relationships from
        
    Returns:
        Formatted prompt for relationship extraction
    """
    return f"""
Analyze the following content and extract entity relationships that should be stored in a knowledge graph for operational monitoring and incident response.

Content: {content}

Extract relationships that represent operational connections between:
- **Monitoring entities**: metrics, alerts, logs, services, hosts
- **Actions**: fetch, query, analyze, monitor, check
- **Conditions**: performance issues, errors, incidents, thresholds
- **Resources**: CPU, memory, network, disk, applications, databases

Extract relationships in this exact JSON format:
[
    {{
        "source": "source_entity",
        "relationship": "DESCRIPTIVE_RELATIONSHIP_TYPE (uppercase, use underscores for spaces)",
        "target": "target_entity",
        "properties": {{"context": "additional context", "confidence": 0.9, "workflow_type": "ACTION|QUERY|INCIDENT"}}
    }}
]

You can create relationship types dynamically based on the actual relationships observed. Be specific and descriptive.

**Example Relationship Types (but not limited to these):**
- **FETCH**: Entity fetches/retrieves data from target
- **REQUIRES**: Entity requires target for operation
- **TRIGGERS**: Entity triggers action on target
- **AFFECTS**: Entity affects target's behavior
- **CONTAINS**: Entity contains target information
- **MONITORS**: Entity monitors target's state
- **INDICATES**: Entity indicates target condition
- **DEPENDS_ON**: Entity depends on target to function
- **CORRELATES_WITH**: Entity correlates with target behavior
- **PRECEDES**: Entity occurs before target
- **FOLLOWS**: Entity occurs after target
- **AUTHENTICATES_WITH**: Entity authenticates using target
- **ROUTES_TO**: Entity routes traffic/data to target
- **BACKS_UP**: Entity backs up target data
- **SCALES_WITH**: Entity scales based on target metrics
- **ALERTS_ON**: Entity creates alerts based on target
- **AGGREGATES**: Entity aggregates data from target
- **VALIDATES**: Entity validates target data/state

Create new relationship types as needed to accurately capture the nature of relationships in the content.

**Examples:**
- {{"source": "cpu_alerts", "relationship": "TRIGGERS", "target": "memory_metrics_fetch", "properties": {{"context": "High CPU usually requires memory analysis", "confidence": 0.9}}}}
- {{"source": "high_memory_usage", "relationship": "INDICATES", "target": "java_heap_issue", "properties": {{"context": "Memory spikes often indicate heap problems", "confidence": 0.8}}}}
- {{"source": "application_errors", "relationship": "REQUIRES", "target": "systemd_logs", "properties": {{"context": "App errors need service log analysis", "confidence": 0.9}}}}
- {{"source": "prometheus_metrics", "relationship": "CONTAINS", "target": "cpu_utilization", "properties": {{"context": "Prometheus stores CPU metrics", "confidence": 1.0}}}}

**Guidelines:**
- Focus on relationships that improve monitoring and troubleshooting
- Use specific entity names (e.g., "redis_service" not just "service")
- Include context that explains why the relationship is important
- Confidence should reflect how reliable this relationship is
- Only extract relationships that would help with future incidents

If no meaningful relationships can be extracted, return an empty array: []
"""


def get_instruction_processing_prompt(instruction: str, context: dict = None) -> str:
    """
    Generate prompt for processing explicit user instructions.
    
    Args:
        instruction: User's explicit instruction
        context: Additional context about the instruction
        
    Returns:
        Formatted prompt for instruction processing
    """
    context_info = ""
    if context:
        context_info = f"Context: {context}"
    
    return f"""
Process the following explicit user instruction and extract actionable operational memories and relationships.

Instruction: {instruction}
{context_info}

This instruction should be stored as a high-priority memory that will influence future workflow decisions and recommendations.

Extract the following information:

1. **Core Instruction**: The main directive or preference
2. **Applicable Scenarios**: When this instruction should be applied
3. **Related Entities**: Services, metrics, logs, or systems mentioned
4. **Priority Level**: How critical this instruction is (high/medium/low)
5. **Action Implications**: What actions should change based on this instruction

Format your response as JSON:
{{
    "memory": {{
        "content": "processed instruction content",
        "type": "instruction",
        "confidence": 1.0,
        "priority": "high|medium|low",
        "applicable_scenarios": ["scenario1", "scenario2"],
        "entities": ["entity1", "entity2"]
    }},
    "relationships": [
        {{
            "source": "instruction_entity",
            "relationship": "REQUIRES|TRIGGERS|AFFECTS|GUIDES",
            "target": "target_entity",
            "properties": {{"context": "how this instruction affects the relationship", "confidence": 1.0}}
        }}
    ]
}}

**Examples:**
- Instruction: "Always check systemd logs when there are application errors"
  - Creates relationship: application_errors REQUIRES systemd_logs
- Instruction: "For memory alerts, also fetch CPU and disk metrics"
  - Creates relationship: memory_alerts TRIGGERS multi_metric_fetch
- Instruction: "Prioritize critical alerts over warning alerts"
  - Creates relationship: critical_alerts PRIORITIZED_OVER warning_alerts

Ensure the extracted memory and relationships will improve future monitoring and incident response decisions.
"""


# System prompts for memory operations
MEMORY_ANALYSIS_SYSTEM_PROMPT = """
You are an expert operational memory analyst for a monitoring and incident response platform. Your role is to identify and extract valuable operational knowledge from workflow interactions that will improve future monitoring effectiveness.

Key principles:
1. Focus on actionable insights that improve monitoring and troubleshooting
2. Extract patterns that connect symptoms to root causes
3. Identify user preferences that improve experience
4. Capture system knowledge that reduces incident resolution time
5. Only extract memories that have clear operational value

IMPORTANT: You must return ONLY valid JSON arrays as specified in the prompt. Do not include any explanatory text before or after the JSON.

Be precise, specific, and focus on operational excellence.
"""

RELATIONSHIP_ANALYSIS_SYSTEM_PROMPT = """
You are an expert knowledge graph analyst for operational monitoring systems. You specialize in identifying relationships between monitoring entities, actions, and conditions that improve incident response.

Key principles:
1. Extract relationships that have clear operational value
2. Focus on causal relationships that help with troubleshooting
3. Identify data flow relationships for monitoring efficiency
4. Map service dependencies and their monitoring implications
5. Connect symptoms to investigation procedures

Be specific about entity names and relationship types to enable effective knowledge graph queries.
"""

INSTRUCTION_PROCESSING_SYSTEM_PROMPT = """
You are an expert at processing explicit user instructions for operational monitoring systems. You translate user directives into actionable memories and operational relationships.

Key principles:
1. Preserve the user's intent precisely
2. Extract operational implications of the instruction
3. Identify when and how the instruction should be applied
4. Map instruction effects to monitoring workflows
5. Ensure instructions can be operationalized in future workflows

Focus on making instructions actionable and measurable for improved monitoring outcomes.
"""