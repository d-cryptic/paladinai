# 🔧 Troubleshooting Guide

Comprehensive troubleshooting guide for diagnosing and resolving common issues with Paladin AI.

## Overview

This guide provides systematic approaches to diagnosing and resolving issues across all components of the Paladin AI platform, from installation problems to production deployment issues.

## Quick Diagnostics

### System Health Check

```bash
# Quick health check for all services
make health-check

# Individual service checks
curl -f http://localhost:8000/health          # Server
curl -f http://localhost:3000/api/health      # Frontend  
curl -f http://localhost:9090/-/healthy       # Prometheus
curl -f http://localhost:3100/ready           # Loki
curl -f http://localhost:6333/health          # Qdrant
```

### Environment Verification

```bash
# Check environment variables
env | grep -E "(OPENAI|MONGODB|QDRANT|NEO4J|DISCORD)"

# Verify API keys
python -c "
import openai
openai.api_key = 'your-key'
print('OpenAI API key valid' if openai.Model.list() else 'Invalid key')
"

# Check network connectivity
ping -c 3 api.openai.com
nslookup localhost
```

## Installation Issues

### 1. Package Installation Problems

#### UV Package Manager Issues

**Problem**: UV installation fails or command not found
```bash
# Error: uv: command not found
```

**Solution**:
```bash
# Install UV package manager
curl -LsSf https://astral.sh/uv/install.sh | sh

# Add to PATH
echo 'export PATH="$HOME/.cargo/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# Verify installation
uv --version
```

#### Python Version Compatibility

**Problem**: Python version compatibility issues
```bash
# Error: Python 3.13+ required
```

**Solution**:
```bash
# Check current Python version
python --version

# Install Python 3.13 (macOS with Homebrew)
brew install python@3.13
brew link python@3.13

# Install Python 3.13 (Ubuntu)
sudo apt update
sudo apt install python3.13 python3.13-venv python3.13-dev

# Update alternatives (Ubuntu)
sudo update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.13 1
```

#### Node.js Dependencies

**Problem**: Node.js version or npm issues
```bash
# Error: Node.js 18+ required
```

**Solution**:
```bash
# Install Node Version Manager
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash

# Install and use Node.js 18
nvm install 18
nvm use 18
nvm alias default 18

# Verify installation
node --version
npm --version
```

### 2. Docker and Infrastructure Issues

#### Docker Daemon Problems

**Problem**: Docker daemon not running
```bash
# Error: Cannot connect to the Docker daemon
```

**Solution**:
```bash
# Start Docker daemon (Linux)
sudo systemctl start docker
sudo systemctl enable docker

# Start Docker Desktop (macOS/Windows)
open -a Docker

# Fix permissions (Linux)
sudo usermod -aG docker $USER
newgrp docker

# Test Docker
docker run hello-world
```

#### Docker Compose Issues

**Problem**: Docker Compose version compatibility
```bash
# Error: Unsupported Compose file version
```

**Solution**:
```bash
# Install latest Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Verify version
docker-compose --version

# Update Compose file version
# Change version: '3.8' to version: '3.9' in docker-compose.yml
```

#### Port Conflicts

**Problem**: Ports already in use
```bash
# Error: Port 8000 is already in use
```

**Solution**:
```bash
# Check port usage
lsof -i :8000
netstat -tuln | grep 8000

# Kill processes using the port
sudo kill -9 $(lsof -ti:8000)

# Use alternative ports
export SERVER_PORT=8001
export FRONTEND_PORT=3001

# Or stop conflicting services
sudo systemctl stop apache2  # If using port 80
sudo systemctl stop nginx    # If using port 80
```

## API and Server Issues

### 1. Server Startup Problems

#### OpenAI API Key Issues

**Problem**: Invalid or missing OpenAI API key
```bash
# Error: Incorrect API key provided
```

**Solution**:
```bash
# Check API key format
echo $OPENAI_API_KEY | grep -E '^sk-[a-zA-Z0-9]{48}$'

# Test API key
curl -H "Authorization: Bearer $OPENAI_API_KEY" \
  https://api.openai.com/v1/models

# Set API key correctly
export OPENAI_API_KEY="sk-your-actual-key-here"

# Add to environment file
echo "OPENAI_API_KEY=sk-your-actual-key-here" >> server/.env
```

