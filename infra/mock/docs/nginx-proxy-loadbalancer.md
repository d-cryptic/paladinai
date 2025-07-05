# Nginx Proxy & Load Balancer Documentation

## Service Overview
Nginx serves as the central reverse proxy and load balancer for the mock infrastructure. It handles incoming HTTP requests, distributes traffic across backend instances, implements rate limiting, and provides SSL termination capabilities.

## Service Details

### Container Information
- **Image**: `nginx:alpine`
- **Container Name**: `mock-nginx`
- **Version**: Nginx 1.x (Alpine Linux)
- **Network**: `mock-network`

### Port Configuration
- **HTTP Port**: 8080 (external) → 80 (internal)
- **Metrics Port**: 9113 (nginx-prometheus-exporter)
- **Access URLs**:
  - Main proxy: http://localhost:8080
  - Nginx status: http://localhost:8080/nginx_status
  - Metrics: http://localhost:8080/metrics

### Volume Mounts
- `./nginx/nginx.conf:/etc/nginx/nginx.conf` - Main configuration
- `./nginx/default.conf:/etc/nginx/conf.d/default.conf` - Server configuration
- `./logs:/var/log/nginx` - Log files

### Dependencies
- backend-1 (Backend service instance 1)
- backend-2 (Backend service instance 2)

## Configuration Structure

### Main Configuration (nginx.conf)
```nginx
user nginx;
worker_processes auto;
error_log /var/log/nginx/error.log warn;
pid /var/run/nginx.pid;

events {
    worker_connections 1024;
    use epoll;
    multi_accept on;
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    # Logging
    log_format main '$remote_addr - $remote_user [$time_local] "$request" '
                    '$status $body_bytes_sent "$http_referer" '
                    '"$http_user_agent" "$http_x_forwarded_for" '
                    'rt=$request_time uct="$upstream_connect_time" '
                    'uht="$upstream_header_time" urt="$upstream_response_time"';

    access_log /var/log/nginx/access.log main;

    # Performance
    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    types_hash_max_size 2048;

    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_comp_level 6;
    gzip_types text/plain text/css text/xml text/javascript 
               application/json application/javascript application/xml+rss;

    # Rate limiting zones
    limit_req_zone $binary_remote_addr zone=api_limit:10m rate=10r/s;
    limit_req_zone $binary_remote_addr zone=general_limit:10m rate=50r/s;

    # Connection limits
    limit_conn_zone $binary_remote_addr zone=addr:10m;
    limit_conn addr 100;

    # Include server configurations
    include /etc/nginx/conf.d/*.conf;
}
```

## Load Balancing Configuration

### Upstream Backend Servers
```nginx
upstream backend_servers {
    least_conn;  # Load balancing algorithm
    
    # Backend instances with health checks
    server backend-1:4000 weight=1 max_fails=3 fail_timeout=30s;
    server backend-2:4000 weight=1 max_fails=3 fail_timeout=30s;
    
    # Keepalive connections for performance
    keepalive 32;
}
```

### Load Balancing Algorithms
1. **least_conn** (Current): Routes to server with fewest active connections
2. **round_robin**: Default, distributes requests sequentially
3. **ip_hash**: Session persistence based on client IP
4. **random**: Random selection with optional weighting

### Health Check Parameters
- **max_fails=3**: Mark server down after 3 failures
- **fail_timeout=30s**: Time to mark server down and retry interval
- **weight=1**: Equal distribution (can be adjusted for capacity)

## Route Configuration

### 1. Frontend Proxy (/)
```nginx
location / {
    proxy_pass http://frontend:3000;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    
    # Timeouts
    proxy_connect_timeout 5s;
    proxy_send_timeout 60s;
    proxy_read_timeout 60s;
    
    # Error handling
    proxy_intercept_errors on;
    error_page 502 503 504 /50x.html;
}
```

### 2. Backend API Load Balancing (/api/backend/)
```nginx
location /api/backend/ {
    # Rate limiting
    limit_req zone=api_limit burst=20 nodelay;
    
    proxy_pass http://backend_servers/api/backend/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    
    # Load balancing headers
    proxy_set_header X-Backend-Server $upstream_addr;
    proxy_set_header X-Backend-Status $upstream_status;
    
    # Timeouts
    proxy_connect_timeout 5s;
    proxy_send_timeout 60s;
    proxy_read_timeout 60s;
    
    # Retry on failure
    proxy_next_upstream error timeout http_502 http_503 http_504;
    proxy_next_upstream_tries 2;
    proxy_next_upstream_timeout 10s;
    
    # Debug headers
    add_header X-Upstream-Server $upstream_addr always;
    add_header X-Upstream-Response-Time $upstream_response_time always;
    add_header X-Service "nginx-proxy" always;
}
```

