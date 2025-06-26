# Valkey Queue Infrastructure for PaladinAI

This directory contains the Docker Compose setup for Valkey (Redis-compatible) queue system used by PaladinAI for concurrent request processing.

## Components

- **Valkey Server**: High-performance key-value store for queue management
- **Valkey Commander**: Web-based management interface for monitoring queues

## Quick Start

1. **Copy environment configuration:**
   ```bash
   cp .env.example .env
   # Edit .env with your preferred settings
   ```

2. **Start the services:**
   ```bash
   docker-compose up -d
   ```

3. **Verify services are running:**
   ```bash
   docker-compose ps
   ```

## Service Endpoints

- **Valkey Server**: `localhost:6379`
- **Valkey Commander (Web UI)**: `http://localhost:8081`
  - Username: `admin`
  - Password: Set in `.env` file (default: `admin123`)

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `VALKEY_PASSWORD` | `paladinai_queue_pass` | Valkey server password |
| `VALKEY_COMMANDER_PASSWORD` | `admin123` | Web UI password |
| `QUEUE_DEFAULT_TIMEOUT` | `300` | Default job timeout (seconds) |
| `QUEUE_RESULT_TTL` | `3600` | Result retention time (seconds) |
| `QUEUE_FAILURE_TTL` | `86400` | Failed job retention time (seconds) |

### Valkey Configuration

The `valkey.conf` file contains optimized settings for queue operations:

- **Memory Management**: 512MB limit with LRU eviction
- **Persistence**: RDB snapshots + AOF logging
- **Performance**: Optimized for list operations (queues)
- **Security**: Password protection enabled

## Monitoring

### Health Checks

```bash
# Check Valkey health
docker-compose exec valkey valkey-cli ping

# View logs
docker-compose logs valkey
docker-compose logs valkey-commander
```

### Queue Monitoring

Use Valkey Commander web interface or CLI:

```bash
# Connect to Valkey CLI
docker-compose exec valkey valkey-cli -a paladinai_queue_pass

# List all keys (queues)
KEYS *

# Check queue length
LLEN queue_name

# View queue contents
LRANGE queue_name 0 -1
```

## Data Persistence

Queue data is persisted in Docker volumes:

- **Volume**: `paladinai-valkey-data`
- **Location**: `/data` inside container
- **Backup**: Use `docker volume` commands for backup/restore

## Troubleshooting

### Common Issues

1. **Connection refused**: Check if services are running with `docker-compose ps`
2. **Authentication failed**: Verify password in `.env` file
3. **Memory issues**: Adjust `maxmemory` in `valkey.conf`

### Performance Tuning

For high-throughput scenarios, consider:

1. Increasing `maxmemory` limit
2. Adjusting `maxclients` setting
3. Tuning persistence settings based on durability requirements

## Integration with PaladinAI

This Valkey instance is used by PaladinAI's queue system for:

- **Job Queuing**: Storing workflow execution requests
- **Result Storage**: Caching workflow results
- **Worker Coordination**: Managing worker processes
- **Status Tracking**: Monitoring job progress and failures
