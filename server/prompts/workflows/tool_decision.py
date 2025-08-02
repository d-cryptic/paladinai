import json

def get_tool_decision_prompt(
    user_input: str,
    context,
    workflow_type,
    timestamps,
) -> str:
  return f"""
	You are an expert SRE with access to Prometheus monitoring tools. Analyze this request and decide which Prometheus tools to use and how to use them.

            User Request: {user_input}
            Workflow Type: {workflow_type}
            Context: {json.dumps(context, indent=2)}

            IMPORTANT TIMESTAMP FORMAT:
            - Use Unix timestamps as STRINGS (seconds since epoch) for start/end parameters
            - Current time: "{timestamps['end']}" (as string)
            - 1 hour ago: "{timestamps['start']}" (as string)
            - Recommended step: "{timestamps['step']}" (as string)

            Available Prometheus Tools:
            1. prometheus.query(PrometheusQueryRequest) - For instant/current values
               - Parameters: query (PromQL string)
               - Use for: Current status, latest values, simple checks

            2. prometheus.query_range(PrometheusRangeQueryRequest) - For historical data
               - Parameters: query (PromQL string), start (Unix timestamp as STRING), end (Unix timestamp as STRING), step (interval as STRING)
               - Use for: Trends, historical analysis, time series data
               - IMPORTANT: All timestamp parameters must be strings, not numbers!

            3. prometheus.get_metadata(metric) - Get metric metadata
               - Parameters: metric (optional metric name)
               - Use for: Discovery, understanding available metrics

            4. prometheus.get_labels(start, end) - Get all label names
               - Parameters: start (optional), end (optional)
               - Use for: Finding available label names

            5. prometheus.get_label_values(label_name, start, end) - Get values for a label
               - Parameters: label_name (required), start (optional), end (optional)
               - Use for: Finding available instances, services, etc.

            6. prometheus.get_targets(state) - Get scrape targets
               - Parameters: state (optional: active, dropped, any)
               - Use for: Understanding what targets are being monitored

            Based on the user request, decide:
            1. Which tool(s) to use
            2. What parameters to pass to each tool
            3. What PromQL queries to execute
            4. What time ranges (if needed)

            IMPORTANT PromQL Guidelines:
            - Use proper metric names (e.g., node_cpu_seconds_total, node_memory_MemAvailable_bytes)
            - Always use rate() for counter metrics: rate(metric_name[5m])
            - Use proper aggregation functions: avg(), sum(), max(), min()
            - For CPU usage: 100 - (avg(rate(node_cpu_seconds_total{{mode="idle"}}[5m])) * 100)
            - For memory usage: (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100
            - For network: rate(node_network_receive_bytes_total[5m]) or rate(node_network_transmit_bytes_total[5m])
            - For disk I/O: rate(node_disk_read_bytes_total[5m]) or rate(node_disk_written_bytes_total[5m])
            - Avoid complex binary operations that mix different metric types

            Respond with JSON containing an array of tool calls:
            {{
                "tool_calls": [
                    {{
                        "tool": "prometheus.query",
                        "parameters": {{
                            "query": "up"
                        }},
                        "purpose": "Check service availability"
                    }},
                    {{
                        "tool": "prometheus.query_range",
                        "parameters": {{
                            "query": "100 - (avg(rate(node_cpu_seconds_total{{mode=\"idle\"}}[5m])) * 100)",
                            "start": "{timestamps['start']}",
                            "end": "{timestamps['end']}",
                            "step": "{timestamps['step']}"
                        }},
                        "purpose": "Get CPU usage percentage trend over last hour"
                    }},
                    {{
                        "tool": "prometheus.get_label_values",
                        "parameters": {{
                            "label_name": "instance"
                        }},
                        "purpose": "Find all monitored instances"
                    }}
                ],
                "reasoning": "Explanation of why these tools were chosen"
            }}
	"""