### 3. Health Check Endpoint (/health)
```nginx
location /health {
    access_log off;
    default_type application/json;
    return 200 '{"status":"healthy","service":"nginx"}';
}
```

### 4. Nginx Status (/nginx_status)
```nginx
location /nginx_status {
    stub_status on;
    access_log off;
    allow all;  # Restrict in production
}
```

### 5. Metrics Endpoint (/metrics)
```nginx
location /metrics {
    access_log off;
    proxy_pass http://localhost:9113/metrics;
}
```

## Rate Limiting

### Configuration
```nginx
# Define rate limit zones
limit_req_zone $binary_remote_addr zone=api_limit:10m rate=10r/s;
limit_req_zone $binary_remote_addr zone=general_limit:10m rate=50r/s;

# Apply rate limits
location /api/ {
    limit_req zone=api_limit burst=20 nodelay;
}
```

### Rate Limit Parameters
- **zone=api_limit**: Named memory zone for tracking
- **rate=10r/s**: 10 requests per second per IP
- **burst=20**: Allow burst of 20 requests
- **nodelay**: Don't delay burst requests

### Rate Limit Responses
- **503 Service Unavailable**: When rate limit exceeded
- **X-RateLimit-Limit**: Requests allowed per window
- **X-RateLimit-Remaining**: Requests remaining
- **X-RateLimit-Reset**: Window reset time

## Security Headers

### Security Configuration
```nginx
# Security headers applied to all responses
add_header X-Frame-Options "SAMEORIGIN" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-XSS-Protection "1; mode=block" always;
add_header Referrer-Policy "no-referrer-when-downgrade" always;
add_header Content-Security-Policy "default-src 'self'" always;
```

### CORS Configuration (if needed)
```nginx
location /api/ {
    if ($request_method = 'OPTIONS') {
        add_header 'Access-Control-Allow-Origin' '*';
        add_header 'Access-Control-Allow-Methods' 'GET, POST, PUT, DELETE, OPTIONS';
        add_header 'Access-Control-Allow-Headers' 'Authorization, Content-Type';
        add_header 'Access-Control-Max-Age' 1728000;
        return 204;
    }
    
    add_header 'Access-Control-Allow-Origin' '*' always;
}
```

## Performance Optimization

### Connection Pooling
```nginx
upstream backend_servers {
    least_conn;
    server backend-1:4000;
    server backend-2:4000;
    
    # Keep connections alive
    keepalive 32;
    keepalive_requests 100;
    keepalive_timeout 60s;
}
```

### Caching Configuration
```nginx
# Define cache
proxy_cache_path /var/cache/nginx levels=1:2 keys_zone=api_cache:10m 
                 max_size=1g inactive=60m use_temp_path=off;

# Use cache for GET requests
location /api/backend/data {
    proxy_cache api_cache;
    proxy_cache_valid 200 5m;
    proxy_cache_valid 404 1m;
    proxy_cache_use_stale error timeout http_500 http_502 http_503 http_504;
    proxy_cache_background_update on;
    proxy_cache_lock on;
    
    add_header X-Cache-Status $upstream_cache_status;
}
```

### Buffer Configuration
```nginx
# Buffering settings
proxy_buffering on;
proxy_buffer_size 4k;
proxy_buffers 8 4k;
proxy_busy_buffers_size 8k;

# Large headers
large_client_header_buffers 4 16k;

# File uploads
client_max_body_size 10m;
client_body_buffer_size 128k;
```

## Monitoring and Metrics

### Nginx Status Metrics
Available at `/nginx_status`:
```
Active connections: 291
server accepts handled requests
 16630948 16630948 31070465
Reading: 6 Writing: 179 Waiting: 106
```

- **Active connections**: Current active client connections
- **accepts**: Total accepted connections
- **handled**: Total handled connections
- **requests**: Total client requests
- **Reading**: Reading client request
- **Writing**: Writing response to client
- **Waiting**: Keep-alive connections

### Prometheus Metrics (via nginx-exporter)
Key metrics exposed at port 9113:

1. **Connection Metrics**:
   - `nginx_connections_active` - Active connections
   - `nginx_connections_accepted` - Total accepted
   - `nginx_connections_handled` - Total handled
   - `nginx_connections_reading` - Reading state
   - `nginx_connections_writing` - Writing state
   - `nginx_connections_waiting` - Waiting state

2. **Request Metrics**:
   - `nginx_http_requests_total` - Total HTTP requests
   - `nginx_http_request_duration_seconds` - Request duration
   - `nginx_http_upstream_latency_seconds` - Upstream latency

3. **Upstream Metrics**:
   - `nginx_upstream_up` - Upstream server status
   - `nginx_upstream_requests_total` - Requests per upstream
   - `nginx_upstream_response_time` - Response time per upstream

## Logging

