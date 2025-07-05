# Grafana Visualization Documentation

## Service Overview
Grafana is the visualization layer of the monitoring stack, providing dashboards and analytics for metrics from Prometheus and logs from Loki. It serves as the primary interface for observing system health, performance, and troubleshooting issues.

## Service Details

### Container Information
- **Image**: `grafana/grafana:10.1.0`
- **Container Name**: `grafana-loki`
- **Network**: `mock-network`
- **Restart Policy**: `unless-stopped`
- **Dependencies**: loki

### Port Configuration
- **HTTP Port**: 3000
- **Access URL**: http://localhost:3000
- **Default Credentials**: 
  - Username: `admin`
  - Password: `admin`

### Environment Variables
- `GF_SECURITY_ADMIN_PASSWORD=admin` - Admin password
- `GF_USERS_ALLOW_SIGN_UP=false` - Disable public registration

### Volume Mounts
- `grafana-data:/var/lib/grafana` - Persistent data
- `./loki/grafana-datasources.yaml:/etc/grafana/provisioning/datasources/datasources.yaml` - Data sources
- `./loki/grafana-dashboards.yaml:/etc/grafana/provisioning/dashboards/dashboards.yaml` - Dashboard provisioning
- `./loki/dashboards:/var/lib/grafana/dashboards` - Dashboard files

## Data Source Configuration

### Prometheus Data Source
```yaml
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    uid: prometheus-uid
    isDefault: true
    jsonData:
      timeInterval: "15s"
      queryTimeout: "60s"
      httpMethod: "POST"
    editable: true
```

### Loki Data Source
```yaml
  - name: Loki
    type: loki
    access: proxy
    url: http://loki:3100
    uid: loki-uid
    jsonData:
      maxLines: 1000
      derivedFields:
        # Link to traces
        - datasourceUid: tempo-uid
          matcherRegex: "trace_id=(\\w+)"
          name: TraceID
          url: '$${__value.raw}'
        # Link to metrics
        - datasourceUid: prometheus-uid
          matcherRegex: "service=(\\w+)"
          name: Service Metrics
          url: '/explore?left={"queries":[{"expr":"{service=\"$${__value.raw}\"}"}]}'
    editable: true
```

### Alertmanager Data Source
```yaml
  - name: Alertmanager
    type: alertmanager
    access: proxy
    url: http://alertmanager:9093
    uid: alertmanager-uid
    jsonData:
      implementation: prometheus
    editable: true
```

## Dashboard Provisioning

### Dashboard Configuration
```yaml
apiVersion: 1
providers:
  - name: 'default'
    orgId: 1
    folder: 'Mock Infrastructure'
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /var/lib/grafana/dashboards
```

## Pre-configured Dashboards

### 1. Infrastructure Overview Dashboard
```json
{
  "title": "Infrastructure Overview",
  "panels": [
    {
      "title": "Service Status",
      "type": "stat",
      "targets": [{
        "expr": "up",
        "legendFormat": "{{job}}"
      }]
    },
    {
      "title": "Total Request Rate",
      "type": "graph",
      "targets": [{
        "expr": "sum(rate(http_requests_total[5m]))",
        "legendFormat": "Requests/sec"
      }]
    },
    {
      "title": "Error Rate by Service",
      "type": "graph",
      "targets": [{
        "expr": "sum(rate(http_requests_total{status_code=~\"5..\"}[5m])) by (service)",
        "legendFormat": "{{service}}"
      }]
    },
    {
      "title": "CPU Usage",
      "type": "gauge",
      "targets": [{
        "expr": "100 - (avg(rate(node_cpu_seconds_total{mode=\"idle\"}[5m])) * 100)"
      }]
    },
    {
      "title": "Memory Usage",
      "type": "gauge",
      "targets": [{
        "expr": "(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100"
      }]
    }
  ]
}
```

