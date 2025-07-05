# Prometheus & Alertmanager Documentation

## Service Overview
Prometheus is the central metrics collection and monitoring system, while Alertmanager handles alert routing, grouping, and notifications. Together they form the monitoring backbone of the infrastructure.

## Prometheus Service Details

### Container Information
- **Image**: `prom/prometheus:latest`
- **Container Name**: `prometheus` (implicit)
- **Network**: `mock-network`
- **Restart Policy**: `unless-stopped`

### Port Configuration
- **Internal Port**: 9090
- **External Port**: 9090
- **Web UI**: http://localhost:9090
- **API Endpoint**: http://localhost:9090/api/v1/

### Volume Mounts
- `./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro` - Main config
- `prometheus_data:/prometheus` - Time series data storage
- `./prometheus/alerts.yml:/etc/prometheus/alerts.yml` - Alert rules

### Command Line Arguments
```yaml
- '--config.file=/etc/prometheus/prometheus.yml'
- '--storage.tsdb.path=/prometheus'
- '--web.console.libraries=/etc/prometheus/console_libraries'
- '--web.console.templates=/etc/prometheus/consoles'
- '--storage.tsdb.retention.time=200h'
- '--web.enable-lifecycle'
```

## Prometheus Configuration

### Global Settings
```yaml
global:
  scrape_interval: 15s        # How often to scrape targets
  evaluation_interval: 15s    # How often to evaluate rules
  external_labels:
    monitor: 'mock-infrastructure'
    environment: 'testing'
```

### Scrape Configurations

#### 1. Self-Monitoring
```yaml
- job_name: 'prometheus'
  static_configs:
    - targets: ['localhost:9090']
      labels:
        service: 'prometheus'
```

#### 2. Application Services
```yaml
# Frontend
- job_name: 'frontend'
  static_configs:
    - targets: ['frontend:3000']
      labels:
        service: 'frontend'
        tier: 'frontend'
  metrics_path: '/metrics'

# Backend services
- job_name: 'backend'
  static_configs:
    - targets: ['backend-1:4000', 'backend-2:4000']
      labels:
        service: 'backend'
        tier: 'backend'
  metrics_path: '/metrics'
```

#### 3. Infrastructure Services
```yaml
# Nginx
- job_name: 'nginx'
  static_configs:
    - targets: ['nginx:9113']
      labels:
        service: 'nginx'
        tier: 'proxy'

# PostgreSQL
- job_name: 'postgres'
  static_configs:
    - targets: ['postgres-exporter:9187']
      labels:
        service: 'postgres'
        tier: 'database'

# Redis/Valkey
- job_name: 'redis'
  static_configs:
    - targets: ['redis-exporter:9121']
      labels:
        service: 'valkey'
        tier: 'cache'

# Node exporter
- job_name: 'node'
  static_configs:
    - targets: ['node-exporter:9100']
      labels:
        service: 'node-exporter'

# Loki
- job_name: 'loki'
  static_configs:
    - targets: ['loki:3100']
      labels:
        service: 'loki'
        tier: 'logging'
```

## Alert Rules Configuration

### Alert Rule Structure
```yaml
groups:
  - name: service_alerts
    interval: 30s
    rules:
      - alert: AlertName
        expr: PromQL expression
        for: duration
        labels:
          severity: critical|warning|info
          tier: application|infrastructure|database
        annotations:
          summary: "Short description"
          description: "Detailed description with {{ $value }}"
```

### Testing Alerts (Low Thresholds)

#### 1. Frontend Monitoring
```yaml
# Minor errors (>2% for testing)
- alert: FrontendMinorErrors
  expr: |
    (
      sum(rate(http_requests_total{status_code=~"5..", service="frontend"}[1m])) 
      /
      sum(rate(http_requests_total{service="frontend"}[1m]))
    ) > 0.02
  for: 30s
  labels:
    severity: info
    tier: frontend
    test_alert: "true"

# Any traffic detection
- alert: FrontendAnyTraffic
  expr: sum(rate(http_requests_total{service="frontend"}[1m])) > 0.01
  labels:
    severity: info
    tier: frontend
    test_alert: "true"
```

#### 2. Backend Monitoring
```yaml
# Latency monitoring (>100ms median)
- alert: BackendMinorLatency
  expr: |
    histogram_quantile(0.50, 
      sum(rate(http_request_duration_seconds_bucket{service=~"backend.*"}[1m])) 
      by (service, le)
    ) > 0.1
  for: 30s
  labels:
    severity: info
    tier: backend
    test_alert: "true"

# Any errors detected
- alert: AnyBackendErrors
  expr: |
    sum(increase(http_requests_total{status_code=~"5..", service=~"backend.*"}[1m])) > 0
  labels:
    severity: warning
    tier: backend
    test_alert: "true"
```

#### 3. Infrastructure Monitoring
```yaml
# Memory usage (>30% for testing)
- alert: MinimalMemoryUsage
  expr: |
    (node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) 
    / node_memory_MemTotal_bytes > 0.3
  for: 1m
  labels:
    severity: info
    tier: infrastructure
    test_alert: "true"

# CPU usage (>20% for testing)
- alert: MinimalCPUUsage
  expr: |
    100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[1m])) * 100) > 20
  for: 1m
  labels:
    severity: info
    tier: infrastructure
    test_alert: "true"
```