#### Database Connection Failures

**Problem**: Cannot connect to MongoDB
```bash
# Error: ServerSelectionTimeoutError
```

**Solution**:
```bash
# Check MongoDB status
docker ps | grep mongodb
docker logs paladin-mongodb

# Test connection
mongosh mongodb://localhost:27017/paladin

# Restart MongoDB
docker-compose restart mongodb

# Check network connectivity
telnet localhost 27017

# Verify credentials
mongosh "mongodb://admin:password@localhost:27017/admin"
```

#### Memory/Resource Issues

**Problem**: Out of memory or high CPU usage
```bash
# Error: Process killed (OOM)
```

**Solution**:
```bash
# Check system resources
free -h
df -h
top

# Monitor Docker container resources
docker stats

# Increase memory limits
# In docker-compose.yml:
deploy:
  resources:
    limits:
      memory: 4G

# Restart with more resources
docker-compose down
docker-compose up -d
```

### 2. API Response Issues

#### Slow Response Times

**Problem**: API responses taking too long
```bash
# Response times > 30 seconds
```

**Solution**:
```bash
# Check server logs
docker logs paladin-server | grep -E "(ERROR|timeout|slow)"

# Monitor response times
curl -w "@curl-format.txt" -o /dev/null -s http://localhost:8000/api/v1/chat

# curl-format.txt content:
#     time_namelookup:  %{time_namelookup}\n
#        time_connect:  %{time_connect}\n
#     time_appconnect:  %{time_appconnect}\n
#    time_pretransfer:  %{time_pretransfer}\n
#       time_redirect:  %{time_redirect}\n
#  time_starttransfer:  %{time_starttransfer}\n
#                     ----------\n
#          time_total:  %{time_total}\n

# Optimize database queries
# Add indexes to MongoDB collections
mongosh paladin
db.checkpoints.createIndex({"session_id": 1})
db.checkpoints.createIndex({"created_at": -1})

# Increase timeout settings
export REQUEST_TIMEOUT=300  # 5 minutes
export OPENAI_TIMEOUT=120   # 2 minutes
```

#### Rate Limiting Issues

**Problem**: OpenAI rate limit exceeded
```bash
# Error: Rate limit exceeded
```

**Solution**:
```bash
# Check rate limit status
curl -H "Authorization: Bearer $OPENAI_API_KEY" \
  https://api.openai.com/v1/usage

# Implement exponential backoff
# Update server configuration:
export OPENAI_MAX_RETRIES=5
export OPENAI_RETRY_DELAY=2

# Use lower-cost model
export OPENAI_MODEL=gpt-4o-mini

# Monitor usage
grep -E "rate.limit" logs/server.log
```

#### Authentication Errors

**Problem**: Authentication failures with external services
```bash
# Error: 401 Unauthorized
```

**Solution**:
```bash
# Verify API credentials
echo "Testing OpenAI API key..."
curl -H "Authorization: Bearer $OPENAI_API_KEY" \
  https://api.openai.com/v1/models

echo "Testing Langfuse connection..."
curl -H "Authorization: Bearer $LANGFUSE_SECRET_KEY" \
  https://cloud.langfuse.com/api/public/health

# Rotate expired keys
# Generate new API key from OpenAI dashboard
# Update environment variables
export OPENAI_API_KEY="new-key-here"

# Restart services with new credentials
docker-compose restart paladin-server
```

## Database Issues

### 1. MongoDB Problems

#### Connection Pool Exhaustion

**Problem**: MongoDB connection pool exhausted
```bash
# Error: connection pool timeout
```

**Solution**:
```bash
# Check current connections
mongosh paladin
db.serverStatus().connections

# Increase connection pool size
# In server/.env:
MONGODB_MAX_POOL_SIZE=100
MONGODB_MIN_POOL_SIZE=10

# Monitor connections
while true; do
  mongosh --eval "db.serverStatus().connections" paladin
  sleep 5
done

# Restart with new settings
docker-compose restart paladin-server
```

