import json

def get_response_type_prompt(user_input, categorization_data) -> str:
  return f"""
	Based on the user's request and categorization, determine the appropriate response type.

            User Request: {user_input}
            Categorization: {json.dumps(categorization_data, indent=2)}

            Response Types:
            1. "simple_data" - User wants specific metrics, values, or direct answers
               - Examples: "what's the CPU usage?", "maximum memory usage", "current disk space"
               - Response should be direct and minimal

            2. "comprehensive_analysis" - User wants detailed analysis, insights, or reports
               - Examples: "analyze performance trends", "generate a report", "detailed analysis"
               - Response should include analysis, trends, recommendations

            3. "reporting" - User specifically wants a formatted report
               - Examples: "create a report", "generate summary", "performance report"
               - Response should be structured as a formal report

            Based on the user's request and categorization, what type of response is most appropriate?

            Respond with JSON containing:
            - response_type: "simple_data", "comprehensive_analysis", or "reporting"
            - reasoning: explanation for the choice
  """