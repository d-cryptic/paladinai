# Paladin AI Monitoring Tools

This directory contains agentic tools for monitoring and observability in the Paladin AI platform. These tools provide programmatic access to Prometheus, Loki, and Alertmanager for use in LangGraph workflows.

## Overview

The monitoring tools are designed to be:
- **Agentic**: Can be used by AI agents in LangGraph workflows
- **Observable**: All methods are traced with Langfuse `@observe` decorators
- **Async**: Built with async/await for high performance
- **Type-safe**: Full Pydantic model validation
- **Environment-configurable**: All settings via environment variables

## Available Tools

### 1. Prometheus Tool (`prometheus/`)
Provides metrics querying and monitoring capabilities.

**Key Features:**
- Instant and range PromQL queries
- Target health monitoring
- Metadata and label discovery
- Metrics exploration

**Example Usage:**
```python
from tools import prometheus
from tools.prometheus.models import PrometheusQueryRequest

# Query current CPU usage
query = PrometheusQueryRequest(
    query="100 - (avg(irate(node_cpu_seconds_total{mode='idle'}[5m])) * 100)"
)
result = await prometheus.query(query)
```

### 2. Loki Tool (`loki/`)
Provides log aggregation and analysis capabilities.

**Key Features:**
- LogQL query execution
- Log streaming and tailing
- Label and series discovery
- Metrics queries from logs

**Example Usage:**
```python
from tools import loki
from tools.loki.models import LokiQueryRequest

# Query error logs
query = LokiQueryRequest(
    query='{job="varlogs"} |= "error"',
    limit=100
)
result = await loki.query(query)
```

### 3. Alertmanager Tool (`alertmanager/`)
Provides alert management and notification capabilities.

**Key Features:**
- Alert retrieval and filtering
- Silence management
- Status and configuration access
- Receiver information

**Example Usage:**
```python
from tools import alertmanager

# Get active alerts
alerts = await alertmanager.get_alerts(active=True)

# Create a silence
from tools.alertmanager.models import AlertmanagerSilenceRequest, AlertmanagerMatcher

silence_request = AlertmanagerSilenceRequest(
    matchers=[AlertmanagerMatcher(name="alertname", value="HighCPU")],
    starts_at="2024-01-01T00:00:00Z",
    ends_at="2024-01-01T01:00:00Z",
    created_by="paladin-ai",
    comment="Maintenance window"
)
result = await alertmanager.create_silence(silence_request)
```

## Configuration

All tools are configured via environment variables:

```bash
# Prometheus Configuration
PROMETHEUS_URL=http://<your_prometheus_url>:9090
PROMETHEUS_TIMEOUT=30
PROMETHEUS_AUTH_TOKEN=  # Optional

# Loki Configuration  
LOKI_URL=http://<your_loki_url>:3100
LOKI_TIMEOUT=30
LOKI_AUTH_TOKEN=  # Optional

# Alertmanager Configuration
ALERTMANAGER_URL=http://<your_alertmanager_url>:9093
ALERTMANAGER_TIMEOUT=30
ALERTMANAGER_AUTH_TOKEN=  # Optional
```

## Integration with LangGraph

These tools are designed to be used as tools in LangGraph workflows:

```python
from tools import get_all_tools

# Get all monitoring tools
monitoring_tools = get_all_tools()

# Use in LangGraph workflow
from langgraph import StateGraph

def monitoring_workflow():
    graph = StateGraph()
    
    # Add tools to workflow
    for tool_name, tool_service in monitoring_tools.items():
        graph.add_tool(tool_name, tool_service)
    
    return graph
```

## Health Checking

All tools support health checking:

```python
from tools import health_check_all_tools

# Check health of all tools
health_status = await health_check_all_tools()

for tool_name, status in health_status.items():
    if status["healthy"]:
        print(f"✅ {tool_name} is healthy")
    else:
        print(f"❌ {tool_name} error: {status['error']}")
```

## Error Handling

All tools follow consistent error handling patterns:

- **Success responses**: `success=True` with data
- **Error responses**: `success=False` with error message
- **Network errors**: Handled gracefully with descriptive messages
- **Timeout handling**: Configurable timeouts per tool

## Observability

All tool methods are instrumented with Langfuse tracing:

- Method calls are automatically traced
- Request/response data is captured
- Performance metrics are collected
- Error tracking is enabled

## File Structure

```
tools/
├── __init__.py              # Main exports and utilities
├── README.md               # This file
├── examples.py             # Usage examples
├── prometheus/             # Prometheus tool
│   ├── __init__.py
│   ├── models.py          # Pydantic models
│   └── service.py         # Main service class
├── loki/                  # Loki tool
│   ├── __init__.py
│   ├── models.py          # Pydantic models
│   └── service.py         # Main service class
└── alertmanager/          # Alertmanager tool
    ├── __init__.py
    ├── models.py          # Pydantic models
    └── service.py         # Main service class
```

## Running Examples

To see the tools in action:

```bash
cd server
uv run python tools/examples.py
```

This will demonstrate:
- Prometheus metrics queries
- Loki log searches
- Alertmanager alert management
- Health checking all tools

## Development

When adding new tools or extending existing ones:

1. Follow the existing patterns in `service.py` files
2. Add Pydantic models in `models.py` files
3. Use `@observe` decorators for tracing
4. Include proper error handling
5. Add examples to `examples.py`
6. Update this README

## Dependencies

The tools require:
- `aiohttp` for HTTP client functionality
- `pydantic` for data validation
- `langfuse` for observability
- `python-dotenv` for environment configuration

These are included in the server's `pyproject.toml` dependencies.
