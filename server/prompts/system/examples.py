SYSTEM_PROMPT_EXAMPLES = [
	{
		"incident_id": "INC-001",
		"incident_type": "performance_degradation",
		"severity": "high",
		"component": "payment_service",
		"description": "High latency alert triggered on Payment Service with significant checkout delays",
		"chain_of_thought_analysis": {
			"step_1": {
				"phase": "data_gathering",
				"action": "Collect logs, metrics, deployment history, and DB query performance data",
				"observation": "P99 latency spike from 200ms to 3.2s coincides with recent increase in database connection pool usage",
				"confidence": 90
			},
			"step_2": {
				"phase": "hypothesis_formation",
				"action": "Hypothesize potential missing index or query performance degradation in database",
				"observation": "Timing of alert closely aligns with change in query patterns",
				"confidence": 85
			},
			"step_3": {
				"phase": "validation_of_hypothesis",
				"action": "Examine database logs and slow query logs, cross-reference with application metrics",
				"observation": "Evidence shows spike in slow queries and increased DB load, supporting index hypothesis",
				"confidence": 80
			},
			"step_4": {
				"phase": "impact_assessment",
				"action": "Quantify business impact through checkout conversion metrics and revenue loss estimation",
				"observation": "Approximately 15% drop in conversion and estimated revenue impact of $2.3K/min",
				"confidence": 80
			},
			"step_5": {
				"phase": "final_recommendation",
				"action": "Recommend adding missing composite index, scaling database read replicas, and optimizing query patterns",
				"confidence": 80
			}
		},
		"consolidated_output": {
			"severity_emoji": "🔴",
			"severity_level": "HIGH",
			"component": "Payment Service",
			"summary": "Latency spike causing checkout failures",
			"evidence": "P99 latency increased from 200ms to 3.2s; DB connection pool at 95%; slow query log spike",
			"overall_confidence": 80,
			"step_by_step_confidence": [90, 85, 80, 80, 80],
			"impact": "~15% conversion drop, $2.3K/min revenue impact",
			"immediate_actions": [
				"Add the identified missing index",
				"Scale DB read replicas", 
				"Optimize query performance and monitor metrics"
			]
		}
	},
	{
		"incident_id": "INC-002",
		"incident_type": "resource_exhaustion",
		"severity": "medium",
		"component": "node_17",
		"description": "Memory usage spike on node-17 potentially affecting service performance",
		"chain_of_thought_analysis": {
			"step_1": {
				"phase": "data_gathering",
				"action": "Inspect node-17's memory metrics and logs for timeframe of spike",
				"observation": "Sudden memory increase noted alongside log entries indicating unusually high query volumes",
				"confidence": 95
			},
			"step_2": {
				"phase": "hypothesis_formation", 
				"action": "Hypothesize unbounded query or memory leak in recently updated service",
				"observation": "Recent deployments include modifications to reporting service that could generate unbounded queries",
				"confidence": 90
			},
			"step_3": {
				"phase": "validation_of_hypothesis",
				"action": "Cross-reference container logs, application error logs, and memory profiling data",
				"observation": "Log analysis confirms recurring unbounded query patterns, memory profiling shows gradual accumulation without proper garbage collection",
				"confidence": 85
			},
			"step_4": {
				"phase": "impact_assessment",
				"action": "Evaluate potential service disruptions due to high memory usage",
				"observation": "Node-17's performance degradation correlates with intermittent service timeouts",
				"confidence": 80
			},
			"step_5": {
				"phase": "final_recommendation",
				"action": "Suggest code review for reporting service, optimization of unbounded queries, and resource allocation adjustments",
				"confidence": 80
			}
		},
		"consolidated_output": {
			"severity_emoji": "🟠",
			"severity_level": "MEDIUM",
			"component": "Node-17",
			"summary": "Memory spike detected, potential unbounded query issue in reporting service",
			"evidence": "Sudden memory increase on node-17 with correlated log entries of high query volumes; memory profiling indicates accumulation patterns",
			"overall_confidence": 83,
			"step_by_step_confidence": [95, 90, 85, 80, 80],
			"impact": "Possible intermittent service timeouts and degraded performance on node-17",
			"immediate_actions": [
				"Initiate a code review of the reporting service",
				"Optimize any unbounded queries",
				"Adjust resource allocations and garbage collection settings if necessary"
			]
		}
	},
	{
		"incident_id": "INC-003",
		"incident_type": "resource_exhaustion",
		"severity": "critical",
		"component": "database_connection_pool",
		"description": "Multiple services reporting database connection timeouts during peak traffic hours",
		"affected_services": ["user_service", "order_service", "inventory_service"],
		"chain_of_thought_analysis": {
			"step_1": {
				"phase": "initial_alert_assessment",
				"action": "Analyze alert pattern and affected services",
				"observation": "Connection timeout errors across 3 microservices started at 14:22 UTC. All affected services use same database cluster",
				"pattern_recognition": "Clear temporal correlation",
				"confidence": 95
			},
			"step_2": {
				"phase": "database_health_investigation",
				"action": "Examine database connection metrics and active connection counts",
				"observation": "Connection pool utilization at 98%, max connections = 100, active connections = 97. No unusual slow queries detected",
				"confidence": 90
			},
			"step_3": {
				"phase": "traffic_pattern_analysis", 
				"action": "Compare current traffic to historical patterns and recent changes",
				"observation": "Traffic increased 40% above normal peak due to flash sale campaign launched at 14:15 UTC",
				"timeline_correlation": "Marketing campaign timing correlates with connection exhaustion",
				"confidence": 85
			},
			"step_4": {
				"phase": "application_configuration_review",
				"action": "Review connection pool configurations across affected services",
				"observation": "Each service configured with max 35 connections, 3 services = 105 total demand",
				"math_validation": "35 × 3 = 105 connections needed > 100 available connections",
				"confidence": 95
			},
			"step_5": {
				"phase": "root_cause_synthesis",
				"action": "Combine evidence to establish definitive root cause",
				"conclusion": "Flash sale traffic spike exceeded database connection capacity",
				"contributing_factors": "Under-provisioned connection pool for peak traffic scenarios",
				"confidence": 90
			},
			"step_6": {
				"phase": "impact_assessment",
				"action": "Quantify business and technical impact",
				"user_impact": "15% of checkout attempts failing, 25% increase in page load times",
				"revenue_impact": "$180/minute in failed transactions during 18-minute incident",
				"sla_impact": "Availability SLA breached (99.9% → 99.7% for the hour)",
				"confidence": 85
			}
		},
		"consolidated_output": {
			"severity_emoji": "🔴",
			"severity_level": "SEV-1",
			"component": "Database Connection Pool",
			"summary": "Multiple services experiencing DB timeouts due to connection exhaustion",
			"evidence_chain": [
				"Connection pool at 98% utilization (95% confidence)",
				"Flash sale traffic +40% above normal (85% confidence)",
				"Pool config: 3×35=105 needed > 100 available (95% confidence)"
			],
			"overall_confidence": 90,
			"impact": "15% checkout failures, $180/min revenue loss, SLA breach",
			"immediate_actions": [
				"Scale DB connection pool to 150 connections (ETA: 5 min)",
				"Enable connection pool overflow handling",
				"Implement traffic shaping for flash sales"
			],
			"follow_up_actions": [
				"Review capacity planning for marketing campaigns",
				"Implement dynamic connection pool scaling",
				"Add connection pool monitoring alerts"
			]
		}
	},
	{
		"incident_id": "INC-004",
		"incident_type": "container_orchestration",
		"severity": "critical",
		"component": "auth_service",
		"description": "Critical authentication service pods entering CrashLoopBackOff state after deployment",
		"chain_of_thought_analysis": {
			"step_1": {
				"phase": "kubernetes_cluster_investigation",
				"action": "Examine pod status, events, and resource allocation",
				"observation": "3/3 auth-service pods in CrashLoopBackOff, restart count increasing. CPU/Memory requests within limits, no resource pressure",
				"events": "Back-off restarting failed container every 30 seconds",
				"confidence": 95
			},
			"step_2": {
				"phase": "container_log_analysis",
				"action": "Extract and analyze container logs from failed pods",
				"observation": "Application startup fails with 'Connection refused' to Redis at 10.0.1.15:6379. Same error across all pod instances",
				"startup_sequence": "Application attempts Redis connection before other services",
				"confidence": 90
			},
			"step_3": {
				"phase": "deployment_change_analysis",
				"action": "Compare current deployment with previous successful version",
				"observation": "Recent deployment changed Redis endpoint from service name to IP address",
				"configuration_diff": "redis.host changed from 'redis-service' to '10.0.1.15'",
				"dns_resolution": "redis-service resolves correctly, but IP 10.0.1.15 is unreachable",
				"confidence": 85
			},
			"step_4": {
				"phase": "network_connectivity_validation",
				"action": "Test network connectivity to Redis from affected pods",
				"network_test": "kubectl exec ping to 10.0.1.15 times out",
				"service_discovery": "redis-service DNS resolves to 10.0.1.20 (different IP)",
				"network_policy": "No network policy changes detected",
				"confidence": 88
			},
			"step_5": {
				"phase": "root_cause_determination",
				"action": "Synthesize evidence to identify root cause",
				"conclusion": "Hardcoded IP address in deployment points to wrong Redis instance",
				"why_it_happened": "Configuration management error during deployment preparation",
				"why_not_caught": "Staging environment uses different IP ranges",
				"confidence": 90
			},
			"step_6": {
				"phase": "blast_radius_assessment",
				"action": "Determine scope of impact across all services",
				"service_impact": "Authentication service down affects 12 downstream services",
				"user_impact": "All user logins failing, API authentication blocked",
				"time_impact": "Issue ongoing for 8 minutes since deployment",
				"confidence": 95
			}
		},
		"consolidated_output": {
			"severity_emoji": "🔴",
			"severity_level": "SEV-1",
			"component": "Auth Service",
			"summary": "CrashLoopBackOff due to hardcoded incorrect Redis IP",
			"evidence_chain": [
				"All 3 pods failing with Redis connection refused (95% confidence)",
				"Deployment changed redis.host to incorrect IP 10.0.1.15 (85% confidence)",
				"Correct Redis IP is 10.0.1.20 via service discovery (88% confidence)"
			],
			"overall_confidence": 90,
			"impact": "Complete authentication failure, 12 services affected, all user logins down",
			"immediate_actions": [
				"Rollback deployment to previous version (ETA: 2 min)",
				"Fix Redis configuration to use service name",
				"Redeploy with correct configuration"
			],
			"follow_up_actions": [
				"Add connectivity tests to deployment pipeline",
				"Align staging/production network configurations",
				"Implement configuration validation checks"
			]
		}
	},
	{
		"incident_id": "INC-005",
		"incident_type": "cascade_failure",
		"severity": "critical",
		"component": "multiple_services",
		"description": "Progressive service degradation starting with user-profile service and spreading to other components",
		"chain_of_thought_analysis": {
			"step_1": {
				"phase": "failure_pattern_recognition",
				"action": "Map timeline and sequence of service failures",
				"observation": "user-profile → recommendations → feed → notifications (5-minute intervals)",
				"dependency_analysis": "Services fail in order of their dependency chain",
				"load_pattern": "Each failure increases load on upstream services",
				"confidence": 92
			},
			"step_2": {
				"phase": "origin_service_deep_dive",
				"action": "Focus investigation on user-profile service as patient zero",
				"observation": "Memory usage spiked to 95% at 15:30 UTC before first failure",
				"error_logs": "OutOfMemoryError and frequent garbage collection warnings",
				"request_pattern": "Normal request volume but response times degraded 10x",
				"confidence": 88
			},
			"step_3": {
				"phase": "memory_leak_investigation",
				"action": "Analyze heap dumps and memory allocation patterns",
				"observation": "UserSession objects accumulating without cleanup",
				"code_analysis": "Recent commit removed session cleanup in finally block",
				"memory_growth": "Linear growth pattern over 3 hours since deployment",
				"confidence": 85
			},
			"step_4": {
				"phase": "circuit_breaker_analysis",
				"action": "Examine circuit breaker states across service mesh",
				"observation": "Recommendation service circuit breakers tripping due to user-profile timeouts",
				"retry_logic": "Exponential backoff causing thundering herd effect",
				"cascade_mechanism": "Each service timeout triggers upstream retries",
				"confidence": 80
			},
			"step_5": {
				"phase": "load_balancer_behavior",
				"action": "Analyze load balancer health checks and traffic distribution",
				"observation": "Unhealthy user-profile pods removed from rotation",
				"traffic_shift": "Remaining healthy pods overwhelmed by redistributed traffic",
				"health_check": "Pods failing health checks due to high response times",
				"confidence": 82
			},
			"step_6": {
				"phase": "business_impact_calculation",
				"action": "Quantify full scope of cascade failure",
				"user_experience": "Complete feature unavailability for personalized content",
				"revenue_impact": "$450/minute loss due to reduced engagement",
				"service_availability": "4 services below SLA thresholds",
				"confidence": 75
			}
		},
		"consolidated_output": {
			"severity_emoji": "🔴",
			"severity_level": "SEV-1",
			"component": "Multi-Service Cascade",
			"summary": "Memory leak in user-profile triggered cascade failure",
			"evidence_chain": [
				"user-profile memory spike → OOM errors (88% confidence)",
				"Removed session cleanup causing leak (85% confidence)",
				"Circuit breakers tripping in dependency order (80% confidence)",
				"Load balancer removing unhealthy pods (82% confidence)"
			],
			"overall_confidence": 84,
			"impact": "4 services degraded, personalization features down, $450/min revenue impact",
			"immediate_actions": [
				"Rollback user-profile deployment (ETA: 3 min)",
				"Reset circuit breakers across service mesh",
				"Scale user-profile pods horizontally as backup"
			],
			"follow_up_actions": [
				"Implement memory monitoring alerts",
				"Add chaos engineering tests for cascade scenarios",
				"Review circuit breaker timeout configurations",
				"Strengthen integration testing for session management"
			]
		}
	},
	{
		"incident_id": "INC-006",
		"incident_type": "security_incident",
		"severity": "high",
		"component": "api_gateway",
		"description": "Anomalous traffic patterns detected with potential security implications",
		"chain_of_thought_analysis": {
			"step_1": {
				"phase": "traffic_anomaly_detection",
				"action": "Analyze traffic patterns and request characteristics",
				"observation": "300% increase in API requests from specific IP ranges",
				"geographic_pattern": "95% of anomalous traffic from previously unseen IP blocks",
				"request_pattern": "High volume of user enumeration and data scraping attempts",
				"confidence": 93
			},
			"step_2": {
				"phase": "attack_vector_analysis",
				"action": "Examine request payloads and authentication patterns",
				"observation": "Rapid-fire requests to /api/users/{id} endpoint with sequential IDs",
				"authentication": "Valid but suspicious API keys from recently created accounts",
				"rate_limiting": "Current limits (1000 req/min) being bypassed by IP rotation",
				"confidence": 88
			},
			"step_3": {
				"phase": "account_creation_pattern_investigation",
				"action": "Analyze recent account creation patterns and validation",
				"observation": "150 new accounts created in last 24 hours vs normal 10-15/day",
				"email_patterns": "Disposable email services and similar naming conventions",
				"verification_bypass": "Email verification completed unusually quickly",
				"confidence": 85
			},
			"step_4": {
				"phase": "data_exposure_risk_assessment",
				"action": "Determine what data is being accessed and potential impact",
				"observation": "User profile data including emails and activity patterns being harvested",
				"pii_exposure": "Names, email addresses, and usage patterns accessible",
				"compliance_impact": "Potential GDPR violation due to unauthorized data access",
				"confidence": 90
			},
			"step_5": {
				"phase": "infrastructure_impact_analysis",
				"action": "Assess performance impact on legitimate users",
				"observation": "API response times increased 40% due to excessive load",
				"database_load": "Read replica CPU at 85% vs normal 30%",
				"user_complaints": "12% increase in support tickets about slow performance",
				"confidence": 87
			},
			"step_6": {
				"phase": "attack_attribution_and_scope",
				"action": "Determine attack sophistication and potential threat level",
				"observation": "Coordinated, automated attack using multiple techniques",
				"ttps_analysis": "Matches known data harvesting campaign patterns",
				"scale_assessment": "Professional-level operation, not script kiddie",
				"confidence": 75
			}
		},
		"consolidated_output": {
			"severity_emoji": "🔴",
			"severity_level": "SEV-2",
			"component": "Security Incident",
			"summary": "Coordinated data scraping attack via compromised API keys",
			"evidence_chain": [
				"300% traffic spike from suspicious IP ranges (93% confidence)",
				"Sequential user ID enumeration pattern (88% confidence)",
				"150 fake accounts created in 24h vs normal 15 (85% confidence)",
				"PII data being harvested at scale (90% confidence)"
			],
			"overall_confidence": 89,
			"impact": "User data exposure, 40% API performance degradation, potential GDPR violation",
			"immediate_actions": [
				"Block suspicious IP ranges at WAF level",
				"Suspend flagged API keys and accounts",
				"Implement emergency rate limiting on user endpoints",
				"Enable enhanced logging for forensics"
			],
			"follow_up_actions": [
				"Strengthen account creation verification",
				"Implement behavioral analysis for API usage",
				"Review and enhance PII access controls",
				"Prepare incident disclosure per compliance requirements",
				"Conduct full security audit of user data access patterns"
			]
		}
	},
	{
		"incident_id": "INC-007",
		"incident_type": "performance_regression",
		"severity": "medium",
		"component": "application_stack",
		"description": "Gradual performance degradation noticed 2 hours after successful deployment",
		"chain_of_thought_analysis": {
			"step_1": {
				"phase": "performance_baseline_comparison",
				"action": "Compare current performance metrics with pre-deployment baseline",
				"observation": "P95 latency increased from 150ms to 400ms over 2 hours",
				"trend_analysis": "Gradual degradation, not immediate spike after deployment",
				"error_rate": "No increase in 4xx/5xx errors, just slower responses",
				"confidence": 92
			},
			"step_2": {
				"phase": "deployment_change_analysis",
				"action": "Review all changes included in recent deployment",
				"code_changes": "New caching layer implementation and query optimization",
				"configuration": "Database connection pool size increased from 50 to 75",
				"dependencies": "Updated ORM library from v2.1 to v2.3",
				"infrastructure": "No infrastructure changes in deployment",
				"confidence": 85
			},
			"step_3": {
				"phase": "caching_layer_investigation",
				"action": "Analyze new caching implementation performance and behavior",
				"cache_hit_rate": "Only 35% vs expected 80% based on testing",
				"cache_warming": "Cache appears to be cold, still populating",
				"cache_eviction": "Frequent evictions due to memory pressure in cache cluster",
				"cache_miss_latency": "Cache misses taking 200ms vs 50ms for direct DB",
				"confidence": 78
			},
			"step_4": {
				"phase": "database_query_performance_analysis",
				"action": "Examine database performance and new query patterns",
				"query_execution": "New optimized queries running 30% faster individually",
				"connection_pool": "Increased pool size showing better utilization",
				"lock_contention": "Slight increase in row-level locks but within normal range",
				"index_usage": "All queries using appropriate indexes",
				"confidence": 70
			},
			"step_5": {
				"phase": "orm_library_impact_investigation",
				"action": "Analyze impact of ORM library upgrade on query generation",
				"query_generation": "New ORM version generating more complex join queries",
				"n_plus_one_problem": "Previously optimized queries now showing N+1 pattern",
				"memory_usage": "ORM objects consuming 25% more memory per instance",
				"gc_pressure": "Increased garbage collection frequency due to ORM object overhead",
				"confidence": 82
			},
			"step_6": {
				"phase": "root_cause_synthesis",
				"action": "Combine evidence to determine primary performance impact",
				"primary_cause": "ORM upgrade introducing performance regressions",
				"secondary_cause": "Cache layer not yet warmed up, adding latency to misses",
				"interaction_effect": "ORM generating more queries + cold cache = compounded latency",
				"confidence": 80
			}
		},
		"consolidated_output": {
			"severity_emoji": "🟡",
			"severity_level": "SEV-3",
			"component": "Performance Regression",
			"summary": "ORM upgrade + cold cache causing gradual latency increase",
			"evidence_chain": [
				"P95 latency increased 150ms→400ms over 2 hours (92% confidence)",
				"Cache hit rate only 35% vs expected 80% (78% confidence)",
				"New ORM generating N+1 queries (82% confidence)",
				"ORM memory overhead increasing GC pressure (82% confidence)"
			],
			"overall_confidence": 80,
			"impact": "2.5x latency increase, user experience degradation, no functional failures",
			"immediate_actions": [
				"Warm cache proactively with common query patterns",
				"Monitor ORM query generation patterns",
				"Consider temporary rollback if degradation continues"
			],
			"investigation_actions": [
				"Profile ORM query generation in staging environment",
				"Optimize cache warming strategy",
				"Review ORM upgrade documentation for breaking changes",
				"Consider gradual rollout strategy for ORM changes"
			],
			"long_term_actions": [
				"Implement performance regression testing in CI/CD",
				"Add cache hit rate monitoring and alerting",
				"Create ORM query analysis tooling"
			]
		}
	},
	{
		"incident_id": "INC-008",
		"incident_type": "resource_exhaustion",
		"severity": "critical",
		"component": "storage_system",
		"description": "Application pods failing with write errors and disk space alerts",
		"chain_of_thought_analysis": {
			"step_1": {
				"phase": "storage_metrics_analysis",
				"action": "Examine disk usage across all nodes and persistent volumes",
				"observation": "Node storage at 95% capacity, persistent volumes at 90%",
				"growth_rate": "Disk usage increased 20GB in last 4 hours vs normal 2GB/day",
				"pattern": "Exponential growth pattern rather than linear",
				"confidence": 96
			},
			"step_2": {
				"phase": "log_volume_investigation",
				"action": "Analyze log generation patterns and retention policies",
				"observation": "Application logs consuming 15GB of the 20GB growth",
				"log_level": "DEBUG logging accidentally enabled in production",
				"rotation": "Log rotation not working properly, files not being compressed",
				"retention": "Old logs not being cleaned up due to permissions issue",
				"confidence": 88
			},
			"step_3": {
				"phase": "database_storage_analysis",
				"action": "Examine database storage growth and maintenance operations",
				"observation": "Database files grew 3GB, temp tablespace at 85% capacity",
				"maintenance": "Weekly vacuum operations failing due to lock timeouts",
				"bloat": "Table bloat analysis shows 40% wasted space in main tables",
				"confidence": 75
			},
			"step_4": {
				"phase": "application_data_growth",
				"action": "Investigate application-generated data and file uploads",
				"observation": "User-uploaded files grew 2GB, within expected range",
				"cache_files": "Local cache files accumulated due to cleanup job failure",
				"temporary_files": "Application creating temp files but not cleaning up",
				"confidence": 70
			},
			"step_5": {
				"phase": "configuration_drift_analysis",
				"action": "Compare current configuration with baseline infrastructure setup",
				"observation": "DEBUG logging enabled by recent config change",
				"change_tracking": "Configuration change deployed 6 hours ago",
				"rollback_history": "Previous config had INFO level logging",
				"deployment": "Change deployed without proper staging validation",
				"confidence": 85
			},
			"step_6": {
				"phase": "immediate_risk_assessment",
				"action": "Determine how quickly system will fail completely",
				"projection": "At current growth rate, complete failure in 2 hours",
				"critical_services": "Database writes will fail first, causing data loss risk",
				"recovery_time": "Storage cleanup will take 30-45 minutes",
				"confidence": 90
			}
		},
		"consolidated_output": {
			"severity_emoji": "🔴",
			"severity_level": "SEV-2",
			"component": "Disk Space Exhaustion",
			"summary": "DEBUG logging enabled causing rapid storage consumption",
			"evidence_chain": [
				"Node storage at 95%, growing 20GB in 4 hours (96% confidence)",
				"DEBUG logging consuming 15GB of growth (88% confidence)",
				"Log rotation and cleanup failing (88% confidence)",
				"Config change 6 hours ago enabled debug mode (85% confidence)"
			],
			"overall_confidence": 89,
			"impact": "System failure imminent in ~2 hours, write operations at risk",
			"immediate_actions": [
				"Disable DEBUG logging immediately (ETA: 1 min)",
				"Manually compress and rotate large log files",
				"Clean up temporary application files",
				"Emergency cleanup of old database maintenance files"
			],
			"follow_up_actions": [
				"Fix log rotation configuration and permissions",
				"Implement disk space monitoring and alerting",
				"Add configuration change validation in deployment pipeline",
				"Schedule database maintenance to reclaim bloated space",
				"Review and implement proper log retention policies"
			]
		}
	},
  
]