# Paladin AI - Server & CLI Setup

This document describes how to set up and run the FastAPI server and CLI client for Paladin AI.

## Prerequisites

- Python 3.13+
- `uv` package manager (will be auto-installed if not present)

## Quick Start

1. **Initial Setup**
   ```bash
   make setup
   make install-all
   ```

2. **Start Development Environment**
   ```bash
   make dev
   ```
   This will start the server and test the CLI connection.

## Individual Components

### FastAPI Server

The server is located in the `server/` folder with its own virtual environment.

**Start the server:**
```bash
make run-server
```

**Or manually:**
```bash
cd server
uv venv --python 3.13
uv pip install -r requirements.txt
uv run python main.py
```

The server will be available at: `http://127.0.0.1:8000`

**Available endpoints:**
- `GET /` - Hello world message
- `GET /health` - Health check
- `GET /api/v1/status` - API status

### CLI Client

The CLI is located in the `cli/` folder with its own virtual environment.

**Test CLI connection:**
```bash
make test-connection
```

**Run CLI with specific options:**
```bash
make run-cli CLI_ARGS="--test"
make run-cli CLI_ARGS="--hello"
make run-cli CLI_ARGS="--status"
make run-cli CLI_ARGS="--all"
```

**Or manually:**
```bash
cd cli
uv venv --python 3.13
uv pip install -r requirements.txt
uv run python main.py --all
```

## Configuration

### Server Configuration (`server/.env`)
```env
SERVER_HOST=127.0.0.1
SERVER_PORT=8000
LOG_LEVEL=info
RELOAD=true
DEBUG=true
```

### CLI Configuration (`cli/.env`)
```env
SERVER_URL=http://127.0.0.1:8000
DEBUG=true
TIMEOUT=30
```

## Development Workflow

1. **Start server in one terminal:**
   ```bash
   make run-server
   ```

2. **Test with CLI in another terminal:**
   ```bash
   make test-connection
   ```

3. **Or use the combined development command:**
   ```bash
   make dev
   ```

## Makefile Commands

- `make help` - Show all available commands
- `make setup` - Initial setup
- `make install-all` - Install all dependencies
- `make run-server` - Start FastAPI server
- `make run-cli` - Run CLI client
- `make test-connection` - Test CLI to server connection
- `make dev` - Start development environment
- `make clean` - Clean virtual environments
- `make clean-all` - Clean everything including uv cache

## Architecture

```
paladin-ai/
├── server/           # FastAPI server
│   ├── main.py      # Server entry point
│   ├── requirements.txt
│   ├── .env         # Server config
│   └── .venv/       # Server virtual environment
├── cli/             # CLI client
│   ├── main.py      # CLI entry point
│   ├── banner.py    # CLI banner
│   ├── requirements.txt
│   ├── .env         # CLI config
│   └── .venv/       # CLI virtual environment
└── Makefile         # Build automation
```

Each component maintains its own virtual environment and dependencies, ensuring clean separation between server and client code.