### 2. Service Performance Dashboard
```json
{
  "title": "Service Performance",
  "panels": [
    {
      "title": "Request Rate by Service",
      "type": "graph",
      "targets": [{
        "expr": "sum(rate(http_requests_total[5m])) by (service)",
        "legendFormat": "{{service}}"
      }]
    },
    {
      "title": "P95 Response Time",
      "type": "graph",
      "targets": [{
        "expr": "histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (service, le))",
        "legendFormat": "{{service}}"
      }]
    },
    {
      "title": "Active Connections",
      "type": "stat",
      "targets": [{
        "expr": "sum(nginx_connections_active)"
      }]
    },
    {
      "title": "Cache Hit Rate",
      "type": "gauge",
      "targets": [{
        "expr": "sum(rate(cache_hits_total[5m])) / (sum(rate(cache_hits_total[5m])) + sum(rate(cache_misses_total[5m]))) * 100"
      }]
    }
  ]
}
```

### 3. Database Performance Dashboard
```json
{
  "title": "Database Performance",
  "panels": [
    {
      "title": "Active Connections",
      "type": "stat",
      "targets": [{
        "expr": "pg_stat_database_numbackends{datname=\"mockdb\"}"
      }]
    },
    {
      "title": "Transaction Rate",
      "type": "graph",
      "targets": [{
        "expr": "rate(pg_stat_database_xact_commit[5m])",
        "legendFormat": "Commits"
      }, {
        "expr": "rate(pg_stat_database_xact_rollback[5m])",
        "legendFormat": "Rollbacks"
      }]
    },
    {
      "title": "Query Duration",
      "type": "heatmap",
      "targets": [{
        "expr": "rate(db_query_duration_seconds_bucket[5m])",
        "format": "heatmap"
      }]
    },
    {
      "title": "Cache Hit Ratio",
      "type": "gauge",
      "targets": [{
        "expr": "pg_stat_database_blks_hit / (pg_stat_database_blks_hit + pg_stat_database_blks_read) * 100"
      }]
    }
  ]
}
```

### 4. Logs Analysis Dashboard
```json
{
  "title": "Logs Analysis",
  "panels": [
    {
      "title": "Log Volume by Service",
      "type": "graph",
      "datasource": "Loki",
      "targets": [{
        "expr": "sum(rate({job=\"docker\"}[5m])) by (service)",
        "legendFormat": "{{service}}"
      }]
    },
    {
      "title": "Error Logs Stream",
      "type": "logs",
      "datasource": "Loki",
      "targets": [{
        "expr": "{level=\"ERROR\"}",
        "refId": "A"
      }]
    },
    {
      "title": "Log Level Distribution",
      "type": "piechart",
      "datasource": "Loki",
      "targets": [{
        "expr": "sum(count_over_time({job=\"docker\"}[5m])) by (level)"
      }]
    }
  ]
}
```

### 5. Alerts Dashboard
```json
{
  "title": "Active Alerts",
  "panels": [
    {
      "title": "Alert Status",
      "type": "alertlist",
      "options": {
        "showOptions": "current",
        "maxItems": 20,
        "sortOrder": 1,
        "dashboardAlerts": false,
        "alertName": "",
        "dashboardTitle": "",
        "tags": []
      }
    },
    {
      "title": "Firing Alerts by Severity",
      "type": "stat",
      "targets": [{
        "expr": "ALERTS{alertstate=\"firing\"}",
        "legendFormat": "{{severity}}"
      }]
    }
  ]
}
```

## Query Examples

### Prometheus Queries

#### Service Health
```promql
# Service availability
avg_over_time(up{job="backend"}[5m])

# Error rate percentage
100 * sum(rate(http_requests_total{status_code=~"5.."}[5m])) by (service)
/ sum(rate(http_requests_total[5m])) by (service)

# Request latency percentiles
histogram_quantile(0.99,
  sum(rate(http_request_duration_seconds_bucket[5m])) by (service, le)
)
```

#### Resource Usage
```promql
# CPU usage per service
rate(container_cpu_usage_seconds_total[5m]) * 100

# Memory usage
container_memory_usage_bytes / container_spec_memory_limit_bytes * 100

# Disk I/O
rate(container_fs_reads_bytes_total[5m])
```

### Loki Queries

#### Log Analysis
```logql
# Error rate by service
sum(rate({level="ERROR"} [5m])) by (service)

# Specific error pattern
{service="backend-1"} |= "database connection failed"

# Response time from logs
avg_over_time(
  {service="backend-1"} 
  | json 
  | response_time_ms > 0 
  | unwrap response_time_ms [5m]
)
```

