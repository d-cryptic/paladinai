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

# Include routers
app.include_router(health_router)
app.include_router(chat_router)


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
