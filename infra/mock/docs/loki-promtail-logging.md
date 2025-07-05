# Loki & Promtail Logging Stack Documentation

## Service Overview
Loki is a horizontally-scalable, highly-available log aggregation system inspired by Prometheus. Promtail is the agent that ships logs to Loki. Together they provide centralized logging for all services in the infrastructure.

## Loki Service Details

### Container Information
- **Image**: `grafana/loki:2.9.0`
- **Container Name**: `loki`
- **Network**: `mock-network`
- **Restart Policy**: `unless-stopped`

### Port Configuration
- **HTTP Port**: 3100
- **API Endpoint**: http://localhost:3100
- **Ready Endpoint**: http://localhost:3100/ready
- **Metrics Endpoint**: http://localhost:3100/metrics

### Volume Mounts
- `./loki/loki-config.yaml:/etc/loki/local-config.yaml` - Configuration
- `loki-data:/loki` - Log data storage

### Command
```yaml
command: -config.file=/etc/loki/local-config.yaml
```

## Loki Configuration

### Server Configuration
```yaml
server:
  http_listen_port: 3100
  grpc_listen_port: 9096
  log_level: info

# Authentication disabled for local development
auth_enabled: false
```

### Storage Configuration
```yaml
common:
  path_prefix: /loki
  storage:
    filesystem:
      chunks_directory: /loki/chunks
      rules_directory: /loki/rules
  replication_factor: 1
  ring:
    instance_addr: 127.0.0.1
    kvstore:
      store: inmemory
```

### Ingester Configuration
```yaml
ingester:
  wal:
    enabled: true
    dir: /loki/wal
  lifecycler:
    address: 127.0.0.1
    ring:
      kvstore:
        store: inmemory
      replication_factor: 1
    final_sleep: 0s
  chunk_idle_period: 1h
  max_chunk_age: 1h
  chunk_target_size: 1048576
  chunk_retain_period: 30s
  max_transfer_retries: 0
```

### Schema Configuration
```yaml
schema_config:
  configs:
    - from: 2020-10-24
      store: boltdb-shipper
      object_store: filesystem
      schema: v11
      index:
        prefix: index_
        period: 24h
```

### Storage Limits
```yaml
storage_config:
  boltdb_shipper:
    active_index_directory: /loki/boltdb-shipper-active
    cache_location: /loki/boltdb-shipper-cache
    cache_ttl: 24h
    shared_store: filesystem
  filesystem:
    directory: /loki/chunks

limits_config:
  retention_period: 168h  # 7 days
  enforce_metric_name: false
  reject_old_samples: true
  reject_old_samples_max_age: 168h
  ingestion_rate_mb: 16
  ingestion_burst_size_mb: 32
  max_query_series: 5000
  max_query_parallelism: 32
```

### Query Configuration
```yaml
querier:
  max_concurrent: 20

query_range:
  align_queries_with_step: true
  max_retries: 5
  cache_results: true
  results_cache:
    cache:
      embedded_cache:
        enabled: true
        max_size_mb: 100
```

## Promtail Service Details

### Container Information
- **Image**: `grafana/promtail:2.9.0`
- **Container Name**: `promtail`
- **Network**: `mock-network`
- **Restart Policy**: `unless-stopped`
- **Dependencies**: loki

### Volume Mounts
- `/var/log:/var/log:ro` - System logs (read-only)
- `/var/lib/docker/containers:/var/lib/docker/containers:ro` - Docker logs
- `./loki/promtail-config.yaml:/etc/promtail/config.yml` - Configuration

### Command
```yaml
command: -config.file=/etc/promtail/config.yml
```

## Promtail Configuration

### Server Configuration
```yaml
server:
  http_listen_port: 9080
  grpc_listen_port: 0

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://loki:3100/loki/api/v1/push
    tenant_id: docker
    batchwait: 1s
    batchsize: 1048576  # 1MB
    timeout: 10s
    backoff_config:
      min_period: 500ms
      max_period: 5m
      max_retries: 10
```

### Scrape Configuration

#### Docker Container Logs
```yaml
scrape_configs:
  - job_name: docker
    static_configs:
      - targets:
          - localhost
        labels:
          job: docker
          __path__: /var/lib/docker/containers/*/*log
    
    pipeline_stages:
      # Extract container ID
      - regex:
          expression: '/var/lib/docker/containers/(?P<container_id>[a-z0-9]+)/.*'
          
      # Get container name from Docker labels
      - docker: {}
      
      # Parse JSON logs
      - json:
          expressions:
            output: log
            stream: stream
            time: time
            
      # Extract severity
      - regex:
          expression: '(?P<severity>DEBUG|INFO|WARN|ERROR|FATAL)'
          
      # Add labels
      - labels:
          severity:
          stream:
          
      # Set timestamp
      - timestamp:
          source: time
          format: RFC3339Nano
```