#### Disk Space Issues

**Problem**: MongoDB running out of disk space
```bash
# Error: No space left on device
```

**Solution**:
```bash
# Check disk usage
df -h
du -sh /var/lib/docker/volumes/paladin_mongodb_data

# Clean up old data
mongosh paladin
db.runCommand({compact: "checkpoints"})

# Enable compression
# In MongoDB configuration:
storage:
  wiredTiger:
    engineConfig:
      configString: "cache_size=1GB"
    collectionConfig:
      blockCompressor: snappy

# Archive old checkpoints
mongosh paladin
db.checkpoints.deleteMany({
  "created_at": {
    $lt: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000)  // 30 days ago
  }
})
```

#### Replica Set Issues

**Problem**: MongoDB replica set configuration problems
```bash
# Error: not master and slaveOk=false
```

**Solution**:
```bash
# Check replica set status
mongosh
rs.status()

# Reconfigure replica set
rs.initiate({
  _id: "rs0",
  members: [
    {_id: 0, host: "localhost:27017"}
  ]
})

# Force primary
rs.stepDown()
rs.freeze(0)

# Fix connection string
MONGODB_URI="mongodb://localhost:27017/paladin?replicaSet=rs0"
```

### 2. Qdrant Vector Database Issues

#### Collection Creation Problems

**Problem**: Cannot create Qdrant collections
```bash
# Error: Collection already exists or creation failed
```

**Solution**:
```bash
# Check collection status
curl http://localhost:6333/collections

# Delete and recreate collection
curl -X DELETE http://localhost:6333/collections/paladin_docs

# Create with correct configuration
curl -X PUT http://localhost:6333/collections/paladin_docs \
  -H "Content-Type: application/json" \
  -d '{
    "vectors": {
      "size": 1536,
      "distance": "Cosine"
    }
  }'

# Verify collection
curl http://localhost:6333/collections/paladin_docs
```

#### Vector Search Performance

**Problem**: Slow vector searches in Qdrant
```bash
# Searches taking > 5 seconds
```

**Solution**:
```bash
# Check collection info
curl http://localhost:6333/collections/paladin_docs

# Optimize collection settings
curl -X PATCH http://localhost:6333/collections/paladin_docs \
  -H "Content-Type: application/json" \
  -d '{
    "optimizers_config": {
      "default_segment_number": 2,
      "max_segment_size": 20000,
      "memmap_threshold": 20000
    }
  }'

# Create index for better performance
curl -X PUT http://localhost:6333/collections/paladin_docs/index \
  -H "Content-Type: application/json" \
  -d '{
    "field_name": "metadata.source",
    "field_schema": "keyword"
  }'
```

### 3. Neo4j Graph Database Issues

#### Authentication Problems

**Problem**: Neo4j authentication failures
```bash
# Error: The client is unauthorized due to authentication failure
```

**Solution**:
```bash
# Reset Neo4j password
docker exec -it paladin-neo4j cypher-shell -u neo4j -p neo4j
CALL dbms.security.changePassword('new_password');

# Update environment variables
export NEO4J_PASSWORD="new_password"

# Test connection
cypher-shell -a bolt://localhost:7687 -u neo4j -p new_password "RETURN 1"

# Update application configuration
echo "NEO4J_PASSWORD=new_password" >> server/.env
```

#### Graph Query Performance

**Problem**: Slow graph queries
```bash
# Queries taking > 10 seconds
```

**Solution**:
```bash
# Create indexes
cypher-shell -a bolt://localhost:7687 -u neo4j -p password "
CREATE INDEX memory_id FOR (m:Memory) ON (m.id);
CREATE INDEX entity_name FOR (e:Entity) ON (e.name);
"

# Analyze query performance
cypher-shell -a bolt://localhost:7687 -u neo4j -p password "
PROFILE MATCH (n:Memory) RETURN count(n);
"

# Optimize relationships
cypher-shell -a bolt://localhost:7687 -u neo4j -p password "
CALL apoc.periodic.iterate(
  'MATCH (n:Memory) WHERE NOT EXISTS(n.indexed) RETURN n',
  'SET n.indexed = true',
  {batchSize: 1000}
);
"
```

