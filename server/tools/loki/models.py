"""
Pydantic models for Loki tool requests and responses.
"""

from typing import Dict, List, Optional, Any, Union
from pydantic import BaseModel, Field
from datetime import datetime


class LokiQueryRequest(BaseModel):
    """Request model for Loki instant log queries."""
    query: str = Field(..., description="LogQL query string")
    time: Optional[str] = Field(None, description="Query timestamp (RFC3339 or Unix nanoseconds)")
    direction: Optional[str] = Field("backward", description="Query direction (forward/backward)")
    limit: Optional[int] = Field(100, description="Maximum number of entries to return")


class LokiRangeQueryRequest(BaseModel):
    """Request model for Loki range log queries."""
    query: str = Field(..., description="LogQL query string")
    start: str = Field(..., description="Start timestamp (RFC3339 or Unix nanoseconds)")
    end: str = Field(..., description="End timestamp (RFC3339 or Unix nanoseconds)")
    direction: Optional[str] = Field("backward", description="Query direction (forward/backward)")
    limit: Optional[int] = Field(100, description="Maximum number of entries to return")
    step: Optional[str] = Field(None, description="Query resolution step width for metric queries")


class LokiLogEntry(BaseModel):
    """Single log entry from Loki."""
    timestamp: str = Field(..., description="Log entry timestamp (nanoseconds)")
    line: str = Field(..., description="Log line content")


class LokiStream(BaseModel):
    """Loki log stream with labels and entries."""
    stream: Dict[str, str] = Field(..., description="Stream labels")
    values: List[List[str]] = Field(..., description="Log entries [timestamp, line]")


class LokiQueryResponse(BaseModel):
    """Response model for Loki queries."""
    success: bool = Field(..., description="Whether the query was successful")
    status: str = Field(..., description="Query status (success/error)")
    data: Optional[Dict[str, Any]] = Field(None, description="Query result data")
    result_type: Optional[str] = Field(None, description="Result type (streams/matrix/vector)")
    result: Optional[List[Union[LokiStream, Dict[str, Any]]]] = Field(None, description="Query results")
    stats: Optional[Dict[str, Any]] = Field(None, description="Query statistics")
    error: Optional[str] = Field(None, description="Error message if query failed")
    error_type: Optional[str] = Field(None, description="Error type if query failed")


class LokiLabelsResponse(BaseModel):
    """Response model for Loki labels."""
    success: bool = Field(..., description="Whether the request was successful")
    labels: Optional[List[str]] = Field(None, description="Available label names")
    error: Optional[str] = Field(None, description="Error message if request failed")


class LokiLabelValuesResponse(BaseModel):
    """Response model for Loki label values."""
    success: bool = Field(..., description="Whether the request was successful")
    values: Optional[List[str]] = Field(None, description="Available label values")
    error: Optional[str] = Field(None, description="Error message if request failed")


class LokiSeriesRequest(BaseModel):
    """Request model for Loki series queries."""
    match: List[str] = Field(..., description="Series selector matchers")
    start: Optional[str] = Field(None, description="Start timestamp")
    end: Optional[str] = Field(None, description="End timestamp")


class LokiSeries(BaseModel):
    """Loki series information."""
    labels: Dict[str, str] = Field(..., description="Series labels")


class LokiSeriesResponse(BaseModel):
    """Response model for Loki series."""
    success: bool = Field(..., description="Whether the request was successful")
    series: Optional[List[LokiSeries]] = Field(None, description="Available series")
    error: Optional[str] = Field(None, description="Error message if request failed")


class LokiTailRequest(BaseModel):
    """Request model for Loki log tailing."""
    query: str = Field(..., description="LogQL query string")
    delay_for: Optional[int] = Field(0, description="Delay in seconds before tailing")
    limit: Optional[int] = Field(100, description="Maximum number of entries to return")
    start: Optional[str] = Field(None, description="Start timestamp for tailing")


class LokiMetricsResponse(BaseModel):
    """Response model for Loki metrics queries."""
    success: bool = Field(..., description="Whether the query was successful")
    status: str = Field(..., description="Query status (success/error)")
    data: Optional[Dict[str, Any]] = Field(None, description="Metrics data")
    result_type: Optional[str] = Field(None, description="Result type (matrix/vector)")
    result: Optional[List[Dict[str, Any]]] = Field(None, description="Metrics results")
    error: Optional[str] = Field(None, description="Error message if query failed")


class LokiStatsResponse(BaseModel):
    """Response model for Loki query statistics."""
    success: bool = Field(..., description="Whether the request was successful")
    ingester: Optional[Dict[str, Any]] = Field(None, description="Ingester statistics")
    store: Optional[Dict[str, Any]] = Field(None, description="Store statistics")
    summary: Optional[Dict[str, Any]] = Field(None, description="Summary statistics")
    error: Optional[str] = Field(None, description="Error message if request failed")
