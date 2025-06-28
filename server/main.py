"""
FastAPI Server for Paladin AI
AI-Powered Monitoring & Incident Response Platform
"""

import os
import uvicorn
from dotenv import load_dotenv
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from routes import health_router, chat_router
from middleware import ErrorHandlerMiddleware, RequestTimeoutMiddleware

# Load environment variables
load_dotenv()

app = FastAPI(
    title="Paladin AI Server",
    description="AI-Powered Monitoring & Incident Response Platform",
    version="0.1.0",
)

# Add error handling middleware (first to catch all errors)
app.add_middleware(ErrorHandlerMiddleware)

# Add request timeout middleware
timeout_seconds = float(os.getenv("REQUEST_TIMEOUT", "300"))
app.add_middleware(RequestTimeoutMiddleware, timeout=timeout_seconds)

# Add CORS middleware for CLI client communication
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # Configure this properly for production
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include routers
app.include_router(health_router)
app.include_router(chat_router)


if __name__ == "__main__":
    host = os.getenv("SERVER_HOST", "127.0.0.1")
    port = int(os.getenv("SERVER_PORT", "8000"))
    reload = os.getenv("RELOAD", "true").lower() == "true"
    log_level = os.getenv("LOG_LEVEL", "info")

    uvicorn.run(
        "main:app",
        host=host,
        port=port,
        reload=reload,
        log_level=log_level,
        # Better production settings
        access_log=True,
        server_header=False,
        date_header=False,
        # Connection settings
        limit_concurrency=100,
        limit_max_requests=1000,
        timeout_keep_alive=30,
        timeout_graceful_shutdown=30
    )
