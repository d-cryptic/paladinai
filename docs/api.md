# 📡 API Reference Documentation

Comprehensive REST API documentation for the Paladin AI server.

## API Overview

- **Base URL**: `http://localhost:8000`
- **API Version**: v1
- **Framework**: FastAPI 0.115.14+
- **Authentication**: None (currently)
- **Content-Type**: `application/json`
- **Documentation**: Available at `/docs` (Swagger UI) and `/redoc` (ReDoc)

## Server Configuration

### Connection Details
- **Default Host**: 127.0.0.1:8000
- **Request Timeout**: 300 seconds (configurable)
- **Concurrent Connections**: 100
- **Max Requests**: 1000
- **CORS**: Enabled for all origins

### Response Format
All API responses follow a consistent format:

```json
{
  "success": true,
  "message": "Optional message",
  "data": {}, // Response data
  "metadata": {}, // Optional metadata
  "error": "Error message if success=false"
}
```

## Health & Status Endpoints

### GET /
Simple hello world endpoint for basic connectivity testing.

**Response:**
```json
{
  "message": "Hello World from Paladin AI Server!"
}
```

### GET /health
Health check endpoint for monitoring and load balancers.

**Response:**
```json
{
  "status": "healthy",
  "service": "paladin-ai-server"
}
```

### GET /api/v1/status
API status endpoint with version information.

**Response:**
```json
{
  "api_version": "v1",
  "server_status": "running",
  "message": "Paladin AI Server is operational"
}
```

## Chat & Workflow Endpoints

### POST /api/v1/chat
Execute LangGraph workflows for AI-powered conversations.

**Request Body:**
```json
{
  "message": "string (required)",
  "system_prompt": "string (optional)",
  "model": "string (optional, default: gpt-4)",
  "max_tokens": "integer (optional)",
  "temperature": "float (optional, 0.0-1.0)",
  "additional_context": {
    "session_id": "string (optional)"
  }
}
```

**Response:**
```json
{
  "success": true,
  "session_id": "string",
  "content": "string (formatted markdown response)",
  "metadata": {
    "workflow_type": "QUERY|INCIDENT|ACTION",
    "execution_time_ms": "integer",
    "execution_path": ["array of workflow nodes"]
  },
  "raw_result": "object (full workflow result)"
}
```

**Example Request:**
```bash
curl -X POST "http://localhost:8000/api/v1/chat" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Check CPU usage on web servers",
    "additional_context": {
      "session_id": "chat_123456"
    }
  }'
```

### POST /api/v1/chat/stream
Execute LangGraph workflows with streaming responses.

**Request Body:** Same as `/api/v1/chat`

**Response:** Server-Sent Events (SSE) stream
- **Content-Type**: `text/plain; charset=utf-8`
- **Headers**: 
  - `Cache-Control: no-cache`
  - `Connection: keep-alive`

**Example (JavaScript):**
```javascript
const eventSource = new EventSource('/api/v1/chat/stream');
eventSource.onmessage = function(event) {
  console.log('Received:', event.data);
};
```

## Document Management Endpoints

### POST /api/v1/documents/upload
Upload and process documents through the RAG pipeline.

**Request:** Multipart form data
- **file**: PDF or Markdown file (max 50MB)

**Response:**
```json
{
  "document_id": "string",
  "status": "pending|processing|completed|failed",
  "message": "string",
  "chunks_created": "integer (optional)",
  "collection_name": "string"
}
```

**Example Request:**
```bash
curl -X POST "http://localhost:8000/api/v1/documents/upload" \
  -F "file=@/path/to/document.pdf"
```

### POST /api/v1/documents/search
Search for relevant document chunks using vector similarity.

**Query Parameters:**
- `query`: string (required) - Search query
- `limit`: integer (optional, default: 5) - Maximum results
- `source_filter`: string (optional) - Filter by document source
- `document_type_filter`: string (optional) - Filter by document type

**Response:**
```json
{
  "query": "string",
  "results": [
    {
      "content": "string",
      "metadata": {
        "source": "string",
        "page": "integer",
        "chunk_id": "string"
      },
      "score": "float"
    }
  ],
  "count": "integer"
}
```

**Example Request:**
```bash
curl -X POST "http://localhost:8000/api/v1/documents/search?query=kubernetes troubleshooting&limit=10"
```

