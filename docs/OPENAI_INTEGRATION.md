# OpenAI Integration Guide

This document explains how to use the OpenAI integration in both the Paladin AI server and CLI components.

## Setup

### 1. Environment Configuration

Copy the example environment file and configure your OpenAI settings:

```bash
cp .env.example .env
```

Edit `.env` and set your OpenAI API key:

```env
OPENAI_API_KEY=your_openai_api_key_here
OPENAI_MODEL=gpt-4o
OPENAI_MAX_TOKENS=4000
OPENAI_TEMPERATURE=0.1
```

### 2. Install Dependencies

For the server:
```bash
cd server
uv sync
```

For the CLI:
```bash
cd cli
uv sync
```

## Server Usage

### Starting the Server

```bash
cd server
uv run python main.py
```

The server will start on `http://127.0.0.1:8000` by default.

### API Endpoints

#### 1. Chat Completion
**POST** `/api/v1/chat`

Send a message to OpenAI with optional context.

```json
{
  "message": "What is the current system status?",
  "system_prompt": "Optional custom system prompt",
  "model": "gpt-4o",
  "max_tokens": 4000,
  "temperature": 0.1,
  "additional_context": {
    "environment": "production",
    "service": "api-gateway"
  }
}
```

#### 2. Incident Analysis
**POST** `/api/v1/analyze/incident`

Analyze an incident with specialized context.

```json
{
  "incident_data": {
    "error": "500 Internal Server Error",
    "timestamp": "2024-01-01T12:00:00Z",
    "service": "api-gateway",
    "affected_users": 1500
  },
  "additional_logs": [
    "2024-01-01T12:00:00Z ERROR Database connection timeout",
    "2024-01-01T12:00:01Z ERROR Retry attempt failed"
  ],
  "metrics": {
    "cpu_usage": 85,
    "memory_usage": 70,
    "error_rate": 15.5
  }
}
```

#### 3. Query Analysis
**POST** `/api/v1/analyze/query`

Analyze a general query with optional context.

```json
{
  "query": "Check the health of our microservices",
  "context_data": {
    "environment": "staging",
    "region": "us-west-2"
  }
}
```

## CLI Usage

### Basic Commands

#### 1. Chat via Server
Send a message to OpenAI through the server:

```bash
cd cli
uv run python main.py --chat "What is the system status?"
```

#### 2. Direct Chat
Send a message directly to OpenAI (bypassing server):

```bash
uv run python main.py --direct-chat "Analyze this error message"
```

#### 3. Incident Analysis
Analyze incident data via server:

```bash
uv run python main.py --analyze-incident '{"error": "Database timeout", "timestamp": "2024-01-01T12:00:00Z"}'
```

#### 4. With Additional Context
Include additional context in your requests:

```bash
uv run python main.py --chat "Check system health" --context '{"environment": "production", "region": "us-east-1"}'
```

### Example Commands

```bash
# Basic health check
uv run python main.py --chat "What is the current system status?"

# Incident analysis with context
uv run python main.py --analyze-incident '{"error": "High CPU usage", "service": "api-gateway", "timestamp": "2024-01-01T12:00:00Z"}' --context '{"environment": "production"}'

# Direct OpenAI call for quick analysis
uv run python main.py --direct-chat "Explain this error: Connection refused on port 5432"

# Complex query with metrics context
uv run python main.py --chat "Analyze system performance" --context '{"metrics": {"cpu": 85, "memory": 70, "disk": 45}, "alerts": ["High CPU", "Memory Warning"]}'
```

## System Prompts

The OpenAI integration uses sophisticated system prompts located in `prompts/system/`. These prompts include:

- **Core Principles**: Evidence-first methodology and systematic investigation
- **Communication Framework**: Structured response protocols
- **Advanced Capabilities**: Specialized SRE knowledge and analysis techniques
- **Quality Assurance**: Confidence scoring and validation frameworks

The main system prompt combines all these components to create a comprehensive SRE agent persona.

## Response Format

All OpenAI responses are returned in JSON format with the following structure:

```json
{
  "success": true,
  "content": "{\"analysis\": \"...\", \"confidence\": 0.8, \"recommendations\": [...]}",
  "model": "gpt-4o",
  "usage": {
    "prompt_tokens": 150,
    "completion_tokens": 200,
    "total_tokens": 350
  },
  "finish_reason": "stop"
}
```

## Error Handling

Both server and CLI components include comprehensive error handling:

- **API Key Missing**: Clear error message if `OPENAI_API_KEY` is not set
- **Network Errors**: Graceful handling of connection issues
- **Rate Limiting**: Proper error reporting for API rate limits
- **Invalid JSON**: Validation of JSON inputs and outputs

## Testing

Run the test suites to verify OpenAI integration:

### Server Tests
```bash
cd server
uv run pytest tests/test_openai_service.py -v
```

### CLI Tests
```bash
cd cli
uv run pytest tests/test_openai_service.py -v
```

## Langfuse Integration

The server OpenAI service includes Langfuse tracing decorators for observability:

- `@observe(name="openai_chat_completion")`: Traces all chat completions
- `@observe(name="openai_analyze_incident")`: Traces incident analysis calls
- `@observe(name="openai_query_analysis")`: Traces general query analysis

Configure Langfuse in your `.env` file:

```env
LANGFUSE_PUBLIC_KEY=your_public_key
LANGFUSE_SECRET_KEY=your_secret_key
LANGFUSE_HOST=https://cloud.langfuse.com
```

## Best Practices

1. **Use Appropriate Endpoints**: Use `/analyze/incident` for incidents, `/analyze/query` for general queries
2. **Provide Context**: Include relevant context data for better analysis
3. **Monitor Usage**: Track token usage to manage costs
4. **Error Handling**: Always check the `success` field in responses
5. **Security**: Never commit your OpenAI API key to version control

## Troubleshooting

### Common Issues

1. **"OPENAI_API_KEY environment variable is required"**
   - Ensure your `.env` file contains a valid OpenAI API key

2. **"Server error: 500"**
   - Check server logs for detailed error information
   - Verify OpenAI API key is valid and has sufficient credits

3. **"Invalid JSON format"**
   - Ensure JSON strings are properly escaped when using CLI
   - Use single quotes around JSON strings in bash

4. **Connection refused**
   - Ensure the server is running on the correct host/port
   - Check `SERVER_URL` in your `.env` file
