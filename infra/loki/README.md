# Loki Infrastructure

This directory contains Docker Compose configuration for running Loki, Promtail, and Grafana for log aggregation and visualization.

## Components

- **Loki**: Log aggregation system
- **Promtail**: Log collection agent
- **Grafana**: Visualization and dashboarding

## Quick Start

1. Start the stack:
```bash
cd infra/loki
docker-compose up -d
```

2. Access services:
- Grafana: http://localhost:3000 (admin/admin)
- Loki API: http://localhost:3100

## Configuration Files

- `docker-compose.yaml`: Main orchestration file
- `loki-config.yaml`: Loki server configuration
- `promtail-config.yaml`: Log collection configuration
- `grafana-datasources.yaml`: Grafana datasource provisioning
- `grafana-dashboards.yaml`: Dashboard provisioning
- `dashboards/loki-logs-dashboard.json`: Pre-built Loki dashboard

## Usage

### Querying Logs

Access Grafana at http://localhost:3000 and use LogQL queries:

```logql
# All logs
{job=~".+"}

# Logs from specific job
{job="varlogs"}

# Error logs
{job=~".+"} |= "error"

# Rate of logs
rate({job=~".+"}[5m])
```

### API Access

Query Loki directly:
```bash
# Get labels
curl -G -s "http://localhost:3100/loki/api/v1/labels"

# Query logs
curl -G -s "http://localhost:3100/loki/api/v1/query_range" \
  --data-urlencode 'query={job="varlogs"}' \
  --data-urlencode 'start=2023-01-01T00:00:00Z' \
  --data-urlencode 'end=2023-12-31T23:59:59Z'
```

## Stopping

```bash
docker-compose down
```

To remove volumes:
```bash
docker-compose down -v
```
