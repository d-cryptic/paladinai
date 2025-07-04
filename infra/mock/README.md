# Mock Infrastructure for PaladinAI Testing

This mock infrastructure simulates a realistic 3-tier application with various failure scenarios for testing PaladinAI's monitoring and incident response capabilities.

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Frontend  │────▶│    Nginx    │────▶│   Backend   │
│   (React)   │     │(Load Balancer)    │  (Node.js)  │
└─────────────┘     └─────────────┘     └──────┬──────┘
                                               │
                                        ┌──────┴──────┐
                                        │             │
                                    ┌───▼───┐    ┌───▼───┐
                                    │Postgres│    │ Valkey │
                                    │   DB   │    │ Queue  │
                                    └────────┘    └────────┘
```

## Services

### Application Stack
- **Frontend**: Mock React application with error generation
- **Backend**: Two Node.js instances with load balancing
- **Nginx**: Reverse proxy and load balancer
- **PostgreSQL**: Database with sample data
- **Valkey**: Redis-compatible queue/cache

### Monitoring Stack
- **Prometheus**: Metrics collection
- **Loki**: Log aggregation
- **Promtail**: Log shipping
- **Alertmanager**: Alert routing
- **Node Exporter**: System metrics
- **Postgres Exporter**: Database metrics
- **Redis Exporter**: Cache metrics

## Features

### Error Simulation
- Configurable error rates (10-20% by default)
- Random timeouts and failures
- Database connection errors
- Cache misses
- Memory pressure simulation

### Metrics
Each service exposes metrics at `/metrics`:
- HTTP request duration and count
- Error rates
- Database query performance
- Cache hit/miss rates
- Queue sizes
- System metrics (CPU, memory, disk)

### Logging
All services log to `./logs/` with service-specific files:
- `frontend.log`, `frontend-error.log`
- `backend-1.log`, `backend-2.log`
- `nginx/access.log`, `nginx/error.log`
- `valkey.log`

### Alerts
Pre-configured alerts for:
- High error rates (>10%)
- Service downtime
- High response times (>2s)
- Database connection failures
- High memory/CPU usage (>85%)
- Cache miss rates (>50%)
- Queue growth

## Quick Start

```bash
# Start all services
make up

# Check status
make status

# View logs
make logs

# Generate test load
make load-test

# Simulate errors
make simulate-errors

# Stop services
make down

# Clean everything (including volumes)
make clean
```

## Service URLs

- Frontend: http://localhost:3001
- Backend (via LB): http://localhost:8080/api/backend/
- Nginx: http://localhost:8080
- Prometheus: http://localhost:9091
- Alertmanager: http://localhost:9093
- Loki: http://localhost:3100

## Testing Scenarios

### 1. Normal Operation
```bash
curl http://localhost:3001/api/data
curl http://localhost:8080/api/backend/data
```

### 2. Error Generation
```bash
# 404 errors
curl http://localhost:3001/error/404

# 500 errors
curl http://localhost:3001/error/500

# Timeouts
curl http://localhost:8080/api/backend/error/timeout
```

### 3. Load Testing
```bash
# Generate sustained load
make load-test

# Or use a tool like hey/ab
hey -z 30s -c 50 http://localhost:8080/api/backend/data
```

### 4. Health Checks
```bash
make health-check
```

## Configuration

### Error Rates
Adjust error rates via environment variables in `docker-compose.yml`:
- `ERROR_RATE`: Probability of random errors (0.0-1.0)

### Log Levels
Set `LOG_LEVEL` to: debug, info, warn, error

### Scaling
Add more backend instances:
```yaml
backend-3:
  build: ./backend
  # ... same config as backend-1/2
```

## Monitoring Integration

### Prometheus Queries
```promql
# Error rate by service
sum(rate(http_requests_total{status_code=~"5.."}[5m])) by (service)

# P95 response time
histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (service, le))

# Cache hit rate
sum(rate(cache_hits_total[5m])) / (sum(rate(cache_hits_total[5m])) + sum(rate(cache_misses_total[5m])))
```

### Loki Queries
```logql
# All errors
{job=~"frontend|backend"} |= "error"

# Slow queries
{service="backend"} |= "slow query"

# By severity
{service="frontend"} | json | level="error"
```

## Troubleshooting

### Services not starting
```bash
# Check logs
docker-compose logs [service-name]

# Verify ports are free
lsof -i :3001,8080,9091,3100
```

### No metrics/logs
- Ensure `./logs` directory exists and is writable
- Check service health endpoints
- Verify Prometheus targets: http://localhost:9091/targets

### High resource usage
- Reduce error rates
- Lower log levels
- Decrease load test intensity