### Production Alerts

#### 1. Service Availability
```yaml
# Service down
- alert: ServiceDown
  expr: up == 0
  for: 1m
  labels:
    severity: critical
    tier: infrastructure
  annotations:
    summary: "Service {{ $labels.instance }} is down"
    description: "{{ $labels.job }} at {{ $labels.instance }} has been down for more than 1 minute"

# High error rate (>10%)
- alert: HighErrorRate
  expr: |
    (
      sum(rate(http_requests_total{status_code=~"5.."}[5m])) by (service)
      /
      sum(rate(http_requests_total[5m])) by (service)
    ) > 0.1
  for: 2m
  labels:
    severity: warning
    tier: application
```

#### 2. Performance Alerts
```yaml
# High response time (P95 > 2s)
- alert: HighResponseTime
  expr: |
    histogram_quantile(0.95, 
      sum(rate(http_request_duration_seconds_bucket[5m])) by (service, le)
    ) > 2
  for: 5m
  labels:
    severity: warning
    tier: application

# Database slow queries (P90 > 50ms)
- alert: BackendSlowQueries
  expr: |
    histogram_quantile(0.90, 
      sum(rate(db_query_duration_seconds_bucket[1m])) by (query_type, le)
    ) > 0.05
  for: 30s
  labels:
    severity: warning
    tier: database
```

#### 3. Resource Alerts
```yaml
# High memory usage (>85%)
- alert: HighMemoryUsage
  expr: |
    (node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes)
    / node_memory_MemTotal_bytes > 0.85
  for: 5m
  labels:
    severity: warning
    tier: infrastructure

# High CPU usage (>80%)
- alert: HighCPUUsage
  expr: |
    100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100) > 80
  for: 5m
  labels:
    severity: warning
    tier: infrastructure

# Disk space low (<15%)
- alert: DiskSpaceLow
  expr: |
    (node_filesystem_avail_bytes{mountpoint="/"}
    / node_filesystem_size_bytes{mountpoint="/"}) < 0.15
  for: 5m
  labels:
    severity: warning
    tier: infrastructure
```

## Alertmanager Service Details

### Container Information
- **Image**: `prom/alertmanager:v0.27.0`
- **Container Name**: `alertmanager`
- **Network**: `mock-network`
- **Restart Policy**: `unless-stopped`

### Port Configuration
- **Internal Port**: 9093
- **External Port**: 9093
- **Cluster Port**: 9094
- **Web UI**: http://localhost:9093

### Volume Mounts
- `./prometheus/alertmanager.yml:/etc/alertmanager/alertmanager.yml` - Config
- `alertmanager-data:/alertmanager` - Alert state storage

### Command Line Arguments
```yaml
- '--config.file=/etc/alertmanager/alertmanager.yml'
- '--storage.path=/alertmanager'
- '--web.external-url=http://localhost:9093'
- '--cluster.listen-address=0.0.0.0:9094'
```

## Alertmanager Configuration

### Global Settings
```yaml
global:
  resolve_timeout: 5m
  smtp_smarthost: 'localhost:25'
  smtp_from: 'alertmanager@example.com'
```

### Route Configuration
```yaml
route:
  group_by: ['alertname', 'cluster', 'service']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'default-receiver'
  
  routes:
    # Critical alerts
    - match:
        severity: critical
      receiver: critical-receiver
      continue: true
      
    # Database alerts
    - match:
        tier: database
      receiver: database-team
      
    # Test alerts
    - match:
        test_alert: "true"
      receiver: test-receiver
      repeat_interval: 5m
```

### Receivers Configuration
```yaml
receivers:
  - name: 'default-receiver'
    webhook_configs:
      - url: 'http://webhook-server:5000/webhook'
        send_resolved: true

  - name: 'critical-receiver'
    webhook_configs:
      - url: 'http://webhook-server:5000/critical'
    email_configs:
      - to: 'oncall@example.com'
        headers:
          Subject: 'Critical Alert: {{ .GroupLabels.alertname }}'

  - name: 'database-team'
    webhook_configs:
      - url: 'http://webhook-server:5000/database'

  - name: 'test-receiver'
    webhook_configs:
      - url: 'http://webhook-server:5000/test'
```

### Inhibition Rules
```yaml
inhibit_rules:
  # Inhibit warnings when critical alert is firing
  - source_match:
      severity: 'critical'
    target_match:
      severity: 'warning'
    equal: ['alertname', 'service']
    
  # Inhibit info alerts when warning is firing
  - source_match:
      severity: 'warning'
    target_match:
      severity: 'info'
    equal: ['alertname', 'service']
```

## PromQL Query Examples

### Basic Queries
```promql
# All up services
up

# Request rate per service
sum(rate(http_requests_total[5m])) by (service)

# Error rate percentage
100 * sum(rate(http_requests_total{status_code=~"5.."}[5m])) by (service)
/ sum(rate(http_requests_total[5m])) by (service)

# P95 latency
histogram_quantile(0.95,
  sum(rate(http_request_duration_seconds_bucket[5m])) by (service, le)
)
```

