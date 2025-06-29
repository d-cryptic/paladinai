"""
Response formatter for Paladin AI CLI.

This module handles formatting of server responses for display in the CLI,
including workflow results, metrics data, and error messages.
"""

from typing import Dict, Any
from .workflow_formatters import format_query_response, format_action_response, format_incident_response
from .utility_formatters import format_error_response, format_generic_response, format_metrics_data
from .analysis_formatters import format_analysis_only, format_complex_value


class ResponseFormatter:
    """Formats server responses for CLI display."""
    
    def __init__(self):
        """Initialize the formatter."""
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
            "time": "🕐"
        }
    
    def format_workflow_response(self, response: Dict[str, Any], interactive: bool = True) -> str:
        """
        Format a LangGraph workflow response for display.
        
        Args:
            response: The workflow response from the server
            interactive: Whether to use interactive formatting
            
        Returns:
            Formatted string for display
        """
        if not response:
            return f"{self.icons['error']} No response received"
        
        # Check if this is an error response
        if not response.get("success", True):
            return self._format_error_response(response, interactive)
        
        # Determine response type and format accordingly
        if "query_result" in response:
            return self._format_query_response(response, interactive)
        elif "action_result" in response:
            return self._format_action_response(response, interactive)
        elif "incident_result" in response:
            return self._format_incident_response(response, interactive)
        else:
            return self._format_generic_response(response, interactive)
    
    def _format_error_response(self, response: Dict[str, Any], interactive: bool) -> str:
        """Format error responses."""
        return format_error_response(response, self.icons, interactive)
    
    def _format_query_response(self, response: Dict[str, Any], interactive: bool) -> str:
        """Format query workflow responses."""
        return format_query_response(response, self.icons, interactive)
    
    def _format_action_response(self, response: Dict[str, Any], interactive: bool) -> str:
        """Format action workflow responses."""
        return format_action_response(response, self.icons, interactive)

    def format_analysis_only(self, response: Dict[str, Any]) -> str:
        """
        Format response to show only the analysis section.

        Args:
            response: The workflow response from the server

        Returns:
            Formatted string showing only analysis data
        """
        return format_analysis_only(response, self.icons)

    def _format_complex_value(self, value):
        """Format complex values (dict/list) for display."""
        return format_complex_value(value)

    def _format_incident_response(self, response: Dict[str, Any], interactive: bool) -> str:
        """Format incident workflow responses."""
        return format_incident_response(response, self.icons, interactive)
    
    def _format_generic_response(self, response: Dict[str, Any], interactive: bool) -> str:
        """Format generic responses that don't match specific patterns."""
        return format_generic_response(response, self.icons, interactive)
    
    def format_metrics_data(self, metrics: Dict[str, Any]) -> str:
        """Format metrics data for display."""
        return format_metrics_data(metrics, self.icons)


# Global formatter instance
formatter = ResponseFormatter()
