# Discord MCP Server for PaladinAI

This MCP (Model Context Protocol) server integrates Discord with PaladinAI, allowing the AI to monitor Discord channels, respond to mentions, and interact with users.

## Features

- **Channel Monitoring**: Monitor specific Discord channels and record all messages
- **Message History**: Store and retrieve message history with user and timestamp information
- **Bot Mentions**: Respond when users mention the bot
- **Message Sending**: Send messages to Discord channels programmatically
- **Full Message Context**: Captures message content, author info, timestamps, attachments, and embeds

## Setup

1. **Install Dependencies**:
   ```bash
   cd mcp/discord-server
   uv pip install -e .
   ```

2. **Configure Discord Bot Token**:
   - Add your Discord bot token to the `.env` file:
   ```
   DISCORD_BOT_TOKEN="your-bot-token-here"
   ```
   
   To get a bot token:
   - Go to https://discord.com/developers/applications
   - Select your application (or create one)
   - Go to the "Bot" section
   - Click "Reset Token" to get a new token

3. **Add Bot to Server**:
   - In the Discord Developer Portal, go to OAuth2 > URL Generator
   - Select scopes: `bot`, `applications.commands`
   - Select bot permissions: `Send Messages`, `Read Messages/View Channels`, `Read Message History`
   - Use the generated URL to add the bot to your server

## Running the Server

### Using uv (Recommended)
```bash
cd mcp/discord-server
uv run python -m discord_mcp.server
```

### Manual activation
```bash
cd mcp/discord-server
source .venv/bin/activate
python -m discord_mcp.server
```

### With Claude Desktop
Add to your Claude Desktop configuration (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "discord": {
      "command": "python",
      "args": ["-m", "discord_mcp.server"],
      "cwd": "/Users/admin/Developers/Tinkering/paladin-ai/mcp/discord-server"
    }
  }
}
```

## Available MCP Tools

### 1. `monitor_channel`
Start monitoring a Discord channel for messages.
```json
{
  "channel_id": "1234567890",
  "channel_name": "general"  // optional
}
```

### 2. `stop_monitoring_channel`
Stop monitoring a specific channel.
```json
{
  "channel_id": "1234567890"
}
```

### 3. `get_channel_messages`
Retrieve recent messages from a channel.
```json
{
  "channel_id": "1234567890",
  "limit": 50  // max 100
}
```

### 4. `send_message`
Send a message to a Discord channel.
```json
{
  "channel_id": "1234567890",
  "content": "Hello from PaladinAI!"
}
```

### 5. `get_message_history`
Get stored message history from monitored channels.
```json
{
  "channel_id": "1234567890",  // optional filter
  "user_id": "0987654321",     // optional filter
  "limit": 100
}
```

## Integration with PaladinAI

The Discord MCP server stores messages with the following information:
- Message ID and content
- Channel ID and name
- Author ID, name, and discriminator
- Timestamp
- Attachments (filename, URL, size)
- Embeds
- Mentions (users and roles)

When the bot is mentioned, it will:
1. Extract the message content (removing the bot mention)
2. Store the message in history
3. Respond to the user (currently with a simple acknowledgment)

Future integration will send the message to the PaladinAI server for intelligent processing.

## Message Storage

Messages are stored in memory with a rolling buffer of 10,000 messages to prevent memory issues. Each message includes:
- Full message metadata
- Author information
- Channel context
- Timestamps in ISO format
- Attachment and embed data

## Next Steps

To fully integrate with PaladinAI:
1. Add HTTP client to send messages to PaladinAI server
2. Implement response handling from PaladinAI
3. Add persistent storage for message history
4. Implement channel-specific configuration
5. Add more Discord event handlers (reactions, edits, etc.)