import json

def get_planning_prompt(user_input, workflow_type, context, timestamps) -> str:
  return f"""
	Generate specific PromQL queries to collect metrics data for this monitoring request.

            User Request: {user_input}
            Workflow Type: {workflow_type}
            Context: {json.dumps(context, indent=2)}

            IMPORTANT: Use proper Unix timestamps for time ranges:
            - Current time (end): {timestamps['end']}
            - 1 hour ago (start): {timestamps['start']}
            - Step interval: {timestamps['step']}

            You are an expert in PromQL and Prometheus metrics. Generate ready-to-execute PromQL queries based on the user's request.

            IMPORTANT PromQL Guidelines:
            - Use proper metric names (e.g., node_cpu_seconds_total, node_memory_MemAvailable_bytes)
            - Always use rate() for counter metrics: rate(metric_name[5m])
            - Use proper aggregation functions: avg(), sum(), max(), min()
            - For CPU usage: 100 - (avg(rate(node_cpu_seconds_total{{mode="idle"}}[5m])) * 100)
            - For memory usage: (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100
            - For network: rate(node_network_receive_bytes_total[5m]) or rate(node_network_transmit_bytes_total[5m])
            - For disk I/O: rate(node_disk_read_bytes_total[5m]) or rate(node_disk_written_bytes_total[5m])
            - Avoid complex binary operations that mix different metric types

            Common metric patterns and their PromQL queries:
            - CPU Usage: rate(node_cpu_seconds_total{{mode!="idle"}}[5m]) * 100
            - Memory Usage: (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100
            - Disk Usage: (1 - (node_filesystem_avail_bytes / node_filesystem_size_bytes)) * 100
            - Network Traffic: rate(node_network_receive_bytes_total[5m]), rate(node_network_transmit_bytes_total[5m])
            - HTTP Requests: rate(http_requests_total[5m])
            - HTTP Response Time: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
            - Service Uptime: up
            - Load Average: node_load1, node_load5, node_load15

            Generate actual PromQL queries that can be executed directly. For each query, specify:
            1. The exact PromQL query string
            2. Whether it's an "instant" or "range" query
            3. Time range for range queries (start, end, step)
            4. A description of what the query measures

            Respond with JSON containing:
            - queries: list of ready-to-execute PromQL query strings
            - query_types: list of "instant" or "range" for each query
            - time_ranges: time range specifications for range queries (format: {{"start": "now-1h", "end": "now", "step": "1m"}})
            - query_descriptions: description of what each query measures
            - collection_strategy: "comprehensive", "targeted", or "minimal"
            - reasoning: explanation for the query selection
  """