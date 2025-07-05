# Valkey (Redis-Compatible) Cache & Queue Documentation

## Service Overview
Valkey is a high-performance, Redis-compatible in-memory data store used for caching, session management, and message queuing in the mock infrastructure. It's a fork of Redis that maintains full compatibility while offering enhanced features.

## Service Details

### Container Information
- **Image**: `valkey/valkey:7-alpine`
- **Container Name**: `mock-valkey`
- **Version**: Valkey 7.x (Alpine Linux)
- **Network**: `mock-network`
- **User**: `999:999` (valkey user)

### Port Configuration
- **Internal Port**: 6379 (Redis-compatible default)
- **External Port**: 6380
- **Connection String**: `redis://localhost:6380`

### Configuration
- **Persistence**: AOF (Append Only File) enabled
- **Log File**: `/logs/valkey.log`
- **Data Directory**: `/data` (persistent volume)

### Volume Mounts
- `valkey-data:/data` - Persistent data storage
- `./logs:/logs:rw` - Log file directory

## Data Structures and Usage Patterns

### 1. Caching Layer

#### User Data Cache
```redis
# Key pattern: user:{id}
# TTL: 300 seconds (5 minutes)
# Example:
SET user:123 '{"id":123,"name":"John Doe","email":"john@example.com"}' EX 300

# Bulk user cache
# Key pattern: users:page:{page}:{limit}
SET users:page:1:20 '[{...}]' EX 300
```

#### API Response Cache
```redis
# Key pattern: api:{endpoint}:{params_hash}
# TTL: 600 seconds (10 minutes)
SET api:data:a1b2c3d4 '{"data":[...],"timestamp":1234567890}' EX 600

# Cache with tags for invalidation
SADD cache:tags:users api:users:list api:users:detail:123
```

#### Session Storage
```redis
# Key pattern: session:{session_id}
# TTL: 3600 seconds (1 hour)
SET session:abc123def456 '{"user_id":123,"roles":["user"],"created_at":1234567890}' EX 3600
```

### 2. Queue System

#### Priority Queues
```redis
# High priority queue
LPUSH queue:high '{"id":"uuid","type":"critical","payload":{...}}'

# Medium priority queue
LPUSH queue:medium '{"id":"uuid","type":"normal","payload":{...}}'

# Low priority queue
LPUSH queue:low '{"id":"uuid","type":"batch","payload":{...}}'

# Dead letter queue
LPUSH queue:dead-letter '{"id":"uuid","error":"timeout","original":{...}}'
```

#### Queue Processing
```redis
# Blocking pop with timeout (30 seconds)
BLPOP queue:high queue:medium queue:low 30

# Move to processing set
SADD queue:processing:worker1 "message_id"

# Remove after processing
SREM queue:processing:worker1 "message_id"
```

### 3. Rate Limiting

#### API Rate Limits
```redis
# Key pattern: rate:api:{client_id}:{endpoint}
# Sliding window rate limiting
ZADD rate:api:client123:data 1234567890 "request_id"
ZREMRANGEBYSCORE rate:api:client123:data 0 (1234567890-60)
ZCARD rate:api:client123:data
```

#### User Action Limits
```redis
# Key pattern: limit:action:{user_id}:{action}
# Simple counter with TTL
INCR limit:action:123:login
EXPIRE limit:action:123:login 3600
```

### 4. Real-time Features

#### Pub/Sub Channels
```redis
# Event notifications
PUBLISH events:user:created '{"user_id":123,"timestamp":1234567890}'
PUBLISH events:data:updated '{"id":456,"changes":{...}}'

# Metrics publishing
PUBLISH metrics:requests '{"service":"backend","count":100,"timestamp":1234567890}'
```

#### Sorted Sets for Rankings
```redis
# Key pattern: ranking:{type}
# User activity ranking
ZINCRBY ranking:active:users 1 "user:123"
ZREVRANGE ranking:active:users 0 9 WITHSCORES
```

### 5. Distributed Locks

#### Lock Implementation
```redis
# Key pattern: lock:{resource}
# NX: Only set if not exists, EX: Expiry time
SET lock:database:migration "worker1" NX EX 30

# Release lock (Lua script for atomicity)
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end
```