## Frontend Issues

### 1. Build and Runtime Errors

#### Next.js Build Failures

**Problem**: Frontend build fails
```bash
# Error: Module not found or compilation failed
```

**Solution**:
```bash
# Clear Next.js cache
cd frontend
rm -rf .next
rm -rf node_modules
npm install

# Fix package vulnerabilities
npm audit fix

# Check for TypeScript errors
npm run type-check

# Build with verbose output
npm run build -- --debug

# Check Node.js memory
node --max-old-space-size=4096 node_modules/.bin/next build
```

#### Runtime JavaScript Errors

**Problem**: Client-side JavaScript errors
```bash
# Error: Cannot read property 'map' of undefined
```

**Solution**:
```bash
# Check browser console
# Open DevTools > Console

# Enable React DevTools
npm install --save-dev @next/bundle-analyzer

# Add error boundary
// components/ErrorBoundary.tsx
class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(error) {
    return { hasError: true };
  }

  componentDidCatch(error, errorInfo) {
    console.error('Error caught by boundary:', error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      return <h1>Something went wrong.</h1>;
    }
    return this.props.children;
  }
}

# Check network requests
# DevTools > Network tab
```

### 2. State Management Issues

#### Zustand Store Problems

**Problem**: State not persisting or updating
```bash
# State resets on page refresh
```

**Solution**:
```bash
# Check localStorage
# DevTools > Application > Local Storage

# Debug store updates
// Add logging to store
const useChatStore = create((set, get) => ({
  // ... store definition
  addMessage: (message) => {
    console.log('Adding message:', message);
    set((state) => ({
      messages: [...state.messages, message]
    }));
  }
}));

# Clear corrupted state
localStorage.removeItem('chat-store');

# Check for circular references
JSON.stringify(storeState);  // Should not throw
```

#### Session Management

**Problem**: Sessions not persisting correctly
```bash
# Sessions disappear or don't save
```

**Solution**:
```bash
# Check session storage
// DevTools > Application > Session Storage

# Debug session creation
console.log('Current sessions:', useChatStore.getState().sessions);

# Verify session ID generation
const sessionId = `session_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
console.log('Generated session ID:', sessionId);

# Check for race conditions
// Add debouncing to session updates
const debouncedSaveSession = debounce(saveSession, 1000);
```

## CLI Issues

### 1. Installation and Setup

#### CLI Command Not Found

**Problem**: paladin command not recognized
```bash
# Error: paladin: command not found
```

**Solution**:
```bash
# Install CLI globally
cd cli
uv run pip install -e .

# Or use UV directly
cd cli
uv run python main.py --help

# Add to PATH
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# Create alias
echo 'alias paladin="cd /path/to/paladin-ai/cli && uv run python main.py"' >> ~/.bashrc
```

#### Configuration Issues

**Problem**: CLI cannot connect to server
```bash
# Error: Connection refused
```

**Solution**:
```bash
# Check server URL
echo $SERVER_URL
export SERVER_URL="http://localhost:8000"

# Test connectivity
curl -f $SERVER_URL/health

# Check environment file
cat cli/.env

# Update configuration
echo "SERVER_URL=http://localhost:8000" > cli/.env
echo "TIMEOUT=300" >> cli/.env

# Test CLI connection
paladin --test
```

### 2. CLI Runtime Issues

#### Memory and Performance

**Problem**: CLI consuming too much memory
```bash
# High memory usage or slow responses
```

**Solution**:
```bash
# Monitor CLI memory usage
ps aux | grep python | grep main.py

# Increase timeout for complex queries
paladin --timeout 600 --chat "complex query"

# Use simpler queries
paladin --chat "simple status check"

# Clear CLI cache
rm -rf ~/.cache/paladin-cli/

# Check for memory leaks
valgrind --tool=memcheck python main.py --test
```

## Memory System Issues

### 1. Memory Storage Problems

#### Memory Extraction Failures

**Problem**: Memories not being extracted or stored
```bash
# No memories found or extraction failed
```

**Solution**:
```bash
# Check memory service health
paladin --memory-health

