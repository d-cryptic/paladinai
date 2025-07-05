# Backend Services Documentation

## Service Overview
The Backend services consist of two Node.js/Express API instances (backend-1 and backend-2) that handle business logic, database operations, and caching. They are load-balanced by Nginx and provide RESTful APIs to the frontend service.

## Service Architecture

### Instance 1: backend-1
- **Container Name**: `mock-backend-1`
- **Error Rate**: 15% (moderately stable)
- **External Port**: 4001
- **Role**: Primary backend instance

### Instance 2: backend-2
- **Container Name**: `mock-backend-2`
- **Error Rate**: 20% (intentionally flakier)
- **External Port**: 4002
- **Role**: Secondary backend instance for load distribution

## Service Details

### Container Information
- **Image**: Custom built from `./backend/Dockerfile`
- **Base Image**: `node:18-alpine`
- **Network**: `mock-network`
- **Internal Port**: 4000 (both instances)

### Environment Variables
| Variable | backend-1 | backend-2 | Description |
|----------|-----------|-----------|-------------|
| SERVICE_NAME | backend-1 | backend-2 | Service identifier |
| DB_HOST | postgres | postgres | PostgreSQL host |
| DB_PORT | 5432 | 5432 | PostgreSQL port |
| REDIS_HOST | valkey | valkey | Redis/Valkey host |
| REDIS_PORT | 6379 | 6379 | Redis/Valkey port |
| ERROR_RATE | 0.15 | 0.2 | Error simulation rate |
| LOG_LEVEL | debug | debug | Logging verbosity |
| PORT | 4000 | 4000 | Service port |

### Dependencies
- postgres (PostgreSQL database)
- valkey (Redis-compatible cache/queue)

## API Endpoints

### 1. GET /health
- **Description**: Health check endpoint
- **Response**: Service health status
- **Example Response**:
  ```json
  {
    "status": "healthy",
    "service": "backend-1",
    "uptime": 123456,
    "timestamp": "2024-01-01T12:00:00Z",
    "dependencies": {
      "database": "connected",
      "cache": "connected"
    }
  }
  ```

### 2. GET /metrics
- **Description**: Prometheus metrics endpoint
- **Response**: Prometheus-formatted metrics
- **Key Metrics**:
  - `http_requests_total` - Total HTTP requests
  - `http_request_duration_seconds` - Request duration
  - `db_query_duration_seconds` - Database query duration
  - `cache_hits_total` - Cache hit count
  - `cache_misses_total` - Cache miss count
  - `queue_size` - Current queue size
  - `active_connections` - Active DB connections

### 3. GET /api/backend/status
- **Description**: Detailed service status
- **Response**: Extended health information
- **Example Response**:
  ```json
  {
    "status": "operational",
    "service": "backend-1",
    "version": "1.0.0",
    "uptime": 123456,
    "connections": {
      "database": {
        "status": "connected",
        "latency": 2.5,
        "pool_size": 10,
        "active_connections": 3
      },
      "cache": {
        "status": "connected",
        "latency": 0.5,
        "memory_usage": "45MB"
      }
    },
    "metrics": {
      "requests_per_minute": 450,
      "error_rate": 0.15,
      "average_response_time": 125
    }
  }
  ```

### 4. GET /api/backend/data
- **Description**: Fetch data with caching
- **Query Parameters**:
  - `limit` - Number of records (default: 100)
  - `offset` - Pagination offset (default: 0)
  - `nocache` - Bypass cache (boolean)
- **Response**: JSON data array
- **Caching**: 5-minute TTL in Valkey

### 5. GET /api/backend/users
- **Description**: Fetch user data
- **Features**:
  - Database query simulation
  - Redis caching
  - Pagination support
- **Example Response**:
  ```json
  {
    "users": [
      {
        "id": 1,
        "name": "John Doe",
        "email": "john@example.com",
        "created_at": "2024-01-01T00:00:00Z"
      }
    ],
    "total": 150,
    "page": 1,
    "limit": 20
  }
  ```