## Performance Metrics

### Redis Exporter Metrics
The redis-exporter container exposes metrics on port 9121:

#### Key Metrics
1. **Connection Metrics**:
   - `redis_connected_clients` - Number of connected clients
   - `redis_blocked_clients` - Clients blocked on lists
   - `redis_connected_slaves` - Number of replicas

2. **Memory Metrics**:
   - `redis_memory_used_bytes` - Total memory usage
   - `redis_memory_used_rss_bytes` - RSS memory
   - `redis_memory_fragmentation_ratio` - Memory fragmentation
   - `redis_memory_used_dataset_bytes` - Dataset size

3. **Performance Metrics**:
   - `redis_commands_processed_total` - Total commands
   - `redis_instantaneous_ops_per_sec` - Current ops/sec
   - `redis_hit_rate` - Cache hit rate
   - `redis_evicted_keys_total` - Evicted keys

4. **Persistence Metrics**:
   - `redis_aof_last_rewrite_duration_sec` - AOF rewrite time
   - `redis_aof_rewrite_in_progress` - AOF rewrite status
   - `redis_rdb_last_save_timestamp_seconds` - Last save time

### Performance Characteristics
| Operation | Latency (P50) | Latency (P99) | Throughput |
|-----------|---------------|---------------|------------|
| GET | 0.1ms | 0.5ms | 100k ops/s |
| SET | 0.2ms | 1ms | 80k ops/s |
| LPUSH | 0.2ms | 1ms | 70k ops/s |
| ZADD | 0.3ms | 1.5ms | 50k ops/s |

## Cache Invalidation Strategies

### 1. TTL-Based Expiration
```redis
# Simple TTL
SET key "value" EX 300

# Sliding expiration
EXPIRE key 300
```

### 2. Pattern-Based Deletion
```redis
# Delete by pattern (use with caution)
EVAL "return redis.call('del', unpack(redis.call('keys', ARGV[1])))" 0 "cache:user:*"
```

### 3. Tag-Based Invalidation
```redis
# Get all keys with tag
SMEMBERS cache:tags:users

# Delete tagged keys
for key in tags:
    DEL key
    
# Clear tag set
DEL cache:tags:users
```

### 4. Event-Driven Invalidation
```javascript
// On data update
redis.del(`user:${userId}`);
redis.del(`api:users:detail:${userId}`);
redis.publish('cache:invalidate', JSON.stringify({
    type: 'user',
    id: userId
}));
```

## Queue Management

### Queue Monitoring
```redis
# Queue lengths
LLEN queue:high
LLEN queue:medium
LLEN queue:low
LLEN queue:dead-letter

# Processing status
SCARD queue:processing:worker1
SMEMBERS queue:processing:worker1
```

### Queue Health Checks
```redis
# Oldest message age
LINDEX queue:high -1
JSON.parse -> check timestamp

# Dead letter queue size alert
if LLEN queue:dead-letter > 100:
    alert("Dead letter queue growing")
```

### Queue Patterns

#### Reliable Queue Pattern
```lua
-- Atomic move from queue to processing
local message = redis.call('RPOPLPUSH', 'queue:high', 'queue:processing')
if message then
    redis.call('EXPIRE', 'queue:processing', 300)
    return message
end
return nil
```

#### Priority Queue Pattern
```javascript
// Process queues in priority order
const queues = ['queue:high', 'queue:medium', 'queue:low'];
const message = await redis.blpop(...queues, 30);
```

## Configuration Best Practices

### Memory Management
```conf
# Maximum memory policy
maxmemory 256mb
maxmemory-policy allkeys-lru

# Memory sampling
maxmemory-samples 5

# Eviction pool size
lazyfree-lazy-eviction yes
```

### Persistence Configuration
```conf
# AOF settings
appendonly yes
appendfsync everysec
no-appendfsync-on-rewrite no
auto-aof-rewrite-percentage 100
auto-aof-rewrite-min-size 64mb

# RDB settings (disabled for AOF)
save ""
```

### Performance Tuning
```conf
# TCP settings
tcp-backlog 511
timeout 0
tcp-keepalive 300

# Threading
io-threads 4
io-threads-do-reads yes

# Client output buffer
client-output-buffer-limit normal 0 0 0
client-output-buffer-limit replica 256mb 64mb 60
client-output-buffer-limit pubsub 32mb 8mb 60
```

