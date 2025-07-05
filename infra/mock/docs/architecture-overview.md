# Mock Infrastructure Architecture Overview

## Executive Summary

This mock infrastructure simulates a production-grade 3-tier web application with comprehensive monitoring, logging, and alerting capabilities. It's designed to test PaladinAI's incident response and monitoring capabilities in a controlled environment with realistic failure scenarios and performance characteristics.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              External Access                                 │
│  http://localhost:3001 (Frontend)  http://localhost:8080 (Nginx)           │
│  http://localhost:9090 (Prometheus) http://localhost:3000 (Grafana)        │
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Load Balancer / Proxy Layer                         │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    Nginx (mock-nginx:8080)                           │   │
│  │  - Reverse Proxy    - Load Balancing    - Rate Limiting             │   │
│  │  - SSL Termination  - Health Checks     - Request Routing           │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
                    │                                   │
                    ▼                                   ▼
┌─────────────────────────────┐       ┌─────────────────────────────────────┐
│     Frontend Layer          │       │        Application Layer            │
│                             │       │                                      │
│  ┌───────────────────────┐  │       │  ┌────────────────┐ ┌────────────┐ │
│  │  Frontend Service     │  │       │  │  Backend-1     │ │ Backend-2  │ │
│  │  (mock-frontend:3001) │  │       │  │  (Port: 4001)  │ │(Port: 4002)│ │
│  │  - React-like SPA     │  │       │  │  - API Server  │ │- API Server│ │
│  │  - Error: 10%         │  │       │  │  - Error: 15%  │ │- Error: 20%│ │
│  └───────────────────────┘  │       │  └────────────────┘ └────────────┘ │
└─────────────────────────────┘       └─────────────────────────────────────┘
                    │                                   │
                    └───────────────┬───────────────────┘
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Data Layer                                         │
│                                                                              │
│  ┌─────────────────────────┐          ┌──────────────────────────────────┐ │
│  │   PostgreSQL Database   │          │   Valkey (Redis) Cache/Queue     │ │
│  │   (mock-postgres:5433)  │          │   (mock-valkey:6380)             │ │
│  │   - Primary datastore   │          │   - Caching layer                │ │
│  │   - ACID compliance     │          │   - Message queue                 │ │
│  │   - 100 connections     │          │   - Session storage              │ │
│  └─────────────────────────┘          └──────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Observability Stack                                  │
│                                                                              │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────────┐  │
│  │ Prometheus   │ │ Alertmanager │ │ Loki         │ │ Grafana          │  │
│  │ (Port: 9090) │ │ (Port: 9093) │ │ (Port: 3100) │ │ (Port: 3000)     │  │
│  │ - Metrics    │ │ - Alerting   │ │ - Logs       │ │ - Visualization  │  │
│  └──────────────┘ └──────────────┘ └──────────────┘ └──────────────────┘  │
│                                                                              │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────────┐  │
│  │ Promtail     │ │ Node Export  │ │ PG Exporter  │ │ Redis Exporter   │  │
│  │ - Log ship   │ │ (Port: 9100) │ │ (Port: 9187) │ │ (Port: 9121)     │  │
│  └──────────────┘ └──────────────┘ └──────────────┘ └──────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Component Overview

### 1. Frontend Layer
- **Technology**: Node.js/Express simulating React SPA
- **Purpose**: User interface and client-side logic
- **Error Rate**: 10% (configurable)
- **Key Features**:
  - Server-side rendered pages
  - API consumption from backend
  - Metrics exposition
  - Structured logging

### 2. Application Layer
- **Technology**: Node.js/Express API servers
- **Instances**: 2 (load balanced)
- **Purpose**: Business logic and API endpoints
- **Error Rates**: 15% (backend-1), 20% (backend-2)
- **Key Features**:
  - RESTful API
  - Database operations
  - Caching logic
  - Queue processing

### 3. Data Layer
- **Database**: PostgreSQL 15
  - Primary data storage
  - Relational data model
  - Connection pooling
  - Full SQL support
  
- **Cache/Queue**: Valkey (Redis-compatible)
  - High-performance caching
  - Message queuing
  - Session management
  - Pub/Sub capabilities

### 4. Infrastructure Layer
- **Load Balancer**: Nginx
  - Request routing
  - Load distribution (least_conn)
  - Rate limiting
  - Health checks
  - SSL termination (configured)

### 5. Observability Stack
- **Metrics**: Prometheus
  - Time-series database
  - Pull-based collection
  - PromQL query language
  - Recording rules

- **Alerting**: Alertmanager
  - Alert routing
  - Grouping and silencing
  - Multiple notification channels
  - Alert deduplication

- **Logging**: Loki + Promtail
  - Log aggregation
  - Label-based indexing
  - LogQL query language
  - Multi-line support

- **Visualization**: Grafana
  - Unified dashboards
  - Multi-datasource support
  - Alert visualization
  - Query builder

## Data Flow

### 1. Request Flow
```
Client → Nginx → Backend (1 or 2) → Database/Cache → Response
         ↓
     Frontend (for UI requests)
```

### 2. Metrics Flow
```
Services → Prometheus (scrape) → Grafana (query)
                ↓
          Alertmanager (alerts)
```

### 3. Logging Flow
```
Services → Log Files → Promtail → Loki → Grafana
```

## Key Architectural Decisions

### 1. Microservices Pattern
- **Decision**: Separate services for frontend, backend, and data
- **Rationale**: Enables independent scaling and failure isolation
- **Trade-offs**: Increased complexity, network overhead

