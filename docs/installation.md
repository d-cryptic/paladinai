# 🚀 Installation Guide

Complete setup and configuration guide for Paladin AI platform.

## Prerequisites

### System Requirements

- **Operating System**: Linux, macOS, or Windows 10+
- **Python**: 3.13+ (recommended) or 3.11+ (minimum)
- **Node.js**: 18.0+ (LTS recommended)
- **Memory**: 8GB RAM minimum, 16GB recommended
- **Disk Space**: 10GB free space minimum

### Required Dependencies

| Component | Version | Purpose |
|-----------|---------|---------|
| Python | 3.13+ | Server runtime |
| UV Package Manager | Latest | Python dependency management |
| Node.js | 18+ | Frontend runtime |
| Docker | 20.10+ | Infrastructure containers |
| Docker Compose | 2.0+ | Multi-container orchestration |

### External Services

| Service | Required | Purpose |
|---------|----------|---------|
| OpenAI API | ✅ Yes | AI capabilities |
| MongoDB | ✅ Yes | Checkpoint storage |
| Qdrant | ⚠️ Optional | Vector embeddings |
| Neo4j | ⚠️ Optional | Memory graph |

## Quick Installation

### 1. Clone Repository

```bash
# Clone the repository
git clone https://github.com/your-org/paladin-ai.git
cd paladin-ai

# Verify directory structure
ls -la
```

### 2. Install UV Package Manager

```bash
# macOS/Linux
curl -LsSf https://astral.sh/uv/install.sh | sh

# Windows (PowerShell)
powershell -c "irm https://astral.sh/uv/install.ps1 | iex"

# Verify installation
uv --version
```

### 3. Install All Dependencies

```bash
# Install all project dependencies
make install-dev

# Or manually:
# Backend dependencies
cd server && uv sync
# Frontend dependencies  
cd ../frontend && npm install
# CLI dependencies
cd ../cli && uv sync
```

## Environment Configuration

### 1. Copy Environment Templates

```bash
# Main environment file
cp .env.example .env

# Frontend environment
cp frontend/.env.example frontend/.env.local

# Server environment
cp server/.env.example server/.env
```

### 2. Configure API Keys

Edit `.env` file:

```bash
# OpenAI API Key (Required)
OPENAI_API_KEY=sk-your-openai-api-key-here

# MongoDB Connection (Required)
MONGODB_URL=mongodb://localhost:27017/paladin

# Optional: Qdrant Vector Database
QDRANT_URL=http://localhost:6333
QDRANT_API_KEY=your-qdrant-key

# Optional: Neo4j Graph Database
NEO4J_URL=bolt://localhost:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=your-neo4j-password
```

### 3. Frontend Configuration

Edit `frontend/.env.local`:

```bash
# Paladin API URL
NEXT_PUBLIC_PALADIN_API_URL=http://localhost:8000

# Optional: Enable analytics
NEXT_PUBLIC_ANALYTICS_ID=your-analytics-id
```

## Infrastructure Setup

### 1. Start Infrastructure Stack

```bash
# Start all infrastructure services
make infra-up

# This starts:
# - MongoDB (port 27017)
# - Qdrant (port 6333)
# - Neo4j (port 7474/7687)
# - Mock monitoring stack
```

### 2. Verify Infrastructure

```bash
# Check running containers
docker ps

# Check logs
make infra-logs

# Expected services:
# - paladin-mongodb
# - paladin-qdrant
# - paladin-neo4j
# - paladin-prometheus
# - paladin-grafana
```

## Service Installation

### 1. Server Installation

```bash
cd server

# Install dependencies
uv sync

# Verify installation
uv run python --version
uv run pip list | grep -E "(fastapi|langchain|langgraph)"
```

### 2. Frontend Installation

```bash
cd frontend

# Install dependencies
npm install

# Verify installation
npm list next react typescript

# Build for production (optional)
npm run build
```

### 3. CLI Installation

```bash
cd cli

# Install dependencies
uv sync

# Install CLI globally (optional)
uv run pip install -e .

# Verify installation
paladin-cli --version
```

## Database Setup

### 1. MongoDB Initialization

```bash
# Connect to MongoDB
mongosh mongodb://localhost:27017/paladin

# Create collections
db.createCollection("checkpoints")
db.createCollection("sessions")
db.createCollection("documents")

# Create indexes
db.checkpoints.createIndex({"session_id": 1})
db.sessions.createIndex({"created_at": -1})
```

### 2. Qdrant Setup (Optional)

```bash
# Verify Qdrant connection
curl http://localhost:6333/collections

# Create collections via API
curl -X PUT http://localhost:6333/collections/paladin-docs \
  -H "Content-Type: application/json" \
  -d '{
    "vectors": {
      "size": 1536,
      "distance": "Cosine"
    }
  }'
```

### 3. Neo4j Setup (Optional)

```bash
# Access Neo4j Browser
open http://localhost:7474

# Initialize schema
CREATE CONSTRAINT memory_id IF NOT EXISTS FOR (m:Memory) REQUIRE m.id IS UNIQUE;
CREATE INDEX memory_created IF NOT EXISTS FOR (m:Memory) ON (m.created_at);
```

## Running the Platform

### 1. Start Core Services

```bash
# Terminal 1: Start server
make run-server
# Server starts on http://localhost:8000

# Terminal 2: Start frontend
make run-frontend  
# Frontend starts on http://localhost:3000

# Terminal 3: CLI (optional)
make run-cli
# Interactive CLI session
```

### 2. Verify Installation

```bash
# Check server health
curl http://localhost:8000/health

# Check frontend
curl http://localhost:3000/api/health

# Check API documentation
open http://localhost:8000/docs
```

