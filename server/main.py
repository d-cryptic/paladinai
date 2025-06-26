"""
FastAPI Server for Paladin AI
Simple hello world endpoint for testing server-client communication.
"""

import os

import uvicorn
from dotenv import load_dotenv
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

# Load environment variables
load_dotenv()

app = FastAPI(
    title="Paladin AI Server",
    description="AI-Powered Monitoring & Incident Response Platform",
    version="0.1.0",
)

# Add CORS middleware for CLI client communication
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # Configure this properly for production
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.get("/")
async def root() -> dict[str, str]:
    """Simple hello world endpoint"""
    return {"message": "Hello World from Paladin AI Server!"}


@app.get("/health")
async def health_check() -> dict[str, str]:
    """Health check endpoint"""
    return {"status": "healthy", "service": "paladin-ai-server"}


@app.get("/api/v1/status")
async def api_status() -> dict[str, str]:
    """API status endpoint for CLI client"""
    return {
        "api_version": "v1",
        "server_status": "running",
        "message": "Paladin AI Server is operational",
    }


if __name__ == "__main__":
    host = os.getenv("SERVER_HOST", "127.0.0.1")
    port = int(os.getenv("SERVER_PORT", "8000"))

    uvicorn.run(
        "main:app",
        host=host,
        port=port,
        reload=True,
        log_level="info"
    )
