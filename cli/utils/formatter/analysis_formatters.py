"""
Analysis formatters for complex values and analysis-only responses.
"""

from typing import Dict, Any, Union, List


def format_complex_value(value: Union[Dict, List, Any]) -> str:
    """Format complex values (dict/list) for display."""
    if isinstance(value, dict):
        if len(value) <= 3:  # Show small dicts inline
            return ", ".join([f"{k}: {v}" for k, v in value.items()])
        else:
            return f"{len(value)} items"
    elif isinstance(value, list):
        if len(value) <= 3:  # Show small lists inline
            return ", ".join([str(item) for item in value])
        else:
            return f"{len(value)} items"
    else:
        return str(value)


def format_analysis_only(response: Dict[str, Any], icons: Dict[str, str]) -> str:
    """
    Format response to show only the analysis section.

    Args:
        response: The workflow response from the server
        icons: Icon dictionary for formatting

    Returns:
        Formatted string showing only analysis data
    """
    if not response:
        return f"{icons['error']} No response received"

    # Check if this is an error response
    if not response.get("success", True):
        return f"{icons['error']} {response.get('error', 'Unknown error occurred')}"

    lines = []
    
    # Check if we have the new server response format
    if "raw_result" in response:
        # Use the raw_result for analysis
        result_data = response["raw_result"]
    else:
        # Use the response directly
        result_data = response

    # Handle different response types
    if "action_result" in result_data:
        action_result = result_data["action_result"]

        # New format - data response
        if "data_response" in action_result:
            lines.append(f"{icons['info']} Analysis:")
            lines.append(f"   {action_result['data_response']}")

        # New format - supporting metrics
        if "supporting_metrics" in action_result:
            lines.append(f"\n{icons['metrics']} Supporting Data:")
            supporting_metrics = action_result["supporting_metrics"]
            if isinstance(supporting_metrics, dict):
                for key, value in supporting_metrics.items():
                    if key != "unit":  # Handle unit separately
                        unit = supporting_metrics.get("unit", "")
                        display_value = f"{value} {unit}".strip() if unit else str(value)
                        lines.append(f"   • {key.replace('_', ' ').title()}: {display_value}")

        # Legacy format - Analysis results
        if "analysis_results" in action_result:
            lines.append(f"{icons['info']} Analysis:")
            analysis = action_result["analysis_results"]
            if isinstance(analysis, str):
                lines.append(f"   {analysis}")
            elif isinstance(analysis, dict):
                for key, value in analysis.items():
                    if isinstance(value, (dict, list)):
                        lines.append(f"   • {key}: {format_complex_value(value)}")
                    else:
                        lines.append(f"   • {key}: {value}")

        # Recommendations if available
        if "recommendations" in action_result:
            lines.append(f"\n💡 Recommendations:")
            recommendations = action_result["recommendations"]
            if isinstance(recommendations, dict):
                for key, value in recommendations.items():
                    lines.append(f"   • {key}: {value}")
            elif isinstance(recommendations, list):
                for rec in recommendations:
                    lines.append(f"   • {rec}")
            else:
                lines.append(f"   {recommendations}")

    elif "query_result" in result_data:
        query_result = result_data["query_result"]

        # New format - data response
        if "data_response" in query_result:
            lines.append(f"{icons['query']} Analysis:")
            lines.append(f"   {query_result['data_response']}")

        # New format - supporting metrics
        if "supporting_metrics" in query_result:
            lines.append(f"\n{icons['metrics']} Supporting Data:")
            supporting_metrics = query_result["supporting_metrics"]
            if isinstance(supporting_metrics, dict):
                for key, value in supporting_metrics.items():
                    if key != "unit":  # Handle unit separately
                        unit = supporting_metrics.get("unit", "")
                        display_value = f"{value} {unit}".strip() if unit else str(value)
                        lines.append(f"   • {key.replace('_', ' ').title()}: {display_value}")

        # Legacy format - Main response
        if "response" in query_result:
            lines.append(f"{icons['query']} Analysis:")
            lines.append(f"   {query_result['response']}")

        # Legacy format - Supporting data as analysis
        if "supporting_data" in query_result:
            lines.append(f"\n{icons['metrics']} Supporting Analysis:")
            supporting_data = query_result["supporting_data"]
            if isinstance(supporting_data, dict):
                for key, value in supporting_data.items():
                    lines.append(f"   • {key}: {value}")

    elif "incident_result" in result_data:
        incident_report = result_data["incident_result"]["incident_report"]

        # Root cause analysis
        if "root_cause_analysis" in incident_report:
            lines.append(f"{icons['info']} Root Cause Analysis:")
            rca = incident_report["root_cause_analysis"]
            if isinstance(rca, dict):
                for key, value in rca.items():
                    lines.append(f"   • {key}: {value}")
            else:
                lines.append(f"   {rca}")

        # Recommendations
        if "recommendations" in incident_report:
            lines.append(f"\n💡 Recommendations:")
            recommendations = incident_report["recommendations"]
            if isinstance(recommendations, list):
                for rec in recommendations:
                    lines.append(f"   • {rec}")
            else:
                lines.append(f"   {recommendations}")

    if not lines:
        return f"{icons['info']} No analysis data available in response"

    return "\n".join(lines)