### GET /api/v1/documents/health
Check document service health and Qdrant connectivity.

**Response:**
```json
{
  "status": "healthy|unhealthy",
  "qdrant_connected": true,
  "collection": "string",
  "collections_count": "integer"
}
```

## Alert Analysis Endpoints

### POST /api/v1/alert-analysis-mode
Analyze alerts from webhook notifications using AI workflows.

**Request Body:**
```json
{
  "alert_data": "object (alert notification data)",
  "session_id": "string (optional)",
  "source": "string (default: 'webhook')",
  "priority": "string (optional, default: 'medium')"
}
```

**Response:**
```json
{
  "success": true,
  "session_id": "string",
  "status": "string",
  "message": "string",
  "analysis_id": "string (optional)",
  "discord_notification_sent": "boolean"
}
```

**Example Request:**
```bash
curl -X POST "http://localhost:8000/api/v1/alert-analysis-mode" \
  -H "Content-Type: application/json" \
  -d '{
    "alert_data": {
      "alertname": "HighCPUUsage",
      "severity": "warning",
      "instance": "web-server-01"
    },
    "priority": "high"
  }'
```

### POST /api/v1/alert-analysis-mode/stream
Stream alert analysis workflow execution.

**Request Body:** Same as `/api/v1/alert-analysis-mode`

**Response:** Server-Sent Events (SSE) stream
- **Content-Type**: `text/event-stream`

## Memory Management Endpoints

### POST /api/memory/instruction
Store explicit instructions as memory for future reference.

**Request Body:**
```json
{
  "instruction": "string (required)",
  "user_id": "string (optional)",
  "context": "object (optional)"
}
```

**Response:**
```json
{
  "success": true,
  "memory_id": "string",
  "relationships_count": "integer",
  "memory_type": "string",
  "error": "string (optional)"
}
```

**Example Request:**
```bash
curl -X POST "http://localhost:8000/api/memory/instruction" \
  -H "Content-Type: application/json" \
  -d '{
    "instruction": "Always check disk space before deploying new services",
    "user_id": "admin",
    "context": {"domain": "deployment"}
  }'
```

### POST /api/memory/search
Search memories using vector similarity.

**Request Body:**
```json
{
  "query": "string (required)",
  "memory_types": ["array of strings (optional)"],
  "user_id": "string (optional)",
  "limit": "integer (default: 10)",
  "confidence_threshold": "float (default: 0.5)"
}
```

**Response:**
```json
{
  "success": true,
  "total_results": "integer",
  "memories": [
    {
      "id": "string",
      "content": "string",
      "memory_type": "string",
      "confidence": "float",
      "metadata": "object"
    }
  ],
  "query": "string",
  "error": "string (optional)"
}
```

### GET /api/memory/contextual
Get contextually relevant memories for workflow execution.

**Query Parameters:**
- `context`: string (required) - Context description
- `workflow_type`: string (required) - QUERY, ACTION, or INCIDENT
- `limit`: integer (default: 5) - Maximum results

**Response:**
```json
{
  "success": true,
  "memories": [
    {
      "id": "string",
      "content": "string",
      "memory_type": "string",
      "relevance_score": "float"
    }
  ],
  "context": "string",
  "workflow_type": "string",
  "error": "string (optional)"
}
```

### POST /api/memory/extract
Extract and store memories from workflow interactions.

**Request Body:**
```json
{
  "content": "string (required)",
  "user_input": "string (required)",
  "workflow_type": "string (required)",
  "user_id": "string (optional)",
  "session_id": "string (optional)",
  "context": "object (optional)"
}
```

**Response:**
```json
{
  "success": true,
  "memories_extracted": "integer",
  "memory_ids": ["array of strings"],
  "error": "string (optional)"
}
```

### GET /api/memory/types
Get available memory types and their descriptions.

**Response:**
```json
{
  "memory_types": "Dynamic - types are created based on content",
  "examples": ["instruction", "pattern", "preference", "knowledge", "operational"],
  "descriptions": {
    "instruction": "Explicit user instructions",
    "pattern": "Learned behavioral patterns",
    "preference": "User preferences and settings",
    "knowledge": "Domain-specific knowledge",
    "operational": "Operational procedures and workflows"
  }
}
```

