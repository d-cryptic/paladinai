# PostgreSQL Database Documentation

## Service Overview
PostgreSQL serves as the primary relational database for the mock infrastructure, storing application data, user information, and transactional records. It's configured for high availability monitoring and performance metrics collection.

## Service Details

### Container Information
- **Image**: `postgres:15-alpine`
- **Container Name**: `mock-postgres`
- **Version**: PostgreSQL 15 (Alpine Linux)
- **Network**: `mock-network`

### Port Configuration
- **Internal Port**: 5432 (PostgreSQL default)
- **External Port**: 5433
- **Connection String**: `postgresql://mockuser:mockpass@localhost:5433/mockdb`

### Environment Variables
- `POSTGRES_USER=mockuser` - Database superuser
- `POSTGRES_PASSWORD=mockpass` - Superuser password
- `POSTGRES_DB=mockdb` - Default database name

### Volume Mounts
- `postgres-data:/var/lib/postgresql/data` - Persistent data storage
- `./database/init.sql:/docker-entrypoint-initdb.d/init.sql` - Initialization script

## Database Schema

### Tables Structure

#### 1. users
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) DEFAULT 'active',
    metadata JSONB
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_users_created_at ON users(created_at);
```

#### 2. data_entries
```sql
CREATE TABLE data_entries (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    key VARCHAR(255) NOT NULL,
    value TEXT,
    type VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    metadata JSONB
);

CREATE INDEX idx_data_entries_user_id ON data_entries(user_id);
CREATE INDEX idx_data_entries_key ON data_entries(key);
CREATE INDEX idx_data_entries_created_at ON data_entries(created_at);
```

#### 3. events
```sql
CREATE TABLE events (
    id SERIAL PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    user_id INTEGER REFERENCES users(id),
    data JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP,
    status VARCHAR(50) DEFAULT 'pending'
);

