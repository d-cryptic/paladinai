# Discord MCP Server Makefile

.PHONY: help install server worker all stop status queue-status test clean

# Fix for macOS fork() issue
export OBJC_DISABLE_INITIALIZE_FORK_SAFETY=YES

# Default target
help:
	@echo "Discord MCP Server - Available commands:"
	@echo ""
	@echo "Setup & Installation:"
	@echo "  install         - Install dependencies with uv"
	@echo ""
	@echo "Running Services:"
	@echo "  server          - Run Discord MCP server"
	@echo "  worker          - Run message processing worker"
	@echo "  all             - Run both server and worker (in separate terminals)"
	@echo ""
	@echo "Monitoring & Testing:"
	@echo "  status          - Check service status"
	@echo "  queue-status    - Check Valkey queue status"
	@echo "  test            - Test message queuing"
	@echo ""
	@echo "Maintenance:"
	@echo "  stop            - Stop all services"
	@echo "  clean           - Clean logs and temporary files"

# Install dependencies
install:
	@echo "📦 Installing Discord MCP dependencies..."
	uv sync
	@echo "✅ Dependencies installed"

# Run Discord MCP server
server:
	@echo "🚀 Starting Discord MCP Server..."
	@echo "Loading environment from ../../.env"
	@if [ ! -f "../../.env" ]; then \
		echo "❌ Error: ../../.env file not found!"; \
		exit 1; \
	fi
	@if ! grep -q "DISCORD_BOT_TOKEN" ../../.env; then \
		echo "❌ Error: DISCORD_BOT_TOKEN not found in .env!"; \
		exit 1; \
	fi
	uv run python -m discord_mcp.enhanced_server

# Run message processing worker
worker:
	@echo "🔧 Starting Message Processing Worker..."
	@echo "Loading environment from ../../.env"
	@if [ ! -f "../../.env" ]; then \
		echo "❌ Error: ../../.env file not found!"; \
		exit 1; \
	fi
	@echo "✅ Using RQ worker with fork safety for macOS"
	@echo "Loading HOST_IP from .env..."
	OBJC_DISABLE_INITIALIZE_FORK_SAFETY=YES NO_PROXY='*' uv run rq worker discord_messages --url redis://$(shell grep ^HOST_IP ../../.env | cut -d= -f2):6379/0

# Run both server and worker (requires tmux or separate terminals)
all:
	@echo "🚀 Starting Discord MCP Server and Worker..."
	@echo ""
	@echo "Option 1: Run these commands in separate terminals:"
	@echo "  Terminal 1: make server"
	@echo "  Terminal 2: make worker"
	@echo ""
	@echo "Option 2: Use tmux (if installed):"
	@if command -v tmux >/dev/null 2>&1; then \
		tmux new-session -d -s discord-mcp "make server"; \
		tmux split-window -h "make worker"; \
		tmux attach -t discord-mcp; \
	else \
		echo "  tmux not installed. Please use separate terminals."; \
	fi

# Check service status
status:
	@echo "📊 Checking service status..."
	@echo ""
	@echo "Discord MCP Server:"
	@ps aux | grep -E "discord_mcp.(enhanced_)?server" | grep -v grep || echo "  ❌ Not running"
	@echo ""
	@echo "Message Worker:"
	@ps aux | grep -E "workers_server|start_worker" | grep -v grep || echo "  ❌ Not running"
	@echo ""
	@echo "Valkey Connection:"
	@uv run python -c "import redis, os; from pathlib import Path; from dotenv import load_dotenv; load_dotenv(Path('../../.env')); r = redis.Redis(host=os.getenv('VALKEY_HOST'), port=6379); r.ping(); print('  ✅ Connected to', os.getenv('VALKEY_HOST'))" 2>/dev/null || echo "  ❌ Connection failed"

# Check queue status
queue-status:
	@echo "📊 Checking Valkey queue status..."
	uv run python check_queue_status.py

# Test message queuing
test:
	@echo "🧪 Testing message queue..."
	uv run python test_queue_message.py

# Stop all services
stop:
	@echo "🛑 Stopping Discord MCP services..."
	@pkill -f "discord_mcp.(enhanced_)?server" || true
	@pkill -f "workers_server|start_worker" || true
	@echo "✅ Services stopped"

# Clean logs and temporary files
clean:
	@echo "🧹 Cleaning up..."
	@rm -f worker.log nohup.out
	@find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	@find . -type f -name "*.pyc" -delete 2>/dev/null || true
	@echo "✅ Cleaned up"

# Development shortcuts
dev-server:
	@echo "🔧 Starting server in development mode..."
	PYTHONUNBUFFERED=1 uv run python -m discord_mcp.enhanced_server

dev-worker:
	@echo "🔧 Starting worker in development mode..."
	@echo "Loading HOST_IP from .env..."
	OBJC_DISABLE_INITIALIZE_FORK_SAFETY=YES NO_PROXY='*' PYTHONUNBUFFERED=1 uv run rq worker discord_messages --url redis://$(shell grep ^HOST_IP ../../.env | cut -d= -f2):6379/0