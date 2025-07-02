# PaladinAI MongoDB Infrastructure

This directory contains the MongoDB infrastructure setup for PaladinAI's checkpointing system.

## Overview

The MongoDB infrastructure provides:
- **MongoDB 7.0+** with replica set configuration for LangGraph checkpointing
- **Single-node replica set** optimized for development and testing
- **Persistent data storage** with Docker volumes
- **Authentication** with configurable credentials
- **MongoDB Express** web interface for database administration
- **Automated initialization** with proper indexes and collections

## Quick Start

### 1. Prerequisites

- Docker and Docker Compose installed
- At least 2GB free disk space
- Ports 27017 and 8081 available

### 2. Environment Setup

Copy the environment template:
```bash
cp .env.example .env
```

Edit `.env` to customize MongoDB credentials (optional):
```bash
# MongoDB Docker Configuration
MONGO_INITDB_ROOT_USERNAME=admin
MONGO_INITDB_ROOT_PASSWORD=your_secure_password

# MongoDB Express Configuration
MONGODB_EXPRESS_USERNAME=admin
MONGODB_EXPRESS_PASSWORD=your_express_password
```

### 3. Start MongoDB Infrastructure

From the project root directory:
```bash
cd infra
docker-compose up -d
```

### 4. Verify Setup

Check service status:
```bash
docker-compose ps
```

Expected output:
```
NAME                           COMMAND                  SERVICE             STATUS
paladinai-mongodb              "docker-entrypoint.s…"   mongodb             Up (healthy)
paladinai-mongodb-express      "tini -- /docker-ent…"   mongodb-express     Up
paladinai-mongodb-replica-init "bash -c 'echo 'Wait…"   mongodb-replica-init Exited (0)
```

### 5. Access MongoDB

**MongoDB Connection:**
- Host: `localhost:27017`
- Username: `admin` (or your configured username)
- Password: `password123` (or your configured password)
- Database: `paladinai_checkpoints`
- Connection String: `mongodb://admin:password123@localhost:27017/paladinai_checkpoints?authSource=admin&replicaSet=rs0`

**MongoDB Express Web Interface:**
- URL: http://localhost:8081
- Username: `admin` (or your configured username)
- Password: `express123` (or your configured password)

## Architecture

### Services

1. **mongodb** - Main MongoDB 7.0 instance
   - Configured as single-node replica set (`rs0`)
   - Persistent data storage in `./data/mongodb`
   - Health checks and automatic restart
   - Authentication enabled

2. **mongodb-express** - Web administration interface
   - Accessible at http://localhost:8081
   - Basic authentication enabled
   - Connected to main MongoDB instance

3. **mongodb-replica-init** - One-time initialization service
   - Initializes replica set configuration
   - Runs database setup scripts
   - Exits after successful initialization

### Data Persistence

- **Volume Mount**: `./data/mongodb:/data/db`
- **Backup Location**: `./data/mongodb` (can be backed up/restored)
- **Permissions**: Ensure Docker has write access to `./data` directory

### Network Configuration

- **Network**: `paladinai-network` (bridge)
- **MongoDB Port**: 27017 (exposed to host)
- **Express Port**: 8081 (exposed to host)
- **Internal Communication**: Services communicate via Docker network

## Database Schema

### Collections

LangGraph AsyncMongoDBSaver automatically creates and manages these collections:

1. **checkpoints_aio** - Main checkpoint storage
   - Stores LangGraph workflow state snapshots
   - Contains checkpoint metadata and state
   - Indexed for optimal query performance

2. **checkpoint_writes_aio** - Checkpoint write operations
   - Stores individual channel writes for each checkpoint
   - Tracks task execution and channel updates
   - Used for checkpoint reconstruction

### Indexes for checkpoints_aio

1. **thread_ns_checkpoint_idx** - `{thread_id: 1, checkpoint_ns: 1, checkpoint_id: 1}`
   - Primary query pattern for checkpoint retrieval

2. **parent_checkpoint_idx** - `{parent_checkpoint_id: 1}` (sparse)
   - Checkpoint hierarchy navigation

3. **type_idx** - `{type: 1}`
   - Filtering by checkpoint type