# Test memory storage
paladin --memory-store "test memory instruction"

# Check Qdrant connection
curl http://localhost:6333/collections/paladinai_memory

# Verify memory extraction configuration
export MEM0_SIMILARITY_THRESHOLD=0.7
export MEM0_MAX_MEMORY_ITEMS=10000

# Debug memory extraction
paladin --debug --chat "test memory extraction"
```

#### Search Performance Issues

**Problem**: Memory searches are slow or return no results
```bash
# Search takes > 10 seconds or returns empty
```

**Solution**:
```bash
# Check memory count
curl http://localhost:6333/collections/paladinai_memory

# Test search directly
paladin --memory-search "CPU" --limit 5

# Lower confidence threshold
paladin --memory-search "CPU" --confidence 0.5

# Check embeddings
curl -X POST http://localhost:6333/collections/paladinai_memory/points/search \
  -H "Content-Type: application/json" \
  -d '{"vector": [0.1, 0.2, ...], "limit": 5}'

# Rebuild memory index
# Delete and recreate collection if needed
```

### 2. Memory Integration Issues

#### Workflow Memory Enhancement

**Problem**: Memory instructions not enhancing workflows
```bash
# Workflows not using stored memories
```

**Solution**:
```bash
# Check memory integration logs
grep -E "memory.*enhance" logs/server.log

# Test memory context retrieval
paladin --memory-context "high CPU usage" --workflow-type ACTION

# Verify memory instruction format
paladin --memory-store "Clear instruction: Always check systemd logs for errors"

# Debug workflow enhancement
export LOG_LEVEL=debug
paladin --chat "test memory enhancement"
```

## Monitoring and Observability Issues

### 1. Prometheus Issues

#### Metrics Collection Problems

**Problem**: Prometheus not collecting metrics
```bash
# Targets showing as down
```

**Solution**:
```bash
# Check Prometheus targets
curl http://localhost:9090/api/v1/targets

# Verify metrics endpoints
curl http://localhost:8000/metrics
curl http://localhost:3000/api/metrics

# Check Prometheus configuration
curl http://localhost:9090/api/v1/config

# Reload configuration
curl -X POST http://localhost:9090/-/reload

# Check scrape intervals
# In prometheus.yml:
scrape_interval: 15s
```

#### Storage Issues

**Problem**: Prometheus storage problems
```bash
# Disk space or retention issues
```

**Solution**:
```bash
# Check Prometheus storage
docker exec paladin-prometheus du -sh /prometheus

# Configure retention
# In prometheus.yml or command args:
--storage.tsdb.retention.time=200h
--storage.tsdb.retention.size=10GB

# Clean up old data
curl -X POST http://localhost:9090/api/v1/admin/tsdb/delete_series?match[]={__name__=~".+"}
curl -X POST http://localhost:9090/api/v1/admin/tsdb/clean_tombstones
```

### 2. Grafana Issues

#### Dashboard Loading Problems

**Problem**: Grafana dashboards not loading
```bash
# Error: Dashboard not found or data source issues
```

**Solution**:
```bash
# Check Grafana logs
docker logs paladin-grafana

# Verify data source connection
curl -u admin:admin http://localhost:3001/api/datasources

# Test Prometheus connection from Grafana
curl -u admin:admin http://localhost:3001/api/datasources/proxy/1/api/v1/query?query=up

# Import dashboards manually
# Go to Grafana UI > + > Import > Paste dashboard JSON

# Reset admin password
docker exec -it paladin-grafana grafana-cli admin reset-admin-password newpassword
```

### 3. Loki Issues

#### Log Ingestion Problems

**Problem**: Logs not appearing in Loki
```bash
# No logs in Grafana or Loki queries return empty
```

**Solution**:
```bash
# Check Loki status
curl http://localhost:3100/ready

# Test log ingestion
curl -X POST http://localhost:3100/loki/api/v1/push \
  -H "Content-Type: application/json" \
  -d '{
    "streams": [
      {
        "stream": {"job": "test"},
        "values": [["'$(date +%s%N)'", "test log message"]]
      }
    ]
  }'

