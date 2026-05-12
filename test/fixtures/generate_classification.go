//go:build ignore

// generate_classification.go generates classification eval fixtures.
// Run with: go run test/fixtures/generate_classification.go
// Output: test/fixtures/classification_smoke.jsonl (500 cases)
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type alert struct {
	Title       string            `json:"title"`
	Severity    string            `json:"severity"`
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Description string            `json:"description"`
}

type testCase struct {
	ID               string `json:"id"`
	Category         string `json:"category"`
	Description      string `json:"description"`
	Alert            alert  `json:"alert"`
	ExpectedSeverity string `json:"expected_severity"`
	ExpectedIntent   string `json:"expected_intent"`
}

type template struct {
	title       string
	description string
	intent      string
	severity    string
	namespace   string
	job         string
	service     string
	annotations map[string]string
}

var templates = []template{
	// ── service_down ──────────────────────────────────────────────────────────
	{"PostgreSQL Replication Lag Critical", "Primary DB replication lag exceeded 30s", "service_down", "P1", "prod", "postgres", "database", map[string]string{"description": "Replication lag at 45s, threshold 30s"}},
	{"MySQL Connection Pool Exhausted", "No connections available to MySQL", "service_down", "P1", "prod", "mysql", "orders", map[string]string{"description": "Connection pool 100/100 in use for >5m"}},
	{"Redis Cluster Primary Down", "Redis primary node unreachable", "service_down", "P1", "prod", "redis", "cache", map[string]string{"description": "Primary shard 0 not responding to PING"}},
	{"Kafka Broker Down", "Kafka broker offline", "service_down", "P1", "prod", "kafka", "messaging", map[string]string{"description": "Broker kafka-0 not in ISR for all partitions"}},
	{"Elasticsearch Node Left Cluster", "ES data node disconnected", "service_down", "P1", "prod", "elasticsearch", "search", map[string]string{"description": "Node es-data-2 left cluster 3m ago, shards unassigned"}},
	{"API Gateway 100% Error Rate", "All requests failing at gateway", "service_down", "P1", "prod", "nginx", "api-gateway", map[string]string{"description": "502 for 100% of requests for >1m"}},
	{"gRPC Service Unavailable", "Downstream gRPC service not responding", "service_down", "P1", "prod", "grpc-server", "auth-service", map[string]string{"description": "UNAVAILABLE for 100% of calls"}},
	{"Payment Service Down", "Payment processor unreachable", "service_down", "P1", "prod", "payment-api", "payments", map[string]string{"description": "All Stripe webhook deliveries failing"}},
	{"Database Failover in Progress", "RDS automated failover triggered", "service_down", "P1", "prod", "rds", "database", map[string]string{"description": "Multi-AZ failover, expected 60-120s downtime"}},
	{"Cassandra DC Unreachable", "Entire Cassandra DC offline", "service_down", "P1", "prod", "cassandra", "timeseries", map[string]string{"description": "DC us-west-2 not responding, quorum lost"}},
	{"CockroachDB Range Unavailable", "Range cannot achieve quorum", "service_down", "P2", "prod", "cockroachdb", "distributed-db", map[string]string{"description": "Range r42 has 1 of 3 replicas, needs 2"}},
	{"Haproxy Backend Down", "All HAProxy backend servers down", "service_down", "P2", "prod", "haproxy", "load-balancer", map[string]string{"description": "0/3 backends UP in pool prod-api"}},
	{"NATS JetStream Leader Lost", "JetStream cluster lost raft leader", "service_down", "P2", "prod", "nats", "messaging", map[string]string{"description": "RAFT election in progress, publish blocked"}},
	{"Worker Deployment Crashloop", "Deployment stuck in CrashLoopBackOff", "service_down", "P2", "prod", "k8s", "worker", map[string]string{"description": "worker-v2 5/5 pods in CrashLoopBackOff for 10m"}},
	{"Service Health Check Failing", "Liveness probe repeatedly failing", "service_down", "P2", "prod", "k8s", "api", map[string]string{"description": "Liveness probe /healthz returning 503"}},
	{"Dependency Timeout", "Upstream service timeout", "service_down", "P2", "prod", "api", "checkout", map[string]string{"description": "inventory-service p99 latency 12s, timeout 5s"}},
	{"Certificate Expiry Imminent", "TLS cert expires in <24h", "service_down", "P2", "prod", "cert-manager", "ingress", map[string]string{"description": "cert api.example.com expires in 8h"}},
	{"Consul Health Check Red", "Consul service unhealthy", "service_down", "P3", "staging", "consul", "service-mesh", map[string]string{"description": "payment-api health check critical in datacenter dc1"}},
	{"Service Pod Evicted", "Pods evicted due to resource pressure", "service_down", "P3", "prod", "k8s", "worker", map[string]string{"description": "3 pods evicted (DiskPressure) in 1h"}},
	{"External Dependency Degraded", "Third-party API returning errors", "service_down", "P3", "prod", "external", "analytics", map[string]string{"description": "Segment API returning 5xx at 30% rate"}},

	// ── metric_spike ──────────────────────────────────────────────────────────
	{"CPU Usage Critical", "Node CPU above 95% for >5m", "metric_spike", "P1", "prod", "node-exporter", "k8s-node", map[string]string{"description": "Node ip-10-0-1-42 CPU at 97%"}},
	{"Memory OOM Kill Imminent", "Container memory near OOM limit", "metric_spike", "P1", "prod", "k8s", "api", map[string]string{"description": "Container using 98% of memory limit, OOM expected"}},
	{"Disk I/O Saturation", "Disk throughput at device limit", "metric_spike", "P1", "prod", "node-exporter", "k8s-node", map[string]string{"description": "Disk /dev/sda at 99% saturation for 5m"}},
	{"Network Bandwidth Saturated", "NIC at 100% capacity", "metric_spike", "P1", "prod", "node-exporter", "k8s-node", map[string]string{"description": "eth0 transmit at 9.9Gbps, max 10Gbps"}},
	{"Database CPU Spike", "RDS CPU above 90%", "metric_spike", "P1", "prod", "rds", "database", map[string]string{"description": "RDS prod-primary CPU at 93% for 10m"}},
	{"Goroutine Leak Detected", "Goroutine count growing unboundedly", "metric_spike", "P2", "prod", "go-metrics", "api", map[string]string{"description": "Goroutine count 50k, growing at 1k/min"}},
	{"JVM Heap Near Capacity", "JVM heap at 90%", "metric_spike", "P2", "prod", "jvm", "java-service", map[string]string{"description": "Heap 4.5GB / 5GB, GC pauses increasing"}},
	{"GC Pause Time High", "JVM GC pause exceeds 2s", "metric_spike", "P2", "prod", "jvm", "java-service", map[string]string{"description": "G1 GC stop-the-world pauses at 2.3s avg"}},
	{"File Descriptor Limit Near", "Open file descriptors at 90%", "metric_spike", "P2", "prod", "node-exporter", "k8s-node", map[string]string{"description": "Open FDs: 900k/1M"}},
	{"HTTP Request Queue Depth High", "Upstream queue backing up", "metric_spike", "P2", "prod", "nginx", "api-gateway", map[string]string{"description": "Queue depth 8k requests, normal <100"}},
	{"Container CPU Throttle Rate High", "CPU throttle above 50%", "metric_spike", "P2", "prod", "k8s", "worker", map[string]string{"description": "cpu_throttling_seconds > 50% over 5m"}},
	{"Kafka Consumer Lag Growing", "Consumer group falling behind", "metric_spike", "P2", "prod", "kafka", "event-consumer", map[string]string{"description": "Group analytics-consumer lag 500k messages, growing"}},
	{"Elasticsearch Indexing Lag", "Index queue backing up", "metric_spike", "P2", "prod", "elasticsearch", "search", map[string]string{"description": "Index queue depth 20k docs, normal 500"}},
	{"Memory Usage High Non-Critical", "Memory at 85%, not yet OOM", "metric_spike", "P3", "staging", "k8s", "api", map[string]string{"description": "Memory usage 85% of limit, no OOM risk yet"}},
	{"CPU Usage Elevated", "CPU above 70% but below threshold", "metric_spike", "P3", "prod", "node-exporter", "k8s-node", map[string]string{"description": "Node CPU at 73%, threshold P1 is 95%"}},
	{"Cache Hit Rate Drop", "Redis cache hit rate dropped", "metric_spike", "P3", "prod", "redis", "cache", map[string]string{"description": "Hit rate dropped from 95% to 80%"}},
	{"Database Connections High", "DB connection count elevated", "metric_spike", "P3", "prod", "postgres", "database", map[string]string{"description": "Connection count 400/500, approaching limit"}},
	{"Disk Usage Above Threshold", "Disk at 80% capacity", "metric_spike", "P3", "prod", "node-exporter", "k8s-node", map[string]string{"description": "Disk /var 80% full, 200GB free"}},
	{"Request Rate Spike", "Unusual traffic spike", "metric_spike", "P3", "prod", "nginx", "api-gateway", map[string]string{"description": "RPS 5k vs baseline 2k, no degradation yet"}},
	{"Thread Pool Saturation", "Thread pool queue filling", "metric_spike", "P3", "prod", "tomcat", "java-service", map[string]string{"description": "Thread pool queue 200/500, processing normally"}},

	// ── log_analysis ──────────────────────────────────────────────────────────
	{"Segfault Detected in Logs", "SIGSEGV in application log", "log_analysis", "P1", "prod", "app", "api", map[string]string{"description": "segfault at rip 0x0000 sp 0x0000 in log stream"}},
	{"Panic Stack Trace in Logs", "Go panic detected", "log_analysis", "P1", "prod", "go-app", "api", map[string]string{"description": "goroutine 1 [running]: runtime: out of memory"}},
	{"NPE Flood in Application Logs", "NullPointerException storm", "log_analysis", "P1", "prod", "java-app", "checkout", map[string]string{"description": "500 NPEs/min in checkout service logs"}},
	{"Database Deadlock Storm", "Repeated DB deadlock errors", "log_analysis", "P1", "prod", "app", "orders", map[string]string{"description": "ERROR 1213: Deadlock found 50x in 1m"}},
	{"Auth Token Forgery Attempt", "Invalid JWT signatures in logs", "log_analysis", "P1", "prod", "auth", "api-gateway", map[string]string{"description": "100 invalid signature errors in 5m from single IP"}},
	{"SQL Injection Attempt Logged", "Malicious SQL in query logs", "log_analysis", "P1", "prod", "waf", "api-gateway", map[string]string{"description": "UNION SELECT detected in request path"}},
	{"Credential Stuffing Pattern", "Repeated login failures", "log_analysis", "P2", "prod", "auth", "api", map[string]string{"description": "1000 failed logins from 50 IPs in 5m"}},
	{"Rate Limit Bypass Detected", "Distributed rate limit evasion", "log_analysis", "P2", "prod", "api-gateway", "nginx", map[string]string{"description": "Single user across 200 IPs bypassing rate limit"}},
	{"Timeout Cascade in Logs", "Cascading timeout errors", "log_analysis", "P2", "prod", "app", "checkout", map[string]string{"description": "context deadline exceeded flooding service logs"}},
	{"Connection Refused Errors", "Downstream rejecting connections", "log_analysis", "P2", "prod", "app", "worker", map[string]string{"description": "connection refused to inventory:8080 1000x in 5m"}},
	{"Repeated 404 from Load Balancer", "LB routing to missing endpoints", "log_analysis", "P2", "prod", "nginx", "api-gateway", map[string]string{"description": "404 spike on /api/v2/users from lb-prod-3"}},
	{"Service Config Parse Error", "Configuration file unreadable", "log_analysis", "P2", "prod", "app", "api", map[string]string{"description": "Error parsing config.yaml: unexpected EOF"}},
	{"HDFS Under-replication Warning", "HDFS blocks under replicated", "log_analysis", "P3", "prod", "hdfs", "data-lake", map[string]string{"description": "550 blocks under replicated, replication factor 3→2"}},
	{"Slow Query Log Threshold Exceeded", "Queries exceeding slow query threshold", "log_analysis", "P3", "prod", "mysql", "database", map[string]string{"description": "200 queries > 2s in last hour, 3 > 10s"}},
	{"Deprecation Warning Storm", "Library deprecation in logs", "log_analysis", "P4", "staging", "app", "api", map[string]string{"description": "DeprecationWarning: Using deprecated API endpoint"}},
	{"Non-Fatal Exception Rate Up", "Exception count elevated but handled", "log_analysis", "P4", "prod", "app", "api", map[string]string{"description": "Handled exceptions up 3x from baseline, all caught"}},
	{"Verbose Debug Log Leak", "Debug logs in production", "log_analysis", "P4", "prod", "app", "worker", map[string]string{"description": "DEBUG level logs emitting to prod log stream"}},
	{"Old TLS Version Warning", "TLS 1.0/1.1 connections logged", "log_analysis", "P4", "prod", "nginx", "api-gateway", map[string]string{"description": "TLS 1.1 connections from 3 legacy clients logged"}},
	{"Certificate Pinning Mismatch", "Cert pin check failing for some clients", "log_analysis", "P3", "prod", "mobile-backend", "api", map[string]string{"description": "Certificate pin mismatch from app version <2.1.0"}},
	{"K8s Scheduler Pending Pods", "Pods unable to schedule", "log_analysis", "P3", "prod", "k8s", "scheduler", map[string]string{"description": "15 pods Pending: Insufficient memory on all nodes"}},

	// ── oom ──────────────────────────────────────────────────────────────────
	{"Container OOMKilled", "Container terminated by OOM killer", "oom", "P1", "prod", "k8s", "api", map[string]string{"description": "Container api OOMKilled, requested 2Gi but used >2Gi"}},
	{"Node OOM Kill Event", "Linux OOM killer activated on node", "oom", "P1", "prod", "node-exporter", "k8s-node", map[string]string{"description": "kernel: Out of memory: Kill process 12345 (api) score 900"}},
	{"JVM OOM Exception", "Java heap exhausted", "oom", "P1", "prod", "jvm", "java-service", map[string]string{"description": "java.lang.OutOfMemoryError: Java heap space"}},
	{"Go OOM Panic", "Go runtime out of memory", "oom", "P1", "prod", "go-app", "worker", map[string]string{"description": "runtime: out of memory: cannot allocate 4GB"}},
	{"Redis OOM Policy Triggered", "Redis evicting keys due to maxmemory", "oom", "P2", "prod", "redis", "cache", map[string]string{"description": "OOM command not allowed when used memory > maxmemory"}},
	{"Elasticsearch OOM", "ES JVM heap exhausted", "oom", "P2", "prod", "elasticsearch", "search", map[string]string{"description": "OutOfMemoryError: Java heap space in ES data node"}},
	{"Spark OOM on Executor", "Spark executor out of memory", "oom", "P2", "prod", "spark", "data-pipeline", map[string]string{"description": "ExecutorLostFailure: Container killed by YARN for exceeding limits"}},
	{"Python Process OOM", "Python worker killed by OOM", "oom", "P2", "prod", "worker", "ml-inference", map[string]string{"description": "MemoryError: Unable to allocate array for model weights"}},
	{"Node.js Heap OOM", "Node heap limit exceeded", "oom", "P2", "prod", "node-app", "api", map[string]string{"description": "FATAL ERROR: CALL_AND_RETRY_LAST Allocation failed - JavaScript heap out of memory"}},
	{"Kafka Broker OOM", "Kafka JVM heap OOM", "oom", "P1", "prod", "kafka", "messaging", map[string]string{"description": "java.lang.OutOfMemoryError: Java heap space in kafka broker"}},
	{"Sidecar Proxy OOM", "Envoy sidecar OOMKilled", "oom", "P3", "prod", "k8s", "service-mesh", map[string]string{"description": "Sidecar envoy OOMKilled, limits 128Mi reached"}},
	{"Build Job OOM", "CI build job out of memory", "oom", "P4", "ci", "k8s-job", "build", map[string]string{"description": "Build runner OOMKilled at 8Gi, increase to 12Gi"}},
}

