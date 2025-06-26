"""
Pydantic models for Prometheus tool requests and responses.
"""

from typing import Dict, List, Optional, Any, Union
from pydantic import BaseModel, Field
from datetime import datetime


class PrometheusQueryRequest(BaseModel):
    """Request model for Prometheus instant queries."""
    query: str = Field(..., description="PromQL query string")
    time: Optional[str] = Field(None, description="Evaluation timestamp (RFC3339 or Unix timestamp)")
    timeout: Optional[str] = Field(None, description="Query timeout duration")


class PrometheusRangeQueryRequest(BaseModel):
    """Request model for Prometheus range queries."""
    query: str = Field(..., description="PromQL query string")
    start: str = Field(..., description="Start timestamp (RFC3339 or Unix timestamp)")
    end: str = Field(..., description="End timestamp (RFC3339 or Unix timestamp)")
    step: str = Field(..., description="Query resolution step width")
    timeout: Optional[str] = Field(None, description="Query timeout duration")


class PrometheusMetricValue(BaseModel):
    """Single metric value with timestamp."""
    timestamp: float = Field(..., description="Unix timestamp")
    value: str = Field(..., description="Metric value as string")


class PrometheusMetricData(BaseModel):
    """Prometheus metric data structure."""
    metric: Dict[str, str] = Field(..., description="Metric labels")
    value: Optional[List[Union[float, str]]] = Field(None, description="Single value [timestamp, value]")
    values: Optional[List[List[Union[float, str]]]] = Field(None, description="Time series values")


class PrometheusQueryResponse(BaseModel):
    """Response model for Prometheus queries."""
    success: bool = Field(..., description="Whether the query was successful")
    status: str = Field(..., description="Query status (success/error)")
    data: Optional[Dict[str, Any]] = Field(None, description="Query result data")
    error: Optional[str] = Field(None, description="Error message if query failed")
    error_type: Optional[str] = Field(None, description="Error type if query failed")
    warnings: Optional[List[str]] = Field(None, description="Query warnings")


class PrometheusTarget(BaseModel):
    """Prometheus scrape target information."""
    discovered_labels: Dict[str, str] = Field(..., description="Discovered target labels")
    labels: Dict[str, str] = Field(..., description="Target labels after relabeling")
    scrape_pool: str = Field(..., description="Scrape pool name")
    scrape_url: str = Field(..., description="Target scrape URL")
    global_url: str = Field(..., description="Global target URL")
    last_error: str = Field(..., description="Last scrape error")
    last_scrape: str = Field(..., description="Last scrape timestamp")
    last_scrape_duration: float = Field(..., description="Last scrape duration in seconds")
    health: str = Field(..., description="Target health status")


class PrometheusTargetsResponse(BaseModel):
    """Response model for Prometheus targets."""
    success: bool = Field(..., description="Whether the request was successful")
    active_targets: Optional[List[PrometheusTarget]] = Field(None, description="Active targets")
    dropped_targets: Optional[List[PrometheusTarget]] = Field(None, description="Dropped targets")
    error: Optional[str] = Field(None, description="Error message if request failed")


class PrometheusMetadata(BaseModel):
    """Prometheus metric metadata."""
    type: str = Field(..., description="Metric type (counter, gauge, histogram, summary)")
    help: str = Field(..., description="Metric help text")
    unit: Optional[str] = Field(None, description="Metric unit")


class PrometheusMetadataResponse(BaseModel):
    """Response model for Prometheus metadata."""
    success: bool = Field(..., description="Whether the request was successful")
    metadata: Optional[Dict[str, List[PrometheusMetadata]]] = Field(None, description="Metric metadata")
    error: Optional[str] = Field(None, description="Error message if request failed")


class PrometheusLabelsResponse(BaseModel):
    """Response model for Prometheus labels."""
    success: bool = Field(..., description="Whether the request was successful")
    labels: Optional[List[str]] = Field(None, description="Available label names")
    error: Optional[str] = Field(None, description="Error message if request failed")


class PrometheusLabelValuesResponse(BaseModel):
    """Response model for Prometheus label values."""
    success: bool = Field(..., description="Whether the request was successful")
    values: Optional[List[str]] = Field(None, description="Available label values")
    error: Optional[str] = Field(None, description="Error message if request failed")
