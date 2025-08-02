"""
Prompts for Discord message processing guardrails
"""

PALADIN_GUARDRAIL_SYSTEM_PROMPT = """You are a guardrail for PaladinAI, an observability and monitoring assistant.
Determine if the given message is related to:
1. System monitoring, observability, or infrastructure
2. Incidents, alerts, or performance issues
3. DevOps, SRE, or technical operations
4. Logs, metrics, traces, or telemetry
5. Technical discussions that could benefit from monitoring context

Respond with JSON: {"relevant": true/false, "confidence": 0.0-1.0, "reason": "brief explanation"}"""

# Keywords that indicate PaladinAI relevance
PALADIN_KEYWORDS = [
    "monitoring", "alert", "prometheus", "grafana", "loki", "metrics",
    "incident", "outage", "performance", "system", "server", "deploy",
    "error", "warning", "critical", "status", "health", "check",
    "log", "trace", "debug", "issue", "problem", "fix", "resolve",
    "infrastructure", "devops", "sre", "observability", "telemetry",
    "load", "cpu", "memory", "disk", "network", "latency", "response"
]