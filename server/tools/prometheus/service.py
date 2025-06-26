"""
Prometheus service for Paladin AI
Handles Prometheus API calls for metrics querying and monitoring.
"""

import os
import aiohttp
from typing import Dict, List, Optional, Any
from dotenv import load_dotenv
from langfuse import observe

from .models import (
    PrometheusQueryRequest,
    PrometheusRangeQueryRequest,
    PrometheusQueryResponse,
    PrometheusTargetsResponse,
    PrometheusMetadataResponse,
    PrometheusLabelsResponse,
    PrometheusLabelValuesResponse,
)

# Load environment variables
load_dotenv()


class PrometheusService:
    """Service class for handling Prometheus API calls."""
    
    def __init__(self):
        """Initialize Prometheus client with environment configuration."""
        self.base_url = os.getenv("PROMETHEUS_URL")
        self.timeout = int(os.getenv("PROMETHEUS_TIMEOUT", "30"))
        self.auth_token = os.getenv("PROMETHEUS_AUTH_TOKEN")
        
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
        """Make HTTP request to Prometheus API."""
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
                    result = await response.json()
                    
                    if response.status == 200:
                        return {"success": True, **result}
                    else:
                        return {
                            "success": False,
                            "error": f"HTTP {response.status}: {result.get('error', 'Unknown error')}",
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
    
    @observe(name="prometheus_instant_query")
    async def query(self, request: PrometheusQueryRequest) -> PrometheusQueryResponse:
        """
        Execute an instant PromQL query.
        
        Args:
            request: Prometheus query request parameters
            
        Returns:
            PrometheusQueryResponse with query results
        """
        params = {"query": request.query}
        
        if request.time:
            params["time"] = request.time
        if request.timeout:
            params["timeout"] = request.timeout
            
        result = await self._make_request("GET", "/api/v1/query", params=params)
        
        return PrometheusQueryResponse(**result)
    
    @observe(name="prometheus_range_query")
    async def query_range(self, request: PrometheusRangeQueryRequest) -> PrometheusQueryResponse:
        """
        Execute a range PromQL query.
        
        Args:
            request: Prometheus range query request parameters
            
        Returns:
            PrometheusQueryResponse with query results
        """
        params = {
            "query": request.query,
            "start": request.start,
            "end": request.end,
            "step": request.step
        }
        
        if request.timeout:
            params["timeout"] = request.timeout
            
        result = await self._make_request("GET", "/api/v1/query_range", params=params)
        
        return PrometheusQueryResponse(**result)
    
    @observe(name="prometheus_get_targets")
    async def get_targets(self, state: Optional[str] = None) -> PrometheusTargetsResponse:
        """
        Get Prometheus scrape targets.
        
        Args:
            state: Filter targets by state (active, dropped, any)
            
        Returns:
            PrometheusTargetsResponse with target information
        """
        params = {}
        if state:
            params["state"] = state
            
        result = await self._make_request("GET", "/api/v1/targets", params=params)
        
        if result.get("success"):
            # Transform the response to match our model
            data = result.get("data", {})
            return PrometheusTargetsResponse(
                success=True,
                active_targets=data.get("activeTargets", []),
                dropped_targets=data.get("droppedTargets", [])
            )
        else:
            return PrometheusTargetsResponse(
                success=False,
                error=result.get("error", "Unknown error")
            )
    
    @observe(name="prometheus_get_metadata")
    async def get_metadata(self, metric: Optional[str] = None) -> PrometheusMetadataResponse:
        """
        Get metric metadata from Prometheus.
        
        Args:
            metric: Specific metric name to get metadata for
            
        Returns:
            PrometheusMetadataResponse with metadata information
        """
        params = {}
        if metric:
            params["metric"] = metric
            
        result = await self._make_request("GET", "/api/v1/metadata", params=params)
        
        if result.get("success"):
            return PrometheusMetadataResponse(
                success=True,
                metadata=result.get("data", {})
            )
        else:
            return PrometheusMetadataResponse(
                success=False,
                error=result.get("error", "Unknown error")
            )
    
    @observe(name="prometheus_get_labels")
    async def get_labels(
        self, 
        start: Optional[str] = None, 
        end: Optional[str] = None
    ) -> PrometheusLabelsResponse:
        """
        Get available label names.
        
        Args:
            start: Start timestamp for label discovery
            end: End timestamp for label discovery
            
        Returns:
            PrometheusLabelsResponse with available labels
        """
        params = {}
        if start:
            params["start"] = start
        if end:
            params["end"] = end
            
        result = await self._make_request("GET", "/api/v1/labels", params=params)
        
        if result.get("success"):
            return PrometheusLabelsResponse(
                success=True,
                labels=result.get("data", [])
            )
        else:
            return PrometheusLabelsResponse(
                success=False,
                error=result.get("error", "Unknown error")
            )
    
    @observe(name="prometheus_get_label_values")
    async def get_label_values(
        self, 
        label_name: str,
        start: Optional[str] = None,
        end: Optional[str] = None
    ) -> PrometheusLabelValuesResponse:
        """
        Get available values for a specific label.
        
        Args:
            label_name: Name of the label to get values for
            start: Start timestamp for label value discovery
            end: End timestamp for label value discovery
            
        Returns:
            PrometheusLabelValuesResponse with available label values
        """
        params = {}
        if start:
            params["start"] = start
        if end:
            params["end"] = end
            
        result = await self._make_request(
            "GET", 
            f"/api/v1/label/{label_name}/values", 
            params=params
        )
        
        if result.get("success"):
            return PrometheusLabelValuesResponse(
                success=True,
                values=result.get("data", [])
            )
        else:
            return PrometheusLabelValuesResponse(
                success=False,
                error=result.get("error", "Unknown error")
            )


# Global service instance
prometheus = PrometheusService()
