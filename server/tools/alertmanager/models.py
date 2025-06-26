"""
Pydantic models for Alertmanager tool requests and responses.
"""

from typing import Dict, List, Optional, Any
from pydantic import BaseModel, Field
from datetime import datetime


class AlertmanagerLabel(BaseModel):
    """Alertmanager label matcher."""
    name: str = Field(..., description="Label name")
    value: str = Field(..., description="Label value")
    is_regex: bool = Field(False, description="Whether value is a regex")
    is_equal: bool = Field(True, description="Whether matcher is equality (true) or inequality (false)")


class AlertmanagerAnnotation(BaseModel):
    """Alertmanager annotation."""
    key: str = Field(..., description="Annotation key")
    value: str = Field(..., description="Annotation value")


class AlertmanagerAlert(BaseModel):
    """Alertmanager alert information."""
    labels: Dict[str, str] = Field(..., description="Alert labels")
    annotations: Dict[str, str] = Field(..., description="Alert annotations")
    starts_at: str = Field(..., description="Alert start time")
    ends_at: str = Field(..., description="Alert end time")
    generator_url: str = Field(..., description="Generator URL")
    status: Dict[str, Any] = Field(..., description="Alert status information")
    receivers: List[str] = Field(..., description="Alert receivers")
    fingerprint: str = Field(..., description="Alert fingerprint")


class AlertmanagerAlertsResponse(BaseModel):
    """Response model for Alertmanager alerts."""
    success: bool = Field(..., description="Whether the request was successful")
    alerts: Optional[List[AlertmanagerAlert]] = Field(None, description="List of alerts")
    error: Optional[str] = Field(None, description="Error message if request failed")


class AlertmanagerMatcher(BaseModel):
    """Alertmanager silence matcher."""
    name: str = Field(..., description="Label name to match")
    value: str = Field(..., description="Label value to match")
    is_regex: bool = Field(False, description="Whether value is a regex")
    is_equal: bool = Field(True, description="Whether matcher is equality or inequality")


class AlertmanagerSilenceRequest(BaseModel):
    """Request model for creating Alertmanager silences."""
    matchers: List[AlertmanagerMatcher] = Field(..., description="Label matchers for the silence")
    starts_at: str = Field(..., description="Silence start time (RFC3339)")
    ends_at: str = Field(..., description="Silence end time (RFC3339)")
    created_by: str = Field(..., description="Creator of the silence")
    comment: str = Field(..., description="Comment describing the silence")


class AlertmanagerSilence(BaseModel):
    """Alertmanager silence information."""
    id: str = Field(..., description="Silence ID")
    matchers: List[AlertmanagerMatcher] = Field(..., description="Label matchers")
    starts_at: str = Field(..., description="Silence start time")
    ends_at: str = Field(..., description="Silence end time")
    updated_at: str = Field(..., description="Last update time")
    created_by: str = Field(..., description="Creator of the silence")
    comment: str = Field(..., description="Silence comment")
    status: Dict[str, Any] = Field(..., description="Silence status")


class AlertmanagerSilencesResponse(BaseModel):
    """Response model for Alertmanager silences."""
    success: bool = Field(..., description="Whether the request was successful")
    silences: Optional[List[AlertmanagerSilence]] = Field(None, description="List of silences")
    error: Optional[str] = Field(None, description="Error message if request failed")


class AlertmanagerSilenceCreateResponse(BaseModel):
    """Response model for creating Alertmanager silences."""
    success: bool = Field(..., description="Whether the request was successful")
    silence_id: Optional[str] = Field(None, description="Created silence ID")
    error: Optional[str] = Field(None, description="Error message if request failed")


class AlertmanagerReceiver(BaseModel):
    """Alertmanager receiver configuration."""
    name: str = Field(..., description="Receiver name")
    integrations: List[Dict[str, Any]] = Field(..., description="Receiver integrations")


class AlertmanagerReceiversResponse(BaseModel):
    """Response model for Alertmanager receivers."""
    success: bool = Field(..., description="Whether the request was successful")
    receivers: Optional[List[AlertmanagerReceiver]] = Field(None, description="List of receivers")
    error: Optional[str] = Field(None, description="Error message if request failed")


class AlertmanagerStatus(BaseModel):
    """Alertmanager status information."""
    cluster: Dict[str, Any] = Field(..., description="Cluster status")
    version_info: Dict[str, str] = Field(..., description="Version information")
    config: Dict[str, Any] = Field(..., description="Configuration status")
    uptime: str = Field(..., description="Uptime duration")


class AlertmanagerStatusResponse(BaseModel):
    """Response model for Alertmanager status."""
    success: bool = Field(..., description="Whether the request was successful")
    status: Optional[AlertmanagerStatus] = Field(None, description="Status information")
    error: Optional[str] = Field(None, description="Error message if request failed")


class AlertmanagerConfigResponse(BaseModel):
    """Response model for Alertmanager configuration."""
    success: bool = Field(..., description="Whether the request was successful")
    config: Optional[Dict[str, Any]] = Field(None, description="Configuration data")
    error: Optional[str] = Field(None, description="Error message if request failed")


class AlertmanagerAlertGroup(BaseModel):
    """Alertmanager alert group."""
    labels: Dict[str, str] = Field(..., description="Group labels")
    group_key: str = Field(..., description="Group key")
    blocks: List[Dict[str, Any]] = Field(..., description="Alert blocks in group")


class AlertmanagerGroupsResponse(BaseModel):
    """Response model for Alertmanager alert groups."""
    success: bool = Field(..., description="Whether the request was successful")
    groups: Optional[List[AlertmanagerAlertGroup]] = Field(None, description="List of alert groups")
    error: Optional[str] = Field(None, description="Error message if request failed")
