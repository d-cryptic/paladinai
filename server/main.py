"""
FastAPI Server for Paladin AI
AI-Powered Monitoring & Incident Response Platform
"""

import os
import warnings
import uvicorn
import asyncio
from contextlib import asynccontextmanager
from dotenv import load_dotenv
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

# Load environment variables FIRST before any other imports
load_dotenv()

from routes import health_router, chat_router
from middleware import ErrorHandlerMiddleware, RequestTimeoutMiddleware
from memory.api import memory_router
from graph.workflow import workflow
from checkpointing import close_checkpointer
from checkpointing.routes import router as checkpoint_router

# Suppress Pydantic deprecation warning from LangGraph
# This is a third-party library issue that should be fixed in their code
warnings.filterwarnings(
    "ignore",
    category=DeprecationWarning,
    message=".*Accessing the 'model_fields' attribute on the instance is deprecated.*",
    module="langgraph.*"
)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan management."""
    # Startup
    print("Initializing Paladin AI Server...")
    try:
        # Initialize workflow with MongoDB checkpointing
        await workflow.initialize()
        print("MongoDB checkpointing initialized successfully")
        
        # Set workflow instance in checkpoint routes
        from checkpointing.routes import set_workflow
        set_workflow(workflow)
        print("Checkpoint routes configured")
    except Exception as e:
        print(f"Failed to initialize checkpointing: {e}")
        # Continue without checkpointing
    
    yield
    
    # Shutdown
    print("Shutting down Paladin AI Server...")
    await close_checkpointer()
    print("Cleanup completed")


app = FastAPI(
    title="Paladin AI Server",
    description="AI-Powered Monitoring & Incident Response Platform",
    version="0.1.0",
    lifespan=lifespan
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
app.include_router(memory_router)
app.include_router(checkpoint_router)


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
        # Use modern websockets implementation
        ws="wsproto",  # Alternative: "websockets-sansio" for Sans-I/O implementation
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
