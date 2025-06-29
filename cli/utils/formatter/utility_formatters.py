"""
Utility formatters for error, generic, and metrics responses.
"""

import json
from typing import Dict, Any


def format_error_response(response: Dict[str, Any], icons: Dict[str, str], interactive: bool) -> str:
    """Format error responses."""
    error_msg = response.get("error", "Unknown error occurred")
    session_id = response.get("session_id", "unknown")
    
    lines = [
        f"{icons['error']} Error occurred during processing",
        f"   Error: {error_msg}",
        f"   Session: {session_id}"
    ]
    
    if "execution_path" in response:
        path = " → ".join(response["execution_path"])
        lines.append(f"   Path: {path}")
    
    return "\n".join(lines)


def format_generic_response(response: Dict[str, Any], icons: Dict[str, str], interactive: bool) -> str:
    """Format generic responses that don't match specific patterns."""
    lines = [f"{icons['info']} Response"]
    
    # Try to extract meaningful content
    if "content" in response:
        lines.append(f"   {response['content']}")
    elif "message" in response:
        lines.append(f"   {response['message']}")
    elif "result" in response:
        result = response["result"]
        if isinstance(result, str):
            lines.append(f"   {result}")
        else:
            lines.append(f"   {json.dumps(result, indent=2)}")
    else:
        # Show the raw response in a formatted way
        lines.append(f"   {json.dumps(response, indent=2)}")
    
    # Add session info if available
    if "session_id" in response:
        lines.append(f"\n{icons['info']} Session: {response['session_id']}")
    
    return "\n".join(lines)


def format_metrics_data(metrics: Dict[str, Any], icons: Dict[str, str]) -> str:
    """Format metrics data for display."""
    lines = [f"{icons['prometheus']} Metrics Data"]
    
    if "processed_metrics" in metrics:
        processed = metrics["processed_metrics"]
        for metric_name, metric_data in processed.items():
            lines.append(f"   • {metric_name}: {metric_data}")
    
    if "key_insights" in metrics:
        lines.append(f"\n💡 Key Insights:")
        insights = metrics["key_insights"]
        if isinstance(insights, list):
            for insight in insights:
                lines.append(f"   • {insight}")
        else:
            lines.append(f"   {insights}")
    
    return "\n".join(lines)