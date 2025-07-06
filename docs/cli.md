# 🖥️ CLI Guide Documentation

Complete guide to using the Paladin AI command-line interface.

## Overview

The Paladin AI CLI provides a powerful command-line interface for interacting with the Paladin AI monitoring and incident response platform. Built with Python and Rich text formatting, it offers both single-shot commands and interactive chat modes.

## Installation

### Prerequisites
- Python 3.13+ (or 3.11+ minimum)
- UV package manager (recommended)
- Access to Paladin AI server

### Installation Methods

#### Method 1: Using Makefile (Recommended)
```bash
# From project root
make install-dev  # Install all dependencies
make run-cli      # Start CLI session
```

#### Method 2: Manual Installation
```bash
# Navigate to CLI directory
cd cli

# Install dependencies
uv sync

# Run CLI
uv run python main.py --help
```

#### Method 3: Global Installation
```bash
cd cli
uv run pip install -e .
paladin --help
```

## Configuration

### Environment Variables
Create or edit `cli/.env`:

```bash
# Server Configuration
SERVER_URL=http://127.0.0.1:8000
DEBUG=false

# Timeout Settings
TIMEOUT=1200000  # 20 minutes (milliseconds)
MAX_RETRIES=3
RETRY_DELAY=1.0  # Seconds

# Optional: OpenAI API Key
OPENAI_API_KEY=your-openai-api-key
```

### Command Line Options
```bash
# Set server URL
paladin --server-url http://your-server:8000

# Enable debug mode
paladin --debug

# Set timeout
paladin --timeout 600  # 10 minutes
```

## Basic Commands

### Connection Testing

#### Test Server Connection
```bash
# Basic connection test
paladin --test

# Get hello message
paladin --hello

# Check API status
paladin --status

# Run all connection tests
paladin --all
```

**Example Output:**
```
🏥 Health Check: ✅ healthy
👋 Hello Response: ✅ Hello World from Paladin AI Server!
📊 Status Check: ✅ API v1 - running
```

### Health Monitoring
```bash
# Check overall health
paladin --health

# Check specific service health
paladin --health --service memory
paladin --health --service documents
```

## Chat and AI Interaction

### Single-Shot Chat
```bash
# Send a single message
paladin --chat "Check CPU usage on web servers"

# Include context
paladin --chat "Monitor memory usage" --context '{"environment": "production"}'

# Show analysis only
paladin --chat "Investigate high latency" --analysis-only
```

### Interactive Chat Mode
```bash
# Start interactive session
paladin --interactive

# Interactive session with specific context
paladin --interactive --context '{"team": "sre", "environment": "prod"}'
```

**Interactive Commands:**
- `/memory help` - Show memory commands
- `/checkpoint help` - Show checkpoint commands
- `/clear` - Clear screen
- `/exit` - Exit session

### Chat Examples

#### Infrastructure Monitoring
```bash
paladin --chat "What's the current CPU usage across all servers?"
paladin --chat "Are there any alerts in the last hour?"
paladin --chat "Show me disk space usage for database servers"
```

#### Incident Investigation
```bash
paladin --chat "Investigate high response times in the API"
paladin --chat "Why are users reporting slow page loads?"
paladin --chat "Analyze the spike in 5xx errors"
```

#### Performance Analysis
```bash
paladin --chat "Compare current performance to last week"
paladin --chat "Identify bottlenecks in the payment service"
paladin --chat "Show me the top 5 slowest endpoints"
```

## Memory Management

### Store Instructions
```bash
# Store operational instructions
paladin --memory-store "Always check systemd logs for application errors"

# Store with context
paladin --memory-store "Database queries should timeout after 30 seconds" --context '{"service": "database"}'

# Store troubleshooting procedures
paladin --memory-store "To fix Redis connection issues, restart redis service and check firewall"
```

### Search Memories
```bash
# Basic memory search
paladin --memory-search "CPU alerts"

# Search with parameters
paladin --memory-search "database troubleshooting" --limit 5 --confidence 0.7

# Search by memory type
paladin --memory-search "deployment procedures" --memory-type instruction
```

### Contextual Memories
```bash
# Get memories for current situation
paladin --memory-context "high memory usage" --workflow-type ACTION

# Get memories for incident investigation
paladin --memory-context "service outage" --workflow-type INCIDENT

# Get memories for information queries
paladin --memory-context "system status" --workflow-type QUERY
```

### Memory Utilities
```bash
# Show available memory types
paladin --memory-types

# Check memory service health
paladin --memory-health

# Show memory help
paladin --memory-help
```

## Checkpoint Management

