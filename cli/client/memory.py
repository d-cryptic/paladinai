"""
Memory client functionality for Paladin AI CLI.
Handles memory-related operations and integration.
"""

import json
from typing import Dict, Any, List, Optional, TYPE_CHECKING
from rich.console import Console
from rich.panel import Panel
from rich.table import Table
from rich.markdown import Markdown

from utils.formatter.markdown_formatter import MarkdownFormatter

if TYPE_CHECKING:
    from .base import BaseHTTPClient


class MemoryMixin:
    """Mixin class providing memory functionality for the CLI client."""

    def _ensure_console(self):
        """Ensure console is initialized."""
        if not hasattr(self, 'console'):
            self.console = Console()
        if not hasattr(self, 'markdown_formatter'):
            self.markdown_formatter = MarkdownFormatter()

    async def store_instruction(self, instruction: str, user_id: Optional[str] = None, context: Optional[Dict[str, Any]] = None) -> None:
        """
        Store an explicit instruction in memory.
        
        Args:
            instruction: The instruction to store
            user_id: Optional user identifier
            context: Optional context information
        """
        self._ensure_console()
        try:
            # Prepare request payload
            payload = {
                "instruction": instruction,
                "user_id": user_id or "cli_user",
                "context": context or {}
            }
            
            self.console.print("\n💾 Storing instruction in memory...", style="blue")
            
            # Send request to server
            success, response = await self._post_json("/api/memory/instruction", payload)  # type: ignore[attr-defined]
            
            if success and response.get("success"):
                self.console.print("✅ Instruction stored successfully!", style="green")
                
                # Display result details
                result_table = Table(title="Memory Storage Result")
                result_table.add_column("Property", style="cyan")
                result_table.add_column("Value", style="white")
                
                result_table.add_row("Memory ID", str(response.get("memory_id", "N/A")))
                result_table.add_row("Memory Type", response.get("memory_type", "N/A"))
                result_table.add_row("Relationships", str(response.get("relationships_count", 0)))
                
                self.console.print(result_table)
                
            else:
                self.console.print(f"❌ Failed to store instruction: {response.get('error')}", style="red")
                
        except Exception as e:
            self.console.print(f"❌ Error storing instruction: {str(e)}", style="red")

    async def search_memories(
        self, 
        query: str, 
        memory_types: Optional[List[str]] = None,
        user_id: Optional[str] = None,
        limit: int = 10,
        confidence_threshold: float = 0.5
    ) -> None:
        """
        Search memories using semantic similarity.
        
        Args:
            query: Search query
            memory_types: Optional list of memory types to filter by
            user_id: Optional user identifier
            limit: Maximum number of results
            confidence_threshold: Minimum confidence score
        """
        self._ensure_console()
        try:
            # Prepare request payload
            payload = {
                "query": query,
                "memory_types": memory_types,
                "user_id": user_id,
                "limit": limit,
                "confidence_threshold": confidence_threshold
            }
            
            self.console.print(f"\n🔍 Searching memories for: '{query}'...", style="blue")
            
            # Send request to server
            success, response = await self._post_json("/api/memory/search", payload)  # type: ignore[attr-defined]
            
            if success and response.get("success"):
                memories = response.get("memories", [])
                total = response.get("total_results", 0)
                
                if total == 0:
                    self.console.print("📭 No memories found matching your query", style="yellow")
                    return
                
                self.console.print(f"✅ Found {total} memories", style="green")
                
                # Display memories in a table
                memories_table = Table(title=f"Memory Search Results - '{query}'")
                memories_table.add_column("Score", style="cyan", width=8)
                memories_table.add_column("Type", style="magenta", width=12)
                memories_table.add_column("Content", style="white")
                memories_table.add_column("Related", style="yellow", width=15)
                
                for memory in memories:
                    score = f"{memory.get('score', 0):.2f}"
                    mem_type = memory.get('memory_type', 'unknown')
                    content = memory.get('memory', '')[:80] + "..." if len(memory.get('memory', '')) > 80 else memory.get('memory', '')
                    
                    # Get related entities
                    related_entities = memory.get('related_entities', [])
                    related_str = ', '.join([rel.get('entity', '') for rel in related_entities[:2]])
                    if len(related_entities) > 2:
                        related_str += f" +{len(related_entities) - 2}"
                    
                    memories_table.add_row(score, mem_type, content, related_str)
                
                self.console.print(memories_table)
                
            else:
                self.console.print(f"❌ Failed to search memories: {response.get('error')}", style="red")
                
        except Exception as e:
            self.console.print(f"❌ Error searching memories: {str(e)}", style="red")

    async def get_contextual_memories(
        self, 
        context: str, 
        workflow_type: str = "QUERY", 
        limit: int = 5
    ) -> None:
        """
        Get contextually relevant memories for current situation.
        
        Args:
            context: Current context or situation
            workflow_type: Type of workflow (QUERY, ACTION, INCIDENT)
            limit: Maximum number of memories
        """
        self._ensure_console()
        try:
            self.console.print(f"\n🧠 Getting contextual memories for {workflow_type} workflow...", style="blue")
            
            # Build URL with params
            url = f"/api/memory/contextual?context={context}&workflow_type={workflow_type}&limit={limit}"
            
            # Send request to server
            success, response = await self._get_json(url)  # type: ignore[attr-defined]
            
            if success and response.get("success"):
                memories = response.get("memories", [])
                
                if not memories:
                    self.console.print("📭 No contextual memories found", style="yellow")
                    return
                
                self.console.print(f"✅ Found {len(memories)} contextual memories", style="green")
                
                # Display contextual memories
                for i, memory in enumerate(memories, 1):
                    memory_content = memory.get('memory', '')
                    memory_type = memory.get('metadata', {}).get('memory_type', 'unknown')
                    confidence = memory.get('score', 0)
                    
                    panel_content = f"**Type:** {memory_type}\n**Confidence:** {confidence:.2f}\n\n{memory_content}"
                    
                    panel = Panel(
                        panel_content,
                        title=f"Contextual Memory {i}",
                        border_style="blue"
                    )
                    self.console.print(panel)
                
            else:
                self.console.print(f"❌ Failed to get contextual memories: {response.get('error')}", style="red")
                
        except Exception as e:
            self.console.print(f"❌ Error getting contextual memories: {str(e)}", style="red")

    async def show_memory_types(self) -> None:
        """Display available memory types and their descriptions."""
        self._ensure_console()
        try:
            self.console.print("\n📊 Getting memory types...", style="blue")
            
            success, response = await self._get_json("/api/memory/types")  # type: ignore[attr-defined]
            
            if success and "memory_types" in response:
                types_table = Table(title="Available Memory Types")
                types_table.add_column("Type", style="cyan")
                types_table.add_column("Description", style="white")
                
                descriptions = response.get("descriptions", {})
                for mem_type in response["memory_types"]:
                    description = descriptions.get(mem_type, "No description available")
                    types_table.add_row(mem_type, description)
                
                self.console.print(types_table)
            else:
                self.console.print("❌ Failed to get memory types", style="red")
                
        except Exception as e:
            self.console.print(f"❌ Error getting memory types: {str(e)}", style="red")

    async def check_memory_health(self) -> None:
        """Check the health of memory backends."""
        self._ensure_console()
        try:
            self.console.print("\n🏥 Checking memory service health...", style="blue")
            
            success, response = await self._get_json("/api/memory/health")  # type: ignore[attr-defined]
            
            if success:
                status = response.get("status", "unknown")
                
                if status == "healthy":
                    self.console.print("✅ Memory service is healthy", style="green")
                    
                    # Display backend status
                    backends = response.get("backends", {})
                    health_table = Table(title="Memory Backend Status")
                    health_table.add_column("Backend", style="cyan")
                    health_table.add_column("Status", style="white")
                    
                    for backend, status in backends.items():
                        style = "green" if status == "connected" else "red"
                        health_table.add_row(backend, status, style=style)
                    
                    self.console.print(health_table)
                    
                else:
                    self.console.print(f"❌ Memory service is unhealthy: {response.get('error', 'Unknown error')}", style="red")
            else:
                self.console.print("❌ Failed to connect to memory service", style="red")
                
        except Exception as e:
            self.console.print(f"❌ Error checking memory health: {str(e)}", style="red")

    async def show_memory_help(self) -> None:
        """Display help information for memory commands."""
        self._ensure_console()
        help_content = """
# Memory Commands Help

## Store Instructions
Store explicit instructions for future reference:
```bash
paladin --memory-store "Always check systemd logs for application errors"
```

## Search Memories
Search through stored memories:
```bash
paladin --memory-search "CPU alerts" --limit 5
```

## Contextual Memories
Get memories relevant to current context:
```bash
paladin --memory-context "high memory usage" --workflow-type ACTION
```

## Memory Types
View available memory types:
```bash
paladin --memory-types
```

## Health Check
Check memory service status:
```bash
paladin --memory-health
```

## Memory Integration
Memory is automatically integrated into chat workflows. When you use:
```bash
paladin --chat "fetch CPU metrics for last hour"
```

The system will:
1. Check for relevant memories before processing
2. Extract new operational knowledge after completion
3. Store patterns for future reference

This helps the system learn from your monitoring patterns and preferences!
        """
        
        markdown = Markdown(help_content)
        panel = Panel(
            markdown,
            title="🧠 Paladin AI Memory Features",
            border_style="blue"
        )
        self.console.print(panel)