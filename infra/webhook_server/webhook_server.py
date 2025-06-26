#!/usr/bin/env python3
"""
Webhook server for receiving Alertmanager notifications.
Handles alerts and dead man's switch notifications.
"""

import json
import logging
from datetime import datetime
from flask import Flask, request, jsonify
from typing import Dict, List, Any
import os
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

app = Flask(__name__)

def format_alert(alert: Dict[str, Any]) -> str:
    """Format an alert for logging/display."""
    status = alert.get('status', 'unknown')
    alertname = alert.get('labels', {}).get('alertname', 'Unknown')
    instance = alert.get('labels', {}).get('instance', 'Unknown')
    severity = alert.get('labels', {}).get('severity', 'Unknown')
    
    starts_at = alert.get('startsAt', '')
    ends_at = alert.get('endsAt', '')
    
    message = f"[{status.upper()}] {alertname} on {instance} (severity: {severity})"
    
    if starts_at:
        message += f" - Started: {starts_at}"
    if ends_at and status == 'resolved':
        message += f" - Ended: {ends_at}"
    
    annotations = alert.get('annotations', {})
    if 'summary' in annotations:
        message += f" - {annotations['summary']}"
    if 'description' in annotations:
        message += f" - {annotations['description']}"
    
    return message

def log_alert_details(alert: Dict[str, Any]) -> None:
    """Log detailed alert information."""
    logger.info("Alert Details:")
    logger.info(f"  Status: {alert.get('status', 'unknown')}")
    logger.info(f"  Labels: {json.dumps(alert.get('labels', {}), indent=2)}")
    logger.info(f"  Annotations: {json.dumps(alert.get('annotations', {}), indent=2)}")
    logger.info(f"  Starts At: {alert.get('startsAt', 'N/A')}")
    logger.info(f"  Ends At: {alert.get('endsAt', 'N/A')}")
    logger.info(f"  Generator URL: {alert.get('generatorURL', 'N/A')}")

@app.route('/', methods=['POST'])
def webhook():
    """Main webhook endpoint for receiving alerts."""
    try:
        data = request.get_json()
        
        if not data:
            logger.warning("Received empty webhook payload")
            return jsonify({'status': 'error', 'message': 'Empty payload'}), 400
        
        logger.info(f"Received webhook with {len(data.get('alerts', []))} alerts")
        
        # Log general information about the webhook
        logger.info(f"Webhook Status: {data.get('status', 'unknown')}")
        logger.info(f"Receiver: {data.get('receiver', 'unknown')}")
        logger.info(f"Group Key: {data.get('groupKey', 'unknown')}")
        
        # Process each alert
        alerts = data.get('alerts', [])
        for i, alert in enumerate(alerts, 1):
            logger.info(f"--- Alert {i}/{len(alerts)} ---")
            logger.info(format_alert(alert))
            
            # Log detailed information in debug mode
            if logger.isEnabledFor(logging.DEBUG):
                log_alert_details(alert)
        
        # Here you can add custom logic for handling alerts:
        # - Send to external systems (Slack, Discord, etc.)
        # - Store in database
        # - Trigger automated responses
        # - etc.
        
        return jsonify({'status': 'success', 'processed_alerts': len(alerts)})
        
    except Exception as e:
        logger.error(f"Error processing webhook: {str(e)}")
        return jsonify({'status': 'error', 'message': str(e)}), 500

@app.route('/deadmansswitch', methods=['POST'])
def deadmansswitch():
    """Endpoint for dead man's switch notifications."""
    try:
        data = request.get_json()
        
        if not data:
            logger.warning("Received empty dead man's switch payload")
            return jsonify({'status': 'error', 'message': 'Empty payload'}), 400
        
        alerts = data.get('alerts', [])
        logger.info(f"Dead Man's Switch: Received {len(alerts)} alerts")
        
        for alert in alerts:
            status = alert.get('status', 'unknown')
            if status == 'firing':
                logger.info("✓ Dead Man's Switch: System is alive")
            else:
                logger.warning(f"⚠ Dead Man's Switch: Unexpected status - {status}")
        
        return jsonify({'status': 'success', 'type': 'deadmansswitch'})
        
    except Exception as e:
        logger.error(f"Error processing dead man's switch: {str(e)}")
        return jsonify({'status': 'error', 'message': str(e)}), 500

@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint."""
    return jsonify({
        'status': 'healthy',
        'timestamp': datetime.utcnow().isoformat(),
        'service': 'alertmanager-webhook'
    })

@app.route('/', methods=['GET'])
def index():
    """Simple index page showing webhook status."""
    return jsonify({
        'service': 'Alertmanager Webhook Server',
        'status': 'running',
        'endpoints': {
            'webhook': '/  (POST)',
            'deadmansswitch': '/deadmansswitch  (POST)',
            'health': '/health  (GET)'
        }
    })

if __name__ == '__main__':
    # Configuration from environment variables
    host = os.getenv('WEBHOOK_HOST', '127.0.0.1')
    port = int(os.getenv('WEBHOOK_PORT', '5001'))
    debug = os.getenv('WEBHOOK_DEBUG', 'false').lower() == 'true'
    
    logger.info(f"Starting Alertmanager webhook server on {host}:{port}")
    logger.info(f"Debug mode: {debug}")
    
    app.run(host=host, port=port, debug=debug)
