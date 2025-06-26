"""
Prometheus monitoring tool for Paladin AI
Provides agentic capabilities for metrics querying and monitoring.
"""

from .service import PrometheusService, prometheus
from .models import (
    PrometheusQueryRequest,
    PrometheusRangeQueryRequest,
    PrometheusQueryResponse,
    PrometheusMetricData,
    PrometheusTargetsResponse,
    PrometheusMetadataResponse,
)

__all__ = [
    "PrometheusService",
    "prometheus",
    "PrometheusQueryRequest",
    "PrometheusRangeQueryRequest", 
    "PrometheusQueryResponse",
    "PrometheusMetricData",
    "PrometheusTargetsResponse",
    "PrometheusMetadataResponse",
]