## Variables and Templates

### Dashboard Variables
```json
{
  "templating": {
    "list": [
      {
        "name": "service",
        "type": "query",
        "datasource": "Prometheus",
        "query": "label_values(up, service)",
        "refresh": 1,
        "multi": true,
        "includeAll": true
      },
      {
        "name": "interval",
        "type": "interval",
        "options": [
          {"text": "1m", "value": "1m"},
          {"text": "5m", "value": "5m"},
          {"text": "15m", "value": "15m"},
          {"text": "1h", "value": "1h"}
        ],
        "current": {"text": "5m", "value": "5m"}
      },
      {
        "name": "environment",
        "type": "custom",
        "options": [
          {"text": "All", "value": ".*"},
          {"text": "Production", "value": "production"},
          {"text": "Testing", "value": "testing"}
        ],
        "current": {"text": "All", "value": ".*"}
      }
    ]
  }
}
```

### Using Variables in Queries
```promql
# Service filter
sum(rate(http_requests_total{service=~"$service"}[$interval])) by (service)

# Environment filter
up{environment=~"$environment"}

# Multi-value variable
{service=~"${service:regex}"}
```

## Alerting Configuration

### Alert Rules in Grafana
```json
{
  "alert": {
    "name": "High Error Rate",
    "conditions": [
      {
        "evaluator": {
          "params": [0.1],
          "type": "gt"
        },
        "query": {
          "params": ["A", "5m", "now"]
        },
        "reducer": {
          "params": [],
          "type": "avg"
        },
        "type": "query"
      }
    ],
    "executionErrorState": "alerting",
    "for": "5m",
    "frequency": "1m",
    "handler": 1,
    "message": "Service {{service}} has error rate > 10%",
    "noDataState": "no_data",
    "notifications": [
      {"uid": "slack-channel"},
      {"uid": "pagerduty"}
    ]
  }
}
```

### Notification Channels
```yaml
notifiers:
  - name: slack-alerts
    type: slack
    uid: slack-channel
    settings:
      url: "${SLACK_WEBHOOK_URL}"
      channel: "#alerts"
      username: "Grafana"

  - name: email-alerts
    type: email
    uid: email-channel
    settings:
      addresses: "ops-team@example.com"
      singleEmail: true

  - name: webhook-alerts
    type: webhook
    uid: webhook-channel
    settings:
      url: "http://webhook-server:5000/grafana-alert"
      httpMethod: "POST"
```

## Panel Types and Visualizations

### Time Series (Graph)
- Line graphs for metrics over time
- Area graphs for stacked metrics
- Bar graphs for discrete time data

### Stat Panels
- Single stat display
- Gauge visualization
- Progress bars

### Table Panels
- Tabular data display
- Sorting and filtering
- Cell coloring based on thresholds

### Heatmaps
- Time-based heat maps
- Histogram heat maps
- Custom color schemes

### Logs Panel
- Log streaming
- Search and filtering
- Context expansion

### Alert List
- Active alerts display
- Alert history
- Filtering by state/label

## Advanced Features

### Annotations
```json
{
  "annotations": {
    "list": [
      {
        "datasource": "Prometheus",
        "enable": true,
        "expr": "ALERTS{alertstate=\"firing\"}",
        "iconColor": "rgba(255, 96, 96, 1)",
        "name": "Firing Alerts",
        "tagKeys": "severity,service",
        "textFormat": "{{alertname}}: {{service}}"
      },
      {
        "datasource": "Loki",
        "enable": true,
        "expr": "{job=\"docker\"} |= \"deployment\"",
        "iconColor": "rgba(0, 211, 255, 1)",
        "name": "Deployments"
      }
    ]
  }
}
```

### Transformations
```json
{
  "transformations": [
    {
      "id": "calculateField",
      "options": {
        "alias": "Error Rate",
        "binary": {
          "left": "errors",
          "operator": "/",
          "right": "total"
        },
        "mode": "binary",
        "reduce": {"reducer": "sum"}
      }
    },
    {
      "id": "organize",
      "options": {
        "excludeByName": {"time": true},
        "indexByName": {
          "service": 0,
          "error_rate": 1,
          "total": 2
        },
        "renameByName": {
          "error_rate": "Error %"
        }
      }
    }
  ]
}
```

