# 🚀 Deployment Guide

Comprehensive guide for deploying Paladin AI in development, staging, and production environments.

## Overview

Paladin AI is designed as a cloud-native, microservices-based platform with comprehensive monitoring and AI capabilities. This guide covers deployment strategies from local development to production-ready environments with high availability, security, and scalability considerations.

## Architecture Overview

```mermaid
graph TB
    subgraph "Frontend Layer"
        LB[Load Balancer<br/>Nginx]
        UI[Next.js Frontend<br/>Port 3000]
    end
    
    subgraph "Application Layer"
        API[FastAPI Server<br/>Port 8000]
        CLI[CLI Client]
        DISCORD[Discord Bot MCP]
        WEBHOOK[Webhook Server<br/>Port 8001]
    end
    
    subgraph "Data Layer"
        MONGO[(MongoDB<br/>Checkpoints)]
        QDRANT[(Qdrant<br/>Vector DB)]
        NEO4J[(Neo4j<br/>Graph DB)]
        VALKEY[(Valkey<br/>Cache/Queue)]
    end
    
    subgraph "Monitoring Stack"
        PROM[Prometheus<br/>Port 9090]
        GRAFANA[Grafana<br/>Port 3001]
        LOKI[Loki<br/>Port 3100]
        ALERT[Alertmanager<br/>Port 9093]
    end
    
    subgraph "External Services"
        OPENAI[OpenAI API]
        LANGFUSE[Langfuse]
    end
    
    LB --> UI
    UI --> API
    CLI --> API
    DISCORD --> API
    WEBHOOK --> API
    
    API --> MONGO
    API --> QDRANT
    API --> NEO4J
    API --> VALKEY
    
    PROM --> API
    PROM --> MONGO
    PROM --> QDRANT
    GRAFANA --> PROM
    GRAFANA --> LOKI
    ALERT --> WEBHOOK
    
    API --> OPENAI
    API --> LANGFUSE
```

## Quick Start Deployment

### Prerequisites

```bash
# System Requirements
- Docker 20.10+
- Docker Compose 2.0+
- 8GB RAM minimum, 16GB recommended
- 20GB free disk space
- Python 3.13+ (for local development)
- Node.js 18+ (for frontend development)

# External Services
- OpenAI API key
- Langfuse account (optional)
```

### 1. Environment Setup

```bash
# Clone repository
git clone https://github.com/your-org/paladin-ai.git
cd paladin-ai

# Set up environment variables
cp .env.example .env
cp server/.env.example server/.env
cp frontend/.env.example frontend/.env.local

# Configure essential variables
export OPENAI_API_KEY="sk-your-openai-api-key"
export HOST_IP="localhost"
export HOST_PROTOCOL="http"
```

### 2. Development Deployment

```bash
# Start infrastructure stack
make infra-up

# Install dependencies
make install-dev

# Start all services
make dev
```

### 3. Production-Ready Deployment

```bash
# Start production stack
docker-compose -f docker-compose.prod.yml up -d

# Verify deployment
make health-check
```

## Infrastructure Components

### 1. Container Orchestration

#### Docker Compose Configuration

**Main Stack** (`docker-compose.yml`):
```yaml
version: '3.8'

services:
  # Application Services
  paladin-server:
    build: ./server
    ports:
      - "8000:8000"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - MONGODB_URI=mongodb://mongodb:27017/paladinai
    depends_on:
      - mongodb
      - qdrant
      - neo4j
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8000/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  paladin-frontend:
    build: ./frontend
    ports:
      - "3000:3000"
    environment:
      - NEXT_PUBLIC_PALADIN_API_URL=http://localhost:8000
    depends_on:
      - paladin-server

  # Load Balancer
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./infra/nginx:/etc/nginx/conf.d
      - ./certs:/etc/ssl/certs
    depends_on:
      - paladin-frontend
      - paladin-server

  # Data Layer
  mongodb:
    image: mongo:7.0
    ports:
      - "27017:27017"
    environment:
      - MONGO_INITDB_ROOT_USERNAME=admin
      - MONGO_INITDB_ROOT_PASSWORD=${MONGODB_PASSWORD}
    volumes:
      - mongodb_data:/data/db
      - ./infra/mongodb/init:/docker-entrypoint-initdb.d
    healthcheck:
      test: ["CMD", "mongosh", "--eval", "db.adminCommand('ping')"]

  qdrant:
    image: qdrant/qdrant:latest
    ports:
      - "6333:6333"
    volumes:
      - qdrant_data:/qdrant/storage
    environment:
      - QDRANT__SERVICE__HTTP_PORT=6333

  neo4j:
    image: neo4j:5.15-community
    ports:
      - "7474:7474"
      - "7687:7687"
    environment:
      - NEO4J_AUTH=neo4j/${NEO4J_PASSWORD}
      - NEO4J_PLUGINS=["apoc"]
    volumes:
      - neo4j_data:/data

  valkey:
    image: valkey/valkey:latest
    ports:
      - "6379:6379"
    volumes:
      - valkey_data:/data
    command: valkey-server --appendonly yes

volumes:
  mongodb_data:
  qdrant_data:
  neo4j_data:
  valkey_data:

networks:
  default:
    driver: bridge
    name: paladin-network
```

