.PHONY: up down logs clean build restart status

# Start all services
up:
	docker-compose up -d
	@echo "Waiting for services to start..."
	@sleep 10
	@make status

# Stop all services
down:
	docker-compose down

# Stop and remove volumes
clean:
	docker-compose down -v
	rm -rf logs/*

# Build images
build:
	docker-compose build

# Restart all services
restart:
	@make down
	@make up

# Show status
status:
	@echo "=== Service Status ==="
	@docker-compose ps
	@echo "\n=== Service URLs ==="
	@echo "Frontend:        http://localhost:3001"
	@echo "Backend (LB):    http://localhost:8080/api/backend/"
	@echo "Nginx:           http://localhost:8080"
	@echo "Prometheus:      http://localhost:9091"
	@echo "Alertmanager:    http://localhost:9093"
	@echo "Loki:            http://localhost:3100"
	@echo "PostgreSQL:      localhost:5433"
	@echo "Valkey/Redis:    localhost:6380"
	@echo "\n=== Metrics Endpoints ==="
	@echo "Frontend:        http://localhost:3001/metrics"
	@echo "Nginx:           http://localhost:8080/nginx_status"
	@echo "Node Exporter:   http://localhost:9100/metrics"
	@echo "Postgres Exp:    http://localhost:9187/metrics"
	@echo "Redis Exporter:  http://localhost:9121/metrics"

# View logs
logs:
	docker-compose logs -f

# View logs for specific service
logs-%:
	docker-compose logs -f $*

# Generate load for testing
load-test:
	@echo "Generating test load..."
	@for i in {1..100}; do \
		curl -s http://localhost:8080/api/backend/data > /dev/null & \
		curl -s http://localhost:3001/api/data > /dev/null & \
		curl -s http://localhost:3001/api/users > /dev/null & \
	done
	@echo "Load test started (100 concurrent requests)"

# Simulate errors
simulate-errors:
	@echo "Simulating errors..."
	@curl -s http://localhost:3001/error/404
	@curl -s http://localhost:3001/error/500
	@curl -s http://localhost:8080/api/backend/error/timeout &
	@echo "Error simulation triggered"

# Check health
health-check:
	@echo "=== Health Check ==="
	@echo -n "Frontend: "
	@curl -s http://localhost:3001/health | jq -r .status || echo "Failed"
	@echo -n "Backend-1: "
	@curl -s http://localhost:8080/api/backend/status | jq -r .status || echo "Failed"
	@echo -n "Nginx: "
	@curl -s http://localhost:8080/health | jq -r .status || echo "Failed"
	@echo -n "Prometheus: "
	@curl -s http://localhost:9091/-/healthy && echo "healthy" || echo "Failed"
	@echo -n "Loki: "
	@curl -s http://localhost:3100/ready && echo "healthy" || echo "Failed"