### Links and Drilldowns
```json
{
  "links": [
    {
      "title": "View in Prometheus",
      "url": "http://localhost:9090/graph?g0.expr=${__url_time_range}&g0.tab=0",
      "targetBlank": true
    },
    {
      "title": "View Service Logs",
      "url": "/explore?left={\"queries\":[{\"expr\":\"{service=\\\"${__field.labels.service}\\\"}\"}]}",
      "targetBlank": false
    },
    {
      "title": "Service Dashboard",
      "url": "/d/service-detail/service-detail?var-service=${__field.labels.service}",
      "targetBlank": false
    }
  ]
}
```

## Performance Optimization

### Query Optimization
1. Use recording rules for expensive queries
2. Limit time ranges with `$__range`
3. Use `__interval` for dynamic step
4. Cache dashboard queries

### Dashboard Performance
1. Limit number of panels per dashboard
2. Use streaming for real-time data
3. Set appropriate refresh intervals
4. Use query caching

### Resource Limits
```ini
[database]
max_open_conns = 100
max_idle_conns = 100
conn_max_lifetime = 14400

[dataproxy]
max_conns_per_host = 5
max_idle_conns_per_host = 5

[rendering]
concurrent_render_request_limit = 5
```

## API Usage

### Dashboard API
```bash
# List dashboards
curl -H "Authorization: Bearer $API_KEY" \
  http://localhost:3000/api/search

# Get dashboard
curl -H "Authorization: Bearer $API_KEY" \
  http://localhost:3000/api/dashboards/uid/infrastructure

# Create dashboard
curl -X POST -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d @dashboard.json \
  http://localhost:3000/api/dashboards/db

# Export dashboard
curl -H "Authorization: Bearer $API_KEY" \
  http://localhost:3000/api/dashboards/uid/infrastructure | jq '.dashboard' > dashboard.json
```

### Data Source API
```bash
# List data sources
curl -H "Authorization: Bearer $API_KEY" \
  http://localhost:3000/api/datasources

# Test data source
curl -X POST -H "Authorization: Bearer $API_KEY" \
  http://localhost:3000/api/datasources/1/test
```

### Query API
```bash
# Execute query
curl -X POST -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "queries": [{
      "datasourceId": 1,
      "expr": "up",
      "refId": "A"
    }],
    "from": "now-1h",
    "to": "now"
  }' \
  http://localhost:3000/api/ds/query
```

## Troubleshooting

### Common Issues

1. **Data Source Connection Failed**
   ```bash
   # Test data source
   curl -X GET http://prometheus:9090/-/healthy
   curl -X GET http://loki:3100/ready
   
   # Check network connectivity
   docker exec grafana-loki ping prometheus
   ```

2. **Dashboard Not Loading**
   ```bash
   # Check Grafana logs
   docker logs grafana-loki
   
   # Verify provisioning
   docker exec grafana-loki ls -la /etc/grafana/provisioning/
   ```

3. **Query Timeout**
   ```bash
   # Increase timeout in data source
   # Optimize query with recording rules
   # Check Prometheus/Loki performance
   ```

4. **High Memory Usage**
   ```bash
   # Check cache settings
   # Reduce concurrent queries
   # Optimize dashboard refresh rates
   ```

### Debug Mode
```ini
[log]
mode = console
level = debug

[log.console]
format = json

[server]
router_logging = true
```

## Best Practices

### Dashboard Design
1. Group related metrics
2. Use consistent color schemes
3. Add helpful descriptions
4. Include relevant links
5. Set appropriate thresholds

### Query Guidelines
1. Use variables for flexibility
2. Optimize for performance
3. Add meaningful legends
4. Handle no-data cases
5. Document complex queries

### Organization
1. Use folders for categorization
2. Implement naming conventions
3. Version control dashboards
4. Regular cleanup of unused dashboards
5. Document dashboard purposes

### Security
1. Use API keys for automation
2. Implement RBAC for teams
3. Audit dashboard changes
4. Secure sensitive queries
5. Regular permission reviews