### 2. Load Balancing Configuration

#### Nginx Configuration

**Main Configuration** (`infra/nginx/default.conf`):
```nginx
upstream paladin_frontend {
    least_conn;
    server paladin-frontend:3000 weight=1 max_fails=3 fail_timeout=30s;
}

upstream paladin_api {
    least_conn;
    server paladin-server:8000 weight=1 max_fails=3 fail_timeout=30s;
}

# Rate limiting
limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;
limit_req_zone $binary_remote_addr zone=frontend:10m rate=100r/s;

server {
    listen 80;
    server_name localhost;
    
    # Security headers
    add_header X-Frame-Options SAMEORIGIN;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains";
    
    # Frontend routes
    location / {
        limit_req zone=frontend burst=20 nodelay;
        proxy_pass http://paladin_frontend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
    
    # API routes
    location /api/ {
        limit_req zone=api burst=5 nodelay;
        proxy_pass http://paladin_api;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Increase timeout for AI operations
        proxy_read_timeout 300s;
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
    }
    
    # Health check endpoint
    location /health {
        access_log off;
        proxy_pass http://paladin_api/health;
    }
}

# HTTPS Configuration
server {
    listen 443 ssl http2;
    server_name localhost;
    
    ssl_certificate /etc/ssl/certs/paladin.crt;
    ssl_certificate_key /etc/ssl/certs/paladin.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;
    
    # Same configuration as HTTP server
    include /etc/nginx/conf.d/common.conf;
}
```

### 3. Database Deployment

#### MongoDB Cluster Configuration

**MongoDB Replica Set** (`infra/mongodb/docker-compose.yml`):
```yaml
version: '3.8'

services:
  mongodb-primary:
    image: mongo:7.0
    ports:
      - "27017:27017"
    environment:
      - MONGO_INITDB_ROOT_USERNAME=admin
      - MONGO_INITDB_ROOT_PASSWORD=${MONGODB_PASSWORD}
      - MONGO_REPLICA_SET_NAME=rs0
    volumes:
      - mongodb_primary:/data/db
      - ./scripts/mongo-init.js:/docker-entrypoint-initdb.d/mongo-init.js
    command: mongod --replSet rs0 --bind_ip_all
    healthcheck:
      test: ["CMD", "mongosh", "--eval", "db.adminCommand('ping')"]
      interval: 10s
      timeout: 5s
      retries: 5

  mongodb-secondary:
    image: mongo:7.0
    ports:
      - "27018:27017"
    environment:
      - MONGO_REPLICA_SET_NAME=rs0
    volumes:
      - mongodb_secondary:/data/db
    command: mongod --replSet rs0 --bind_ip_all
    depends_on:
      - mongodb-primary

volumes:
  mongodb_primary:
  mongodb_secondary:
```

**Initialization Script** (`infra/mongodb/scripts/mongo-init.js`):
```javascript
// Initialize replica set
rs.initiate({
  _id: "rs0",
  members: [
    { _id: 0, host: "mongodb-primary:27017", priority: 1 },
    { _id: 1, host: "mongodb-secondary:27017", priority: 0.5 }
  ]
});

// Create application database and user
use paladinai;
db.createUser({
  user: "paladin",
  pwd: "secure_password",
  roles: [
    { role: "readWrite", db: "paladinai" }
  ]
});

// Create collections with indexes
db.createCollection("checkpoints");
db.checkpoints.createIndex({ "session_id": 1 });
db.checkpoints.createIndex({ "created_at": -1 });

db.createCollection("sessions");
db.sessions.createIndex({ "user_id": 1 });
db.sessions.createIndex({ "created_at": -1 });
```