### 6. POST /api/backend/data
- **Description**: Create new data entry
- **Request Body**:
  ```json
  {
    "name": "string",
    "value": "any",
    "metadata": {}
  }
  ```
- **Response**: Created resource with ID
- **Side Effects**:
  - Database write
  - Cache invalidation
  - Queue message

### 7. PUT /api/backend/data/:id
- **Description**: Update existing data
- **Parameters**: `id` - Resource ID
- **Cache**: Invalidates related cache entries

### 8. DELETE /api/backend/data/:id
- **Description**: Delete data entry
- **Parameters**: `id` - Resource ID
- **Cache**: Clears from cache

### 9. GET /api/backend/error/:type
- **Description**: Simulate specific errors
- **Types**:
  - `timeout` - 10-second delay
  - `crash` - Process exit (auto-restarts)
  - `memory` - Memory spike
  - `database` - DB connection error

### 10. POST /api/backend/queue
- **Description**: Add item to queue
- **Request Body**:
  ```json
  {
    "type": "string",
    "payload": {},
    "priority": "high|medium|low"
  }
  ```
- **Queue**: Stored in Valkey

## Database Operations

### Connection Pool
- **Size**: 10 connections
- **Timeout**: 5 seconds
- **Retry**: 3 attempts
- **Backoff**: Exponential

### Query Patterns
1. **Simple Queries**:
   - SELECT with pagination
   - INSERT with returning ID
   - UPDATE with affected rows
   - DELETE with cascade

2. **Complex Queries**:
   - JOIN operations
   - Aggregations
   - Transactions
   - Stored procedures (simulated)

### Performance Metrics
- **Average Query Time**: 10-50ms
- **Slow Query Threshold**: 100ms
- **Connection Overhead**: 5ms

## Caching Strategy

### Cache Configuration
- **Provider**: Valkey (Redis-compatible)
- **Default TTL**: 300 seconds (5 minutes)
- **Eviction Policy**: LRU
- **Max Memory**: 256MB

### Cache Patterns
1. **Read-Through Cache**:
   ```javascript
   // Check cache first
   const cached = await redis.get(key);
   if (cached) return cached;
   
   // Fetch from DB
   const data = await db.query(...);
   
   // Store in cache
   await redis.setex(key, 300, data);
   return data;
   ```

2. **Write-Through Cache**:
   - Update database first
   - Update cache on success
   - Invalidate on failure

3. **Cache Invalidation**:
   - On UPDATE: Clear specific keys
   - On DELETE: Clear related keys
   - On bulk operations: Clear pattern

### Cache Metrics
- **Hit Rate**: ~70% (typical)
- **Miss Penalty**: +50-200ms
- **Memory Usage**: Monitored via Redis exporter

## Queue Operations

### Queue Configuration
- **Backend**: Valkey
- **Queue Names**:
  - `high-priority`
  - `medium-priority`
  - `low-priority`
  - `dead-letter`

### Message Format
```json
{
  "id": "uuid",
  "type": "event_type",
  "payload": {},
  "timestamp": "2024-01-01T00:00:00Z",
  "attempts": 0,
  "max_attempts": 3
}
```

### Processing Logic
1. Pull from highest priority queue
2. Process with timeout (30s)
3. Acknowledge on success
4. Retry with backoff on failure
5. Move to dead-letter after max attempts

## Error Handling

### Error Simulation
Based on ERROR_RATE environment variable:
- **backend-1**: 15% error rate
- **backend-2**: 20% error rate

### Error Types
1. **HTTP Errors**:
   - 400 Bad Request (validation)
   - 404 Not Found
   - 500 Internal Server Error
   - 502 Bad Gateway (DB issues)
   - 503 Service Unavailable

2. **Application Errors**:
   - Database connection failures
   - Cache timeouts
   - Queue overflow
   - Memory exhaustion

3. **Network Errors**:
   - Connection refused
   - Timeout
   - DNS resolution

