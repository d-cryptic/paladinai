"""
Loki log aggregation tool for Paladin AI
Provides agentic capabilities for log querying and analysis.
"""

from .service import LokiService, loki
from .models import (
    LokiQueryRequest,
    LokiRangeQueryRequest,
    LokiQueryResponse,
    LokiLogEntry,
    LokiStream,
    LokiLabelsResponse,
    LokiLabelValuesResponse,
    LokiSeriesResponse,
)

__all__ = [
    "LokiService",
    "loki",
    "LokiQueryRequest",
    "LokiRangeQueryRequest",
    "LokiQueryResponse",
    "LokiLogEntry",
    "LokiStream",
    "LokiLabelsResponse",
    "LokiLabelValuesResponse",
    "LokiSeriesResponse",
]