#### Vector Database Setup

**Qdrant Configuration** (`infra/qdrant/docker-compose.yml`):
```yaml
version: '3.8'

services:
  qdrant:
    image: qdrant/qdrant:latest
    ports:
      - "6333:6333"
      - "6334:6334"
    environment:
      - QDRANT__SERVICE__HTTP_PORT=6333
      - QDRANT__SERVICE__GRPC_PORT=6334
      - QDRANT__STORAGE__STORAGE_PATH=/qdrant/storage
    volumes:
      - qdrant_data:/qdrant/storage
      - ./config/qdrant.yaml:/qdrant/config/config.yaml
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:6333/health"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  qdrant_data:
```

### 4. Monitoring Stack Deployment

#### Prometheus Configuration

**Prometheus Setup** (`infra/prometheus/docker-compose.yml`):
```yaml
version: '3.8'

services:
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - ./rules:/etc/prometheus/rules
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--storage.tsdb.retention.time=200h'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
      - '--web.enable-lifecycle'

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3001:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD}
    volumes:
      - grafana_data:/var/lib/grafana
      - ./grafana/dashboards:/etc/grafana/provisioning/dashboards
      - ./grafana/datasources:/etc/grafana/provisioning/datasources

  alertmanager:
    image: prom/alertmanager:latest
    ports:
      - "9093:9093"
    volumes:
      - ./alertmanager.yml:/etc/alertmanager/alertmanager.yml
      - alertmanager_data:/alertmanager

volumes:
  prometheus_data:
  grafana_data:
  alertmanager_data:
```

**Prometheus Configuration** (`infra/prometheus/prometheus.yml`):
```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "/etc/prometheus/rules/*.yml"

scrape_configs:
  - job_name: 'paladin-server'
    static_configs:
      - targets: ['paladin-server:8000']
    metrics_path: /metrics
    scrape_interval: 15s

  - job_name: 'mongodb'
    static_configs:
      - targets: ['mongodb-exporter:9216']

  - job_name: 'qdrant'
    static_configs:
      - targets: ['qdrant:6333']
    metrics_path: /metrics

  - job_name: 'neo4j'
    static_configs:
      - targets: ['neo4j:2004']

alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093
```

## Environment Configurations

### 1. Development Environment

**Environment Variables** (`.env.development`):
```bash
# Environment
NODE_ENV=development
DEBUG=true
LOG_LEVEL=debug

# API Configuration
SERVER_HOST=localhost
SERVER_PORT=8000
FRONTEND_PORT=3000

# Database URLs (local)
MONGODB_URI=mongodb://localhost:27017/paladinai_dev
QDRANT_URL=http://localhost:6333
NEO4J_URL=bolt://localhost:7687
VALKEY_URL=redis://localhost:6379

# AI Services
OPENAI_API_KEY=sk-dev-key
OPENAI_MODEL=gpt-4o-mini
LANGFUSE_ENABLED=false

# Monitoring (optional in dev)
PROMETHEUS_URL=http://localhost:9090
GRAFANA_URL=http://localhost:3001

# Discord (dev bot)
DISCORD_ENABLED=false
```

### 2. Staging Environment

**Environment Variables** (`.env.staging`):
```bash
# Environment
NODE_ENV=staging
DEBUG=false
LOG_LEVEL=info

# API Configuration
SERVER_HOST=staging.paladin.internal
SERVER_PORT=8000
CORS_ORIGINS=https://staging.paladin.com

# Database URLs (staging cluster)
MONGODB_URI=mongodb://mongo-staging.internal:27017/paladinai_staging
QDRANT_URL=http://qdrant-staging.internal:6333
NEO4J_URL=bolt://neo4j-staging.internal:7687
VALKEY_URL=redis://valkey-staging.internal:6379

# AI Services
OPENAI_API_KEY=${STAGING_OPENAI_API_KEY}
LANGFUSE_ENABLED=true
LANGFUSE_HOST=https://staging-langfuse.paladin.com

# Monitoring
PROMETHEUS_URL=http://prometheus-staging.internal:9090
GRAFANA_URL=https://grafana-staging.paladin.com

# Security
SSL_ENABLED=true
JWT_SECRET=${STAGING_JWT_SECRET}
```