### Access Log Format
```
$remote_addr - $remote_user [$time_local] "$request" 
$status $body_bytes_sent "$http_referer" 
"$http_user_agent" "$http_x_forwarded_for"
rt=$request_time uct="$upstream_connect_time"
uht="$upstream_header_time" urt="$upstream_response_time"
```

### Log Files
- **Access Log**: `/var/log/nginx/access.log`
- **Error Log**: `/var/log/nginx/error.log`

### Log Analysis Examples
```bash
# Top IPs by request count
awk '{print $1}' /var/log/nginx/access.log | sort | uniq -c | sort -rn | head -10

# Requests per minute
awk '{print $4}' /var/log/nginx/access.log | cut -d: -f1-3 | uniq -c

# Slow requests (>1s)
grep "rt=[1-9]" /var/log/nginx/access.log

# Error responses
awk '$9 ~ /^[4-5]/ {print $9}' /var/log/nginx/access.log | sort | uniq -c

# Upstream response times
grep -o 'urt="[0-9.]*"' /var/log/nginx/access.log | cut -d'"' -f2 | sort -n
```

## Error Handling

### Error Pages
```nginx
error_page 404 /404.html;
error_page 500 502 503 504 /50x.html;

location = /50x.html {
    root /usr/share/nginx/html;
    internal;
}
```

### Upstream Failure Handling
```nginx
# Retry logic
proxy_next_upstream error timeout invalid_header http_500 http_502 http_503;
proxy_next_upstream_tries 2;
proxy_next_upstream_timeout 10s;

# Fallback for all upstream failures
error_page 502 503 504 @fallback;
location @fallback {
    return 503 '{"error":"Service temporarily unavailable"}';
}
```

## SSL/TLS Configuration (Production)

### SSL Configuration Example
```nginx
server {
    listen 443 ssl http2;
    server_name example.com;

    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;
    
    # Modern configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;
    ssl_prefer_server_ciphers off;
    
    # OCSP stapling
    ssl_stapling on;
    ssl_stapling_verify on;
    
    # Session cache
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;
    
    # HSTS
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
}
```

## Troubleshooting

### Common Issues

1. **502 Bad Gateway**
   ```bash
   # Check upstream connectivity
   docker exec mock-nginx curl -I http://backend-1:4000/health
   
   # Check error logs
   docker logs mock-nginx | grep error
   
   # Verify upstream configuration
   docker exec mock-nginx nginx -t
   ```

2. **High Response Times**
   ```bash
   # Check upstream response times
   grep "urt=" /var/log/nginx/access.log | tail -100
   
   # Monitor active connections
   watch -n 1 'curl -s http://localhost:8080/nginx_status'
   ```

3. **Rate Limit Issues**
   ```bash
   # Check rate limit hits
   grep "limiting requests" /var/log/nginx/error.log
   
   # Adjust rate limits
   # Edit nginx.conf and increase rate/burst values
   ```

4. **Memory Issues**
   ```bash
   # Check Nginx process memory
   docker stats mock-nginx
   
   # Review buffer sizes
   grep buffer /etc/nginx/nginx.conf
   ```

### Debug Commands
```bash
# Test configuration
docker exec mock-nginx nginx -t

# Reload configuration
docker exec mock-nginx nginx -s reload

# Check processes
docker exec mock-nginx ps aux | grep nginx

# Test upstream
docker exec mock-nginx curl -v http://backend-1:4000/health

# Monitor logs
docker logs -f mock-nginx
```

## Performance Tuning

### System Limits
```bash
# File descriptors
worker_rlimit_nofile 65535;

# In container
ulimit -n 65535
```

### Kernel Parameters (Host)
```bash
# Increase connection backlog
sysctl -w net.core.somaxconn=65535

# TCP optimization
sysctl -w net.ipv4.tcp_fin_timeout=30
sysctl -w net.ipv4.tcp_keepalive_time=300
sysctl -w net.ipv4.tcp_tw_reuse=1
```

### Nginx Optimization
```nginx
# Worker processes
worker_processes auto;
worker_cpu_affinity auto;

# Events
events {
    worker_connections 2048;
    use epoll;
    multi_accept on;
}

# HTTP optimizations
http {
    open_file_cache max=1000 inactive=20s;
    open_file_cache_valid 30s;
    open_file_cache_min_uses 2;
    open_file_cache_errors on;
}
```

## Integration Points

### Frontend Service
- Proxies all requests to frontend:3000
- Handles static assets
- WebSocket support if needed

### Backend Services
- Load balances between backend-1 and backend-2
- Health-check based routing
- Automatic failover

### Monitoring Stack
- Exports stub_status for metrics
- Prometheus scrapes every 15s
- Alerts on high error rates

### Logging Stack
- Logs shipped to Loki via Promtail
- Structured logging for analysis
- Real-time log streaming