### Advanced Queries
```promql
# Service availability over time
avg_over_time(up[1h])

# Request rate change
rate(http_requests_total[5m]) - rate(http_requests_total[5m] offset 1h)

# Predicted disk full time
predict_linear(node_filesystem_free_bytes[1h], 4*3600) < 0

# Top 5 memory consumers
topk(5, 
  (node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) 
  / node_memory_MemTotal_bytes
)
```

### Debugging Queries
```promql
# Check metric existence
{__name__=~"http_request.*"}

# Label values for a metric
group by (service) (http_requests_total)

# Metric cardinality
count by (__name__)({__name__=~".+"})
```

## Recording Rules

### Configuration
```yaml
groups:
  - name: recording_rules
    interval: 30s
    rules:
      # Request rate
      - record: service:http_requests:rate5m
        expr: |
          sum(rate(http_requests_total[5m])) by (service)
      
      # Error rate
      - record: service:http_errors:rate5m
        expr: |
          sum(rate(http_requests_total{status_code=~"5.."}[5m])) by (service)
      
      # P95 latency
      - record: service:http_request_duration:p95_5m
        expr: |
          histogram_quantile(0.95,
            sum(rate(http_request_duration_seconds_bucket[5m])) by (service, le)
          )
```

## API Usage

### Query API
```bash
# Instant query
curl 'http://localhost:9090/api/v1/query?query=up'

# Range query
curl 'http://localhost:9090/api/v1/query_range?query=rate(http_requests_total[5m])&start=2024-01-01T00:00:00Z&end=2024-01-01T01:00:00Z&step=60s'

# Metadata
curl 'http://localhost:9090/api/v1/metadata'

# Targets
curl 'http://localhost:9090/api/v1/targets'
```

### Admin API
```bash
# Delete time series
curl -X POST 'http://localhost:9090/api/v1/admin/tsdb/delete_series?match[]=up{job="prometheus"}'

# Clean tombstones
curl -X POST 'http://localhost:9090/api/v1/admin/tsdb/clean_tombstones'

# Snapshot
curl -X POST 'http://localhost:9090/api/v1/admin/tsdb/snapshot'
```

## Storage and Retention

### TSDB Configuration
- **Retention Time**: 200 hours (~8 days)
- **Block Duration**: 2 hours (default)
- **Compaction**: Automatic
- **WAL**: Enabled

### Storage Calculation
```
storage_needed = retention_time * ingestion_rate * bytes_per_sample

Example:
- 1000 metrics
- 15s scrape interval
- 200h retention
- ~2 bytes per sample

Storage = 200h * (1000 * 4/min * 60min/h) * 2 bytes = ~96MB
```

### Optimization Tips
1. Increase scrape interval for less critical metrics
2. Use recording rules for expensive queries
3. Implement metric relabeling to drop unnecessary labels
4. Use remote storage for long-term retention

## Troubleshooting

### Common Issues

1. **High Memory Usage**
   ```bash
   # Check TSDB status
   curl http://localhost:9090/api/v1/status/tsdb
   
   # Check top memory series
   curl 'http://localhost:9090/api/v1/label/__name__/values' | jq
   ```

2. **Slow Queries**
   ```bash
   # Enable query log
   curl -X POST http://localhost:9090/api/v1/admin/tsdb/clean_tombstones
   
   # Check query stats
   curl http://localhost:9090/api/v1/query_exemplars
   ```

3. **Target Down**
   ```bash
   # Check target health
   curl http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.health=="down")'
   
   # Debug scrape
   curl http://localhost:9090/api/v1/targets/metadata
   ```

4. **Alert Not Firing**
   ```bash
   # Check rule evaluation
   curl http://localhost:9090/api/v1/rules | jq
   
   # Test query
   curl 'http://localhost:9090/api/v1/query?query=ALERTS'
   ```

### Debug Commands
```bash
# Check configuration
docker exec prometheus promtool check config /etc/prometheus/prometheus.yml

# Validate rules
docker exec prometheus promtool check rules /etc/prometheus/alerts.yml

# Test alertmanager config
docker exec alertmanager amtool check-config /etc/alertmanager/alertmanager.yml

# Check alertmanager alerts
docker exec alertmanager amtool alert query

# Silence alerts
docker exec alertmanager amtool silence add alertname="TestAlert"
```

## Best Practices

### Metric Naming
- Use lowercase with underscores
- Include unit suffix (_seconds, _bytes, _total)
- Be descriptive but concise
- Follow Prometheus conventions

### Label Usage
- Keep cardinality low
- Avoid high-cardinality labels (user_id, request_id)
- Use consistent label names
- Document label meanings

### Alert Design
- Alert on symptoms, not causes
- Include playbook links in descriptions
- Set appropriate severity levels
- Test alerts regularly

### Query Optimization
- Use recording rules for dashboards
- Avoid regex where possible
- Limit time ranges
- Use aggregation operators efficiently