### 3. Production Environment

**Environment Variables** (`.env.production`):
```bash
# Environment
NODE_ENV=production
DEBUG=false
LOG_LEVEL=warn

# API Configuration
SERVER_HOST=api.paladin.com
SERVER_PORT=8000
CORS_ORIGINS=https://app.paladin.com,https://paladin.com

# Database URLs (production cluster)
MONGODB_URI=mongodb+srv://prod-user:${MONGODB_PASSWORD}@mongo-cluster.paladin.com/paladinai
QDRANT_URL=https://qdrant.paladin.com:443
NEO4J_URL=bolt+s://neo4j.paladin.com:7687
VALKEY_URL=rediss://valkey.paladin.com:6380

# AI Services
OPENAI_API_KEY=${PROD_OPENAI_API_KEY}
LANGFUSE_ENABLED=true
LANGFUSE_HOST=https://langfuse.paladin.com

# Monitoring
PROMETHEUS_URL=https://prometheus.paladin.com
GRAFANA_URL=https://grafana.paladin.com
ALERTMANAGER_URL=https://alertmanager.paladin.com

# Security
SSL_ENABLED=true
JWT_SECRET=${PROD_JWT_SECRET}
ENCRYPTION_KEY=${PROD_ENCRYPTION_KEY}

# Performance
REQUEST_TIMEOUT=300
MAX_CONCURRENT_REQUESTS=1000
RATE_LIMIT_ENABLED=true

# Backup
BACKUP_ENABLED=true
BACKUP_SCHEDULE=0 2 * * *  # Daily at 2 AM
BACKUP_RETENTION_DAYS=30
```

## Kubernetes Deployment

### 1. Namespace Configuration

**Namespace** (`k8s/namespace.yaml`):
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: paladin-ai
  labels:
    name: paladin-ai
    environment: production
```

### 2. Application Deployment

**Paladin Server** (`k8s/paladin-server.yaml`):
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: paladin-server
  namespace: paladin-ai
spec:
  replicas: 3
  selector:
    matchLabels:
      app: paladin-server
  template:
    metadata:
      labels:
        app: paladin-server
    spec:
      containers:
      - name: paladin-server
        image: paladin-ai/server:latest
        ports:
        - containerPort: 8000
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: paladin-secrets
              key: openai-api-key
        - name: MONGODB_URI
          valueFrom:
            secretKeyRef:
              name: paladin-secrets
              key: mongodb-uri
        resources:
          requests:
            memory: "1Gi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "1"
        livenessProbe:
          httpGet:
            path: /health
            port: 8000
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8000
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: paladin-server-service
  namespace: paladin-ai
spec:
  selector:
    app: paladin-server
  ports:
  - protocol: TCP
    port: 8000
    targetPort: 8000
  type: ClusterIP
```

**Frontend Deployment** (`k8s/paladin-frontend.yaml`):
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: paladin-frontend
  namespace: paladin-ai
spec:
  replicas: 2
  selector:
    matchLabels:
      app: paladin-frontend
  template:
    metadata:
      labels:
        app: paladin-frontend
    spec:
      containers:
      - name: paladin-frontend
        image: paladin-ai/frontend:latest
        ports:
        - containerPort: 3000
        env:
        - name: NEXT_PUBLIC_PALADIN_API_URL
          value: "https://api.paladin.com"
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "500m"
---
apiVersion: v1
kind: Service
metadata:
  name: paladin-frontend-service
  namespace: paladin-ai
spec:
  selector:
    app: paladin-frontend
  ports:
  - protocol: TCP
    port: 3000
    targetPort: 3000
  type: ClusterIP
```

### 3. Ingress Configuration

**Ingress** (`k8s/ingress.yaml`):
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: paladin-ingress
  namespace: paladin-ai
  annotations:
    kubernetes.io/ingress.class: "nginx"
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/rate-limit: "100"
    nginx.ingress.kubernetes.io/rate-limit-window: "1m"
spec:
  tls:
  - hosts:
    - app.paladin.com
    - api.paladin.com
    secretName: paladin-tls
  rules:
  - host: app.paladin.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: paladin-frontend-service
            port:
              number: 3000
  - host: api.paladin.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: paladin-server-service
            port:
              number: 8000
```

### 4. Secrets Management

**Secrets** (`k8s/secrets.yaml`):
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: paladin-secrets
  namespace: paladin-ai
