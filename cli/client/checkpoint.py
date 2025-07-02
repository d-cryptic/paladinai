"""
Checkpoint client functionality for Paladin AI CLI.
Provides commands to interact with checkpoint management endpoints.
"""

from typing import Optional, Dict, Any, List
from datetime import datetime

from rich.console import Console
from rich.markdown import Markdown


class CheckpointMixin:
    """Mixin class for checkpoint-related commands."""

    async def get_checkpoint(self, session_id: str) -> None:
        """
        Retrieve and display checkpoint for a session.
        
        Args:
            session_id: The session identifier
        """
        endpoint = f"/api/v1/checkpoints/{session_id}"
        
        try:
            console = Console()
            with console.status("[bold green]Fetching checkpoint..."):
                success, data = await self._get_json(endpoint)
            
            if success and data.get("success"):
                checkpoint_data = data.get("data", {})
                
                # Format checkpoint information
                formatted_output = f"# Checkpoint for Session: {session_id}\n\n"
                
                if checkpoint_data:
                    # Basic info
                    formatted_output += "## Basic Information\n"
                    formatted_output += f"- **Thread ID**: {checkpoint_data.get('thread_id', 'N/A')}\n"
                    formatted_output += f"- **Checkpoint NS**: {checkpoint_data.get('checkpoint_ns', 'N/A')}\n"
                    formatted_output += f"- **Checkpoint ID**: {checkpoint_data.get('checkpoint_id', 'N/A')}\n"
                    
                    # Timestamp
                    if 'timestamp' in checkpoint_data:
                        formatted_output += f"- **Created**: {checkpoint_data['timestamp']}\n"
                    
                    formatted_output += "\n"
                    
                    # State information
                    if 'state' in checkpoint_data:
                        formatted_output += "## State Information\n"
                        state = checkpoint_data['state']
                        
                        if 'current_node' in state:
                            formatted_output += f"- **Current Node**: {state['current_node']}\n"
                        
                        if 'workflow_type' in state:
                            formatted_output += f"- **Workflow Type**: {state['workflow_type']}\n"
                        
                        if 'session_id' in state:
                            formatted_output += f"- **Session ID**: {state['session_id']}\n"
                        
                        formatted_output += "\n"
                    
                    # Metadata
                    if 'metadata' in checkpoint_data and checkpoint_data['metadata']:
                        formatted_output += "## Metadata\n"
                        for key, value in checkpoint_data['metadata'].items():
                            formatted_output += f"- **{key}**: {value}\n"
                        formatted_output += "\n"
                
                else:
                    formatted_output += "*No checkpoint data available*\n"
                
                console.print(Markdown(formatted_output))
                
            else:
                print(f"❌ {data.get('message', 'Failed to retrieve checkpoint')}")
                
        except Exception as e:
            print(f"❌ Error retrieving checkpoint: {str(e)}")

    async def check_checkpoint_exists(self, session_id: str) -> None:
        """
        Check if a checkpoint exists for a session.
        
        Args:
            session_id: The session identifier
        """
        endpoint = f"/api/v1/checkpoints/{session_id}/exists"
        
        try:
            console = Console()
            with console.status("[bold green]Checking checkpoint..."):
                success, data = await self._get_json(endpoint)
            
            if success and data.get("exists"):
                print(f"✅ Checkpoint exists for session: {session_id}")
            else:
                print(f"❌ No checkpoint found for session: {session_id}")
                
        except Exception as e:
            print(f"❌ Error checking checkpoint: {str(e)}")

    async def list_checkpoints(self, session_id: Optional[str] = None, limit: int = 10) -> None:
        """
        List available checkpoints.
        
        Args:
            session_id: Optional session ID to filter by
            limit: Maximum number of checkpoints to return
        """
        endpoint = "/api/v1/checkpoints/"
        params = {"limit": limit}
        
        if session_id:
            params["session_id"] = session_id
        
        try:
            console = Console()
            with console.status("[bold green]Fetching checkpoints..."):
                # Build query string from params
                query_string = "&".join([f"{k}={v}" for k, v in params.items() if v is not None])
                full_endpoint = f"{endpoint}?{query_string}" if query_string else endpoint
                success, data = await self._get_json(full_endpoint)
            
            if success and data.get("success"):
                checkpoint_info = data.get("data", {})
                checkpoints = checkpoint_info.get("checkpoints", [])
                count = checkpoint_info.get("count", 0)
                
                # Format checkpoint list
                formatted_output = "# Available Checkpoints\n\n"
                formatted_output += f"Found **{count}** checkpoint(s)\n\n"
                
                if checkpoints:
                    for idx, cp in enumerate(checkpoints, 1):
                        # thread_id is the session_id in this system
                        session_id = cp.get('thread_id', cp.get('session_id', 'Unknown'))
                        formatted_output += f"## {idx}. Session: {session_id}\n"
                        formatted_output += f"- **Checkpoint ID**: {cp.get('checkpoint_id', 'N/A')}\n"
                        
                        if 'timestamp' in cp:
                            formatted_output += f"- **Created**: {cp['timestamp']}\n"
                        
                        if 'current_node' in cp:
                            formatted_output += f"- **Current Node**: {cp['current_node']}\n"
                        
                        if 'workflow_type' in cp:
                            formatted_output += f"- **Workflow Type**: {cp['workflow_type']}\n"
                        
                        formatted_output += "\n"
                else:
                    formatted_output += "*No checkpoints found*\n"
                
                console.print(Markdown(formatted_output))
                
            else:
                print(f"❌ {data.get('message', 'Failed to list checkpoints')}")
                
        except Exception as e:
            print(f"❌ Error listing checkpoints: {str(e)}")

    async def delete_checkpoint(self, session_id: str) -> None:
        """
        Delete all checkpoints for a session.
        
        Args:
            session_id: The session identifier
        """
        endpoint = f"/api/v1/checkpoints/{session_id}"
        
        # Confirm deletion
        print(f"⚠️  Are you sure you want to delete all checkpoints for session '{session_id}'?")
        confirmation = input("Type 'yes' to confirm: ").strip().lower()
        
        if confirmation != 'yes':
            print("❌ Deletion cancelled")
            return
        
        try:
            console = Console()
            with console.status("[bold green]Deleting checkpoint..."):
                # BaseHTTPClient doesn't have _delete_json, so we'll use the session directly
                self._ensure_session()
                url = f"{self.server_url}{endpoint}"
                async with self.session.delete(url) as response:
                    response.raise_for_status()
                    data = await response.json()
            
            if data.get("success"):
                print(f"✅ {data.get('message', 'Checkpoints deleted successfully')}")
            else:
                print(f"❌ {data.get('message', 'Failed to delete checkpoints')}")
                
        except Exception as e:
            print(f"❌ Error deleting checkpoint: {str(e)}")

    async def show_checkpoint_help(self) -> None:
        """Display help information for checkpoint commands."""
        help_text = """
# Checkpoint Management Help

Checkpoints allow you to save and restore the state of workflow executions.

## Available Commands:

### Get Checkpoint
Retrieve the checkpoint for a specific session:
```bash
paladin checkpoint get <session_id>
```

### Check Checkpoint Existence
Check if a checkpoint exists for a session:
```bash
paladin checkpoint exists <session_id>
```

### List Checkpoints
List all available checkpoints:
```bash
paladin checkpoint list [--limit 20]
```

List checkpoints for a specific session:
```bash
paladin checkpoint list --session <session_id>
```

### Delete Checkpoint
Delete all checkpoints for a session:
```bash
paladin checkpoint delete <session_id>
```

## Session ID Format
Session IDs should follow the format: `user_<identifier>_<timestamp>`
Example: `user_john_1234567890`

## MongoDB Storage
Checkpoints are stored in MongoDB with the following structure:
- Database: Configured in server settings
- Collection: `checkpoints`
- TTL: Checkpoints expire after configured number of days (default: 30)

## Use Cases:
1. **Resume Failed Workflows**: Restore state after errors or interruptions
2. **Audit Trail**: Review workflow execution history
3. **Debugging**: Inspect workflow state at specific points
4. **State Management**: Manage long-running workflow sessions
"""
        console = Console()
        console.print(Markdown(help_text))