### GET /api/memory/health
Check memory service health and backend connectivity.

**Response:**
```json
{
  "status": "healthy|unhealthy",
  "backends": {
    "mem0": "connected",
    "qdrant": "connected",
    "neo4j": "connected"
  },
  "search_test": "boolean"
}
```

## Checkpoint Management Endpoints

### GET /api/v1/checkpoints/{session_id}
Retrieve the latest checkpoint for a session.

**Path Parameters:**
- `session_id`: string (required) - Session identifier

**Response:**
```json
{
  "success": true,
  "message": "string",
  "data": {
    "session_id": "string",
    "checkpoint_id": "string",
    "timestamp": "string (ISO 8601)",
    "workflow_state": "object",
    "metadata": "object"
  }
}
```

### GET /api/v1/checkpoints/{session_id}/exists
Check if a checkpoint exists for a session.

**Path Parameters:**
- `session_id`: string (required) - Session identifier

**Response:**
```json
{
  "exists": "boolean",
  "session_id": "string"
}
```

### GET /api/v1/checkpoints/
List available checkpoints with optional filtering.

**Query Parameters:**
- `session_id`: string (optional) - Filter by session ID
- `limit`: integer (default: 10, max: 100) - Maximum results

**Response:**
```json
{
  "success": true,
  "message": "string",
  "data": {
    "checkpoints": [
      {
        "session_id": "string",
        "checkpoint_id": "string",
        "timestamp": "string (ISO 8601)",
        "workflow_type": "string"
      }
    ],
    "count": "integer"
  }
}
```

### DELETE /api/v1/checkpoints/{session_id}
Delete all checkpoints for a session.

**Path Parameters:**
- `session_id`: string (required) - Session identifier

**Response:**
```json
{
  "success": true,
  "message": "Checkpoints deleted successfully",
  "data": {
    "deleted_count": "integer"
  }
}
```

## Error Handling

### Standard Error Response
```json
{
  "success": false,
  "error": "string (error message)",
  "error_type": "string (error type)",
  "path": "string (request path)",
  "details": "string (optional debug details)"
}
```

### HTTP Status Codes

| Status Code | Description | When Used |
|-------------|-------------|-----------|
| 200 | OK | Successful request |
| 400 | Bad Request | Invalid request format or parameters |
| 404 | Not Found | Resource not found |
| 413 | Request Entity Too Large | File upload exceeds size limit |
| 422 | Unprocessable Entity | Validation error |
| 500 | Internal Server Error | Server-side error |
| 503 | Service Unavailable | External service unavailable |
| 504 | Gateway Timeout | Request timeout exceeded |

### Common Error Types

1. **ValidationError**: Invalid request parameters
2. **AuthenticationError**: Authentication required or failed
3. **AuthorizationError**: Insufficient permissions
4. **ResourceNotFoundError**: Requested resource not found
5. **ExternalServiceError**: External service unavailable
6. **TimeoutError**: Request timeout exceeded
7. **RateLimitError**: Rate limit exceeded

## API Client Examples

### Python Client
```python
import requests
import json

class PaladinClient:
    def __init__(self, base_url="http://localhost:8000"):
        self.base_url = base_url
        
    def chat(self, message, session_id=None):
        url = f"{self.base_url}/api/v1/chat"
        data = {
            "message": message,
            "additional_context": {"session_id": session_id} if session_id else {}
        }
        response = requests.post(url, json=data)
        return response.json()
    
    def upload_document(self, file_path):
        url = f"{self.base_url}/api/v1/documents/upload"
        with open(file_path, 'rb') as f:
            files = {'file': f}
            response = requests.post(url, files=files)
        return response.json()
    
    def search_documents(self, query, limit=5):
        url = f"{self.base_url}/api/v1/documents/search"
        params = {"query": query, "limit": limit}
        response = requests.post(url, params=params)
        return response.json()

# Usage
client = PaladinClient()
result = client.chat("Check server status", session_id="my_session")
print(result)
```