type: Opaque
data:
  openai-api-key: <base64-encoded-key>
  mongodb-uri: <base64-encoded-uri>
  jwt-secret: <base64-encoded-secret>
  neo4j-password: <base64-encoded-password>
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: paladin-config
  namespace: paladin-ai
data:
  SERVER_HOST: "api.paladin.com"
  LOG_LEVEL: "info"
  PROMETHEUS_URL: "http://prometheus.monitoring.svc.cluster.local:9090"
```

## Scaling and High Availability

### 1. Horizontal Pod Autoscaler

**HPA Configuration** (`k8s/hpa.yaml`):
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: paladin-server-hpa
  namespace: paladin-ai
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: paladin-server
  minReplicas: 3
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 100
        periodSeconds: 15
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 10
        periodSeconds: 60
```

### 2. Pod Disruption Budget

**PDB Configuration** (`k8s/pdb.yaml`):
```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: paladin-server-pdb
  namespace: paladin-ai
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: paladin-server
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: paladin-frontend-pdb
  namespace: paladin-ai
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: paladin-frontend
```

## Security Configuration

### 1. Network Policies

**Network Security** (`k8s/network-policy.yaml`):
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: paladin-network-policy
  namespace: paladin-ai
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    - podSelector:
        matchLabels:
          app: paladin-frontend
    - podSelector:
        matchLabels:
          app: paladin-server
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
  - to:
    - podSelector:
        matchLabels:
          app: mongodb
  - to: []
    ports:
    - protocol: TCP
      port: 443  # HTTPS outbound
    - protocol: TCP
      port: 53   # DNS
    - protocol: UDP
      port: 53   # DNS
```

### 2. Security Context

**Pod Security** (`k8s/security-context.yaml`):
```yaml
apiVersion: v1
kind: SecurityContext
metadata:
  name: paladin-security-context
spec:
  runAsNonRoot: true
  runAsUser: 1001
  runAsGroup: 1001
  fsGroup: 1001
  capabilities:
    drop:
      - ALL
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
```

## Backup and Disaster Recovery

### 1. Database Backup Strategy

**MongoDB Backup Script** (`scripts/backup-mongodb.sh`):
```bash
#!/bin/bash

# MongoDB backup script
BACKUP_DIR="/backups/mongodb"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_NAME="mongodb_backup_${DATE}"

# Create backup directory
mkdir -p ${BACKUP_DIR}

# Perform backup
mongodump \
  --uri="${MONGODB_URI}" \
  --out="${BACKUP_DIR}/${BACKUP_NAME}" \
  --gzip

# Upload to cloud storage
aws s3 cp ${BACKUP_DIR}/${BACKUP_NAME} \
  s3://paladin-backups/mongodb/${BACKUP_NAME} \
  --recursive

# Cleanup local backups older than 7 days
find ${BACKUP_DIR} -type d -mtime +7 -exec rm -rf {} \;

echo "MongoDB backup completed: ${BACKUP_NAME}"
```

**Kubernetes CronJob** (`k8s/backup-cronjob.yaml`):
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: mongodb-backup
  namespace: paladin-ai
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: mongodb-backup
            image: mongo:7.0
            env:
            - name: MONGODB_URI
              valueFrom:
                secretKeyRef:
                  name: paladin-secrets
                  key: mongodb-uri
            command:
            - /bin/bash
            - -c
            - |
              mongodump --uri="${MONGODB_URI}" --gzip --out=/backup
              aws s3 cp /backup s3://paladin-backups/mongodb/$(date +%Y%m%d_%H%M%S) --recursive
            volumeMounts:
            - name: backup-storage
              mountPath: /backup
          volumes:
          - name: backup-storage
            emptyDir: {}
          restartPolicy: OnFailure
```

### 2. Disaster Recovery Procedures

**Recovery Playbook**:
```bash
# 1. Assessment Phase
kubectl get pods -n paladin-ai
kubectl describe nodes
kubectl get events -n paladin-ai

# 2. Database Recovery
# Restore MongoDB from backup
mongorestore --uri="${MONGODB_URI}" --gzip /backup/latest

# Restore Qdrant collections
curl -X POST http://qdrant:6333/collections/paladin_docs/snapshots/recover \
  -H "Content-Type: application/json" \
  -d '{"location": "s3://paladin-backups/qdrant/latest"}'

# 3. Application Recovery
kubectl rollout restart deployment/paladin-server -n paladin-ai
kubectl rollout restart deployment/paladin-frontend -n paladin-ai

# 4. Verification
kubectl get pods -n paladin-ai
curl -f https://api.paladin.com/health
curl -f https://app.paladin.com
```

