const express = require('express');
const client = require('prom-client');
const winston = require('winston');
const { Pool } = require('pg');
const redis = require('redis');
const fs = require('fs');
const path = require('path');

const app = express();
const PORT = process.env.PORT || 4000;
const SERVICE_NAME = process.env.SERVICE_NAME || 'backend';
const ERROR_RATE = parseFloat(process.env.ERROR_RATE || '0.15');

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

// Database connection
const pgPool = new Pool({
    host: process.env.DB_HOST || 'postgres',
    port: process.env.DB_PORT || 5432,
    database: process.env.DB_NAME || 'mockdb',
    user: process.env.DB_USER || 'mockuser',
    password: process.env.DB_PASSWORD || 'mockpass',
    max: 20,
    idleTimeoutMillis: 30000,
    connectionTimeoutMillis: 2000,
});

// Redis connection
const redisClient = redis.createClient({
    socket: {
        host: process.env.REDIS_HOST || 'valkey',
        port: process.env.REDIS_PORT || 6379
    }
});

redisClient.on('error', err => logger.error('Redis Client Error', err));
redisClient.connect().catch(console.error);

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

const dbQueryDuration = new client.Histogram({
    name: 'db_query_duration_seconds',
    help: 'Duration of database queries in seconds',
    labelNames: ['query_type', 'table'],
    buckets: [0.01, 0.05, 0.1, 0.5, 1]
});

const cacheHits = new client.Counter({
    name: 'cache_hits_total',
    help: 'Total number of cache hits',
    labelNames: ['cache_type']
});

const cacheMisses = new client.Counter({
    name: 'cache_misses_total',
    help: 'Total number of cache misses',
    labelNames: ['cache_type']
});

const queueSize = new client.Gauge({
    name: 'queue_size',
    help: 'Current queue size',
    labelNames: ['queue_name']
});

const errorRate = new client.Gauge({
    name: 'error_rate',
    help: 'Current error rate',
    labelNames: ['service']
});

register.registerMetric(httpRequestDuration);
register.registerMetric(httpRequestsTotal);
register.registerMetric(dbQueryDuration);
register.registerMetric(cacheHits);
register.registerMetric(cacheMisses);
register.registerMetric(queueSize);
register.registerMetric(errorRate);

// Middleware
app.use(express.json());

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
app.get('/api/backend/data', async (req, res) => {
    try {
        // Simulate random errors
        if (Math.random() < ERROR_RATE) {
            const errors = [
                'Database connection timeout',
                'Cache service unavailable',
                'Memory allocation failed',
                'Query execution error',
                'Transaction deadlock detected'
            ];
            throw new Error(errors[Math.floor(Math.random() * errors.length)]);
        }
        
        // Try cache first
        const cacheKey = 'backend:data';
        try {
            const cached = await redisClient.get(cacheKey);
            if (cached) {
                cacheHits.inc({ cache_type: 'redis' });
                logger.info('Cache hit for data endpoint');
                return res.json(JSON.parse(cached));
            }
            cacheMisses.inc({ cache_type: 'redis' });
        } catch (err) {
            logger.error('Redis error', { error: err.message });
        }
        
        // Query database
        const start = Date.now();
        const result = await pgPool.query('SELECT * FROM mock_data ORDER BY RANDOM() LIMIT 10');
        const dbDuration = (Date.now() - start) / 1000;
        dbQueryDuration.observe({ query_type: 'select', table: 'mock_data' }, dbDuration);
        
        const responseData = {
            service: SERVICE_NAME,
            data: result.rows,
            timestamp: new Date().toISOString(),
            dbQueryTime: dbDuration
        };
        
        // Cache the result
        try {
            await redisClient.setEx(cacheKey, 60, JSON.stringify(responseData));
        } catch (err) {
            logger.error('Failed to cache data', { error: err.message });
        }
        
        logger.info('Data endpoint successful', { rowCount: result.rows.length });
        res.json(responseData);
    } catch (error) {
        logger.error('Error in data endpoint', { 
            error: error.message, 
            stack: error.stack,
            service: SERVICE_NAME 
        });
        res.status(500).json({ 
            error: 'Internal Server Error', 
            service: SERVICE_NAME,
            details: error.message 
        });
    }
});