## Development Setup

### 1. Development Dependencies

```bash
# Install development tools
make install-dev

# This installs:
# - Pre-commit hooks
# - Code formatters (black, ruff)
# - Type checkers (mypy)
# - Testing frameworks (pytest)
```

### 2. IDE Configuration

#### VS Code Setup

```json
// .vscode/settings.json
{
  "python.defaultInterpreterPath": "./server/.venv/bin/python",
  "python.linting.enabled": true,
  "python.linting.ruffEnabled": true,
  "python.formatting.provider": "black",
  "typescript.preferences.importModuleSpecifier": "relative"
}
```

#### PyCharm Setup

1. **Open Project**: File → Open → Select `paladin-ai` directory
2. **Set Python Interpreter**: Settings → Project → Python Interpreter → Add → UV Environment
3. **Configure Ruff**: Settings → Tools → External Tools → Add Ruff

### 3. Git Hooks

```bash
# Install pre-commit hooks
pre-commit install

# Run hooks manually
pre-commit run --all-files
```

## Production Setup

### 1. Environment Variables

```bash
# Production environment
export ENVIRONMENT=production
export DEBUG=false
export OPENAI_API_KEY=your-production-key

# Database URLs
export MONGODB_URL=mongodb://prod-mongo:27017/paladin
export QDRANT_URL=https://prod-qdrant.com
export NEO4J_URL=bolt://prod-neo4j:7687

# Security
export JWT_SECRET=your-secure-jwt-secret
export CORS_ORIGINS=https://paladin.yourdomain.com
```

### 2. Docker Production Build

```bash
# Build production images
docker-compose -f docker-compose.prod.yml build

# Start production stack
docker-compose -f docker-compose.prod.yml up -d
```

### 3. Reverse Proxy Setup

```nginx
# nginx.conf
server {
    listen 80;
    server_name paladin.yourdomain.com;
    
    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
    
    location /api {
        proxy_pass http://localhost:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Verification & Testing

### 1. System Health Checks

```bash
# Run all health checks
make health-check

# Individual checks
curl -f http://localhost:8000/health || echo "Server unhealthy"
curl -f http://localhost:3000/api/health || echo "Frontend unhealthy"
```

### 2. Integration Tests

```bash
# Run integration tests
make test-integration

# Run specific test categories
python tests/run_integration_tests.py --type workflows
python tests/run_integration_tests.py --type tools
```

### 3. End-to-End Verification

```bash
# Test chat functionality
curl -X POST http://localhost:8000/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello Paladin", "session_id": "test-session"}'

# Test document upload
curl -X POST http://localhost:8000/api/v1/documents/rag \
  -F "file=@sample.pdf" \
  -F "session_id=test-session"
```

## Troubleshooting

### Common Installation Issues

#### 1. UV Installation Problems

```bash
# Check UV installation
which uv

# Reinstall if needed
curl -LsSf https://astral.sh/uv/install.sh | sh
source ~/.bashrc
```

#### 2. Python Version Issues

```bash
# Check Python version
python --version

# Install Python 3.13 (macOS)
brew install python@3.13

# Install Python 3.13 (Ubuntu)
sudo apt update
sudo apt install python3.13 python3.13-venv
```

#### 3. Node.js Issues

```bash
# Check Node.js version
node --version

# Install Node.js (using nvm)
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
nvm install 18
nvm use 18
```

#### 4. Docker Issues

```bash
# Check Docker status
docker --version
docker-compose --version

# Start Docker daemon
sudo systemctl start docker

# Fix permissions (Linux)
sudo usermod -aG docker $USER
newgrp docker
```

### Service-Specific Issues

#### 1. MongoDB Connection

```bash
# Check MongoDB status
docker ps | grep mongodb

# Check MongoDB logs
docker logs paladin-mongodb

# Test connection
mongosh mongodb://localhost:27017/paladin
```

#### 2. API Key Issues

```bash
# Verify OpenAI API key
export OPENAI_API_KEY=your-key
curl -H "Authorization: Bearer $OPENAI_API_KEY" \
  https://api.openai.com/v1/models
```

#### 3. Port Conflicts

```bash
# Check port usage
lsof -i :8000  # Server port
lsof -i :3000  # Frontend port
lsof -i :27017 # MongoDB port

# Kill processes if needed
kill -9 $(lsof -ti:8000)
```

## Security Considerations

### 1. API Key Security

- Never commit API keys to version control
- Use environment variables for sensitive data
- Rotate API keys regularly
- Use different keys for development/production

### 2. Network Security

- Use HTTPS in production
- Configure proper CORS origins
- Set up firewall rules
- Use VPN for database access

### 3. Authentication

```bash
# Generate secure JWT secret
openssl rand -hex 32

# Set strong database passwords
export MONGODB_PASSWORD=$(openssl rand -base64 32)
export NEO4J_PASSWORD=$(openssl rand -base64 32)
```

## Next Steps

After successful installation:

1. **Read the [Architecture Guide](architecture.md)** to understand system components
2. **Follow the [CLI Guide](cli.md)** to start using the command-line interface
3. **Check the [API Reference](api.md)** for integration details
4. **Set up [Monitoring Integration](monitoring.md)** for production use
5. **Configure [Discord Integration](discord.md)** for team collaboration

## Getting Help

- **Documentation**: [docs/](../docs/)
- **Issues**: [GitHub Issues](https://github.com/your-org/paladin-ai/issues)
- **Discussions**: [GitHub Discussions](https://github.com/your-org/paladin-ai/discussions)
- **Community**: [Discord Server](https://discord.gg/paladin-ai)

---

**Installation complete!** 🎉

Visit http://localhost:3000 to start using Paladin AI.