"""
Agentic tools package for Paladin AI Server
Contains monitoring and observability tools for LangGraph workflows.
"""

# Import all tool services
from .prometheus import PrometheusService, prometheus
from .loki import LokiService, loki
from .alertmanager import AlertmanagerService, alertmanager

# Import all models for external use
from .prometheus.models import (
    PrometheusQueryRequest,
    PrometheusRangeQueryRequest,
    PrometheusQueryResponse,
    PrometheusMetricData,
    PrometheusTargetsResponse,
    PrometheusMetadataResponse,
    PrometheusLabelsResponse,
    PrometheusLabelValuesResponse,
)

from .loki.models import (
    LokiQueryRequest,
    LokiRangeQueryRequest,
    LokiQueryResponse,
    LokiLogEntry,
    LokiStream,
    LokiLabelsResponse,
    LokiLabelValuesResponse,
    LokiSeriesRequest,
    LokiSeriesResponse,
    LokiTailRequest,
    LokiMetricsResponse,
)

from .alertmanager.models import (
    AlertmanagerAlert,
    AlertmanagerAlertsResponse,
    AlertmanagerSilence,
    AlertmanagerSilenceRequest,
    AlertmanagerSilencesResponse,
    AlertmanagerSilenceCreateResponse,
    AlertmanagerStatusResponse,
    AlertmanagerReceiversResponse,
    AlertmanagerConfigResponse,
    AlertmanagerGroupsResponse,
)

# Export all services and commonly used models
__all__ = [
    # Services
    "PrometheusService",
    "prometheus",
    "LokiService", 
    "loki",
    "AlertmanagerService",
    "alertmanager",
    
    # Prometheus models
    "PrometheusQueryRequest",
    "PrometheusRangeQueryRequest",
    "PrometheusQueryResponse",
    "PrometheusMetricData",
    "PrometheusTargetsResponse",
    "PrometheusMetadataResponse",
    "PrometheusLabelsResponse",
    "PrometheusLabelValuesResponse",
    
    # Loki models
    "LokiQueryRequest",
    "LokiRangeQueryRequest", 
    "LokiQueryResponse",
    "LokiLogEntry",
    "LokiStream",
    "LokiLabelsResponse",
    "LokiLabelValuesResponse",
    "LokiSeriesRequest",
    "LokiSeriesResponse",
    "LokiTailRequest",
    "LokiMetricsResponse",
    
    # Alertmanager models
    "AlertmanagerAlert",
    "AlertmanagerAlertsResponse",
    "AlertmanagerSilence",
    "AlertmanagerSilenceRequest",
    "AlertmanagerSilencesResponse",
    "AlertmanagerSilenceCreateResponse",
    "AlertmanagerStatusResponse",
    "AlertmanagerReceiversResponse",
    "AlertmanagerConfigResponse",
    "AlertmanagerGroupsResponse",
]


def get_all_tools():
    """
    Get all available monitoring tools as a dictionary.
    
    Returns:
        Dict[str, Any]: Dictionary of tool name to service instance
    """
    return {
        "prometheus": prometheus,
        "loki": loki,
        "alertmanager": alertmanager,
    }


def get_tool_descriptions():
    """
    Get descriptions of all available tools.
    
    Returns:
        Dict[str, str]: Dictionary of tool name to description
    """
    return {
        "prometheus": "Metrics querying and monitoring tool for Prometheus",
        "loki": "Log aggregation and analysis tool for Loki",
        "alertmanager": "Alert management and notification tool for Alertmanager",
    }


async def health_check_all_tools():
    """
    Perform health checks on all monitoring tools.
    
    Returns:
        Dict[str, Dict[str, Any]]: Health status for each tool
    """
    tools = get_all_tools()
    health_status = {}
    
    for tool_name, tool_service in tools.items():
        try:
            if hasattr(tool_service, 'get_status'):
                # For Alertmanager
                result = await tool_service.get_status()
                health_status[tool_name] = {
                    "healthy": result.success,
                    "error": result.error if not result.success else None
                }
            elif hasattr(tool_service, 'get_labels'):
                # For Prometheus and Loki
                result = await tool_service.get_labels()
                health_status[tool_name] = {
                    "healthy": result.success,
                    "error": result.error if not result.success else None
                }
            else:
                health_status[tool_name] = {
                    "healthy": False,
                    "error": "No health check method available"
                }
        except Exception as e:
            health_status[tool_name] = {
                "healthy": False,
                "error": f"Health check failed: {str(e)}"
            }
    
    return health_status
