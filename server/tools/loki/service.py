"""
Loki service for Paladin AI
Handles Loki API calls for log querying and analysis.
"""

import os
import aiohttp
from typing import Dict, List, Optional, Any, Union, Tuple
from dotenv import load_dotenv
from langfuse import observe

from .models import (
    LokiQueryRequest,
    LokiRangeQueryRequest,
    LokiQueryResponse,
    LokiLabelsResponse,
    LokiLabelValuesResponse,
    LokiSeriesRequest,
    LokiSeriesResponse,
    LokiTailRequest,
    LokiMetricsResponse,
)

# Load environment variables
load_dotenv()


class LokiService:
    """Service class for handling Loki API calls."""
    
    def __init__(self):
        """Initialize Loki client with environment configuration."""
        import logging
        self.logger = logging.getLogger(__name__)
        
        self.base_url = os.getenv("LOKI_URL")
        if not self.base_url:
            self.logger.warning("LOKI_URL environment variable not set")
            self.base_url = "http://localhost:3100"  # Default fallback
        
        self.timeout = int(os.getenv("LOKI_TIMEOUT", "30"))
        self.auth_token = os.getenv("LOKI_AUTH_TOKEN")
        
        # Remove trailing slash from base URL
        self.base_url = self.base_url.rstrip('/')
        
        self.logger.info(f"Initialized Loki client with URL: {self.base_url}")
        
        # Setup headers
        self.headers = {"Content-Type": "application/json"}
        if self.auth_token:
            self.headers["Authorization"] = f"Bearer {self.auth_token}"
    
    async def _make_request(
        self, 
        method: str, 
        endpoint: str, 
        params: Optional[Union[Dict[str, Any], List[Tuple[str, str]]]] = None,
        data: Optional[Dict[str, Any]] = None
    ) -> Dict[str, Any]:
        """Make HTTP request to Loki API."""
        url = f"{self.base_url}{endpoint}"
        
        import logging
        logger = logging.getLogger(__name__)
        logger.info(f"Making {method} request to {url} with params: {params}")
        
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
                    # Check content type
                    content_type = response.headers.get('Content-Type', '')
                    
                    if 'application/json' in content_type:
                        result = await response.json()
                    else:
                        # Handle non-JSON responses
                        text = await response.text()
                        logger.warning(f"Received non-JSON response (content-type: {content_type}): {text[:200]}")
                        
                        if response.status == 400:
                            # Parse error message from text response
                            return {
                                "success": False,
                                "status": "error",
                                "error": f"Bad Request: {text}",
                                "status_code": 400
                            }
                        else:
                            result = {"message": text}
                    
                    if response.status == 200:
                        logger.info(f"Request successful, got response with keys: {list(result.keys()) if isinstance(result, dict) else 'non-dict response'}")
                        return {"success": True, "status": "success", **result}
                    else:
                        logger.error(f"Request failed with status {response.status}: {result}")
                        error_msg = result.get('error', result.get('message', 'Unknown error')) if isinstance(result, dict) else str(result)
                        return {
                            "success": False,
                            "status": "error",
                            "error": f"HTTP {response.status}: {error_msg}",
                            "status_code": response.status
                        }
                        
        except aiohttp.ClientError as e:
            logger.error(f"Connection error to {url}: {str(e)}")
            return {
                "success": False,
                "status": "error",
                "error": f"Connection error: {str(e)}"
            }
        except Exception as e:
            return {
                "success": False,
                "status": "error",
                "error": f"Unexpected error: {str(e)}"
            }
    
    @observe(name="loki_instant_query")
    async def query(self, request: LokiQueryRequest) -> LokiQueryResponse:
        """
        Execute an instant LogQL query.
        
        Args:
            request: Loki query request parameters
            
        Returns:
            LokiQueryResponse with query results
        """
        params = {
            "query": request.query,
            "direction": request.direction,
            "limit": request.limit
        }
        
        if request.time:
            params["time"] = request.time
            
        result = await self._make_request("GET", "/loki/api/v1/query", params=params)

        # Parse the nested structure from Loki API
        if result.get("success") and "data" in result:
            data = result["data"]
            return LokiQueryResponse(
                success=True,
                status="success",
                data=data,
                result_type=data.get("resultType"),
                result=data.get("result", []),
                stats=result.get("stats")
            )
        else:
            return LokiQueryResponse(
                success=result.get("success", False),
                status=result.get("status", "error"),
                error=result.get("error", "Unknown error")
            )
    
    @observe(name="loki_range_query")
    async def query_range(self, request: LokiRangeQueryRequest) -> LokiQueryResponse:
        """
        Execute a range LogQL query.
        
        Args:
            request: Loki range query request parameters
            
        Returns:
            LokiQueryResponse with query results
        """
        params = {
            "query": request.query,
            "start": request.start,
            "end": request.end,
            "direction": request.direction,
            "limit": request.limit
        }
        
        if request.step:
            params["step"] = request.step
            
        result = await self._make_request("GET", "/loki/api/v1/query_range", params=params)
        
        # Parse the nested structure from Loki API
        if result.get("success") and "data" in result:
            data = result["data"]
            return LokiQueryResponse(
                success=True,
                status="success",
                data=data,
                result_type=data.get("resultType"),
                result=data.get("result", []),
                stats=result.get("stats")
            )
        else:
            return LokiQueryResponse(
                success=result.get("success", False),
                status=result.get("status", "error"),
                error=result.get("error", "Unknown error")
            )
    
    @observe(name="loki_get_labels")
    async def get_labels(
        self, 
        start: Optional[str] = None, 
        end: Optional[str] = None
    ) -> LokiLabelsResponse:
        """
        Get available label names.
        
        Args:
            start: Start timestamp for label discovery
            end: End timestamp for label discovery
            
        Returns:
            LokiLabelsResponse with available labels
        """
        params = {}
        if start:
            params["start"] = start
        if end:
            params["end"] = end
            
        result = await self._make_request("GET", "/loki/api/v1/labels", params=params)
        
        if result.get("success"):
            return LokiLabelsResponse(
                success=True,
                labels=result.get("data", [])
            )
        else:
            return LokiLabelsResponse(
                success=False,
                error=result.get("error", "Unknown error")
            )
    
    @observe(name="loki_get_label_values")
    async def get_label_values(
        self, 
        label_name: str,
        start: Optional[str] = None,
        end: Optional[str] = None
    ) -> LokiLabelValuesResponse:
        """
        Get available values for a specific label.
        
        Args:
            label_name: Name of the label to get values for
            start: Start timestamp for label value discovery
            end: End timestamp for label value discovery
            
        Returns:
            LokiLabelValuesResponse with available label values
        """
        params = {}
        if start:
            params["start"] = start
        if end:
            params["end"] = end
            
        result = await self._make_request(
            "GET", 
            f"/loki/api/v1/label/{label_name}/values", 
            params=params
        )
        
        if result.get("success"):
            return LokiLabelValuesResponse(
                success=True,
                values=result.get("data", [])
            )
        else:
            return LokiLabelValuesResponse(
                success=False,
                error=result.get("error", "Unknown error")
            )
    
    @observe(name="loki_get_series")
    async def get_series(self, request: LokiSeriesRequest) -> LokiSeriesResponse:
        """
        Get series information from Loki.
        
        Args:
            request: Series request parameters
            
        Returns:
            LokiSeriesResponse with series information
        """
        # Build params list for proper handling of array parameters
        params = []
        for match in request.match:
            params.append(("match[]", match))
        
        if request.start:
            params.append(("start", request.start))
        if request.end:
            params.append(("end", request.end))
            
        # Use params as list of tuples for aiohttp
        result = await self._make_request("GET", "/loki/api/v1/series", params=params)
        
        if result.get("success"):
            return LokiSeriesResponse(
                success=True,
                series=result.get("data", [])
            )
        else:
            return LokiSeriesResponse(
                success=False,
                error=result.get("error", "Unknown error")
            )
    
    @observe(name="loki_tail_logs")
    async def tail_logs(self, request: LokiTailRequest) -> LokiQueryResponse:
        """
        Tail logs in real-time from Loki.
        
        Args:
            request: Tail request parameters
            
        Returns:
            LokiQueryResponse with streaming log results
        """
        params = {
            "query": request.query,
            "delay_for": request.delay_for,
            "limit": request.limit
        }
        
        if request.start:
            params["start"] = request.start
            
        result = await self._make_request("GET", "/loki/api/v1/tail", params=params)
        
        return LokiQueryResponse(**result)
    
    @observe(name="loki_query_metrics")
    async def query_metrics(self, request: LokiRangeQueryRequest) -> LokiMetricsResponse:
        """
        Execute a LogQL metrics query.
        
        Args:
            request: Loki range query request for metrics
            
        Returns:
            LokiMetricsResponse with metrics results
        """
        params = {
            "query": request.query,
            "start": request.start,
            "end": request.end
        }
        
        if request.step:
            params["step"] = request.step
            
        result = await self._make_request("GET", "/loki/api/v1/query_range", params=params)
        
        return LokiMetricsResponse(**result)


# Global service instance
loki = LokiService()
