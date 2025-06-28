"""
Response formatter for Paladin AI CLI.

This module handles formatting of server responses for display in the CLI,
including workflow results, metrics data, and error messages.
"""

import json
from typing import Dict, Any, Optional
from datetime import datetime


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
        error_msg = response.get("error", "Unknown error occurred")
        session_id = response.get("session_id", "unknown")
        
        lines = [
            f"{self.icons['error']} Error occurred during processing",
            f"   Error: {error_msg}",
            f"   Session: {session_id}"
        ]
        
        if "execution_path" in response:
            path = " → ".join(response["execution_path"])
            lines.append(f"   Path: {path}")
        
        return "\n".join(lines)
    
    def _format_query_response(self, response: Dict[str, Any], interactive: bool) -> str:
        """Format query workflow responses."""
        query_result = response.get("query_result", {})
        
        lines = [f"{self.icons['query']} Query Results"]

        # Main data response (new action format)
        if "data_response" in query_result:
            lines.append(f"\n{query_result['data_response']}")
        # Legacy query format - main response
        elif "response" in query_result:
            lines.append(f"\n{query_result['response']}")

        # Supporting metrics (new action format)
        if "supporting_metrics" in query_result:
            lines.append(f"\n{self.icons['metrics']} Metrics:")
            supporting_metrics = query_result["supporting_metrics"]
            if isinstance(supporting_metrics, dict):
                for key, value in supporting_metrics.items():
                    if key != "unit":  # Handle unit separately
                        # Handle structured metric values
                        if isinstance(value, dict) and "value" in value:
                            metric_value = value["value"]
                            metric_unit = value.get("unit", "")
                            display_value = f"{metric_value}{metric_unit}".strip()
                        else:
                            unit = supporting_metrics.get("unit", "")
                            display_value = f"{value} {unit}".strip() if unit else str(value)
                        lines.append(f"   • {key.replace('_', ' ').title()}: {display_value}")
        # Legacy query format - supporting data
        elif "supporting_data" in query_result:
            lines.append(f"\n{self.icons['metrics']} Supporting Data:")
            supporting_data = query_result["supporting_data"]
            if isinstance(supporting_data, dict):
                for key, value in supporting_data.items():
                    lines.append(f"   • {key.replace('_', ' ').title()}: {value}")

        # Data source and timestamp
        metadata_parts = []
        if "data_source" in query_result:
            metadata_parts.append(f"Source: {query_result['data_source']}")
        if "timestamp" in query_result:
            metadata_parts.append(f"Time: {query_result['timestamp']}")
        if "confidence" in query_result:
            confidence = query_result["confidence"]
            confidence_pct = f"{confidence * 100:.0f}%" if confidence <= 1 else f"{confidence:.0f}%"
            metadata_parts.append(f"Confidence: {confidence_pct}")

        if metadata_parts:
            lines.append(f"\n{self.icons['time']} Metadata:")
            lines.append(f"   {' | '.join(metadata_parts)}")
        
        return "\n".join(lines)
    
    def _format_action_response(self, response: Dict[str, Any], interactive: bool) -> str:
        """Format action workflow responses."""
        action_result = response.get("action_result", {})

        lines = [f"{self.icons['action']} Action Results"]



        # Prioritize metrics data display for simple data requests
        has_metrics = False

        # Supporting metrics (prioritized display)
        if "supporting_metrics" in action_result:
            lines.append(f"\n{self.icons['metrics']} Metrics:")
            supporting_metrics = action_result["supporting_metrics"]
            if isinstance(supporting_metrics, dict):
                for key, value in supporting_metrics.items():
                    if key != "unit":  # Handle unit separately
                        # Handle structured metric values
                        if isinstance(value, dict) and "value" in value:
                            metric_value = value["value"]
                            metric_unit = value.get("unit", "")
                            display_value = f"{metric_value}{metric_unit}".strip()
                        else:
                            unit = supporting_metrics.get("unit", "")
                            display_value = f"{value} {unit}".strip() if unit else str(value)
                        lines.append(f"   • {key.replace('_', ' ').title()}: {display_value}")
                        has_metrics = True

        # Check for processed_metrics field (alternative format)
        if "processed_metrics" in action_result and not has_metrics:
            lines.append(f"\n{self.icons['metrics']} Metrics:")
            processed_metrics = action_result["processed_metrics"]
            if isinstance(processed_metrics, dict):
                for key, value in processed_metrics.items():
                    lines.append(f"   • {key}: {value}")
                    has_metrics = True

        # Check for current_values field (alternative format)
        if "current_values" in action_result and not has_metrics:
            lines.append(f"\n{self.icons['info']} Current Values:")
            current_values = action_result["current_values"]
            if isinstance(current_values, dict):
                for key, value in current_values.items():
                    lines.append(f"   • {key}: {value}")
                    has_metrics = True

        # Main data response (show after metrics for simple data requests)
        if "data_response" in action_result:
            # Only show if it's not just a brief summary when we have metrics
            data_response = action_result["data_response"]
            if not has_metrics or len(data_response) > 50:  # Show if no metrics or substantial content
                lines.append(f"\n{data_response}")

        # Check for basic_statistics field (alternative format)
        if "basic_statistics" in action_result:
            lines.append(f"\n{self.icons['metrics']} Statistics:")
            basic_statistics = action_result["basic_statistics"]
            if isinstance(basic_statistics, dict):
                for key, value in basic_statistics.items():
                    lines.append(f"   • {key}: {value}")

        # Data source and timestamp
        metadata_parts = []
        if "data_source" in action_result:
            metadata_parts.append(f"Source: {action_result['data_source']}")
        if "timestamp" in action_result:
            metadata_parts.append(f"Time: {action_result['timestamp']}")
        if "confidence" in action_result:
            confidence = action_result["confidence"]
            confidence_pct = f"{confidence * 100:.0f}%" if confidence <= 1 else f"{confidence:.0f}%"
            metadata_parts.append(f"Confidence: {confidence_pct}")

        if metadata_parts:
            lines.append(f"\n{self.icons['time']} Metadata:")
            lines.append(f"   {' | '.join(metadata_parts)}")

        # Legacy format support - Data report
        if "data_report" in action_result:
            lines.append(f"\n{self.icons['metrics']} Data Report:")
            data_report = action_result["data_report"]
            if isinstance(data_report, str):
                lines.append(f"   {data_report}")
            elif isinstance(data_report, dict):
                for key, value in data_report.items():
                    lines.append(f"   • {key}: {value}")

        # Legacy format support - Analysis results
        if "analysis_results" in action_result:
            lines.append(f"\n{self.icons['info']} Analysis:")
            analysis = action_result["analysis_results"]
            if isinstance(analysis, str):
                lines.append(f"   {analysis}")
            elif isinstance(analysis, dict):
                for key, value in analysis.items():
                    lines.append(f"   • {key}: {value}")

        # If no metrics found, try to extract from any available data fields
        if not has_metrics:
            # Check for any data that looks like metrics in the response
            for key, value in action_result.items():
                if key in ["data", "metrics", "values", "results"] and isinstance(value, dict):
                    lines.append(f"\n{self.icons['metrics']} Data:")
                    for metric_key, metric_value in value.items():
                        lines.append(f"   • {metric_key.replace('_', ' ').title()}: {metric_value}")
                        has_metrics = True
                    break

        # Recommendations (only show if no metrics data or if specifically requested)
        if "recommendations" in action_result:
            # Check if user specifically requested recommendations or analysis
            user_wants_recommendations = any(word in action_result.get("data_response", "").lower()
                                           for word in ["recommend", "suggest", "advice", "should"])

            # Show recommendations if no metrics were displayed or if specifically requested
            if not has_metrics or user_wants_recommendations:
                lines.append(f"\n💡 Recommendations:")
                recommendations = action_result["recommendations"]
                if isinstance(recommendations, dict):
                    # Handle malformed dict recommendations (like {'1': {...}, '2': {...}})
                    for key, rec in recommendations.items():
                        if isinstance(rec, dict):
                            action = rec.get("action", str(rec))
                            priority = rec.get("priority", "")
                            if priority:
                                lines.append(f"   • {action} (Priority: {priority})")
                            else:
                                lines.append(f"   • {action}")
                        else:
                            lines.append(f"   • {rec}")
                elif isinstance(recommendations, list):
                    for rec in recommendations:
                        if isinstance(rec, dict):
                            # Handle structured recommendations
                            action = rec.get("action", str(rec))
                            priority = rec.get("priority", "")
                            if priority:
                                lines.append(f"   • {action} (Priority: {priority})")
                            else:
                                lines.append(f"   • {action}")
                        else:
                            lines.append(f"   • {rec}")
                else:
                    lines.append(f"   {recommendations}")

        return "\n".join(lines)

    def format_analysis_only(self, response: Dict[str, Any]) -> str:
        """
        Format response to show only the analysis section.

        Args:
            response: The workflow response from the server

        Returns:
            Formatted string showing only analysis data
        """
        if not response:
            return f"{self.icons['error']} No response received"

        # Check if this is an error response
        if not response.get("success", True):
            return f"{self.icons['error']} {response.get('error', 'Unknown error occurred')}"

        lines = []

        # Handle different response types
        if "action_result" in response:
            action_result = response["action_result"]

            # New format - data response
            if "data_response" in action_result:
                lines.append(f"{self.icons['info']} Analysis:")
                lines.append(f"   {action_result['data_response']}")

            # New format - supporting metrics
            if "supporting_metrics" in action_result:
                lines.append(f"\n{self.icons['metrics']} Supporting Data:")
                supporting_metrics = action_result["supporting_metrics"]
                if isinstance(supporting_metrics, dict):
                    for key, value in supporting_metrics.items():
                        if key != "unit":  # Handle unit separately
                            unit = supporting_metrics.get("unit", "")
                            display_value = f"{value} {unit}".strip() if unit else str(value)
                            lines.append(f"   • {key.replace('_', ' ').title()}: {display_value}")

            # Legacy format - Analysis results
            if "analysis_results" in action_result:
                lines.append(f"{self.icons['info']} Analysis:")
                analysis = action_result["analysis_results"]
                if isinstance(analysis, str):
                    lines.append(f"   {analysis}")
                elif isinstance(analysis, dict):
                    for key, value in analysis.items():
                        if isinstance(value, (dict, list)):
                            lines.append(f"   • {key}: {self._format_complex_value(value)}")
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

        elif "query_result" in response:
            query_result = response["query_result"]

            # New format - data response
            if "data_response" in query_result:
                lines.append(f"{self.icons['query']} Analysis:")
                lines.append(f"   {query_result['data_response']}")

            # New format - supporting metrics
            if "supporting_metrics" in query_result:
                lines.append(f"\n{self.icons['metrics']} Supporting Data:")
                supporting_metrics = query_result["supporting_metrics"]
                if isinstance(supporting_metrics, dict):
                    for key, value in supporting_metrics.items():
                        if key != "unit":  # Handle unit separately
                            unit = supporting_metrics.get("unit", "")
                            display_value = f"{value} {unit}".strip() if unit else str(value)
                            lines.append(f"   • {key.replace('_', ' ').title()}: {display_value}")

            # Legacy format - Main response
            if "response" in query_result:
                lines.append(f"{self.icons['query']} Analysis:")
                lines.append(f"   {query_result['response']}")

            # Legacy format - Supporting data as analysis
            if "supporting_data" in query_result:
                lines.append(f"\n{self.icons['metrics']} Supporting Analysis:")
                supporting_data = query_result["supporting_data"]
                if isinstance(supporting_data, dict):
                    for key, value in supporting_data.items():
                        lines.append(f"   • {key}: {value}")

        elif "incident_result" in response:
            incident_result = response["incident_result"]

            # Root cause analysis
            if "root_cause_analysis" in incident_result:
                lines.append(f"{self.icons['info']} Root Cause Analysis:")
                rca = incident_result["root_cause_analysis"]
                if isinstance(rca, dict):
                    for key, value in rca.items():
                        lines.append(f"   • {key}: {value}")
                else:
                    lines.append(f"   {rca}")

            # Recommendations
            if "recommendations" in incident_result:
                lines.append(f"\n💡 Recommendations:")
                recommendations = incident_result["recommendations"]
                if isinstance(recommendations, list):
                    for rec in recommendations:
                        lines.append(f"   • {rec}")
                else:
                    lines.append(f"   {recommendations}")

        if not lines:
            return f"{self.icons['info']} No analysis data available in response"

        return "\n".join(lines)

    def _format_complex_value(self, value):
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

    def _format_incident_response(self, response: Dict[str, Any], interactive: bool) -> str:
        """Format incident workflow responses."""
        incident_result = response.get("incident_result", {})
        
        lines = [f"{self.icons['incident']} Incident Report"]
        
        # Severity
        if "severity" in incident_result:
            severity = incident_result["severity"].upper()
            severity_icon = "🔴" if severity == "CRITICAL" else "🟡" if severity == "HIGH" else "🟢"
            lines.append(f"   Severity: {severity_icon} {severity}")
        
        # Incident report
        if "incident_report" in incident_result:
            lines.append(f"\n📋 Summary:")
            lines.append(f"   {incident_result['incident_report']}")
        
        # Impact assessment
        if "impact_assessment" in incident_result:
            lines.append(f"\n💥 Impact:")
            impact = incident_result["impact_assessment"]
            if isinstance(impact, str):
                lines.append(f"   {impact}")
            elif isinstance(impact, dict):
                for key, value in impact.items():
                    lines.append(f"   • {key}: {value}")
        
        # Root cause analysis
        if "root_cause_analysis" in incident_result:
            lines.append(f"\n🔍 Root Cause:")
            rca = incident_result["root_cause_analysis"]
            if isinstance(rca, str):
                lines.append(f"   {rca}")
            elif isinstance(rca, dict):
                for key, value in rca.items():
                    lines.append(f"   • {key}: {value}")
        
        # Next steps
        if "next_steps" in incident_result:
            lines.append(f"\n⏭️ Next Steps:")
            next_steps = incident_result["next_steps"]
            if isinstance(next_steps, list):
                for step in next_steps:
                    lines.append(f"   • {step}")
            else:
                lines.append(f"   {next_steps}")
        
        return "\n".join(lines)
    
    def _format_generic_response(self, response: Dict[str, Any], interactive: bool) -> str:
        """Format generic responses that don't match specific patterns."""
        lines = [f"{self.icons['info']} Response"]
        
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
            lines.append(f"\n{self.icons['info']} Session: {response['session_id']}")
        
        return "\n".join(lines)
    
    def format_metrics_data(self, metrics: Dict[str, Any]) -> str:
        """Format metrics data for display."""
        lines = [f"{self.icons['prometheus']} Metrics Data"]
        
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


# Global formatter instance
formatter = ResponseFormatter()