### 2. Load Balancing Strategy
- **Decision**: Least connections algorithm
- **Rationale**: Better distribution for varying request processing times
- **Alternative**: Round-robin (simpler but less adaptive)

### 3. Monitoring Approach
- **Decision**: Pull-based metrics (Prometheus)
- **Rationale**: Better for dynamic environments, reduced network load
- **Alternative**: Push-based (simpler but less flexible)

### 4. Logging Architecture
- **Decision**: Centralized with Loki
- **Rationale**: Prometheus-like experience, cost-effective
- **Alternative**: ELK stack (more features but heavier)

## Failure Scenarios

### Simulated Failures
1. **Random HTTP Errors**: Configurable error rates per service
2. **Timeout Simulation**: Deliberate slow responses
3. **Connection Failures**: Database/cache disconnections
4. **Resource Exhaustion**: Memory/CPU spikes
5. **Cascading Failures**: Error propagation between services

### Failure Injection Points
- Frontend: `/error/:code` endpoint
- Backend: `/api/backend/error/:type` endpoint
- Database: Connection pool exhaustion
- Cache: Memory pressure simulation

## Performance Characteristics

### Service Baselines
| Service | Avg Response Time | P95 Response Time | Throughput |
|---------|-------------------|-------------------|------------|
| Frontend | 50ms | 200ms | 1000 req/s |
| Backend | 100ms | 500ms | 500 req/s |
| Database | 10ms | 50ms | 1000 qps |
| Cache | 1ms | 5ms | 10000 ops/s |

### Resource Limits
| Service | CPU Limit | Memory Limit | Connection Limit |
|---------|-----------|--------------|------------------|
| Frontend | 1 core | 256MB | 1000 |
| Backend | 1 core | 256MB | 1000 |
| PostgreSQL | 2 cores | 1GB | 100 |
| Valkey | 1 core | 256MB | 10000 |
| Nginx | 1 core | 128MB | 2048 |

## Monitoring Strategy

### Metrics Collection
- **Interval**: 15 seconds
- **Retention**: 200 hours (~8 days)
- **Coverage**: All services expose `/metrics` endpoint

### Alert Categories
1. **Infrastructure Alerts**:
   - Service down
   - High CPU/Memory
   - Disk space

2. **Application Alerts**:
   - High error rate
   - Slow response time
   - Queue backup

3. **Data Layer Alerts**:
   - Connection pool exhaustion
   - Slow queries
   - Cache miss rate

### Log Aggregation
- **Sources**: Application logs, system logs, container logs
- **Retention**: 7 days
- **Indexing**: By service, level, timestamp

## Security Considerations

### Network Security
- All services on isolated Docker network
- No direct external access to data layer
- Rate limiting on API endpoints

### Access Control
- Grafana: Authentication required
- Prometheus: Read-only API
- Database: Restricted user permissions

### Data Protection
- Logs sanitized of sensitive data
- Metrics don't include PII
- Encrypted connections (configurable)

## Scaling Considerations

### Horizontal Scaling
- Backend: Add more instances to Nginx upstream
- Frontend: Deploy behind CDN
- Database: Read replicas (not implemented)

### Vertical Scaling
- Adjust container resource limits
- Increase connection pools
- Tune cache sizes

### Bottlenecks
1. Database connection limit (100)
2. Nginx worker connections (1024)
3. Prometheus storage (time-series growth)

## Operational Procedures

### Deployment
```bash
cd infra/mock
make build
make up
make status
```

### Health Checks
- All services expose health endpoints
- Automated health monitoring via Prometheus
- Manual checks: `make health-check`

### Troubleshooting
1. Check service logs: `make logs-<service>`
2. View metrics: http://localhost:9090
3. Check alerts: http://localhost:9093
4. Analyze in Grafana: http://localhost:3000

### Maintenance
- Log rotation: Handled by Docker
- Metric retention: 200 hours automatic
- Database vacuum: PostgreSQL autovacuum

## Integration with PaladinAI

### Data Collection Points
1. **Metrics**: Prometheus API at :9090/api/v1/
2. **Logs**: Loki API at :3100/loki/api/v1/
3. **Alerts**: Alertmanager API at :9093/api/v1/
4. **Traces**: Not implemented (future: Jaeger/Tempo)

### Webhook Integration
- Alertmanager sends to webhook server
- Custom alert routing based on severity
- Structured webhook payloads

### RAG Data Sources
- Service documentation in `/docs`
- Runbook annotations in alerts
- Historical metrics for analysis

## Testing Scenarios

### Load Testing
```bash
make load-test  # Generates concurrent requests
```

### Chaos Testing
```bash
make simulate-errors  # Triggers various error conditions
```

### Performance Testing
- Gradual load increase
- Sustained load testing
- Spike testing

## Future Enhancements

### Planned Features
1. **Distributed Tracing**: Jaeger/Tempo integration
2. **Service Mesh**: Istio/Linkerd for advanced routing
3. **Auto-scaling**: Based on metrics
4. **Multi-region**: Geo-distributed setup

### Potential Improvements
1. **Security**: mTLS between services
2. **Resilience**: Circuit breakers
3. **Observability**: Custom business metrics
4. **Performance**: Read replicas, caching layers

## Conclusion

This mock infrastructure provides a comprehensive testing environment for PaladinAI with realistic failure scenarios, performance characteristics, and operational complexity. It simulates common production patterns while remaining simple enough to run on a single machine.

The architecture emphasizes observability, allowing PaladinAI to practice incident detection, root cause analysis, and automated remediation in a safe, controlled environment.