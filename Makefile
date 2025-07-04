# Paladin AI Makefile
# Uses uv workspace for Python package management

.PHONY: help install install-dev sync-all test format lint clean run-server run-cli dev mcp-server mcp-worker mcp-all mcp-status

# Default target
help:
	@echo "Paladin AI - Available commands:"
	@echo ""
	@echo "Setup & Installation:"
	@echo "  install         - Install all workspace dependencies"
	@echo "  install-dev     - Install all dependencies including dev tools"
	@echo "  sync-all        - Sync all workspace dependencies"
	@echo ""
	@echo "Development:"
	@echo "  run-server      - Start FastAPI server"
	@echo "  run-cli         - Run CLI client (use CLI_ARGS for arguments)"
	@echo "  dev             - Start server in background and test connection"
	@echo ""
	@echo "Discord MCP:"
	@echo "  mcp-server      - Start Discord MCP server"
	@echo "  mcp-worker      - Start Discord message processing worker"
	@echo "  mcp-all         - Start both MCP server and worker"
	@echo "  mcp-status      - Check MCP services status"
	@echo ""
	@echo "Code Quality:"
	@echo "  test            - Run tests for all components"
	@echo "  format          - Format code with black and isort"
	@echo "  lint            - Run linting with flake8 and mypy"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean           - Clean build artifacts"
	@echo "  clean-all       - Clean everything including uv cache"
	@echo ""
	@echo "Examples:"
	@echo "  make install"
	@echo "  make run-server"
	@echo "  make CLI_ARGS='--help' run-cli"

# Install workspace dependencies
install:
	@echo "📦 Installing workspace dependencies with uv..."
	uv sync
	@echo "✅ Workspace dependencies installed"

# Install all dependencies including dev tools
install-dev:
	@echo "📦 Installing all dependencies including dev tools..."
	uv sync --all-extras
	cd server && uv sync --all-extras
	cd cli && uv sync --all-extras
	@echo "✅ All dependencies installed"

# Sync all workspace dependencies
sync-all:
	@echo "🔄 Syncing all workspace dependencies..."
	uv sync
	cd server && uv sync
	cd cli && uv sync
	@echo "✅ All workspaces synced"

# Run FastAPI server
run-server:
	@echo "🚀 Starting Paladin AI FastAPI server..."
	cd server && uv run python main.py

# Run CLI client
CLI_ARGS ?= --help
run-cli:
	@echo "🖥️  Running Paladin AI CLI client..."
	cd cli && uv run python main.py $(CLI_ARGS)

# Development workflow - start server in background and test
dev:
	@echo "🔧 Starting development environment..."
	@echo "Starting server in background..."
	cd server && uv run python main.py &
	@sleep 3
	@echo "Testing connection..."
	cd cli && uv run python main.py --help
	@echo "✅ Development environment ready!"
	@echo "Server is running at http://127.0.0.1:8000"
	@echo "Use 'make run-cli CLI_ARGS=\"--test\"' to test again"

# Format code with black and isort
format:
	@echo "🎨 Formatting code..."
	cd server && uv run black . && uv run isort .
	cd cli && uv run black . && uv run isort .
	@echo "✅ Code formatted"

# Run linting with flake8 and mypy
lint:
	@echo "🔍 Running linting..."
	cd server && uv run flake8 . && uv run mypy .
	cd cli && uv run flake8 . && uv run mypy .
	@echo "✅ Linting completed"

# Run linting for cli
lint-cli:
	@echo "🔍 Running linting for CLI..."
	cd cli && uv run flake8 . && uv run mypy .
	@echo "✅ Linting completed for CLI"

# Run linting for server
lint-server:
	@echo "🔍 Running linting for server..."
	cd server && uv run flake8 . && uv run mypy .
	@echo "✅ Linting completed for server"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	find . -type f -name "*.pyc" -delete
	find . -type d -name "__pycache__" -delete
	find . -type d -name "*.egg-info" -exec rm -rf {} +
	find . -type d -name ".pytest_cache" -exec rm -rf {} +
	find . -type d -name ".mypy_cache" -exec rm -rf {} +
	@echo "✅ Build artifacts cleaned"

# Clean everything including uv cache
clean-all: clean
	@echo "🧹 Cleaning uv cache..."
	uv cache clean
	@echo "✅ Everything cleaned"

# Discord MCP commands
mcp-server:
	@echo "🚀 Starting Discord MCP Server..."
	@cd mcp/discord-server && $(MAKE) server

mcp-worker:
	@echo "🔧 Starting Discord Message Worker..."
	@cd mcp/discord-server && $(MAKE) worker

mcp-all:
	@echo "🚀 Starting Discord MCP Server and Worker..."
	@cd mcp/discord-server && $(MAKE) all

mcp-status:
	@echo "📊 Checking Discord MCP status..."
	@cd mcp/discord-server && $(MAKE) status

# Webhook server commands
webhook-install:
	@echo "📦 Installing webhook server dependencies..."
	uv sync
	@echo "✅ Webhook dependencies installed"

webhook-run:
	@echo "🚀 Starting webhook server..."
	cd webhook && uv run python server.py

webhook-test:
	@echo "🧪 Testing webhook server..."
	@curl -X POST http://localhost:5000/webhook \
		-H "Content-Type: application/json" \
		-d '{"event": "test", "message": "Hello from webhook test!"}'
	@echo ""
	@echo "✅ Webhook test sent"