## Monitoring and Observability

### 1. Application Metrics

**Custom Metrics** (`monitoring/metrics.yaml`):
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-dashboards
  namespace: monitoring
data:
  paladin-dashboard.json: |
    {
      "dashboard": {
        "title": "Paladin AI Dashboard",
        "panels": [
          {
            "title": "Request Rate",
            "targets": [
              {
                "expr": "rate(http_requests_total{job=\"paladin-server\"}[5m])"
              }
            ]
          },
          {
            "title": "Response Time",
            "targets": [
              {
                "expr": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket{job=\"paladin-server\"}[5m]))"
              }
            ]
          },
          {
            "title": "Error Rate",
            "targets": [
              {
                "expr": "rate(http_requests_total{job=\"paladin-server\",status=~\"5..\"}[5m])"
              }
            ]
          }
        ]
      }
    }
```

### 2. Alert Rules

**Prometheus Alert Rules** (`monitoring/alerts.yaml`):
```yaml
groups:
- name: paladin.rules
  rules:
  - alert: PaladinServerDown
    expr: up{job="paladin-server"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Paladin server is down"
      description: "Paladin server has been down for more than 1 minute"

  - alert: HighErrorRate
    expr: rate(http_requests_total{job="paladin-server",status=~"5.."}[5m]) > 0.1
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High error rate detected"
      description: "Error rate is above 10% for more than 2 minutes"

  - alert: HighResponseTime
    expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket{job="paladin-server"}[5m])) > 2
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High response time detected"
      description: "95th percentile response time is above 2 seconds"

  - alert: DatabaseConnectionIssue
    expr: mongodb_up == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Database connection issue"
      description: "Cannot connect to MongoDB"
```

## Performance Optimization

### 1. Resource Limits and Requests

**Resource Optimization** (`k8s/resources.yaml`):
```yaml
# Paladin Server Resources
resources:
  requests:
    memory: "1Gi"
    cpu: "500m"
  limits:
    memory: "4Gi"
    cpu: "2"

# Frontend Resources
resources:
  requests:
    memory: "256Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"

# Database Resources
resources:
  requests:
    memory: "2Gi"
    cpu: "1"
  limits:
    memory: "8Gi"
    cpu: "4"
```

### 2. Caching Strategy

**Redis/Valkey Configuration**:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: valkey-config
  namespace: paladin-ai
data:
  valkey.conf: |
    maxmemory 2gb
    maxmemory-policy allkeys-lru
    save 900 1
    save 300 10
    save 60 10000
    appendonly yes
    appendfsync everysec
```

## CI/CD Pipeline

### 1. GitHub Actions Workflow

**Build and Deploy** (`.github/workflows/deploy.yml`):
```yaml
name: Build and Deploy

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Python
      uses: actions/setup-python@v4
      with:
        python-version: '3.13'
    
    - name: Install dependencies
      run: |
        cd server
        pip install uv
        uv sync
    
    - name: Run tests
      run: |
        cd server
        uv run pytest tests/
    
    - name: Lint code
      run: |
        cd server
        uv run ruff check .
        uv run black --check .

  build:
    needs: test
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Docker Buildx
      uses: docker/setup-buildx-action@v2
    
    - name: Login to Container Registry
      uses: docker/login-action@v2
      with:
        registry: ghcr.io
        username: ${{ github.actor }}
        password: ${{ secrets.GITHUB_TOKEN }}
    
    - name: Build and push server image
      uses: docker/build-push-action@v4
      with:
        context: ./server
        push: true
        tags: ghcr.io/${{ github.repository }}/server:latest
    
    - name: Build and push frontend image
      uses: docker/build-push-action@v4
      with:
        context: ./frontend
        push: true
        tags: ghcr.io/${{ github.repository }}/frontend:latest

  deploy:
    needs: build
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Deploy to Kubernetes
      run: |
        echo "${{ secrets.KUBECONFIG }}" | base64 -d > kubeconfig
        export KUBECONFIG=kubeconfig
        kubectl apply -f k8s/
        kubectl rollout status deployment/paladin-server -n paladin-ai
        kubectl rollout status deployment/paladin-frontend -n paladin-ai
```