CREATE INDEX idx_events_type ON events(event_type);
CREATE INDEX idx_events_status ON events(status);
CREATE INDEX idx_events_created_at ON events(created_at);
```

#### 4. metrics
```sql
CREATE TABLE metrics (
    id SERIAL PRIMARY KEY,
    metric_name VARCHAR(255) NOT NULL,
    metric_value NUMERIC NOT NULL,
    tags JSONB,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_metrics_name_timestamp ON metrics(metric_name, timestamp);
```

### Database Functions

#### 1. Update Timestamp Trigger
```sql
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    
CREATE TRIGGER update_data_entries_updated_at BEFORE UPDATE ON data_entries
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

#### 2. Data Aggregation Function
```sql
CREATE OR REPLACE FUNCTION get_user_statistics(user_id_param INTEGER)
RETURNS TABLE (
    total_entries BIGINT,
    total_events BIGINT,
    last_activity TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        COUNT(DISTINCT de.id),
        COUNT(DISTINCT e.id),
        MAX(GREATEST(de.updated_at, e.created_at))
    FROM users u
    LEFT JOIN data_entries de ON u.id = de.user_id
    LEFT JOIN events e ON u.id = e.user_id
    WHERE u.id = user_id_param;
END;
$$ LANGUAGE plpgsql;
```

## Connection Configuration

### Connection Pool Settings
- **Max Connections**: 100
- **Shared Buffers**: 256MB
- **Work Memory**: 4MB
- **Maintenance Work Memory**: 64MB

### Client Connection Parameters
```javascript
{
  host: 'postgres',
  port: 5432,
  database: 'mockdb',
  user: 'mockuser',
  password: 'mockpass',
  max: 10,              // Pool max size
  idleTimeoutMillis: 30000,
  connectionTimeoutMillis: 2000,
}
```

## Performance Tuning

### PostgreSQL Configuration
```conf
# Memory
shared_buffers = 256MB
effective_cache_size = 1GB
work_mem = 4MB
maintenance_work_mem = 64MB

# Connections
max_connections = 100
superuser_reserved_connections = 3

# Write Performance
checkpoint_segments = 32
checkpoint_completion_target = 0.9
wal_buffers = 16MB

# Query Planning
random_page_cost = 1.1
cpu_tuple_cost = 0.01
cpu_index_tuple_cost = 0.005
cpu_operator_cost = 0.0025
effective_io_concurrency = 200

# Logging
log_statement = 'mod'
log_duration = on
log_min_duration_statement = 100ms
log_checkpoints = on
log_connections = on
log_disconnections = on
log_line_prefix = '%t [%p]: [%l-1] user=%u,db=%d,app=%a,client=%h '
```

### Index Strategy
1. **Primary Keys**: B-tree indexes automatically created
2. **Foreign Keys**: Indexed for join performance
3. **Timestamp Columns**: For time-based queries
4. **JSONB Columns**: GIN indexes for JSON queries
5. **Composite Indexes**: For common query patterns

## Monitoring

### PostgreSQL Exporter Metrics
The postgres-exporter container exposes metrics on port 9187:

#### Key Metrics
1. **Connection Metrics**:
   - `pg_stat_database_numbackends` - Active connections
   - `pg_stat_database_connections` - Total connections
   - `pg_settings_max_connections` - Max allowed connections

2. **Transaction Metrics**:
   - `pg_stat_database_xact_commit` - Committed transactions
   - `pg_stat_database_xact_rollback` - Rolled back transactions
   - `pg_stat_database_blks_read` - Disk blocks read
   - `pg_stat_database_blks_hit` - Buffer cache hits

3. **Query Performance**:
   - `pg_stat_user_tables_seq_scan` - Sequential scans
   - `pg_stat_user_tables_idx_scan` - Index scans
   - `pg_stat_user_tables_n_tup_ins` - Rows inserted
   - `pg_stat_user_tables_n_tup_upd` - Rows updated
   - `pg_stat_user_tables_n_tup_del` - Rows deleted

4. **Database Size**:
   - `pg_database_size_bytes` - Database size
   - `pg_stat_user_tables_n_live_tup` - Live tuples
   - `pg_stat_user_tables_n_dead_tup` - Dead tuples

### Query Analysis
```sql
-- Slow queries
SELECT query, calls, total_time, mean_time, stddev_time
FROM pg_stat_statements
WHERE mean_time > 100
ORDER BY mean_time DESC
LIMIT 10;

-- Table sizes
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- Index usage
SELECT 
    schemaname,
    tablename,
    indexname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes
ORDER BY idx_scan DESC;
```

## Backup and Recovery

### Backup Strategy
1. **Full Backups**: Daily at 2 AM
2. **WAL Archiving**: Continuous
3. **Retention**: 7 days
4. **Storage**: Volume mount

### Backup Commands
```bash
# Manual backup
docker exec mock-postgres pg_dump -U mockuser mockdb > backup.sql

# Backup with compression
docker exec mock-postgres pg_dump -U mockuser -F c mockdb > backup.dump

# Backup specific tables
docker exec mock-postgres pg_dump -U mockuser -t users -t data_entries mockdb > tables.sql
```

### Restore Commands
```bash
# Restore from SQL
docker exec -i mock-postgres psql -U mockuser mockdb < backup.sql

# Restore from compressed
docker exec -i mock-postgres pg_restore -U mockuser -d mockdb < backup.dump

# Restore specific table
docker exec -i mock-postgres psql -U mockuser mockdb < tables.sql
```

## Security Configuration

### Access Control
1. **Authentication**: md5 password authentication
2. **SSL**: Disabled for local development
3. **Host-based Access**: Limited to Docker network

### User Permissions
```sql
-- Application user (limited privileges)
CREATE USER app_user WITH PASSWORD 'app_pass';
GRANT CONNECT ON DATABASE mockdb TO app_user;
GRANT USAGE ON SCHEMA public TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_user;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app_user;

-- Read-only user for reporting
CREATE USER readonly_user WITH PASSWORD 'readonly_pass';
GRANT CONNECT ON DATABASE mockdb TO readonly_user;
GRANT USAGE ON SCHEMA public TO readonly_user;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO readonly_user;
```

## Common Queries

### Health Check Query
```sql
SELECT 1 as health_check;
```

### Connection Status
```sql
SELECT 
    pid,
    usename,
    application_name,
    client_addr,
    backend_start,
    state,
    query
FROM pg_stat_activity
WHERE datname = 'mockdb';
```

### Table Statistics
```sql
SELECT 
    schemaname,
    tablename,
    n_live_tup as live_rows,
    n_dead_tup as dead_rows,
    last_vacuum,
    last_autovacuum
FROM pg_stat_user_tables;
```

## Troubleshooting

### Common Issues

1. **Connection Refused**
   ```bash
   # Check if container is running
   docker ps | grep mock-postgres
   
   # Check PostgreSQL logs
   docker logs mock-postgres
   
   # Test connection
   docker exec mock-postgres pg_isready -U mockuser
   ```

2. **High Connection Count**
   ```sql
   -- Check active connections
   SELECT count(*) FROM pg_stat_activity;
   
   -- Kill idle connections
   SELECT pg_terminate_backend(pid) 
   FROM pg_stat_activity 
   WHERE datname = 'mockdb' 
   AND state = 'idle' 
   AND state_change < current_timestamp - interval '10 minutes';
   ```

3. **Slow Queries**
   ```sql
   -- Enable query logging
   ALTER SYSTEM SET log_min_duration_statement = 100;
   SELECT pg_reload_conf();
   
   -- Check slow queries
   SELECT query, calls, mean_time 
   FROM pg_stat_statements 
   WHERE mean_time > 100 
   ORDER BY mean_time DESC;
   ```

4. **Disk Space Issues**
   ```bash
   # Check disk usage
   docker exec mock-postgres df -h /var/lib/postgresql/data
   
   # Find large tables
   docker exec mock-postgres psql -U mockuser -d mockdb -c "
   SELECT tablename, pg_size_pretty(pg_total_relation_size(tablename::regclass))
   FROM pg_tables WHERE schemaname = 'public' ORDER BY pg_total_relation_size(tablename::regclass) DESC;"
   ```

### Performance Diagnostics
```sql
-- Cache hit ratio (should be > 90%)
SELECT 
    sum(heap_blks_hit) / (sum(heap_blks_hit) + sum(heap_blks_read)) as cache_hit_ratio
FROM pg_statio_user_tables;

-- Index usage ratio
SELECT 
    schemaname,
    tablename,
    100 * idx_scan / (seq_scan + idx_scan) as index_usage_ratio
FROM pg_stat_user_tables
WHERE seq_scan + idx_scan > 0;

-- Lock monitoring
SELECT 
    locktype,
    database,
    relation::regclass,
    mode,
    granted
FROM pg_locks
WHERE NOT granted;
```

## Integration with Other Services

### Backend Services
- Connection pool of 10 connections per instance
- Automatic retry with exponential backoff
- Health checks every 30 seconds

### Monitoring Stack
- Metrics exported via postgres-exporter
- Prometheus scrapes every 15 seconds
- Alerts configured for connection issues

### Backup Integration
- Scheduled backups via cron
- Backup notifications to monitoring