## Monitoring and Alerting

### Key Metrics to Monitor

1. **Memory Usage**:
   - Alert if > 80% of maxmemory
   - Track fragmentation ratio
   - Monitor eviction rate

2. **Performance**:
   - Commands/sec threshold
   - Slow queries (SLOWLOG)
   - Client connections

3. **Queue Health**:
   - Queue depth thresholds
   - Processing time
   - Dead letter accumulation

4. **Cache Efficiency**:
   - Hit rate < 70% warning
   - Eviction rate spikes
   - Key expiration patterns

### Alert Rules
```yaml
# High memory usage
- alert: ValkeyHighMemory
  expr: redis_memory_used_bytes / redis_memory_max_bytes > 0.8
  for: 5m
  severity: warning

# High eviction rate
- alert: ValkeyHighEviction
  expr: rate(redis_evicted_keys_total[5m]) > 100
  for: 5m
  severity: warning

# Queue backup
- alert: QueueBackup
  expr: redis_list_length{list="queue:high"} > 1000
  for: 5m
  severity: critical
```

## Troubleshooting

### Common Issues

1. **Memory Pressure**
   ```bash
   # Check memory usage
   docker exec mock-valkey valkey-cli INFO memory
   
   # Find large keys
   docker exec mock-valkey valkey-cli --bigkeys
   
   # Memory doctor
   docker exec mock-valkey valkey-cli MEMORY DOCTOR
   ```

2. **Slow Operations**
   ```bash
   # Check slow log
   docker exec mock-valkey valkey-cli SLOWLOG GET 10
   
   # Monitor commands
   docker exec mock-valkey valkey-cli MONITOR
   ```

3. **Connection Issues**
   ```bash
   # Check connected clients
   docker exec mock-valkey valkey-cli CLIENT LIST
   
   # Check configuration
   docker exec mock-valkey valkey-cli CONFIG GET maxclients
   ```

4. **Queue Problems**
   ```bash
   # Check queue lengths
   docker exec mock-valkey valkey-cli LLEN queue:high
   
   # Inspect dead letters
   docker exec mock-valkey valkey-cli LRANGE queue:dead-letter 0 10
   ```

### Debug Commands
```bash
# Interactive CLI
docker exec -it mock-valkey valkey-cli

# Check server info
INFO server
INFO clients
INFO memory
INFO stats

# Test connectivity
PING

# Check persistence
CONFIG GET appendonly
CONFIG GET save

# Flush cache (careful!)
FLUSHDB  # Current database
FLUSHALL # All databases
```

## Security Considerations

1. **Access Control**:
   - No password set (development only)
   - Bind to Docker network only
   - No external exposure

2. **Command Restrictions**:
   - Dangerous commands disabled in production
   - FLUSHDB, FLUSHALL, CONFIG restricted

3. **Network Security**:
   - TLS not configured (internal network)
   - Port 6380 mapped for local access only

4. **Data Protection**:
   - AOF persistence for durability
   - Regular backups recommended

## Backup and Recovery

### Backup Methods

1. **AOF Backup**:
   ```bash
   # Stop writes
   docker exec mock-valkey valkey-cli BGREWRITEAOF
   
   # Copy AOF file
   docker cp mock-valkey:/data/appendonly.aof ./backup/
   ```

2. **RDB Snapshot**:
   ```bash
   # Create snapshot
   docker exec mock-valkey valkey-cli BGSAVE
   
   # Copy RDB file
   docker cp mock-valkey:/data/dump.rdb ./backup/
   ```

### Recovery Process
```bash
# Stop Valkey
docker stop mock-valkey

# Restore data file
docker cp ./backup/appendonly.aof mock-valkey:/data/

# Start Valkey
docker start mock-valkey
```

## Integration Patterns

### Backend Service Integration
- Connection pool size: 10 per service
- Retry logic with exponential backoff
- Circuit breaker for failures

### Monitoring Integration
- Metrics scraped every 15 seconds
- Alerts routed through Alertmanager
- Grafana dashboards for visualization