OUTPUT_JSON_SCHEMA = {
  "incident_id": "string - Unique incident identifier",
  "incident_type": "string - Category of incident (e.g., performance_degradation, security_incident, resource_exhaustion)",
  "severity": "string - critical|high|medium|low",
  "component": "string - Primary affected component/service",
  "description": "string - Brief description of the incident",
  "affected_services": ["array - List of impacted services (optional)"],
  "timeline": {
    "start_time": "string - ISO timestamp when incident started",
    "detection_time": "string - ISO timestamp when incident was detected",
    "investigation_start": "string - ISO timestamp when investigation began"
  },
  "chain_of_thought_analysis": {
    "step_1": {
      "phase": "string - Investigation phase name",
      "action": "string - Action taken in this step",
      "observation": "string - What was observed/discovered",
      "metrics": {},
      "logs": [],
      "additional_details": "string - Any extra context",
      "confidence": 0
    },
    "step_2": {
      "phase": "string",
      "action": "string",
      "observation": "string", 
      "metrics": {},
      "logs": [],
      "additional_details": "string",
      "confidence": 0
    },
    "step_3": {
      "phase": "string",
      "action": "string",
      "observation": "string",
      "metrics": {},
      "logs": [],
      "additional_details": "string", 
      "confidence": 0
    },
    "step_4": {
      "phase": "string",
      "action": "string",
      "observation": "string",
      "metrics": {},
      "logs": [],
      "additional_details": "string",
      "confidence": 0
    },
    "step_5": {
      "phase": "string",
      "action": "string",
      "observation": "string",
      "conclusion": "string - Root cause conclusion",
      "contributing_factors": "string",
      "additional_details": "string",
      "confidence": 0
    },
    "step_6": {
      "phase": "string",
      "action": "string",
      "observation": "string",
      "business_impact": "string",
      "technical_impact": "string",
      "additional_details": "string",
      "confidence": 0
    }
  },
  "consolidated_output": {
    "severity_emoji": "string - Visual severity indicator",
    "severity_level": "string - SEV-1|SEV-2|SEV-3|SEV-4 or HIGH|MEDIUM|LOW",
    "component": "string - Primary component name",
    "summary": "string - Brief incident summary",
    "evidence_chain": [
      "array of strings - Key evidence with confidence scores"
    ],
    "overall_confidence": 0,
    "step_by_step_confidence": [0, 0, 0, 0, 0, 0],
    "impact": "string - Overall impact description",
    "immediate_actions": [
      "array of strings - Actions to take immediately"
    ],
    "follow_up_actions": [
      "array of strings - Preventive measures and improvements"
    ],
    "investigation_duration": "string - Time taken to reach conclusion",
    "estimated_resolution_time": "string - ETA for full resolution"
  },
  "metadata": {
    "investigated_by": "string - SRE agent or human investigator",
    "investigation_tools_used": [],
    "related_incidents": [],
    "post_mortem_required": "false",
    "lessons_learned": []
  }
}