### Error Response Format
```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message",
    "details": {},
    "timestamp": "2024-01-01T00:00:00Z",
    "request_id": "uuid"
  }
}
```

## Performance Characteristics

### Resource Usage
| Metric | Typical | Peak |
|--------|---------|------|
| CPU | 5-10% | 50% |
| Memory | 128MB | 256MB |
| Connections | 10-20 | 100 |
| Disk I/O | Low | Medium |

### Response Times
| Operation | P50 | P95 | P99 |
|-----------|-----|-----|-----|
| Simple GET | 20ms | 100ms | 500ms |
| DB Query | 50ms | 200ms | 1s |
| Cache Hit | 5ms | 20ms | 50ms |
| Complex Operation | 100ms | 500ms | 2s |

## Monitoring and Alerting

### Key Metrics to Monitor
1. **Availability**:
   - Uptime percentage
   - Health check success rate
   - Container restarts

2. **Performance**:
   - Response time percentiles
   - Throughput (req/s)
   - Error rate

3. **Resources**:
   - CPU usage
   - Memory usage
   - Connection pool usage

4. **Dependencies**:
   - Database connection status
   - Cache hit rate
   - Queue depth

### Alert Thresholds

| Alert | Condition | Severity |
|-------|-----------|----------|
| High Error Rate | >10% for 2min | Warning |
| Service Down | Health check fails 1min | Critical |
| High Response Time | P95 >2s for 5min | Warning |
| DB Connection Lost | No queries for 2min | Critical |
| Cache Miss Rate High | >50% for 5min | Warning |
| Queue Overflow | >1000 items | Warning |

## Load Balancing

### Nginx Configuration
- **Algorithm**: Least connections
- **Health Checks**: Every 30s
- **Fail Timeout**: 30s
- **Max Fails**: 3

### Session Affinity
- Not implemented (stateless design)
- All state in database/cache

### Request Distribution
- backend-1: ~50% (when healthy)
- backend-2: ~50% (more errors)

## Troubleshooting Guide

### Common Issues

1. **High Error Rate**
   ```bash
   # Check error logs
   docker logs mock-backend-1 | grep ERROR
   
   # Check error rate metric
   curl http://localhost:4001/metrics | grep error_rate
   ```

2. **Database Connection Issues**
   ```bash
   # Test DB connectivity
   docker exec mock-backend-1 nc -zv postgres 5432
   
   # Check connection pool
   curl http://localhost:4001/api/backend/status | jq .connections.database
   ```

3. **Cache Problems**
   ```bash
   # Test Redis connectivity
   docker exec mock-backend-1 nc -zv valkey 6379
   
   # Check cache metrics
   curl http://localhost:4001/metrics | grep cache
   ```

4. **Memory Leaks**
   ```bash
   # Monitor memory usage
   docker stats mock-backend-1
   
   # Check for large objects
   docker exec mock-backend-1 node --inspect
   ```

### Debug Commands
```bash
# Live logs
docker logs -f mock-backend-1

# Access container
docker exec -it mock-backend-1 sh

# Test endpoints
curl http://localhost:4001/health
curl http://localhost:4001/api/backend/status

# Force garbage collection
docker exec mock-backend-1 kill -USR1 1
```

## Security Considerations

1. **Authentication**: Bearer token validation (simulated)
2. **Authorization**: Role-based access control
3. **Input Validation**: All inputs sanitized
4. **SQL Injection**: Parameterized queries
5. **Rate Limiting**: Handled by Nginx
6. **Secrets Management**: Environment variables
7. **Logging**: No sensitive data logged

## Scaling Considerations

### Horizontal Scaling
- Add more backend instances
- Update Nginx upstream config
- Ensure cache consistency

### Vertical Scaling
- Increase container resources
- Adjust connection pool sizes
- Tune Node.js memory limits

### Database Scaling
- Read replicas for queries
- Connection pooling
- Query optimization

### Cache Scaling
- Redis cluster mode
- Increase memory allocation
- Cache partitioning