var (
	namespaces = []string{"prod", "staging", "dev", "ci", "canary"}
	clusters   = []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1", "us-central-1"}
	statuses   = []string{"firing", "resolved"}
)

func main() {
	var cases []testCase

	// First 100: use the existing smoke test data as-is
	// (we prepend them by reading the existing file)
	existingIDs := map[string]bool{}
	existingData, err := os.ReadFile("test/fixtures/classification_smoke.jsonl")
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(existingData)), "\n") {
			var tc testCase
			if err := json.Unmarshal([]byte(line), &tc); err == nil {
				cases = append(cases, tc)
				existingIDs[tc.ID] = true
			}
		}
	}

	// Generate additional cases from templates × variant dimensions.
	id := len(cases) + 1
	for _, ns := range namespaces {
		for _, cluster := range clusters {
			for _, tmpl := range templates {
				caseID := fmt.Sprintf("cls-%03d", id)
				if existingIDs[caseID] {
					id++
					caseID = fmt.Sprintf("cls-%03d", id)
				}
				sev := tmpl.severity
				// Downgrade severity in non-prod namespaces.
				if ns != "prod" && sev == "P1" {
					sev = "P2"
				}
				status := "firing"
				if id%7 == 0 {
					status = "resolved"
				}
				cases = append(cases, testCase{
					ID:       caseID,
					Category: "classification",
					Description: fmt.Sprintf("[%s/%s] %s", ns, cluster, tmpl.description),
					Alert: alert{
						Title:    tmpl.title,
						Severity: sev,
						Status:   status,
						Labels: map[string]string{
							"namespace": ns,
							"job":       tmpl.job,
							"cluster":   cluster,
							"service":   tmpl.service,
						},
						Annotations: tmpl.annotations,
						Description: "",
					},
					ExpectedSeverity: sev,
					ExpectedIntent:   tmpl.intent,
				})
				id++
				if len(cases) >= 520 { // slight overshoot so we land at 500+ after dedup
					break
				}
			}
			if len(cases) >= 520 {
				break
			}
		}
		if len(cases) >= 520 {
			break
		}
	}

	// Truncate to exactly 500 if we overshot.
	if len(cases) > 500 {
		cases = cases[:500]
	}

	f, err := os.Create("test/fixtures/classification_smoke.jsonl")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, tc := range cases {
		if err := enc.Encode(tc); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Printf("Wrote %d classification cases\n", len(cases))
}