# Check Promtail configuration
docker logs paladin-promtail

# Verify log paths
docker exec paladin-promtail ls -la /var/log/
```

## Discord Integration Issues

### 1. Bot Connection Problems

#### Bot Not Responding

**Problem**: Discord bot not responding to commands
```bash
# Bot appears online but doesn't respond
```

**Solution**:
```bash
# Check bot status
curl http://localhost:9000/health

# Verify bot token
python -c "
import discord
bot = discord.Bot()
@bot.event
async def on_ready():
    print(f'Bot ready: {bot.user}')
bot.run('YOUR_BOT_TOKEN')
"

# Check bot permissions
# Ensure bot has required permissions in Discord server:
# - Send Messages
# - Read Message History
# - Create Threads
# - Use Slash Commands

# Restart bot services
docker-compose restart discord-mcp
```

#### Message Processing Issues

**Problem**: Messages not being processed or queued
```bash
# Messages sent but no response or processing
```

**Solution**:
```bash
# Check message queue
redis-cli -h localhost -p 6379 LLEN discord_message_queue

# Monitor queue processing
redis-cli -h localhost -p 6379 MONITOR

# Check worker status
docker logs paladin-discord-workers

# Test guardrail system
python -c "
from workers_server import should_process_message
result = should_process_message('test monitoring message')
print(f'Should process: {result}')
"

# Flush queue if stuck
redis-cli -h localhost -p 6379 FLUSHALL
```

### 2. MCP Integration Issues

#### MCP Server Connection

**Problem**: MCP server not accessible
```bash
# Error: Cannot connect to MCP server
```

**Solution**:
```bash
# Check MCP server status
curl http://localhost:9000/mcp/status

# Verify MCP server logs
docker logs paladin-discord-mcp

# Test MCP tools directly
curl -X POST http://localhost:9000/mcp/tools/get_channel_messages \
  -H "Content-Type: application/json" \
  -d '{"channel_id": "123456789", "limit": 10}'

# Restart MCP server
docker-compose restart discord-mcp
```

## Performance Optimization

### 1. Database Performance

#### MongoDB Optimization

```bash
# Create indexes
mongosh paladin
db.checkpoints.createIndex({"session_id": 1})
db.checkpoints.createIndex({"created_at": -1})
db.sessions.createIndex({"user_id": 1, "created_at": -1})

# Analyze slow queries
db.setProfilingLevel(2, { slowms: 100 })
db.system.profile.find().limit(5).sort({ ts: -1 }).pretty()

# Compact collections
db.runCommand({compact: "checkpoints"})
db.runCommand({compact: "sessions"})

# Monitor performance
mongostat --host localhost:27017
```

#### Qdrant Optimization

```bash
# Optimize vector collections
curl -X PATCH http://localhost:6333/collections/paladinai_memory \
  -H "Content-Type: application/json" \
  -d '{
    "optimizers_config": {
      "default_segment_number": 2,
      "max_segment_size": 20000
    }
  }'

# Create payload indexes
curl -X PUT http://localhost:6333/collections/paladinai_memory/index \
  -H "Content-Type: application/json" \
  -d '{
    "field_name": "memory_type",
    "field_schema": "keyword"
  }'
```

### 2. Application Performance

#### Memory Usage Optimization

```bash
# Monitor memory usage
docker stats

# Optimize Python memory usage
export PYTHONOPTIMIZE=1
export PYTHONDONTWRITEBYTECODE=1

# Reduce Docker image size
# Use multi-stage builds
# Use alpine base images
# Remove unnecessary packages

# Garbage collection tuning
export MALLOC_TRIM_THRESHOLD_=100000
export MALLOC_MMAP_THRESHOLD_=100000
```

#### Response Time Optimization

```bash
# Enable caching
export ENABLE_REDIS_CACHE=true
export CACHE_TTL=300

# Optimize OpenAI requests
export OPENAI_MAX_TOKENS=2000
export OPENAI_TEMPERATURE=0.1

# Use connection pooling
export DB_POOL_SIZE=20
export DB_POOL_TIMEOUT=30