#### Application Logs
```yaml
  - job_name: application
    static_configs:
      - targets:
          - localhost
        labels:
          job: application
          __path__: /app/logs/*.log
    
    pipeline_stages:
      # Multiline handling for stack traces
      - multiline:
          firstline: '^\d{4}-\d{2}-\d{2}'
          max_wait_time: 3s
          
      # Parse log format
      - regex:
          expression: '^(?P<timestamp>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z)\s+(?P<level>\w+)\s+(?P<service>\w+)\s+(?P<message>.*)'
          
      # Add labels
      - labels:
          level:
          service:
          
      # Parse JSON in message
      - match:
          selector: '{level="ERROR"}'
          stages:
            - json:
                expressions:
                  error: error
                  stack: stack_trace
                  
      # Drop debug logs in production
      - match:
          selector: '{level="DEBUG"}'
          action: drop
          drop_counter_reason: debug_logs
```

#### System Logs
```yaml
  - job_name: syslog
    static_configs:
      - targets:
          - localhost
        labels:
          job: syslog
          __path__: /var/log/syslog
    
    pipeline_stages:
      - regex:
          expression: '^(?P<timestamp>\w+\s+\d+\s+\d{2}:\d{2}:\d{2})\s+(?P<hostname>\S+)\s+(?P<program>\S+?)(\[(?P<pid>\d+)\])?:\s+(?P<message>.*)'
          
      - labels:
          hostname:
          program:
          
      - timestamp:
          source: timestamp
          format: 'Jan 2 15:04:05'
```

## Log Query Language (LogQL)

### Basic Queries
```logql
# All logs from a service
{service="frontend"}

# Logs with specific level
{service="backend-1", level="ERROR"}

# Multiple label filters
{job="docker", container_name="mock-frontend", stream="stderr"}

# Regex label matching
{service=~"backend.*", level!="DEBUG"}
```

### Log Pipeline Queries
```logql
# Search for text
{service="frontend"} |= "error"

# Regex matching
{service="backend-1"} |~ "database.*timeout"

# JSON parsing
{service="backend-1"} | json | error_code="DB_CONNECTION_FAILED"

# Line formatting
{service="frontend"} | line_format "{{.timestamp}} [{{.level}}] {{.message}}"
```

### Metric Queries
```logql
# Count logs per minute
rate({service="frontend"}[1m])

# Count errors
count_over_time({level="ERROR"}[5m])

# Bytes processed
bytes_rate({job="docker"}[5m])

# P95 of parsed duration values
{service="backend-1"} 
  | json 
  | duration > 0 
  | quantile_over_time(0.95, duration[5m])
```

### Aggregation Queries
```logql
# Sum by service
sum by (service) (rate({job="docker"}[5m]))

# Top 5 error producers
topk(5, sum by (service) (rate({level="ERROR"}[5m])))

# Average response time
avg_over_time(
  {service="backend-1"} 
  | json 
  | response_time_ms > 0 
  | unwrap response_time_ms [5m]
)
```

## Label Management

### Static Labels
```yaml
static_configs:
  - targets:
      - localhost
    labels:
      environment: "production"
      region: "us-east-1"
      cluster: "main"
```

### Dynamic Labels
```yaml
pipeline_stages:
  # From log content
  - regex:
      expression: 'user_id=(?P<user_id>\d+)'
  - labels:
      user_id:
      
  # From JSON
  - json:
      expressions:
        request_id: request_id
        trace_id: trace_id
  - labels:
      request_id:
      trace_id:
```

### Label Cardinality Control
```yaml
# Drop high cardinality labels
- labeldrop:
    - request_id
    - session_id
    
# Keep only specific labels
- labelallow:
    - service
    - level
    - environment
```

## Performance Optimization

### Promtail Optimization
```yaml
# Batch configuration
clients:
  - url: http://loki:3100/loki/api/v1/push
    batchwait: 2s        # Wait time before sending
    batchsize: 1048576   # 1MB batches
    
# File watching
file_watch_config:
  min_poll_frequency: 250ms
  max_poll_frequency: 250ms
  
# Position sync
positions:
  filename: /tmp/positions.yaml
  sync_period: 10s
```

### Loki Query Optimization
```yaml
# Query limits
limits_config:
  max_entries_limit_per_query: 5000
  max_streams_per_user: 10000
  max_chunks_per_query: 2000000
  
# Caching
query_range:
  cache_results: true
  results_cache:
    cache:
      embedded_cache:
        enabled: true
        max_size_mb: 100
        ttl: 1h
```

## Retention and Storage

### Retention Policy
```yaml
table_manager:
  retention_deletes_enabled: true
  retention_period: 168h  # 7 days

compactor:
  working_directory: /loki/compactor
  shared_store: filesystem
  compaction_interval: 2h
  retention_enabled: true
  retention_delete_delay: 2h
  retention_delete_worker_count: 150
```