### Session Management
```bash
# Get checkpoint for specific session
paladin --checkpoint-get user_john_1234567890

# Check if checkpoint exists
paladin --checkpoint-exists user_john_1234567890

# List all checkpoints
paladin --checkpoint-list

# List with filters
paladin --checkpoint-list --session user_john --limit 10
```

### Checkpoint Operations
```bash
# Delete session checkpoints
paladin --checkpoint-delete user_john_1234567890

# Show checkpoint help
paladin --checkpoint-help
```

## Advanced Usage

### Workflow Type Detection
The CLI automatically detects workflow types based on your input:

#### QUERY Workflows
- Information retrieval
- Status checks
- Simple questions

```bash
paladin --chat "What's the current system status?"
paladin --chat "How many users are currently online?"
```

#### ACTION Workflows
- System changes
- Configuration updates
- Operational tasks

```bash
paladin --chat "Scale up the web service to 5 instances"
paladin --chat "Update the database connection pool size"
```

#### INCIDENT Workflows
- Problem investigation
- Root cause analysis
- Complex troubleshooting

```bash
paladin --chat "Investigate why users can't login"
paladin --chat "Analyze the cause of the recent outage"
```

### Context Integration
```bash
# JSON context for enhanced responses
paladin --chat "Check service health" --context '{
  "environment": "production",
  "team": "platform",
  "priority": "high"
}'

# Interactive mode with context
paladin --interactive --context '{"role": "sre", "oncall": true}'
```

### Batch Operations
```bash
# Multiple commands in sequence
paladin --test --status --memory-health

# Complex workflow with context
paladin --chat "System overview" --memory-context "system monitoring" --workflow-type QUERY
```

## Interactive Features

### Rich Formatting
The CLI provides rich text formatting with:
- **Markdown rendering** with syntax highlighting
- **Color-coded output** for different message types
- **Tables and panels** for structured data
- **Icons and symbols** for visual clarity

### Loading Animations
- Animated spinners during long operations
- Progress indicators for multi-step processes
- Context-aware loading messages

### Error Handling
- Clear error messages with suggestions
- Automatic retry for transient failures
- Graceful degradation when services are unavailable

## Common Use Cases

### 1. Daily Operations
```bash
# Morning health check
paladin --health --memory-search "daily checklist"

# Interactive monitoring session
paladin --interactive --context '{"shift": "day", "team": "ops"}'
```

### 2. Incident Response
```bash
# Quick incident assessment
paladin --chat "What alerts are currently firing?"

# Deep investigation
paladin --chat "Investigate payment service failures" --workflow-type INCIDENT
```

### 3. Knowledge Management
```bash
# Store new procedures
paladin --memory-store "New deployment requires health check validation"

# Search existing knowledge
paladin --memory-search "deployment procedures" --limit 10
```

### 4. Performance Analysis
```bash
# Regular performance check
paladin --chat "Show me key performance metrics"

# Trend analysis
paladin --chat "Compare current performance to last week"
```

## Command Reference

### Connection Commands
| Command | Description | Example |
|---------|-------------|---------|
| `--test` | Test server connection | `paladin --test` |
| `--hello` | Get hello message | `paladin --hello` |
| `--status` | Check API status | `paladin --status` |
| `--all` | Run all connection tests | `paladin --all` |
| `--health` | Check service health | `paladin --health` |

### Chat Commands
| Command | Description | Example |
|---------|-------------|---------|
| `--chat "message"` | Send single message | `paladin --chat "Check CPU"` |
| `--interactive` | Start interactive mode | `paladin --interactive` |
| `--analysis-only` | Show analysis sections only | `paladin --chat "Analyze" --analysis-only` |
| `--context '{json}'` | Add JSON context | `paladin --chat "Status" --context '{"env":"prod"}'` |

### Memory Commands
| Command | Description | Example |
|---------|-------------|---------|
| `--memory-store "text"` | Store instruction | `paladin --memory-store "Check logs first"` |
| `--memory-search "query"` | Search memories | `paladin --memory-search "CPU alerts"` |
| `--memory-context "situation"` | Get contextual memories | `paladin --memory-context "high CPU"` |
| `--memory-types` | Show memory types | `paladin --memory-types` |
| `--memory-health` | Check memory service | `paladin --memory-health` |
| `--memory-help` | Show memory help | `paladin --memory-help` |

### Checkpoint Commands
| Command | Description | Example |
|---------|-------------|---------|
| `--checkpoint-get id` | Get checkpoint | `paladin --checkpoint-get session_123` |
| `--checkpoint-exists id` | Check existence | `paladin --checkpoint-exists session_123` |
| `--checkpoint-list` | List checkpoints | `paladin --checkpoint-list` |
| `--checkpoint-delete id` | Delete checkpoint | `paladin --checkpoint-delete session_123` |
| `--checkpoint-help` | Show checkpoint help | `paladin --checkpoint-help` |

