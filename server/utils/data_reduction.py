"""
Data Reduction Utilities for Large Monitoring Data.

This module provides utilities to reduce the size of monitoring data
to fit within LLM context windows while preserving the most important information.
"""

import json
import logging
from typing import Dict, Any, List, Optional, Tuple
from datetime import datetime, timedelta
import tiktoken

logger = logging.getLogger(__name__)


class DataReducer:
    """Handles intelligent data reduction for monitoring data."""
    
    def __init__(self, max_tokens: int = 100000):
        """
        Initialize the data reducer.
        
        Args:
            max_tokens: Maximum tokens to allow (leaving buffer for prompts)
        """
        self.max_tokens = max_tokens
        self.encoder = tiktoken.encoding_for_model("gpt-4")
    
    def estimate_tokens(self, data: Any) -> int:
        """Estimate token count for data."""
        try:
            if isinstance(data, str):
                return len(self.encoder.encode(data))
            else:
                # Convert to JSON string for estimation
                json_str = json.dumps(data, indent=2)
                return len(self.encoder.encode(json_str))
        except Exception as e:
            logger.warning(f"Token estimation failed: {e}, using character count / 4")
            # Fallback: rough estimate of 4 characters per token
            json_str = json.dumps(data) if not isinstance(data, str) else data
            return len(json_str) // 4
    
    def reduce_prometheus_data(self, prometheus_data: Dict[str, Any], 
                             priority: str = "recent") -> Dict[str, Any]:
        """
        Reduce Prometheus data size intelligently.
        
        Args:
            prometheus_data: Raw prometheus data
            priority: Strategy - "recent", "anomalies", "critical"
            
        Returns:
            Reduced data that fits within token limits
        """
        reduced = {
            "metadata": {
                "reduction_applied": True,
                "original_metrics_count": 0,
                "reduced_metrics_count": 0,
                "strategy": priority
            },
            "metrics": {},
            "alerts": [],
            "summary": {}
        }
        
        # Handle metrics
        if "metrics" in prometheus_data:
            metrics = prometheus_data["metrics"]
            reduced["metadata"]["original_metrics_count"] = len(metrics)
            
            # Strategy 1: Time-based filtering (keep recent data)
            if priority == "recent":
                metrics = self._filter_recent_metrics(metrics, hours=2)
            
            # Strategy 2: Aggregate similar metrics
            metrics = self._aggregate_metrics(metrics)
            
            # Strategy 3: Remove verbose fields
            metrics = self._strip_verbose_fields(metrics)
            
            # Strategy 4: Sample data points if still too large
            current_tokens = self.estimate_tokens(metrics)
            if current_tokens > self.max_tokens * 0.7:  # Use 70% for metrics
                metrics = self._sample_metrics(metrics, target_tokens=int(self.max_tokens * 0.7))
            
            reduced["metrics"] = metrics
            reduced["metadata"]["reduced_metrics_count"] = len(metrics)
        
        # Handle alerts with priority
        if "alerts" in prometheus_data:
            alerts = prometheus_data.get("alerts", [])
            reduced["alerts"] = self._prioritize_alerts(alerts)
        
        # Generate summary
        reduced["summary"] = self._generate_summary(reduced)
        
        return reduced
    
    def reduce_alertmanager_data(self, alert_data: Dict[str, Any]) -> Dict[str, Any]:
        """
        Reduce AlertManager data size.
        
        Args:
            alert_data: Raw alertmanager data
            
        Returns:
            Reduced alert data
        """
        reduced = {
            "metadata": {
                "reduction_applied": True,
                "original_alerts_count": len(alert_data.get("alerts", [])),
                "reduced_alerts_count": 0
            },
            "alerts": [],
            "groups": [],
            "summary": {}
        }
        
        # Prioritize alerts by severity and recency
        if "alerts" in alert_data:
            alerts = alert_data["alerts"]
            
            # Sort by severity and time
            priority_order = {"critical": 0, "error": 1, "warning": 2, "info": 3}
            alerts.sort(key=lambda x: (
                priority_order.get(x.get("labels", {}).get("severity", "info"), 3),
                x.get("startsAt", "")
            ))
            
            # Keep only essential fields
            reduced_alerts = []
            for alert in alerts:
                reduced_alert = {
                    "name": alert.get("labels", {}).get("alertname", "Unknown"),
                    "severity": alert.get("labels", {}).get("severity", "info"),
                    "instance": alert.get("labels", {}).get("instance", ""),
                    "job": alert.get("labels", {}).get("job", ""),
                    "startsAt": alert.get("startsAt", ""),
                    "status": alert.get("status", {}).get("state", ""),
                    "summary": alert.get("annotations", {}).get("summary", ""),
                    "description": alert.get("annotations", {}).get("description", "")[:200]  # Truncate long descriptions
                }
                
                # Remove empty fields
                reduced_alert = {k: v for k, v in reduced_alert.items() if v}
                reduced_alerts.append(reduced_alert)
                
                # Check token limit
                if self.estimate_tokens(reduced_alerts) > self.max_tokens * 0.8:
                    break
            
            reduced["alerts"] = reduced_alerts
            reduced["metadata"]["reduced_alerts_count"] = len(reduced_alerts)
        
        # Handle alert groups
        if "groups" in alert_data:
            groups = alert_data["groups"]
            reduced["groups"] = self._summarize_alert_groups(groups)
        
        # Generate summary
        reduced["summary"] = self._generate_alert_summary(reduced)
        
        return reduced
    
    def reduce_loki_logs(self, log_data: Dict[str, Any], 
                        max_lines: int = 1000) -> Dict[str, Any]:
        """
        Reduce Loki log data size.
        
        Args:
            log_data: Raw loki log data
            max_lines: Maximum number of log lines to keep
            
        Returns:
            Reduced log data
        """
        reduced = {
            "metadata": {
                "reduction_applied": True,
                "original_log_count": 0,
                "reduced_log_count": 0
            },
            "logs": [],
            "patterns": {},
            "summary": {}
        }
        
        if "logs" in log_data:
            logs = log_data["logs"]
            reduced["metadata"]["original_log_count"] = len(logs)
            
            # Strategy 1: Filter by log level (errors and warnings first)
            error_logs = [l for l in logs if any(level in l.get("message", "").lower() 
                                                for level in ["error", "critical", "fatal"])]
            warning_logs = [l for l in logs if "warning" in l.get("message", "").lower() 
                           and l not in error_logs]
            info_logs = [l for l in logs if l not in error_logs and l not in warning_logs]
            
            # Combine with priority
            prioritized_logs = error_logs[:max_lines//2] + warning_logs[:max_lines//3] + info_logs[:max_lines//6]
            
            # Strategy 2: Group similar logs
            grouped_logs = self._group_similar_logs(prioritized_logs)
            
            reduced["logs"] = grouped_logs[:max_lines]
            reduced["metadata"]["reduced_log_count"] = len(reduced["logs"])
            
            # Extract patterns
            reduced["patterns"] = self._extract_log_patterns(logs)
        
        # Generate summary
        reduced["summary"] = self._generate_log_summary(reduced)
        
        return reduced
    
    def _filter_recent_metrics(self, metrics: Dict[str, Any], hours: int = 2) -> Dict[str, Any]:
        """Filter metrics to keep only recent data points."""
        cutoff_time = datetime.now() - timedelta(hours=hours)
        filtered = {}
        
        for metric_name, metric_data in metrics.items():
            if isinstance(metric_data, list):
                # Filter data points
                recent_points = []
                for point in metric_data:
                    if isinstance(point, dict) and "timestamp" in point:
                        try:
                            point_time = datetime.fromisoformat(point["timestamp"].replace("Z", "+00:00"))
                            if point_time > cutoff_time:
                                recent_points.append(point)
                        except:
                            recent_points.append(point)
                    else:
                        recent_points.append(point)
                
                if recent_points:
                    filtered[metric_name] = recent_points[-100:]  # Keep last 100 points max
            else:
                filtered[metric_name] = metric_data
        
        return filtered
    
    def _aggregate_metrics(self, metrics: Dict[str, Any]) -> Dict[str, Any]:
        """Aggregate similar metrics to reduce size."""
        aggregated = {}
        
        for metric_name, metric_data in metrics.items():
            if isinstance(metric_data, list) and len(metric_data) > 50:
                # Calculate statistics instead of keeping all points
                values = [p.get("value", 0) for p in metric_data if isinstance(p, dict)]
                if values:
                    aggregated[metric_name] = {
                        "count": len(values),
                        "min": min(values),
                        "max": max(values),
                        "avg": sum(values) / len(values),
                        "latest": metric_data[-1] if metric_data else None,
                        "first": metric_data[0] if metric_data else None
                    }
                else:
                    aggregated[metric_name] = metric_data
            else:
                aggregated[metric_name] = metric_data
        
        return aggregated
    
    def _strip_verbose_fields(self, data: Any) -> Any:
        """Remove verbose fields from data."""
        if isinstance(data, dict):
            # Fields to remove
            verbose_fields = {"__name__", "help", "unit", "labels", "metadata"}
            return {k: self._strip_verbose_fields(v) 
                   for k, v in data.items() 
                   if k not in verbose_fields}
        elif isinstance(data, list):
            return [self._strip_verbose_fields(item) for item in data]
        else:
            return data
    
    def _sample_metrics(self, metrics: Dict[str, Any], target_tokens: int) -> Dict[str, Any]:
        """Sample metrics to fit within token limit."""
        current_tokens = self.estimate_tokens(metrics)
        if current_tokens <= target_tokens:
            return metrics
        
        # Calculate sampling rate
        sample_rate = target_tokens / current_tokens
        sampled = {}
        
        for metric_name, metric_data in metrics.items():
            if isinstance(metric_data, list):
                # Sample data points
                step = max(1, int(1 / sample_rate))
                sampled[metric_name] = metric_data[::step]
            else:
                sampled[metric_name] = metric_data
        
        return sampled
    
    def _prioritize_alerts(self, alerts: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
        """Prioritize alerts by severity and recency."""
        # Define priority
        severity_priority = {"critical": 0, "error": 1, "warning": 2, "info": 3}
        
        # Sort alerts
        sorted_alerts = sorted(alerts, key=lambda x: (
            severity_priority.get(x.get("labels", {}).get("severity", "info"), 3),
            x.get("status", {}).get("state", "") != "active",  # Active alerts first
            x.get("startsAt", "")
        ))
        
        return sorted_alerts
    
    def _group_similar_logs(self, logs: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
        """Group similar log entries to reduce redundancy."""
        grouped = {}
        
        for log in logs:
            # Create a signature for the log (remove timestamps and specific values)
            message = log.get("message", "")
            # Remove numbers and timestamps to create pattern
            pattern = "".join([c if not c.isdigit() else "#" for c in message])
            pattern = pattern[:100]  # Use first 100 chars as pattern
            
            if pattern not in grouped:
                grouped[pattern] = {
                    "pattern": pattern,
                    "count": 0,
                    "examples": [],
                    "first_seen": log.get("timestamp", ""),
                    "last_seen": log.get("timestamp", "")
                }
            
            grouped[pattern]["count"] += 1
            if len(grouped[pattern]["examples"]) < 3:
                grouped[pattern]["examples"].append(log)
            grouped[pattern]["last_seen"] = log.get("timestamp", "")
        
        # Convert to list
        result = []
        for pattern_data in grouped.values():
            if pattern_data["count"] > 1:
                result.append({
                    "type": "grouped",
                    "count": pattern_data["count"],
                    "first_seen": pattern_data["first_seen"],
                    "last_seen": pattern_data["last_seen"],
                    "examples": pattern_data["examples"]
                })
            else:
                result.extend(pattern_data["examples"])
        
        return result
    
    def _extract_log_patterns(self, logs: List[Dict[str, Any]]) -> Dict[str, int]:
        """Extract common patterns from logs."""
        patterns = {
            "errors": 0,
            "warnings": 0,
            "info": 0,
            "debug": 0,
            "other": 0
        }
        
        for log in logs:
            message = log.get("message", "").lower()
            if any(level in message for level in ["error", "critical", "fatal"]):
                patterns["errors"] += 1
            elif "warning" in message:
                patterns["warnings"] += 1
            elif "info" in message:
                patterns["info"] += 1
            elif "debug" in message:
                patterns["debug"] += 1
            else:
                patterns["other"] += 1
        
        return patterns
    
    def _generate_summary(self, data: Dict[str, Any]) -> Dict[str, Any]:
        """Generate summary for prometheus data."""
        summary = {
            "metrics_count": data["metadata"]["reduced_metrics_count"],
            "alerts_count": len(data.get("alerts", [])),
            "critical_alerts": len([a for a in data.get("alerts", []) 
                                  if a.get("severity") == "critical"]),
            "has_anomalies": False  # Could be enhanced with anomaly detection
        }
        
        return summary
    
    def _generate_alert_summary(self, data: Dict[str, Any]) -> Dict[str, Any]:
        """Generate summary for alert data."""
        alerts = data.get("alerts", [])
        
        summary = {
            "total_alerts": len(alerts),
            "active_alerts": len([a for a in alerts if a.get("status") == "active"]),
            "by_severity": {},
            "most_recent": alerts[0] if alerts else None,
            "affected_instances": list(set(a.get("instance", "") for a in alerts if a.get("instance")))
        }
        
        # Count by severity
        for alert in alerts:
            severity = alert.get("severity", "unknown")
            summary["by_severity"][severity] = summary["by_severity"].get(severity, 0) + 1
        
        return summary
    
    def _generate_log_summary(self, data: Dict[str, Any]) -> Dict[str, Any]:
        """Generate summary for log data."""
        return {
            "total_logs": data["metadata"]["original_log_count"],
            "reduced_logs": data["metadata"]["reduced_log_count"],
            "patterns": data.get("patterns", {}),
            "time_range": self._get_time_range(data.get("logs", []))
        }
    
    def _get_time_range(self, logs: List[Dict[str, Any]]) -> Dict[str, str]:
        """Get time range from logs."""
        if not logs:
            return {"start": "", "end": ""}
        
        timestamps = [l.get("timestamp", "") for l in logs if l.get("timestamp")]
        if not timestamps:
            return {"start": "", "end": ""}
        
        return {
            "start": min(timestamps),
            "end": max(timestamps)
        }
    
    def _summarize_alert_groups(self, groups: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
        """Summarize alert groups."""
        summarized = []
        
        for group in groups[:10]:  # Keep top 10 groups
            summary = {
                "name": group.get("name", ""),
                "file": group.get("file", ""),
                "alerts_count": len(group.get("alerts", [])),
                "rules_count": len(group.get("rules", []))
            }
            
            # Add alert summaries
            alerts = group.get("alerts", [])
            if alerts:
                summary["alert_names"] = list(set(a.get("labels", {}).get("alertname", "") 
                                                for a in alerts[:5]))
            
            summarized.append(summary)
        
        return summarized


# Singleton instance
data_reducer = DataReducer(max_tokens=100000)


def reduce_data_for_context(data: Any, max_chars: int = 10000) -> Any:
    """
    Reduce data size to fit within character limits.
    
    Args:
        data: The data to reduce (dict, list, or string)
        max_chars: Maximum character limit
        
    Returns:
        Reduced data that fits within the character limit
    """
    from pydantic import BaseModel
    
    # Handle Pydantic models
    if isinstance(data, BaseModel):
        data = data.model_dump()
    
    # If it's already a string, truncate it
    if isinstance(data, str):
        if len(data) <= max_chars:
            return data
        return data[:max_chars] + "... (truncated)"
    
    # Convert to string to check size
    try:
        data_str = json.dumps(data, indent=2, default=str)
    except Exception:
        # If JSON serialization fails, convert to string
        data_str = str(data)
        
    if len(data_str) <= max_chars:
        return data
    
    # Use DataReducer for more intelligent reduction
    # Estimate tokens from characters (rough approximation)
    max_tokens = max_chars // 4
    reducer = DataReducer(max_tokens=max_tokens)
    
    # Try to intelligently reduce based on data type
    if isinstance(data, dict):
        # Check if it's monitoring data
        if "metrics" in data or "alerts" in data:
            return reducer.reduce_prometheus_data(data)
        elif "logs" in data:
            return reducer.reduce_loki_logs(data)
        elif any(key in data for key in ["alert_groups", "alerts"]):
            return reducer.reduce_alertmanager_data(data)
        else:
            # Generic reduction - truncate values
            reduced = {}
            for key, value in data.items():
                if isinstance(value, (dict, list)):
                    reduced[key] = reduce_data_for_context(value, max_chars // len(data))
                else:
                    reduced[key] = value
            return reduced
    
    elif isinstance(data, list):
        # Sample list items
        if not data:
            return data
        
        # Handle list of Pydantic models
        from pydantic import BaseModel
        processed_data = []
        for item in data:
            if isinstance(item, BaseModel):
                processed_data.append(item.model_dump())
            else:
                processed_data.append(item)
        data = processed_data
        
        # Calculate how many items we can keep
        try:
            sample_item_str = json.dumps(data[0], indent=2, default=str)
        except:
            sample_item_str = str(data[0])
        item_size = len(sample_item_str)
        max_items = max(1, max_chars // item_size)
        
        if len(data) <= max_items:
            return data
        
        # Return sampled items with metadata
        step = len(data) // max_items
        sampled = data[::step][:max_items]
        
        return {
            "sampled": True,
            "original_count": len(data),
            "sampled_count": len(sampled),
            "data": sampled
        }
    
    # Fallback - convert to string and truncate
    return str(data)[:max_chars] + "... (truncated)"