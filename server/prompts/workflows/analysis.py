def get_incident_analysis_prompt(user_input) -> str:
  return f"""
	Analyze this incident report and determine the investigation requirements.

            Incident Description: {user_input}

            Incident types that typically require comprehensive metrics data:
            - Performance degradation (slow response times, high latency)
            - Service outages and availability issues
            - Resource exhaustion (CPU, memory, disk, network)
            - Error rate increases and failure patterns
            - Capacity and scaling issues
            - Infrastructure failures
            - Data pipeline and processing failures

            Incident types that might require minimal metrics:
            - Simple configuration questions
            - Documentation requests
            - General troubleshooting guidance
            - Procedural inquiries

            Analyze the incident and determine:
            1. What type of incident is being reported
            2. What severity level this appears to be
            3. Whether comprehensive metrics data is needed
            4. What specific investigation areas to focus on
            5. What time range might be relevant

            Respond with JSON containing:
            - needs_metrics: boolean (true if metrics data is required)
            - incident_type: string describing the type of incident
            - severity: "critical", "high", "medium", or "low"
            - investigation_focus: list of key areas to investigate
            - time_range_suggestion: suggested time range for investigation
            - urgency: "immediate", "high", "normal", or "low"
            - reasoning: explanation for the analysis
	"""

def get_query_analysis_prompt(user_input) -> str:
  return f"""
  Analyze this user query and determine if it requires metrics data from monitoring systems like Prometheus.

            User Query: {user_input}

            Metrics-requiring queries typically ask about:
            - CPU, memory, disk, network usage/performance
            - Service health, uptime, response times
            - Error rates, request counts, throughput
            - Resource utilization, capacity planning
            - Performance metrics, latency measurements
            - System load, queue depths, connection counts

            Non-metrics queries typically ask about:
            - General information or explanations
            - Configuration questions
            - Procedural or how-to questions
            - Theoretical concepts
            - Simple greetings or conversational queries

            Respond with JSON containing:
            - needs_metrics: boolean (true if metrics data is required)
            - reasoning: explanation for the decision
            - metric_types: list of metric types that might be needed (if any)
  """
  
def get_action_analysis_prompt(user_input) -> str:
	return f"""
	Analyze this user action request and determine the requirements for fulfilling it.

	User Request: {user_input}

							Action types that typically require metrics data:
							- Performance analysis and reporting
							- Data retrieval from monitoring systems
							- Trend analysis and historical data requests
							- System health assessments
							- Capacity planning and resource analysis
							- Alert analysis and incident investigation
							- Comparative analysis between time periods

							Action types that typically don't require metrics:
							- Configuration explanations
							- Procedural documentation
							- General information requests
							- Theoretical explanations
							- Simple administrative tasks

							Analyze the request and determine:
							1. What type of action is being requested
							2. Whether metrics data is needed
							3. What specific data requirements exist
							4. What time range might be appropriate
							5. What level of analysis is needed

							Respond with JSON containing:
							- needs_metrics: boolean (true if metrics data is required)
							- action_type: string describing the type of action
							- data_requirements: object with specific data needs
							- time_range_suggestion: suggested time range for data collection
							- analysis_level: "basic", "intermediate", or "comprehensive"
							- reasoning: explanation for the analysis
	"""