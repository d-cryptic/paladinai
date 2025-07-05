#!/usr/bin/env python3
"""
Simple Webhook Server using FastAPI
"""

from fastapi import FastAPI, Request, BackgroundTasks
from datetime import datetime
import logging
import uvicorn
import aiohttp
import os
from typing import Dict, Any
from pathlib import Path
from dotenv import load_dotenv

# Load environment variables
load_dotenv(Path(__file__).parent.parent / '.env')

app = FastAPI(title="Paladin Webhook Server")

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

async def send_to_paladin_alert_analysis(alert_data: Dict[str, Any]):
    """Send alert to Paladin server for analysis."""
    try:
        paladin_url = f"http://{os.getenv('SERVER_HOST', '127.0.0.1')}:{os.getenv('SERVER_PORT', '8000')}/api/v1/alert-analysis-mode"
        
        async with aiohttp.ClientSession() as session:
            async with session.post(
                paladin_url,
                json={
                    "alert_data": alert_data,
                    "source": "webhook",
                    "priority": alert_data.get("labels", {}).get("severity", "medium")
                },
                timeout=aiohttp.ClientTimeout(total=300)
            ) as response:
                if response.status == 200:
                    result = await response.json()
                    logger.info(f"Alert sent to Paladin for analysis: {result}")
                else:
                    error_text = await response.text()
                    logger.error(f"Failed to send alert to Paladin: {response.status} - {error_text}")
                    
    except Exception as e:
        logger.error(f"Error sending to Paladin: {e}")


def is_alert_notification(body: Dict[str, Any]) -> bool:
    """Check if the webhook is an alert notification."""
    # Check for Alertmanager format
    if 'alerts' in body and isinstance(body.get('alerts'), list):
        return True
    
    # Check for single alert format
    if 'alertname' in body or 'labels' in body:
        return True
    
    return False


@app.post("/webhook")
async def receive_webhook(request: Request, background_tasks: BackgroundTasks):
    """Receive webhook endpoint"""
    try:
        # Get request data
        body = await request.json()
        headers = dict(request.headers)
        
        webhook_data = {
            'timestamp': datetime.utcnow().isoformat(),
            'method': request.method,
            'url': str(request.url),
            'headers': headers,
            'body': body,
            'client': request.client.host if request.client else None
        }
        
        # Log webhook
        logger.info(f"Webhook received: {webhook_data}")
        
        # Check if this is an alert notification
        if is_alert_notification(body):
            logger.info("Alert notification detected, sending to Paladin for analysis")
            
            # If it's an Alertmanager webhook with multiple alerts
            if 'alerts' in body:
                for alert in body['alerts']:
                    # Add webhook metadata to each alert
                    alert_with_metadata = {
                        **alert,
                        'webhook_received_at': webhook_data['timestamp'],
                        'webhook_source': webhook_data['client'],
                        'group_key': body.get('groupKey'),
                        'receiver': body.get('receiver')
                    }
                    background_tasks.add_task(send_to_paladin_alert_analysis, alert_with_metadata)
            else:
                # Single alert format
                background_tasks.add_task(send_to_paladin_alert_analysis, body)
        
        return {
            'status': 'success',
            'message': 'Webhook received',
            'timestamp': webhook_data['timestamp'],
            'is_alert': is_alert_notification(body)
        }
        
    except Exception as e:
        logger.error(f"Error processing webhook: {str(e)}")
        return {
            'status': 'error',
            'message': str(e)
        }


@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {
        'status': 'healthy',
        'service': 'paladin-webhook-server',
        'timestamp': datetime.utcnow().isoformat()
    }


@app.get("/")
async def root():
    """Root endpoint showing service info."""
    return {
        'service': 'Paladin Webhook Server',
        'version': '1.0.0',
        'endpoints': {
            'webhook': '/webhook (POST) - Receive webhooks',
            'health': '/health (GET) - Health check'
        }
    }

if __name__ == '__main__':
    host = os.getenv('WEBHOOK_HOST', '0.0.0.0')
    port = int(os.getenv('WEBHOOK_PORT', '5001'))
    uvicorn.run(app, host=host, port=port)