## Health Checks and Readiness

### 1. Application Health Endpoints

**Health Check Implementation**:
```python
# server/health.py
from fastapi import APIRouter
from typing import Dict, Any
import asyncio
import httpx

router = APIRouter()

@router.get("/health")
async def health_check() -> Dict[str, Any]:
    """Comprehensive health check"""
    
    health_status = {
        "status": "healthy",
        "version": "1.0.0",
        "timestamp": datetime.utcnow().isoformat(),
        "checks": {}
    }
    
    # Database connectivity checks
    try:
        # MongoDB check
        await mongodb_client.admin.command('ping')
        health_status["checks"]["mongodb"] = "healthy"
    except Exception as e:
        health_status["checks"]["mongodb"] = f"unhealthy: {e}"
        health_status["status"] = "degraded"
    
    # External service checks
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(
                "https://api.openai.com/v1/models",
                headers={"Authorization": f"Bearer {openai_api_key}"},
                timeout=5.0
            )
            health_status["checks"]["openai"] = "healthy" if response.status_code == 200 else "unhealthy"
    except Exception as e:
        health_status["checks"]["openai"] = f"unhealthy: {e}"
    
    return health_status

@router.get("/ready")
async def readiness_check() -> Dict[str, Any]:
    """Readiness probe for Kubernetes"""
    
    # Check if application is ready to serve requests
    ready_checks = await asyncio.gather(
        check_database_ready(),
        check_ai_services_ready(),
        check_cache_ready(),
        return_exceptions=True
    )
    
    all_ready = all(check is True for check in ready_checks if not isinstance(check, Exception))
    
    return {
        "ready": all_ready,
        "checks": ready_checks
    }
```

### 2. Kubernetes Probes

**Probe Configuration**:
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8000
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /ready
    port: 8000
  initialDelaySeconds: 5
  periodSeconds: 5
  timeoutSeconds: 3
  successThreshold: 1
  failureThreshold: 3

startupProbe:
  httpGet:
    path: /health
    port: 8000
  initialDelaySeconds: 10
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 30
```

## Troubleshooting Deployment Issues

### Common Problems and Solutions

#### 1. Container Startup Issues

```bash
# Check container logs
kubectl logs -f deployment/paladin-server -n paladin-ai

# Describe pod for events
kubectl describe pod <pod-name> -n paladin-ai

# Check resource constraints
kubectl top pods -n paladin-ai
```

#### 2. Database Connection Problems

```bash
# Test database connectivity
kubectl exec -it <pod-name> -n paladin-ai -- mongosh "${MONGODB_URI}"

# Check network policies
kubectl get networkpolicies -n paladin-ai

# Verify secrets
kubectl get secrets -n paladin-ai
kubectl describe secret paladin-secrets -n paladin-ai
```

#### 3. Performance Issues

```bash
# Monitor resource usage
kubectl top pods -n paladin-ai
kubectl top nodes

# Check HPA status
kubectl get hpa -n paladin-ai

# Review metrics
curl http://prometheus.monitoring.svc.cluster.local:9090/api/v1/query?query=rate(http_requests_total[5m])
```

## Best Practices

### 1. Security

- **Principle of Least Privilege**: Minimal required permissions
- **Secrets Management**: External secret management systems
- **Network Segmentation**: Network policies and service mesh
- **Regular Updates**: Automated security patching

### 2. Reliability

- **Graceful Degradation**: Fallback mechanisms for failures
- **Circuit Breakers**: Prevent cascade failures
- **Retries with Backoff**: Resilient external service calls
- **Health Monitoring**: Comprehensive health checks

### 3. Performance

- **Resource Planning**: Appropriate resource allocation
- **Caching Strategy**: Multi-level caching
- **Database Optimization**: Proper indexing and query optimization
- **CDN Usage**: Static asset delivery optimization

### 4. Monitoring

- **Observability**: Three pillars (metrics, logs, traces)
- **SLI/SLO Definitions**: Service level objectives
- **Alert Fatigue Prevention**: Meaningful alerts only
- **Runbook Automation**: Automated incident response

---

This deployment guide provides a comprehensive foundation for deploying Paladin AI across various environments, from development to production. The modular approach allows for customization based on specific requirements while maintaining security, reliability, and performance standards.