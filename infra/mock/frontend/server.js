const express = require('express');
const client = require('prom-client');
const winston = require('winston');
const axios = require('axios');
const fs = require('fs');
const path = require('path');

const app = express();
const PORT = process.env.PORT || 3000;
const SERVICE_NAME = process.env.SERVICE_NAME || 'frontend';
const BACKEND_URL = process.env.BACKEND_URL || 'http://nginx:80';
const ERROR_RATE = parseFloat(process.env.ERROR_RATE || '0.1');

// Ensure logs directory exists
const logsDir = '/app/logs';
if (!fs.existsSync(logsDir)) {
    fs.mkdirSync(logsDir, { recursive: true });
}

// Configure Winston logger
const logger = winston.createLogger({
    level: process.env.LOG_LEVEL || 'info',
    format: winston.format.combine(
        winston.format.timestamp(),
        winston.format.json()
    ),
    defaultMeta: { service: SERVICE_NAME },
    transports: [
        new winston.transports.File({ 
            filename: path.join(logsDir, `${SERVICE_NAME}-error.log`), 
            level: 'error' 
        }),
        new winston.transports.File({ 
            filename: path.join(logsDir, `${SERVICE_NAME}.log`) 
        }),
        new winston.transports.Console({
            format: winston.format.simple()
        })
    ]
});

// Prometheus metrics
const register = new client.Registry();
register.setDefaultLabels({
    app: SERVICE_NAME
});
client.collectDefaultMetrics({ register });

// Custom metrics
const httpRequestDuration = new client.Histogram({
    name: 'http_request_duration_seconds',
    help: 'Duration of HTTP requests in seconds',
    labelNames: ['method', 'route', 'status_code'],
    buckets: [0.1, 0.5, 1, 2, 5]
});

const httpRequestsTotal = new client.Counter({
    name: 'http_requests_total',
    help: 'Total number of HTTP requests',
    labelNames: ['method', 'route', 'status_code']
});

const errorRate = new client.Gauge({
    name: 'error_rate',
    help: 'Current error rate',
    labelNames: ['service']
});

const activeUsers = new client.Gauge({
    name: 'active_users',
    help: 'Number of active users',
    labelNames: ['service']
});

register.registerMetric(httpRequestDuration);
register.registerMetric(httpRequestsTotal);
register.registerMetric(errorRate);
register.registerMetric(activeUsers);

// Middleware to track metrics
app.use((req, res, next) => {
    const start = Date.now();
    
    res.on('finish', () => {
        const duration = (Date.now() - start) / 1000;
        httpRequestDuration.observe(
            { method: req.method, route: req.route?.path || req.path, status_code: res.statusCode },
            duration
        );
        httpRequestsTotal.inc({
            method: req.method,
            route: req.route?.path || req.path,
            status_code: res.statusCode
        });
        
        logger.info({
            method: req.method,
            path: req.path,
            statusCode: res.statusCode,
            duration: duration * 1000,
            ip: req.ip,
            userAgent: req.get('user-agent')
        });
    });
    
    next();
});

// Routes
app.get('/', (req, res) => {
    logger.info('Home page accessed');
    res.json({ 
        message: 'Mock Frontend Service',
        service: SERVICE_NAME,
        timestamp: new Date().toISOString()
    });
});

app.get('/api/data', async (req, res) => {
    try {
        // Simulate random errors
        if (Math.random() < ERROR_RATE) {
            throw new Error('Random frontend error occurred');
        }
        
        // Make request to backend
        const response = await axios.get(`${BACKEND_URL}/api/backend/data`);
        
        logger.info('Data fetched successfully from backend');
        res.json({
            frontend: SERVICE_NAME,
            data: response.data,
            timestamp: new Date().toISOString()
        });
    } catch (error) {
        logger.error('Error fetching data', { error: error.message, stack: error.stack });
        res.status(500).json({ 
            error: 'Internal Server Error', 
            service: SERVICE_NAME,
            details: error.message 
        });
    }
});

app.get('/api/users', async (req, res) => {
    try {
        // Simulate user activity
        const users = Math.floor(Math.random() * 1000) + 100;
        activeUsers.set({ service: SERVICE_NAME }, users);
        
        // Random error
        if (Math.random() < ERROR_RATE) {
            throw new Error('Failed to fetch users');
        }
        
        logger.info(`Active users: ${users}`);
        res.json({ 
            users: users,
            service: SERVICE_NAME,
            timestamp: new Date().toISOString()
        });
    } catch (error) {
        logger.error('Error in users endpoint', { error: error.message });
        res.status(500).json({ error: error.message });
    }
});

app.get('/health', (req, res) => {
    const health = {
        status: 'healthy',
        service: SERVICE_NAME,
        uptime: process.uptime(),
        timestamp: new Date().toISOString()
    };
    
    // Randomly report unhealthy
    if (Math.random() < 0.05) { // 5% chance
        health.status = 'unhealthy';
        health.error = 'Service degradation detected';
        logger.warn('Health check failed', health);
        return res.status(503).json(health);
    }
    
    res.json(health);
});

app.get('/metrics', (req, res) => {
    res.set('Content-Type', register.contentType);
    register.metrics().then(metrics => {
        res.end(metrics);
    });
});

// Error generation endpoints
app.get('/error/404', (req, res) => {
    logger.warn('404 error endpoint accessed');
    res.status(404).json({ error: 'Not Found', service: SERVICE_NAME });
});

app.get('/error/500', (req, res) => {
    logger.error('500 error endpoint accessed');
    res.status(500).json({ error: 'Internal Server Error', service: SERVICE_NAME });
});

app.get('/error/timeout', async (req, res) => {
    logger.warn('Timeout simulation started');
    await new Promise(resolve => setTimeout(resolve, 30000)); // 30 second timeout
    res.json({ message: 'This should timeout' });
});

// Simulate various error scenarios periodically
setInterval(() => {
    // Update error rate metric
    errorRate.set({ service: SERVICE_NAME }, ERROR_RATE);
    
    // Generate random log entries
    const logLevels = ['info', 'warn', 'error'];
    const messages = [
        'User session started',
        'API call initiated',
        'Cache miss detected',
        'Slow query warning',
        'Memory usage high',
        'Connection pool exhausted',
        'Rate limit exceeded',
        'Authentication failed',
        'Database connection error',
        'External API timeout'
    ];
    
    const level = logLevels[Math.floor(Math.random() * logLevels.length)];
    const message = messages[Math.floor(Math.random() * messages.length)];
    
    logger[level](message, {
        userId: `user_${Math.floor(Math.random() * 10000)}`,
        sessionId: `session_${Math.floor(Math.random() * 100000)}`,
        duration: Math.floor(Math.random() * 5000),
        endpoint: `/api/${['users', 'data', 'products', 'orders'][Math.floor(Math.random() * 4)]}`
    });
}, 5000); // Every 5 seconds

// Global error handler
app.use((err, req, res, next) => {
    logger.error('Unhandled error', { 
        error: err.message, 
        stack: err.stack,
        path: req.path,
        method: req.method
    });
    res.status(500).json({ 
        error: 'Internal Server Error', 
        service: SERVICE_NAME 
    });
});

app.listen(PORT, () => {
    logger.info(`${SERVICE_NAME} listening on port ${PORT}`);
    console.log(`${SERVICE_NAME} listening on port ${PORT}`);
});