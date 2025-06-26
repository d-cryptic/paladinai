"""
Alertmanager service for Paladin AI
Handles Alertmanager API calls for alert management and notifications.
"""

import os
import aiohttp
from typing import Dict, List, Optional, Any
from dotenv import load_dotenv
from langfuse import observe

from .models import (
    AlertmanagerAlertsResponse,
    AlertmanagerSilenceRequest,
    AlertmanagerSilencesResponse,
    AlertmanagerSilenceCreateResponse,
    AlertmanagerStatusResponse,
    AlertmanagerReceiversResponse,
    AlertmanagerConfigResponse,
    AlertmanagerGroupsResponse,
)

# Load environment variables
load_dotenv()


class AlertmanagerService:
    """Service class for handling Alertmanager API calls."""
    
    def __init__(self):
        """Initialize Alertmanager client with environment configuration."""
        self.base_url = os.getenv("ALERTMANAGER_URL")
        self.timeout = int(os.getenv("ALERTMANAGER_TIMEOUT", "30"))
        self.auth_token = os.getenv("ALERTMANAGER_AUTH_TOKEN")
        
        # Remove trailing slash from base URL
        self.base_url = self.base_url.rstrip('/')
        
        # Setup headers
        self.headers = {"Content-Type": "application/json"}
        if self.auth_token:
            self.headers["Authorization"] = f"Bearer {self.auth_token}"
    
    async def _make_request(
        self, 
        method: str, 
        endpoint: str, 
        params: Optional[Dict[str, Any]] = None,
        data: Optional[Dict[str, Any]] = None
    ) -> Dict[str, Any]:
        """Make HTTP request to Alertmanager API."""
        url = f"{self.base_url}{endpoint}"
        
        timeout = aiohttp.ClientTimeout(total=self.timeout)
        
        try:
            async with aiohttp.ClientSession(timeout=timeout) as session:
                async with session.request(
                    method=method,
                    url=url,
                    headers=self.headers,
                    params=params,
                    json=data
                ) as response:
                    if response.content_type == 'application/json':
                        result = await response.json()
                    else:
                        result = {"message": await response.text()}
                    
                    if response.status in [200, 201, 202]:
                        return {"success": True, "data": result}
                    else:
                        return {
                            "success": False,
                            "error": f"HTTP {response.status}: {result.get('error', result.get('message', 'Unknown error'))}",
                            "status_code": response.status
                        }
                        
        except aiohttp.ClientError as e:
            return {
                "success": False,
                "error": f"Connection error: {str(e)}"
            }
        except Exception as e:
            return {
                "success": False,
                "error": f"Unexpected error: {str(e)}"
            }
    
    @observe(name="alertmanager_get_alerts")
    async def get_alerts(
        self, 
        active: Optional[bool] = None,
        silenced: Optional[bool] = None,
        inhibited: Optional[bool] = None,
        unprocessed: Optional[bool] = None,
        filter: Optional[str] = None
    ) -> AlertmanagerAlertsResponse:
        """
        Get alerts from Alertmanager.
        
        Args:
            active: Filter for active alerts
            silenced: Filter for silenced alerts
            inhibited: Filter for inhibited alerts
            unprocessed: Filter for unprocessed alerts
            filter: Additional filter expression
            
        Returns:
            AlertmanagerAlertsResponse with alerts
        """
        params = {}
        if active is not None:
            params["active"] = str(active).lower()
        if silenced is not None:
            params["silenced"] = str(silenced).lower()
        if inhibited is not None:
            params["inhibited"] = str(inhibited).lower()
        if unprocessed is not None:
            params["unprocessed"] = str(unprocessed).lower()
        if filter:
            params["filter"] = filter
            
        result = await self._make_request("GET", "/api/v1/alerts", params=params)
        
        if result.get("success"):
            return AlertmanagerAlertsResponse(
                success=True,
                alerts=result.get("data", [])
            )
        else:
            return AlertmanagerAlertsResponse(
                success=False,
                error=result.get("error", "Unknown error")
            )
    
    @observe(name="alertmanager_get_silences")
    async def get_silences(self, filter: Optional[str] = None) -> AlertmanagerSilencesResponse:
        """
        Get silences from Alertmanager.
        
        Args:
            filter: Filter expression for silences
            
        Returns:
            AlertmanagerSilencesResponse with silences
        """
        params = {}
        if filter:
            params["filter"] = filter
            
        result = await self._make_request("GET", "/api/v1/silences", params=params)
        
        if result.get("success"):
            return AlertmanagerSilencesResponse(
                success=True,
                silences=result.get("data", [])
            )
        else:
            return AlertmanagerSilencesResponse(
                success=False,
                error=result.get("error", "Unknown error")
            )
    
    @observe(name="alertmanager_create_silence")
    async def create_silence(self, request: AlertmanagerSilenceRequest) -> AlertmanagerSilenceCreateResponse:
        """
        Create a new silence in Alertmanager.
        
        Args:
            request: Silence creation request
            
        Returns:
            AlertmanagerSilenceCreateResponse with silence ID
        """
        # Convert request to Alertmanager API format
        silence_data = {
            "matchers": [
                {
                    "name": matcher.name,
                    "value": matcher.value,
                    "isRegex": matcher.is_regex,
                    "isEqual": matcher.is_equal
                }
                for matcher in request.matchers
            ],
            "startsAt": request.starts_at,
            "endsAt": request.ends_at,
            "createdBy": request.created_by,
            "comment": request.comment
        }
        
        result = await self._make_request("POST", "/api/v1/silences", data=silence_data)
        
        if result.get("success"):
            data = result.get("data", {})
            return AlertmanagerSilenceCreateResponse(
                success=True,
                silence_id=data.get("silenceID")
            )
        else:
            return AlertmanagerSilenceCreateResponse(
                success=False,
                error=result.get("error", "Unknown error")
            )
    
    @observe(name="alertmanager_delete_silence")
    async def delete_silence(self, silence_id: str) -> Dict[str, Any]:
        """
        Delete a silence from Alertmanager.
        
        Args:
            silence_id: ID of the silence to delete
            
        Returns:
            Dict with success status and any error message
        """
        result = await self._make_request("DELETE", f"/api/v1/silence/{silence_id}")
        
        return {
            "success": result.get("success", False),
            "error": result.get("error")
        }
    
    @observe(name="alertmanager_get_status")
    async def get_status(self) -> AlertmanagerStatusResponse:
        """
        Get Alertmanager status information.
        
        Returns:
            AlertmanagerStatusResponse with status data
        """
        result = await self._make_request("GET", "/api/v1/status")
        
        if result.get("success"):
            return AlertmanagerStatusResponse(
                success=True,
                status=result.get("data")
            )
        else:
            return AlertmanagerStatusResponse(
                success=False,
                error=result.get("error", "Unknown error")
            )
    
    @observe(name="alertmanager_get_receivers")
    async def get_receivers(self) -> AlertmanagerReceiversResponse:
        """
        Get Alertmanager receivers configuration.
        
        Returns:
            AlertmanagerReceiversResponse with receivers data
        """
        result = await self._make_request("GET", "/api/v1/receivers")
        
        if result.get("success"):
            return AlertmanagerReceiversResponse(
                success=True,
                receivers=result.get("data", [])
            )
        else:
            return AlertmanagerReceiversResponse(
                success=False,
                error=result.get("error", "Unknown error")
            )
    
    @observe(name="alertmanager_get_config")
    async def get_config(self) -> AlertmanagerConfigResponse:
        """
        Get Alertmanager configuration.
        
        Returns:
            AlertmanagerConfigResponse with configuration data
        """
        result = await self._make_request("GET", "/api/v1/config")
        
        if result.get("success"):
            return AlertmanagerConfigResponse(
                success=True,
                config=result.get("data")
            )
        else:
            return AlertmanagerConfigResponse(
                success=False,
                error=result.get("error", "Unknown error")
            )
    
    @observe(name="alertmanager_get_alert_groups")
    async def get_alert_groups(
        self, 
        active: Optional[bool] = None,
        silenced: Optional[bool] = None,
        inhibited: Optional[bool] = None,
        filter: Optional[str] = None
    ) -> AlertmanagerGroupsResponse:
        """
        Get alert groups from Alertmanager.
        
        Args:
            active: Filter for active alerts
            silenced: Filter for silenced alerts
            inhibited: Filter for inhibited alerts
            filter: Additional filter expression
            
        Returns:
            AlertmanagerGroupsResponse with alert groups
        """
        params = {}
        if active is not None:
            params["active"] = str(active).lower()
        if silenced is not None:
            params["silenced"] = str(silenced).lower()
        if inhibited is not None:
            params["inhibited"] = str(inhibited).lower()
        if filter:
            params["filter"] = filter
            
        result = await self._make_request("GET", "/api/v1/alerts/groups", params=params)
        
        if result.get("success"):
            return AlertmanagerGroupsResponse(
                success=True,
                groups=result.get("data", [])
            )
        else:
            return AlertmanagerGroupsResponse(
                success=False,
                error=result.get("error", "Unknown error")
            )


# Global service instance
alertmanager = AlertmanagerService()