app.post('/api/backend/process', async (req, res) => {
    try {
        const { data } = req.body;
        
        // Simulate processing time
        await new Promise(resolve => setTimeout(resolve, Math.random() * 1000));
        
        // Simulate queue operations
        const queueName = 'processing_queue';
        const currentSize = Math.floor(Math.random() * 100);
        queueSize.set({ queue_name: queueName }, currentSize);
        
        // Random errors
        if (Math.random() < ERROR_RATE) {
            throw new Error('Processing failed');
        }
        
        // Store in database
        const start = Date.now();
        await pgPool.query(
            'INSERT INTO process_log (data, processed_at, service) VALUES ($1, $2, $3)',
            [JSON.stringify(data), new Date(), SERVICE_NAME]
        );
        dbQueryDuration.observe(
            { query_type: 'insert', table: 'process_log' }, 
            (Date.now() - start) / 1000
        );
        
        logger.info('Data processed successfully', { queueSize: currentSize });
        res.json({
            status: 'processed',
            service: SERVICE_NAME,
            queueSize: currentSize,
            timestamp: new Date().toISOString()
        });
    } catch (error) {
        logger.error('Processing error', { error: error.message });
        res.status(500).json({ error: error.message, service: SERVICE_NAME });
    }
});

app.get('/api/backend/status', async (req, res) => {
    try {
        const checks = {
            database: 'unknown',
            cache: 'unknown',
            service: SERVICE_NAME
        };
        
        // Check database
        try {
            await pgPool.query('SELECT 1');
            checks.database = 'healthy';
        } catch (err) {
            checks.database = 'unhealthy';
            logger.error('Database health check failed', { error: err.message });
        }
        
        // Check cache
        try {
            await redisClient.ping();
            checks.cache = 'healthy';
        } catch (err) {
            checks.cache = 'unhealthy';
            logger.error('Cache health check failed', { error: err.message });
        }
        
        const allHealthy = Object.values(checks).every(v => v === 'healthy' || v === SERVICE_NAME);
        
        res.status(allHealthy ? 200 : 503).json({
            ...checks,
            timestamp: new Date().toISOString(),
            uptime: process.uptime()
        });
    } catch (error) {
        logger.error('Status check error', { error: error.message });
        res.status(500).json({ error: error.message, service: SERVICE_NAME });
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

// Background tasks
setInterval(async () => {
    // Update error rate
    errorRate.set({ service: SERVICE_NAME }, ERROR_RATE);
    
    // Simulate background job processing
    const jobs = ['email_send', 'report_generation', 'data_sync', 'cache_cleanup'];
    const job = jobs[Math.floor(Math.random() * jobs.length)];
    
    if (Math.random() < 0.3) { // 30% chance of job
        logger.info(`Background job started: ${job}`, {
            jobId: `job_${Math.floor(Math.random() * 100000)}`,
            queueSize: Math.floor(Math.random() * 50)
        });
        
        // Simulate job completion or failure
        setTimeout(() => {
            if (Math.random() < ERROR_RATE) {
                logger.error(`Background job failed: ${job}`, {
                    error: 'Job execution timeout',
                    duration: Math.floor(Math.random() * 30000)
                });
            } else {
                logger.info(`Background job completed: ${job}`, {
                    duration: Math.floor(Math.random() * 10000)
                });
            }
        }, Math.random() * 5000);
    }
    
    // Generate various log levels
    const scenarios = [
        { level: 'warn', message: 'High memory usage detected', meta: { memory: Math.random() * 90 + 10 } },
        { level: 'error', message: 'Failed to connect to external API', meta: { api: 'payment_gateway', retries: 3 } },
        { level: 'info', message: 'Cache cleared successfully', meta: { items: Math.floor(Math.random() * 1000) } },
        { level: 'warn', message: 'Slow query detected', meta: { duration: Math.random() * 5000, query: 'SELECT * FROM users' } },
        { level: 'error', message: 'Unhandled promise rejection', meta: { stack: 'Error: Connection timeout' } }
    ];
    
    if (Math.random() < 0.2) { // 20% chance
        const scenario = scenarios[Math.floor(Math.random() * scenarios.length)];
        logger[scenario.level](scenario.message, scenario.meta);
    }
}, 3000); // Every 3 seconds

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