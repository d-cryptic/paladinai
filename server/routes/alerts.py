"""Alert Analysis Routes"""

from fastapi import APIRouter, HTTPException, BackgroundTasks
from pydantic import BaseModel, Field
from typing import Dict, Any, Optional
from datetime import datetime
import aiohttp
import os

from graph.alert_workflow import alert_workflow

router = APIRouter(prefix="/api/v1", tags=["alerts"])


class AlertAnalysisRequest(BaseModel):
    """Request model for alert analysis."""
    alert_data: Dict[str, Any] = Field(..., description="Alert notification data from webhook")
    session_id: Optional[str] = Field(None, description="Session ID for tracking")
    source: str = Field("webhook", description="Source of the alert")
    priority: Optional[str] = Field("medium", description="Alert priority level")


class AlertAnalysisResponse(BaseModel):
    """Response model for alert analysis."""
    success: bool
    session_id: str
    status: str
    message: str
    analysis_id: Optional[str] = None
    discord_notification_sent: bool = False


@router.post("/alert-analysis-mode", response_model=AlertAnalysisResponse)
async def analyze_alert(
    request: AlertAnalysisRequest,
    background_tasks: BackgroundTasks
):
    """
    Analyze an alert received from the webhook server.
    
    This endpoint:
    1. Extracts context from the alert
    2. Runs iterative analysis using multiple tools
    3. Searches documentation and memories
    4. Generates a comprehensive report
    5. Sends the report to Discord
    """
    try:
        # Generate session ID if not provided
        session_id = request.session_id or f"alert_{datetime.now().strftime('%Y%m%d_%H%M%S')}"
        
        print(f"[ALERT ENDPOINT] Starting analysis for session: {session_id}")
        
        # Add metadata to alert data
        alert_data = {
            **request.alert_data,
            "received_at": datetime.now().isoformat(),
            "source": request.source,
            "priority": request.priority,
            "session_id": session_id
        }
        
        # Create graph config for this session
        config = {"configurable": {"thread_id": session_id}}
        
        # Run the alert analysis workflow
        result = await alert_workflow.invoke(alert_data, config)
        
        if result.get("success"):
            # Extract the report
            alert_report = result.get("alert_report", {})
            markdown_content = result.get("markdown_content", "")
            
            # Send to Discord in background
            background_tasks.add_task(
                send_to_discord,
                session_id=session_id,
                alert_report=alert_report,
                markdown_content=markdown_content
            )
            
            # Check if Discord is enabled
            discord_enabled = os.getenv('DISCORD_ENABLED', 'false').lower() == 'true'
            
            return AlertAnalysisResponse(
                success=True,
                session_id=session_id,
                status="completed",
                message="Alert analysis completed successfully",
                analysis_id=alert_report.get("alert_id"),
                discord_notification_sent=discord_enabled
            )
        else:
            return AlertAnalysisResponse(
                success=False,
                session_id=session_id,
                status="failed",
                message="Alert analysis failed",
                discord_notification_sent=False
            )
            
    except Exception as e:
        print(f"[ALERT ENDPOINT] Error analyzing alert: {e}")
        print(f"[ALERT ENDPOINT] Error type: {type(e).__name__}")
        import traceback
        traceback.print_exc()
        raise HTTPException(
            status_code=500,
            detail=f"Failed to analyze alert: {str(e)}"
        )


async def send_to_discord(session_id: str, alert_report: Dict[str, Any], markdown_content: str):
    """Send alert analysis report to Discord via MCP server."""
    # Check if Discord integration is enabled
    discord_enabled = os.getenv('DISCORD_ENABLED', 'false').lower() == 'true'
    if not discord_enabled:
        print(f"[DISCORD] Discord integration is disabled, skipping notification for session: {session_id}")
        return
        
    try:
        # Discord MCP server URL
        discord_url = f"http://{os.getenv('DISCORD_MCP_HOST', 'localhost')}:{os.getenv('DISCORD_MCP_PORT', '9000')}/send-alert-report"
        
        # Prepare payload
        payload = {
            "session_id": session_id,
            "alert_id": alert_report.get("alert_id"),
            "severity": alert_report.get("severity"),
            "markdown_content": markdown_content,
            "generated_at": alert_report.get("generated_at"),
            "confidence_score": alert_report.get("confidence_score", 0),
            "channel": "alerts",  # Target channel
            "format": "pdf"  # Request PDF format
        }
        
        print(f"[DISCORD] Attempting to send alert report to: {discord_url}")
        
        async with aiohttp.ClientSession() as session:
            async with session.post(
                discord_url,
                json=payload,
                timeout=aiohttp.ClientTimeout(total=10)  # Reduced timeout
            ) as response:
                if response.status == 200:
                    result = await response.json()
                    print(f"[DISCORD] Alert report sent successfully: {result}")
                else:
                    error_text = await response.text()
                    print(f"[DISCORD] Failed to send alert report: {response.status} - {error_text}")
                    
    except aiohttp.ClientConnectorError as e:
        print(f"[DISCORD] Cannot connect to Discord MCP server at {discord_url}. Is the Discord MCP server running?")
        print(f"[DISCORD] To enable Discord notifications, start the Discord MCP server or set DISCORD_ENABLED=false")
    except Exception as e:
        print(f"[DISCORD] Error sending to Discord: {e}")
        # Don't raise - this is a background task


@router.post("/alert-analysis-mode/stream")
async def analyze_alert_stream(request: AlertAnalysisRequest):
    """
    Stream the alert analysis workflow execution.
    
    Returns server-sent events with real-time updates.
    """
    from fastapi.responses import StreamingResponse
    import json
    
    async def event_generator():
        try:
            # Generate session ID
            session_id = request.session_id or f"alert_{datetime.now().strftime('%Y%m%d_%H%M%S')}"
            
            # Add metadata
            alert_data = {
                **request.alert_data,
                "received_at": datetime.now().isoformat(),
                "source": request.source,
                "priority": request.priority,
                "session_id": session_id
            }
            
            # Create config
            config = {"configurable": {"thread_id": session_id}}
            
            # Stream workflow events
            async for event in alert_workflow.stream(alert_data, config):
                # Format as SSE
                event_data = {
                    "session_id": session_id,
                    "event": event
                }
                yield f"data: {json.dumps(event_data)}\n\n"
                
            # Send completion event
            yield f"data: {json.dumps({'session_id': session_id, 'status': 'completed'})}\n\n"
            
        except Exception as e:
            error_event = {
                "session_id": session_id,
                "status": "error",
                "error": str(e)
            }
            yield f"data: {json.dumps(error_event)}\n\n"
    
    return StreamingResponse(
        event_generator(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no"
        }
    )