### Storage Calculation
```
Storage = (log_volume_per_day * retention_days * compression_ratio)

Example:
- 10GB logs/day (uncompressed)
- 7 days retention  
- 10:1 compression ratio

Storage = 10GB * 7 * 0.1 = 7GB
```

## Monitoring Loki & Promtail

### Loki Metrics
Key metrics exposed at `/metrics`:

1. **Ingestion Metrics**:
   - `loki_ingester_chunks_created_total` - Chunks created
   - `loki_ingester_chunks_stored_total` - Chunks stored
   - `loki_ingester_received_samples_total` - Samples received
   - `loki_ingester_samples_per_chunk` - Compression ratio

2. **Query Metrics**:
   - `loki_logql_querystats_latency_seconds` - Query latency
   - `loki_logql_querystats_bytes_processed_total` - Bytes scanned
   - `loki_logql_querystats_entries_returned_total` - Results returned

3. **Storage Metrics**:
   - `loki_chunk_store_chunks_stored_total` - Total chunks
   - `loki_chunk_store_chunk_cache_hits_total` - Cache performance
   - `loki_boltdb_shipper_uploads_total` - Index uploads

### Promtail Metrics
Key metrics:

1. **Scraping Metrics**:
   - `promtail_read_bytes_total` - Bytes read
   - `promtail_read_lines_total` - Lines read
   - `promtail_dropped_entries_total` - Dropped logs

2. **Processing Metrics**:
   - `promtail_sent_entries_total` - Entries sent
   - `promtail_sent_bytes_total` - Bytes sent
   - `promtail_request_duration_seconds` - Send latency

## Integration with Grafana

### Data Source Configuration
```yaml
apiVersion: 1
datasources:
  - name: Loki
    type: loki
    access: proxy
    url: http://loki:3100
    jsonData:
      maxLines: 1000
      derivedFields:
        - datasourceUid: prometheus-uid
          matcherRegex: "trace_id=(\\w+)"
          name: TraceID
          url: '$${__value.raw}'
```

### Example Dashboards

#### Service Logs Dashboard
```json
{
  "panels": [
    {
      "title": "Log Volume",
      "targets": [{
        "expr": "sum(rate({job=\"docker\"}[5m])) by (service)"
      }]
    },
    {
      "title": "Error Logs",
      "targets": [{
        "expr": "{level=\"ERROR\"}"
      }]
    },
    {
      "title": "Response Times",
      "targets": [{
        "expr": "avg_over_time({service=\"backend-1\"} | json | unwrap response_time_ms [5m])"
      }]
    }
  ]
}
```

## Troubleshooting

### Common Issues

1. **Logs Not Appearing**
   ```bash
   # Check Promtail targets
   curl http://localhost:9080/targets
   
   # Check positions file
   docker exec promtail cat /tmp/positions.yaml
   
   # Verify log path
   docker exec promtail ls -la /var/log/
   ```

2. **High Memory Usage**
   ```bash
   # Check chunk cache size
   curl http://localhost:3100/metrics | grep chunk_cache
   
   # Reduce cache size in config
   # Increase chunk_target_size
   ```

3. **Slow Queries**
   ```bash
   # Check query stats
   curl http://localhost:3100/metrics | grep logql_querystats
   
   # Use time range filters
   # Add more specific label filters
   ```

4. **Missing Labels**
   ```bash
   # Debug pipeline stages
   docker logs promtail | grep pipeline
   
   # Test regex patterns
   # Check label drop rules
   ```

### Debug Commands
```bash
# Loki readiness
curl http://localhost:3100/ready

# Loki configuration
curl http://localhost:3100/config

# Promtail targets
curl http://localhost:9080/targets

# Test log push
curl -X POST http://localhost:3100/loki/api/v1/push \
  -H "Content-Type: application/json" \
  -d '{
    "streams": [{
      "stream": {"service": "test"},
      "values": [["'$(date +%s)000000000'", "test log"]]
    }]
  }'

# Query logs
curl -G http://localhost:3100/loki/api/v1/query_range \
  --data-urlencode 'query={service="frontend"}' \
  --data-urlencode 'start=2024-01-01T00:00:00Z' \
  --data-urlencode 'end=2024-01-01T01:00:00Z'
```

## Best Practices

### Log Format
1. Use structured logging (JSON)
2. Include standard fields: timestamp, level, service, message
3. Add trace/request IDs for correlation
4. Keep messages concise but informative

### Label Strategy
1. Use low-cardinality labels
2. Standard labels: service, level, environment, region
3. Avoid: user_id, request_id, session_id as labels
4. Parse high-cardinality data at query time

### Pipeline Design
1. Parse early, drop unnecessary logs
2. Use multiline for stack traces
3. Extract metrics from logs sparingly
4. Test regex patterns thoroughly

### Query Optimization
1. Always include time ranges
2. Use specific label selectors
3. Limit result size with `limit`
4. Use recording rules for repeated queries