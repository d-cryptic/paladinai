#!/bin/bash

# Start Alertmanager Webhook Server
# This script sets up and starts the webhook server for receiving Alertmanager notifications

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🚀 Starting Alertmanager Webhook Server..."

# Check if Python 3 is available
if ! command -v python3 &> /dev/null; then
    echo "❌ Python 3 is required but not installed."
    exit 1
fi

# Check if pip is available
if ! command -v pip3 &> /dev/null; then
    echo "❌ pip3 is required but not installed."
    exit 1
fi

# Create virtual environment if it doesn't exist
if [ ! -d "venv" ]; then
    echo "📦 Creating virtual environment..."
    python3 -m venv venv
fi

# Activate virtual environment
echo "🔧 Activating virtual environment..."
source venv/bin/activate

# Install dependencies
echo "📥 Installing dependencies..."
pip install -r requirements.txt

# Copy .env.example to .env if .env doesn't exist
if [ ! -f ".env" ]; then
    echo "⚙️  Creating .env file from .env.example..."
    cp .env.example .env
    echo "📝 Please edit .env file to configure your webhook server settings."
fi

# Start the webhook server
echo "🌐 Starting webhook server..."
echo "📍 Server will be available at: http://127.0.0.1:5001"
echo "🔍 Health check: http://127.0.0.1:5001/health"
echo "📋 To stop the server, press Ctrl+C"
echo ""

python3 webhook_server.py