### Parameters
| Parameter | Description | Example |
|-----------|-------------|---------|
| `--workflow-type TYPE` | Set workflow type | `--workflow-type ACTION` |
| `--limit N` | Limit results | `--limit 10` |
| `--confidence N` | Confidence threshold | `--confidence 0.8` |
| `--session ID` | Filter by session | `--session user_123` |
| `--server-url URL` | Server URL | `--server-url http://host:8000` |
| `--debug` | Enable debug mode | `--debug` |
| `--timeout N` | Set timeout (seconds) | `--timeout 300` |

## Error Handling

### Connection Errors
```bash
# Connection refused
❌ Connection Error: Could not connect to server at http://127.0.0.1:8000
💡 Suggestion: Check if the server is running and the URL is correct

# Timeout errors
❌ Timeout Error: Request timed out after 300 seconds
💡 Suggestion: Try increasing timeout with --timeout 600
```

### Server Errors
```bash
# Server unavailable
❌ Server Error: Service temporarily unavailable
💡 Suggestion: Try again in a few moments

# Invalid request
❌ Validation Error: Invalid request parameters
💡 Suggestion: Check your input format and try again
```

### Memory Errors
```bash
# Memory service unavailable
❌ Memory Error: Memory service is not available
💡 Suggestion: Check memory service health with --memory-health

# Search no results
ℹ️ No memories found for query: "nonexistent topic"
💡 Suggestion: Try broader search terms or check --memory-types
```

## Troubleshooting

### Common Issues

#### 1. Connection Issues
```bash
# Check server status
paladin --test

# Verify server URL
paladin --server-url http://your-server:8000 --test

# Check environment variables
cat cli/.env
```

#### 2. Timeout Issues
```bash
# Increase timeout for complex queries
paladin --timeout 600 --chat "Complex analysis query"

# Check server performance
paladin --health
```

#### 3. Memory Issues
```bash
# Check memory service health
paladin --memory-health

# Verify memory configuration
paladin --memory-types
```

#### 4. Permission Issues
```bash
# Check file permissions
ls -la cli/.env

# Verify Python environment
which python
python --version
```

### Debug Mode
```bash
# Enable debug output
paladin --debug --chat "Debug message"

# Check detailed error information
DEBUG=true paladin --chat "Test message"
```

## Best Practices

### 1. Efficient Usage
- Use `--all` for comprehensive health checks
- Leverage `--memory-search` before storing duplicate instructions
- Use `--interactive` for complex troubleshooting sessions

### 2. Context Management
- Include relevant context in complex queries
- Use workflow types to optimize AI responses
- Maintain session continuity for related operations

### 3. Knowledge Management
- Store frequently used procedures in memory
- Use descriptive memory instructions
- Regularly search memories to avoid duplication

### 4. Error Recovery
- Use `--test` to verify connectivity before complex operations
- Increase timeout for complex analysis queries
- Check service health when experiencing issues

## Integration Examples

### Shell Scripts
```bash
#!/bin/bash
# Daily health check script

echo "Starting daily health check..."

# Test connectivity
if ! paladin --test > /dev/null 2>&1; then
    echo "❌ Paladin server unavailable"
    exit 1
fi

# Get system status
paladin --chat "Daily system health summary" --context '{"type": "daily-check"}'

# Check for critical alerts
paladin --chat "Any critical alerts in the last 24 hours?"

echo "✅ Daily health check complete"
```

### Cron Jobs
```bash
# Add to crontab
# 0 9 * * * /usr/local/bin/paladin --chat "Daily system summary" >> /var/log/paladin-daily.log 2>&1
```

### CI/CD Integration
```yaml
# GitHub Actions example
- name: Check system health
  run: |
    paladin --health
    paladin --chat "Pre-deployment health check"
```

## Performance Considerations

### Response Times
- **Simple queries**: < 5 seconds
- **Complex analysis**: 30-60 seconds
- **Memory operations**: < 2 seconds
- **Checkpoint operations**: < 1 second

### Resource Usage
- **Memory**: ~50MB for CLI process
- **CPU**: Minimal when idle
- **Network**: Depends on query complexity

### Optimization Tips
1. Use specific queries instead of broad requests
2. Leverage memory system for repeated operations
3. Use appropriate timeout settings
4. Monitor server performance regularly

---

The Paladin AI CLI provides a powerful and flexible interface for monitoring and incident response operations. Its rich feature set and intuitive design make it suitable for both quick operational tasks and complex troubleshooting scenarios.