-- Create mock tables for testing
CREATE TABLE IF NOT EXISTS mock_data (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    value DECIMAL(10, 2),
    status VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS process_log (
    id SERIAL PRIMARY KEY,
    data JSONB,
    processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    service VARCHAR(100),
    status VARCHAR(50) DEFAULT 'pending'
);

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    total_amount DECIMAL(10, 2),
    status VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert sample data
INSERT INTO mock_data (name, value, status) VALUES
    ('Product A', 99.99, 'active'),
    ('Product B', 149.99, 'active'),
    ('Product C', 79.99, 'inactive'),
    ('Product D', 199.99, 'active'),
    ('Product E', 59.99, 'pending');

INSERT INTO users (username, email) VALUES
    ('testuser1', 'test1@example.com'),
    ('testuser2', 'test2@example.com'),
    ('testuser3', 'test3@example.com');

-- Create indexes
CREATE INDEX idx_mock_data_status ON mock_data(status);
CREATE INDEX idx_process_log_service ON process_log(service);
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_created_at ON orders(created_at);

-- Create a function to simulate slow queries
CREATE OR REPLACE FUNCTION slow_query_simulation()
RETURNS TABLE(id INTEGER, name VARCHAR, sleep_time FLOAT)
AS $$
BEGIN
    PERFORM pg_sleep(RANDOM() * 2); -- Random sleep 0-2 seconds
    RETURN QUERY SELECT m.id, m.name, RANDOM()::FLOAT FROM mock_data m;
END;
$$ LANGUAGE plpgsql;