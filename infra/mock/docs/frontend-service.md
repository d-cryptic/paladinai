# Frontend Service Documentation

## Service Overview
The Frontend service is a Node.js/Express-based application that simulates a React-like frontend application. It serves as the user-facing component of the 3-tier architecture, handling web requests and communicating with backend services through the Nginx proxy.

## Service Details

### Container Information
- **Image**: Custom built from `./frontend/Dockerfile`
- **Container Name**: `mock-frontend`
- **Base Image**: `node:18-alpine`
- **Network**: `mock-network`

### Port Configuration
- **Internal Port**: 3000
- **External Port**: 3001
- **Access URL**: http://localhost:3001

### Environment Variables
- `SERVICE_NAME=frontend` - Identifies the service in logs and metrics
- `BACKEND_URL=http://nginx:80` - URL for backend API calls through Nginx
- `ERROR_RATE=0.1` - Configures 10% simulated error rate
- `LOG_LEVEL=debug` - Logging verbosity level

### Dependencies
- backend-1 (Backend service instance 1)
- backend-2 (Backend service instance 2)
- nginx (Reverse proxy and load balancer)

## API Endpoints

### 1. GET /
- **Description**: Home page
- **Response**: HTML page with service information
- **Metrics**: Increments page view counter

### 2. GET /health
- **Description**: Health check endpoint
- **Response**: JSON with service status
- **Example Response**:
  ```json
  {
    "status": "healthy",
    "service": "frontend",
    "uptime": 123456,
    "timestamp": "2024-01-01T12:00:00Z"
  }
  ```

### 3. GET /metrics
- **Description**: Prometheus metrics endpoint
- **Response**: Prometheus-formatted metrics
- **Key Metrics**:
  - `http_requests_total` - Total HTTP requests by method, route, status
  - `http_request_duration_seconds` - Request duration histogram
  - `page_views_total` - Total page views
  - `active_users` - Current active users (simulated)
  - `frontend_errors_total` - Total frontend errors

### 4. GET /api/data
- **Description**: Fetches data from backend through Nginx
- **Response**: JSON data from backend
- **Error Handling**: Returns error page on backend failure

### 5. GET /api/users
- **Description**: Fetches user list from backend
- **Response**: JSON array of users
- **Caching**: Results are cached in memory

### 6. GET /error/:code
- **Description**: Simulates HTTP errors for testing
- **Parameters**: 
  - `code` - HTTP error code (404, 500, etc.)
- **Response**: Error page with specified code

## Logging

### Log Configuration
- **Log File**: `/app/logs/frontend.log`
- **Format**: JSON structured logs
- **Fields**:
  - timestamp
  - level (debug, info, warn, error)
  - message
  - service
  - metadata (request details, errors)

### Log Examples
```json
{
  "timestamp": "2024-01-01T12:00:00Z",
  "level": "info",
  "service": "frontend",
  "message": "Request received",
  "method": "GET",
  "path": "/api/data",
  "ip": "172.18.0.5"
}
```

## Metrics Details

### HTTP Metrics
- **Metric**: `http_requests_total`
- **Type**: Counter
- **Labels**: method, route, status_code, service
- **Description**: Total number of HTTP requests

### Duration Metrics
- **Metric**: `http_request_duration_seconds`
- **Type**: Histogram
- **Labels**: method, route, service
- **Description**: Request processing duration

### Business Metrics
- **Metric**: `page_views_total`
- **Type**: Counter
- **Description**: Total page views

### Error Metrics
- **Metric**: `frontend_errors_total`
- **Type**: Counter
- **Labels**: error_type
- **Description**: Total frontend errors by type

## Error Simulation

The frontend service simulates various error conditions:

1. **Random Errors** (10% rate):
   - 500 Internal Server Error
   - 502 Bad Gateway
   - 503 Service Unavailable

2. **Timeout Errors**:
   - Simulated slow responses (5+ seconds)
   - Connection timeouts to backend

3. **Client Errors**:
   - 404 Not Found for invalid routes
   - 400 Bad Request for malformed data

## Performance Characteristics

### Request Handling
- **Concurrent Connections**: Up to 1000
- **Average Response Time**: 50-200ms (normal)
- **Timeout**: 30 seconds
- **Keep-Alive**: Enabled

### Resource Usage
- **Memory**: ~128MB typical
- **CPU**: Low usage, spikes during load
- **Disk**: Minimal (logs only)

## Monitoring Alerts

### High Error Rate Alert
- **Threshold**: Error rate > 10% for 2 minutes
- **Severity**: Warning
- **Action**: Check backend connectivity and error logs

### High Response Time Alert
- **Threshold**: P95 latency > 2 seconds for 5 minutes
- **Severity**: Warning
- **Action**: Check backend performance and resource usage

### Service Down Alert
- **Threshold**: Health check fails for 1 minute
- **Severity**: Critical
- **Action**: Check container status and logs

## Troubleshooting

### Common Issues

1. **Cannot connect to backend**
   - Check Nginx is running
   - Verify BACKEND_URL configuration
   - Check network connectivity

2. **High error rate**
   - Check ERROR_RATE environment variable
   - Review backend service health
   - Check error logs for patterns

3. **Slow performance**
   - Monitor backend response times
   - Check CPU/memory usage
   - Review concurrent connection count

### Debug Commands
```bash
# Check container logs
docker logs mock-frontend

# Access container shell
docker exec -it mock-frontend sh

# Check metrics
curl http://localhost:3001/metrics

# Test backend connectivity
docker exec mock-frontend curl http://nginx:80/health
```

## Integration Points

### Nginx Proxy
- All backend API calls go through Nginx
- Load balanced between backend-1 and backend-2
- Handles SSL termination in production

### Backend Services
- RESTful API communication
- JSON data format
- Authentication headers passed through

### Monitoring Stack
- Exports metrics to Prometheus
- Logs shipped to Loki via Promtail
- Alerts configured in Prometheus

## Security Considerations

1. **Input Validation**: All user inputs are validated
2. **XSS Protection**: Content Security Policy headers
3. **CORS**: Configured for same-origin only
4. **Rate Limiting**: Handled by Nginx proxy
5. **Authentication**: Token-based (simulated)