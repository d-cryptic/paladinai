"""
Error handling middleware for Paladin AI Server.
Provides comprehensive error handling and logging for FastAPI applications.
"""

import logging
import traceback
from typing import Callable, Any
from fastapi import Request, Response, HTTPException
from fastapi.responses import JSONResponse
from starlette.middleware.base import BaseHTTPMiddleware
import asyncio

logger = logging.getLogger(__name__)


class ErrorHandlerMiddleware(BaseHTTPMiddleware):
    """
    Middleware to handle errors and provide consistent error responses.
    
    This middleware catches unhandled exceptions and formats them
    into consistent JSON error responses.
    """

    async def dispatch(self, request: Request, call_next: Callable) -> Response:
        """
        Process request and handle any errors that occur.
        
        Args:
            request: FastAPI request object
            call_next: Next middleware/handler in the chain
            
        Returns:
            Response object
        """
        try:
            # Set a timeout for the entire request
            import os
            timeout_seconds = float(os.getenv("REQUEST_TIMEOUT", "300"))
            response = await asyncio.wait_for(
                call_next(request),
                timeout=timeout_seconds  # Use configurable timeout from environment
            )
            return response
            
        except asyncio.TimeoutError:
            logger.error(f"Request timeout for {request.url.path}")
            return JSONResponse(
                status_code=504,
                content={
                    "success": False,
                    "error": "Request timeout - the operation took too long to complete",
                    "error_type": "timeout_error",
                    "path": str(request.url.path)
                }
            )
            
        except HTTPException as e:
            # Re-raise HTTP exceptions to be handled by FastAPI
            raise e
            
        except ConnectionResetError as e:
            logger.error(f"Connection reset error for {request.url.path}: {str(e)}")
            return JSONResponse(
                status_code=503,
                content={
                    "success": False,
                    "error": "Connection was reset - please try again",
                    "error_type": "connection_reset",
                    "path": str(request.url.path)
                }
            )
            
        except Exception as e:
            # Log the full traceback for debugging
            logger.error(f"Unhandled error for {request.url.path}: {str(e)}")
            logger.error(f"Traceback: {traceback.format_exc()}")
            
            # Return a generic error response
            return JSONResponse(
                status_code=500,
                content={
                    "success": False,
                    "error": "An internal server error occurred",
                    "error_type": "internal_error",
                    "path": str(request.url.path),
                    "details": str(e) if logger.isEnabledFor(logging.DEBUG) else None
                }
            )


class RequestTimeoutMiddleware(BaseHTTPMiddleware):
    """
    Middleware to add request timeout handling.
    
    This middleware ensures that long-running requests don't hang indefinitely.
    """
    
    def __init__(self, app: Any, timeout: float = 300.0):
        """
        Initialize the timeout middleware.
        
        Args:
            app: FastAPI application
            timeout: Request timeout in seconds
        """
        super().__init__(app)
        self.timeout = timeout

    async def dispatch(self, request: Request, call_next: Callable) -> Response:
        """
        Process request with timeout handling.
        
        Args:
            request: FastAPI request object
            call_next: Next middleware/handler in the chain
            
        Returns:
            Response object
        """
        try:
            return await asyncio.wait_for(call_next(request), timeout=self.timeout)
        except asyncio.TimeoutError:
            logger.warning(f"Request timeout ({self.timeout}s) for {request.url.path}")
            return JSONResponse(
                status_code=504,
                content={
                    "success": False,
                    "error": f"Request timeout after {self.timeout} seconds",
                    "error_type": "timeout_error",
                    "timeout": self.timeout,
                    "path": str(request.url.path)
                }
            )