### JavaScript Client
```javascript
class PaladinClient {
    constructor(baseUrl = 'http://localhost:8000') {
        this.baseUrl = baseUrl;
    }
    
    async chat(message, sessionId = null) {
        const url = `${this.baseUrl}/api/v1/chat`;
        const data = {
            message: message,
            additional_context: sessionId ? { session_id: sessionId } : {}
        };
        
        const response = await fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        
        return await response.json();
    }
    
    async uploadDocument(file) {
        const url = `${this.baseUrl}/api/v1/documents/upload`;
        const formData = new FormData();
        formData.append('file', file);
        
        const response = await fetch(url, {
            method: 'POST',
            body: formData
        });
        
        return await response.json();
    }
    
    async searchDocuments(query, limit = 5) {
        const url = `${this.baseUrl}/api/v1/documents/search`;
        const params = new URLSearchParams({ query, limit });
        
        const response = await fetch(`${url}?${params}`, {
            method: 'POST'
        });
        
        return await response.json();
    }
}

// Usage
const client = new PaladinClient();
client.chat('Check CPU usage').then(result => console.log(result));
```

### cURL Examples
```bash
# Chat request
curl -X POST "http://localhost:8000/api/v1/chat" \
  -H "Content-Type: application/json" \
  -d '{"message": "Check server status"}'

# Document upload
curl -X POST "http://localhost:8000/api/v1/documents/upload" \
  -F "file=@document.pdf"

# Document search
curl -X POST "http://localhost:8000/api/v1/documents/search?query=kubernetes&limit=5"

# Memory instruction
curl -X POST "http://localhost:8000/api/memory/instruction" \
  -H "Content-Type: application/json" \
  -d '{"instruction": "Always check logs before restarting services"}'

# Health check
curl -X GET "http://localhost:8000/health"
```

## Rate Limiting & Performance

### Current Limitations
- **No rate limiting** currently implemented
- **No authentication** required
- **No request queuing** for high loads
- **Connection limits**: 100 concurrent, 1000 max requests

### Recommended Usage
- **Chat requests**: Max 10 requests/minute per session
- **Document uploads**: Max 5 files/minute
- **Memory operations**: Max 50 requests/minute
- **Bulk operations**: Use appropriate batch sizes

## Environment Configuration

### Required Environment Variables
```bash
# OpenAI API
OPENAI_API_KEY=your-openai-api-key

# Database connections
MONGODB_URL=mongodb://localhost:27017/paladin
QDRANT_URL=http://localhost:6333
NEO4J_URL=bolt://localhost:7687

# Server configuration
SERVER_HOST=127.0.0.1
SERVER_PORT=8000
REQUEST_TIMEOUT=300
```

### Optional Environment Variables
```bash
# Discord integration
DISCORD_ENABLED=false
DISCORD_MCP_HOST=localhost
DISCORD_MCP_PORT=3001

# Development
RELOAD=true
LOG_LEVEL=info
DEBUG=false
```

## API Versioning

### Current Version
- **API Version**: v1
- **Base Path**: `/api/v1/`
- **Backward Compatibility**: Maintained for major version

### Future Versions
- **v2**: Planned with authentication and enhanced features
- **Deprecation Policy**: 6-month notice for breaking changes
- **Migration Path**: Automatic migration tools provided

## Security Considerations

### Current Security Status
- **Authentication**: None implemented
- **Authorization**: None implemented
- **HTTPS**: Not enforced (development mode)
- **Input Validation**: Basic validation via Pydantic models
- **Rate Limiting**: None implemented

### Recommended Security Measures
1. **Add API key authentication**
2. **Implement rate limiting**
3. **Enable HTTPS in production**
4. **Add request logging and monitoring**
5. **Implement proper error handling**
6. **Add input sanitization**

## Monitoring & Observability

### Built-in Monitoring
- **Health endpoints**: `/health`, `/api/v1/status`
- **Service health checks**: Individual service health endpoints
- **Request logging**: Basic request/response logging
- **Error tracking**: Structured error responses

### Recommended Monitoring
- **Metrics**: Request rate, response time, error rate
- **Logging**: Request logs, error logs, audit logs
- **Tracing**: Distributed tracing for complex workflows
- **Alerting**: Health check failures, high error rates

---

This API documentation provides comprehensive information for integrating with the Paladin AI platform. For additional support, see the [troubleshooting guide](troubleshooting.md) or visit our [GitHub repository](https://github.com/your-org/paladin-ai).