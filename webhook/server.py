#!/usr/bin/env python3
"""
Simple Webhook Server using FastAPI
"""

from fastapi import FastAPI, Request
from datetime import datetime
import logging
import uvicorn

app = FastAPI(title="Paladin Webhook Server")

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

@app.post("/webhook")
async def receive_webhook(request: Request):
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
        
        return {
            'status': 'success',
            'message': 'Webhook received',
            'timestamp': webhook_data['timestamp']
        }
        
    except Exception as e:
        logger.error(f"Error processing webhook: {str(e)}")
        return {
            'status': 'error',
            'message': str(e)
        }

if __name__ == '__main__':
    uvicorn.run(app, host='0.0.0.0', port=5001)