# Monitor response times
curl -w "@curl-format.txt" -o /dev/null -s http://localhost:8000/api/v1/chat
```

## Debugging Tools and Commands

### 1. Log Analysis

```bash
# Centralized logging
docker-compose logs -f --tail=100

# Service-specific logs
docker logs -f paladin-server
docker logs -f paladin-frontend
docker logs -f paladin-mongodb

# Search logs for errors
docker logs paladin-server 2>&1 | grep -E "(ERROR|Exception|Failed)"

# Real-time log monitoring
tail -f logs/*.log | grep -E "(ERROR|WARNING|CRITICAL)"
```

### 2. Network Debugging

```bash
# Check port connectivity
nmap -p 8000,3000,27017,6333 localhost

# Test internal networking
docker exec paladin-server curl -f http://mongodb:27017
docker exec paladin-server nc -zv qdrant 6333

# Monitor network traffic
sudo tcpdump -i any port 8000
sudo netstat -tulpn | grep -E "(8000|3000|27017)"
```

### 3. Performance Profiling

```bash
# Python profiling
python -m cProfile -s cumulative server/main.py

# Memory profiling
python -m memory_profiler server/main.py

# HTTP load testing
ab -n 1000 -c 10 http://localhost:8000/health
wrk -t12 -c400 -d30s http://localhost:8000/api/v1/chat

# Database profiling
mongosh --eval "db.setProfilingLevel(2)"
mongosh --eval "db.system.profile.find().pretty()"
```

## Recovery Procedures

### 1. Complete System Recovery

```bash
# Stop all services
docker-compose down

# Clean up volumes (WARNING: Data loss)
docker volume prune -f

# Rebuild from scratch
docker-compose build --no-cache
docker-compose up -d

# Restore from backup
./scripts/restore-backup.sh latest
```

### 2. Database Recovery

```bash
# MongoDB recovery
mongorestore --uri="mongodb://localhost:27017/paladin" /backup/mongodb/latest

# Qdrant recovery
curl -X POST http://localhost:6333/collections/paladinai_memory/snapshots/recover \
  -H "Content-Type: application/json" \
  -d '{"location": "/backup/qdrant/latest"}'

# Neo4j recovery
docker exec paladin-neo4j neo4j-admin restore --from=/backup/neo4j/latest
```

### 3. Configuration Recovery

```bash
# Reset to default configuration
cp .env.example .env
cp server/.env.example server/.env
cp frontend/.env.example frontend/.env.local

# Restore specific configuration
git checkout HEAD -- server/.env
git checkout HEAD -- frontend/.env.local

# Regenerate secrets
export OPENAI_API_KEY="new-key"
export JWT_SECRET=$(openssl rand -hex 32)
export MONGODB_PASSWORD=$(openssl rand -base64 32)
```

## Getting Help

### 1. Log Collection

```bash
# Collect all logs for support
mkdir -p /tmp/paladin-logs
docker-compose logs > /tmp/paladin-logs/docker-compose.log
docker logs paladin-server > /tmp/paladin-logs/server.log
docker logs paladin-frontend > /tmp/paladin-logs/frontend.log
docker logs paladin-mongodb > /tmp/paladin-logs/mongodb.log

# System information
uname -a > /tmp/paladin-logs/system.info
docker version >> /tmp/paladin-logs/system.info
docker-compose version >> /tmp/paladin-logs/system.info

# Create support bundle
tar -czf paladin-support-$(date +%Y%m%d-%H%M%S).tar.gz -C /tmp paladin-logs
```

### 2. Support Channels

- **GitHub Issues**: [Create issue](https://github.com/your-org/paladin-ai/issues)
- **Documentation**: [docs/](../docs/)
- **Discord Community**: [Join server](https://discord.gg/paladin-ai)
- **Email Support**: support@paladin-ai.com

### 3. Providing Information

When seeking help, include:
- Error messages and stack traces
- Steps to reproduce the issue
- System information (OS, Docker version, etc.)
- Configuration files (with secrets redacted)
- Relevant log excerpts
- What you've already tried

---

This troubleshooting guide covers the most common issues encountered with Paladin AI. For issues not covered here, please check the GitHub issues or reach out to the community for assistance.