"""
Alertmanager tool for Paladin AI
Provides agentic capabilities for alert management and notifications.
"""

from .service import AlertmanagerService, alertmanager
from .models import (
    AlertmanagerAlert,
    AlertmanagerAlertsResponse,
    AlertmanagerSilence,
    AlertmanagerSilenceRequest,
    AlertmanagerSilencesResponse,
    AlertmanagerStatusResponse,
    AlertmanagerReceiversResponse,
    AlertmanagerConfigResponse,
)

__all__ = [
    "AlertmanagerService",
    "alertmanager",
    "AlertmanagerAlert",
    "AlertmanagerAlertsResponse",
    "AlertmanagerSilence",
    "AlertmanagerSilenceRequest",
    "AlertmanagerSilencesResponse",
    "AlertmanagerStatusResponse",
    "AlertmanagerReceiversResponse",
    "AlertmanagerConfigResponse",
]