### Indexes for checkpoint_writes_aio

1. **thread_ns_checkpoint_writes_idx** - `{thread_id: 1, checkpoint_ns: 1, checkpoint_id: 1}`
   - Retrieving writes for specific checkpoints

2. **task_id_idx** - `{task_id: 1}`
   - Task-based write queries

3. **channel_idx** - `{channel: 1}`
   - Channel-specific write queries

Note: LangGraph manages its own cleanup and TTL policies internally

## Operations

### Starting Services

```bash
cd infra
docker-compose up -d
```

### Stopping Services

```bash
cd infra
docker-compose down
```

### Viewing Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f mongodb
docker-compose logs -f mongodb-express
```

### Restarting Services

```bash
# Restart all
docker-compose restart

# Restart specific service
docker-compose restart mongodb
```

### Reinitializing Database

```bash
# Stop services
docker-compose down

# Remove data (WARNING: This deletes all data)
sudo rm -rf data/mongodb

# Start services (will reinitialize)
docker-compose up -d
```

## Troubleshooting

### Common Issues

1. **Port 27017 already in use**
   ```bash
   # Check what's using the port
   lsof -i :27017
   
   # Stop local MongoDB if running
   brew services stop mongodb-community  # macOS
   sudo systemctl stop mongod            # Linux
   ```

2. **Permission denied on data directory**
   ```bash
   # Create data directory with proper permissions
   mkdir -p data/mongodb
   chmod 755 data/mongodb
   ```

3. **Replica set initialization failed**
   ```bash
   # Check initialization logs
   docker-compose logs mongodb-replica-init
   
   # Manually reinitialize
   docker-compose restart mongodb-replica-init
   ```

4. **MongoDB Express not accessible**
   ```bash
   # Check if MongoDB is healthy
   docker-compose ps
   
   # Restart MongoDB Express
   docker-compose restart mongodb-express
   ```

### Health Checks

```bash
# Check MongoDB health
docker exec paladinai-mongodb mongosh --eval "db.adminCommand('ping')"

# Check replica set status
docker exec paladinai-mongodb mongosh -u admin -p password123 --authenticationDatabase admin --eval "rs.status()"

# Test checkpoint collection
docker exec paladinai-mongodb mongosh -u admin -p password123 --authenticationDatabase admin paladinai_checkpoints --eval "db.langgraph_checkpoints.stats()"
```

## Security Considerations

### Development Environment
- Default credentials are used for ease of setup
- No TLS/SSL encryption (not recommended for production)
- Basic authentication for MongoDB Express

### Production Environment
- Change all default passwords
- Enable TLS/SSL encryption
- Use MongoDB Atlas or properly secured self-hosted instance
- Implement network security (VPC, firewalls)
- Regular security updates

## Backup and Recovery

### Backup
```bash
# Create backup
docker exec paladinai-mongodb mongodump --uri="mongodb://admin:password123@localhost:27017/paladinai_checkpoints?authSource=admin" --out=/tmp/backup

# Copy backup from container
docker cp paladinai-mongodb:/tmp/backup ./backup-$(date +%Y%m%d)
```

### Restore
```bash
# Copy backup to container
docker cp ./backup-20240101 paladinai-mongodb:/tmp/restore

# Restore backup
docker exec paladinai-mongodb mongorestore --uri="mongodb://admin:password123@localhost:27017/paladinai_checkpoints?authSource=admin" /tmp/restore/paladinai_checkpoints
```

## Monitoring

### Key Metrics
- Connection count
- Database size
- Index usage
- Query performance
- Replica set health

### Monitoring Commands
```bash
# Connection stats
docker exec paladinai-mongodb mongosh -u admin -p password123 --authenticationDatabase admin --eval "db.serverStatus().connections"

# Database stats
docker exec paladinai-mongodb mongosh -u admin -p password123 --authenticationDatabase admin paladinai_checkpoints --eval "db.stats()"

# Collection stats
docker exec paladinai-mongodb mongosh -u admin -p password123 --authenticationDatabase admin paladinai_checkpoints --eval "db.langgraph_checkpoints.stats()"
```
