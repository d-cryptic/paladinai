"""
Markdown formatter for Paladin AI CLI using Rich.

This module handles formatting of markdown content from server responses
using the Rich library for beautiful terminal rendering.
"""

from typing import Dict, Any, Optional
from rich.console import Console
from rich.markdown import Markdown
from rich.panel import Panel
from rich.text import Text
from rich.rule import Rule
from rich.table import Table
from rich.columns import Columns


class MarkdownFormatter:
    """Formats markdown content using Rich for terminal display."""
    
    def __init__(self):
        """Initialize the markdown formatter with a Rich console."""
        self.console = Console()
        self.icons = {
            "success": "✅",
            "error": "❌", 
            "warning": "⚠️",
            "info": "ℹ️",
            "metrics": "📊",
            "query": "🔍",
            "action": "⚡",
            "incident": "🚨",
            "prometheus": "📈",
            "time": "🕐",
            "session": "📋",
            "robot": "🤖"
        }
    
    def format_response(self, response: Dict[str, Any]) -> None:
        """
        Format and display a response with markdown content.
        
        Args:
            response: The response from the server
        """
        # Check for formatted_markdown in the response
        if "formatted_markdown" in response and response["formatted_markdown"]:
            self._display_markdown_content(response["formatted_markdown"], response)
        elif "content" in response and isinstance(response["content"], str):
            # If content is markdown, render it
            self._display_markdown_content(response["content"], response)
        else:
            # Fall back to standard display
            self._display_standard_response(response)
    
    def _display_markdown_content(self, markdown_content: str, response: Dict[str, Any]) -> None:
        """
        Display markdown content with Rich formatting.
        
        Args:
            markdown_content: The markdown string to display
            response: The full response for metadata
        """
        # Create header based on response type
        workflow_type = self._get_workflow_type(response)
        header = self._create_header(workflow_type)
        
        # Display header
        if header:
            self.console.print(header)
            self.console.print()
        
        # Render the markdown content
        if markdown_content.strip():
            # Configure markdown rendering options
            markdown = Markdown(
                markdown_content,
                code_theme="monokai",  # Syntax highlighting theme
                hyperlinks=True,       # Enable hyperlinks
                justify="left"         # Text justification
            )
            
            # Determine panel style based on workflow type
            panel_styles = {
                "QUERY": ("cyan", "🔍 Query Response"),
                "ACTION": ("yellow", "⚡ Action Result"),
                "INCIDENT": ("red", "🚨 Incident Analysis")
            }
            border_style, title = panel_styles.get(
                workflow_type.upper(), 
                ("blue", f"{self.icons['robot']} Paladin AI Response")
            )
            
            # Wrap in a panel for better visual separation
            panel = Panel(
                markdown,
                title=title,
                border_style=border_style,
                padding=(1, 2),
                expand=False
            )
            self.console.print(panel)
        
        # Display metadata footer
        self._display_metadata_footer(response)
    
    def _display_standard_response(self, response: Dict[str, Any]) -> None:
        """
        Display a standard response when markdown is not available.
        
        Args:
            response: The response to display
        """
        # Display success/error status
        if response.get("success", True):
            status_text = Text(f"{self.icons['success']} Request completed successfully", style="green")
        else:
            status_text = Text(f"{self.icons['error']} Request failed", style="red")
        
        self.console.print(status_text)
        
        # Display the response content
        if "error" in response:
            self.console.print(f"\n[red]Error: {response['error']}[/red]")
        
        # Display any available results
        for key in ["query_result", "action_result", "incident_result"]:
            if key in response:
                self.console.print(f"\n[bold]{key.replace('_', ' ').title()}:[/bold]")
                self._display_complex_value(response[key])
        
        # Display metadata footer
        self._display_metadata_footer(response)
    
    def _create_header(self, workflow_type: str) -> Optional[Rule]:
        """
        Create a header for the response based on workflow type.
        
        Args:
            workflow_type: The type of workflow
            
        Returns:
            A Rich Rule object or None
        """
        headers = {
            "QUERY": Rule(f"{self.icons['query']} Query Response", style="cyan"),
            "ACTION": Rule(f"{self.icons['action']} Action Result", style="yellow"),
            "INCIDENT": Rule(f"{self.icons['incident']} Incident Analysis", style="red")
        }
        return headers.get(workflow_type.upper())
    
    def _get_workflow_type(self, response: Dict[str, Any]) -> str:
        """
        Extract workflow type from response.
        
        Args:
            response: The response dictionary
            
        Returns:
            The workflow type string
        """
        # Try multiple locations for workflow type
        if "metadata" in response and "workflow_type" in response["metadata"]:
            return response["metadata"]["workflow_type"]
        elif "categorization" in response and "workflow_type" in response["categorization"]:
            return response["categorization"]["workflow_type"]
        elif "execution_metadata" in response and "workflow_type" in response["execution_metadata"]:
            return response["execution_metadata"]["workflow_type"]
        return "UNKNOWN"
    
    def _display_metadata_footer(self, response: Dict[str, Any]) -> None:
        """
        Display metadata footer with session info and execution details.
        
        Args:
            response: The response containing metadata
        """
        footer_items = []
        
        # Session ID
        session_id = response.get("session_id")
        if session_id:
            footer_items.append(f"{self.icons['session']} Session: [cyan]{session_id}[/cyan]")
        
        # Execution time
        if "metadata" in response and "execution_time_ms" in response["metadata"]:
            exec_time = response["metadata"]["execution_time_ms"]
            footer_items.append(f"{self.icons['time']} Time: [yellow]{exec_time}ms[/yellow]")
        elif "execution_time_ms" in response:
            exec_time = response["execution_time_ms"]
            footer_items.append(f"{self.icons['time']} Time: [yellow]{exec_time}ms[/yellow]")
        
        # Display footer if we have items
        if footer_items:
            self.console.print()
            self.console.print(Rule(style="dim"))
            footer_text = "  •  ".join(footer_items)
            self.console.print(footer_text, style="dim")
    
    def _display_complex_value(self, value: Any, indent: int = 0) -> None:
        """
        Display complex values (dicts, lists) with proper formatting.
        
        Args:
            value: The value to display
            indent: Current indentation level
        """
        if isinstance(value, dict):
            for k, v in value.items():
                if isinstance(v, (dict, list)):
                    self.console.print(f"{'  ' * indent}[bold]{k}:[/bold]")
                    self._display_complex_value(v, indent + 1)
                else:
                    self.console.print(f"{'  ' * indent}[bold]{k}:[/bold] {v}")
        elif isinstance(value, list):
            for item in value:
                if isinstance(item, (dict, list)):
                    self.console.print(f"{'  ' * indent}•")
                    self._display_complex_value(item, indent + 1)
                else:
                    self.console.print(f"{'  ' * indent}• {item}")
        else:
            self.console.print(f"{'  ' * indent}{value}")
    
    def format_error_response(self, error_msg: str, error_type: str = "unknown") -> None:
        """
        Format and display error responses with Rich styling.
        
        Args:
            error_msg: The error message
            error_type: Type of error
        """
        error_panel = Panel(
            f"[red]{error_msg}[/red]",
            title=f"{self.icons['error']} Error",
            border_style="red",
            padding=(1, 2)
        )
        self.console.print(error_panel)
        
        # Add suggestions based on error type
        if error_type == "connection_error":
            suggestions = Table(show_header=False, box=None)
            suggestions.add_column(style="yellow")
            suggestions.add_row("💡 Suggestions:")
            suggestions.add_row("   • Check if the server is running: make run-server")
            suggestions.add_row("   • Verify server URL in cli/.env")
            suggestions.add_row("   • Check network connectivity")
            self.console.print(suggestions)
        elif error_type == "timeout_error":
            suggestions = Table(show_header=False, box=None)
            suggestions.add_column(style="yellow")
            suggestions.add_row("💡 The request took too long to complete")
            suggestions.add_row("   • Try a simpler query")
            suggestions.add_row("   • Check server logs for issues")
            self.console.print(suggestions)


# Global markdown formatter instance
markdown_formatter = MarkdownFormatter()