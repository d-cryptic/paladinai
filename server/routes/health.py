"""
Health check and status routes for Paladin AI Server.
Contains endpoints for monitoring server health and API status.
"""

from fastapi import APIRouter

router = APIRouter()


@router.get("/")
async def root() -> dict[str, str]:
    """Simple hello world endpoint"""
    return {"message": "Hello World from Paladin AI Server!"}


@router.get("/health")
async def health_check() -> dict[str, str]:
    """Health check endpoint"""
    return {"status": "healthy", "service": "paladin-ai-server"}


@router.get("/api/v1/status")
async def api_status() -> dict[str, str]:
    """API status endpoint for CLI client"""
    return {
        "api_version": "v1",
        "server_status": "running",
        "message": "Paladin AI Server is operational",
    }
