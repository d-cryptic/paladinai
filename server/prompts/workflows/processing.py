import json

def get_action_query_processing_prompt(workflow_type,user_input, serialized_raw_data) -> str:
  return f"""
  Extract and format this Prometheus metrics data for a {workflow_type} workflow.

                User Request: {user_input}
                Raw Metrics Data: {json.dumps(serialized_raw_data, indent=2)}

                CRITICAL: Extract the actual numerical values requested by the user. Do NOT provide recommendations.

                For {workflow_type} workflows, provide simple data extraction:
                - Extract the specific metrics requested by the user (CPU usage, memory usage, network latency)
                - Calculate basic statistics if needed (averages, maximums, minimums)
                - Format timestamps in IST (yyyy/mm/dd hh:mm:ss)
                - Include units for numerical values (%, GB, MB, ms)
                - Keep response focused on actual data values

                Respond with JSON containing:
                - processed_metrics: the specific metric values requested with units (e.g., {{"average_cpu_usage": "45.2%", "maximum_memory_usage": "8.4GB"}})
                - current_values: current/latest values with units
                - basic_statistics: simple calculations if relevant (averages, totals, maximums)
                - timestamp: when data was processed (IST format)
                - data_source: "Prometheus"
                - total_data_points: number of data points collected
  """

def get_incident_processing_prompt(user_input, serialized_raw_data, context)->str:
  return f"""
	Process and format this Prometheus metrics data for incident analysis.

                User Request: {user_input}
                Raw Metrics Data: {json.dumps(serialized_raw_data, indent=2)}
                Workflow Context: {json.dumps(context, indent=2)}

                For INCIDENT workflows, provide comprehensive analysis:
                - Extract key metrics and values
                - Calculate relevant statistics (averages, percentiles, trends)
                - Identify anomalies or notable patterns
                - Perform correlation analysis
                - Assess performance characteristics
                - Compare against baselines
                - Format timestamps in IST (yyyy/mm/dd hh:mm:ss)
                - Include units for all numerical values
                - Provide data source attribution

                Respond with JSON containing:
                - processed_metrics: cleaned and formatted metric values
                - key_insights: important findings from the data
                - data_summary: statistical summary of collected data
                - anomalies: any unusual patterns or outliers detected
                - correlation_analysis: relationships between metrics
                - performance_assessment: evaluation of system performance
                - comparative_analysis: comparison against baselines
                - data_quality: assessment of data completeness and reliability
                - timestamp: when data was processed (IST format)
                - total_data_points: number of data points collected
                - recommendations: actionable recommendations based on the data
	"""