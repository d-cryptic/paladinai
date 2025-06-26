# Alertmanager Webhook Server

A simple Python webhook server for receiving and processing Alertmanager notifications.

## Features

- ✅ Receives Alertmanager webhook notifications
- ✅ Handles dead man's switch alerts
- ✅ Structured logging with detailed alert information
- ✅ Health check endpoint
- ✅ Environment-based configuration
- ✅ Easy setup with virtual environment

## Quick Start

1. **Start the webhook server:**
   ```bash
   cd infra
   ./start_webhook.sh
   ```

2. **The server will be available at:**
   - Main webhook: `http://127.0.0.1:5001/`
   - Dead man's switch: `http://127.0.0.1:5001/deadmansswitch`
   - Health check: `http://127.0.0.1:5001/health`

## Configuration

Copy `.env.example` to `.env` and modify as needed:

```bash
cp .env.example .env
```

Available environment variables:
- `WEBHOOK_HOST`: Server host (default: 127.0.0.1)
- `WEBHOOK_PORT`: Server port (default: 5001)
- `WEBHOOK_DEBUG`: Enable debug mode (default: false)

## Endpoints

### POST /
Main webhook endpoint for receiving Alertmanager notifications.

**Example payload:**
```json
{
  "receiver": "web.hook",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "HighMemoryUsage",
        "instance": "localhost:9090",
        "severity": "warning"
      },
      "annotations": {
        "summary": "High memory usage detected",
        "description": "Memory usage is above 80%"
      },
      "startsAt": "2023-01-01T12:00:00Z"
    }
  ]
}
```

### POST /deadmansswitch
Endpoint for dead man's switch notifications to ensure the monitoring system is alive.

### GET /health
Health check endpoint that returns server status.

### GET /
Returns basic server information and available endpoints.

## Alertmanager Configuration

The webhook server is configured to work with the Alertmanager configuration in `alertmanager.yml`:

```yaml
receivers:
- name: 'web.hook'
  webhook_configs:
  - url: 'http://127.0.0.1:5001/'
    send_resolved: true

- name: 'deadmansswitch'
  webhook_configs:
  - url: 'http://127.0.0.1:5001/deadmansswitch'
    send_resolved: false
```

## Extending the Webhook Server

You can extend the webhook server to:

1. **Send notifications to external services** (Slack, Discord, Teams)
2. **Store alerts in a database**
3. **Trigger automated responses**
4. **Filter and route alerts based on labels**

Example extension for Slack notifications:
```python
import requests

def send_to_slack(alert):
    slack_url = os.getenv('SLACK_WEBHOOK_URL')
    if slack_url:
        payload = {
            "text": format_alert(alert)
        }
        requests.post(slack_url, json=payload)
```

## Logs

The webhook server provides structured logging:
- Alert status and basic information
- Detailed alert labels and annotations
- Dead man's switch status
- Error handling and debugging information

## Dependencies

- Flask 3.0.0
- python-dotenv 1.0.0

## Manual Setup

If you prefer manual setup instead of using the startup script:

```bash
# Create virtual environment
python3 -m venv venv
source venv/bin/activate

# Install dependencies
pip install -r requirements.txt

# Copy environment file
cp .env.example .env

# Start server
python3 webhook_server.py
```
