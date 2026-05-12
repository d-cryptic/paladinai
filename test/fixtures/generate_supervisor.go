//go:build ignore

// generate_supervisor.go generates supervisor routing eval fixtures.
// Run with: go run test/fixtures/generate_supervisor.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type supervisorCase struct {
	ID                string            `json:"id"`
	Category          string            `json:"category"`
	Description       string            `json:"description"`
	Alert             supervisorAlert   `json:"alert"`
	ExpectedAgentType string            `json:"expected_agent_type"` // "triage" | "rca" | "runbook"
	ExpectedIntent    string            `json:"expected_intent"`
	ExpectedSeverity  string            `json:"expected_severity"`
}

type supervisorAlert struct {
	Title       string            `json:"title"`
	Severity    string            `json:"severity"`
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type superTemplate struct {
	title       string
	description string
	intent      string
	severity    string
	agentType   string // which downstream agent to route to
	service     string
	notes       map[string]string
}

var superTemplates = []superTemplate{
	// ── Routes to TRIAGE agent ────────────────────────────────────────────────
	{"PostgreSQL Primary Down", "Production DB primary node not responding", "service_down", "P1", "triage", "database", map[string]string{"description": "Primary replication lag 45s, all writes failing"}},
	{"API Gateway 100% Error Rate", "All requests returning 502", "service_down", "P1", "triage", "api-gateway", map[string]string{"description": "502 for 100% of requests for >1m, circuit breaker open"}},
	{"Payment Service Unresponsive", "Payment processor not accepting connections", "service_down", "P1", "triage", "payments", map[string]string{"description": "TCP connect timeout to payment-api:8080"}},
	{"Kubernetes Control Plane Down", "API server unreachable", "service_down", "P1", "triage", "k8s-control-plane", map[string]string{"description": "kubectl times out, no new pods scheduling"}},
	{"Redis Cluster Split Brain", "Redis nodes disagree on primary", "service_down", "P1", "triage", "cache", map[string]string{"description": "Two nodes claiming primary role, writes failing"}},
	{"Network Partition Detected", "Pods unable to reach each other", "service_down", "P1", "triage", "network", map[string]string{"description": "DNS resolution failing across pod-to-pod"}},
	{"Container OOMKill Loop", "Production service killed repeatedly by OOM", "oom", "P1", "triage", "api", map[string]string{"description": "OOMKilled 3 times in 5min, restartPolicy not helping"}},
	{"Certificate Expired", "TLS cert for api.example.com expired", "service_down", "P1", "triage", "ingress", map[string]string{"description": "x509: certificate has expired or is not yet valid"}},
	{"Node NotReady", "Kubernetes node stopped responding to kubelet", "service_down", "P1", "triage", "k8s-node", map[string]string{"description": "Node ip-10-0-1-5 in NotReady state for 5m"}},
	{"Kafka Consumer Group Stuck", "Consumer group offset not advancing", "service_down", "P1", "triage", "messaging", map[string]string{"description": "Group payment-consumer lag 2M msgs, offset frozen"}},
	{"Auth Service 100% Failure", "JWT validation failing for all requests", "service_down", "P1", "triage", "auth", map[string]string{"description": "401 for 100% of authenticated requests"}},
	{"Disk Full on Postgres Node", "Postgres running out of disk space", "metric_spike", "P1", "triage", "database", map[string]string{"description": "Postgres data dir at 98%, writes will fail soon"}},
	{"High CPU Sustained 30min", "Node CPU pegged at 99% for 30 minutes", "metric_spike", "P2", "triage", "k8s-node", map[string]string{"description": "Node CPU 99% for 30m, load average 24.0"}},
	{"Memory Leak Detected", "Memory growing without bound", "metric_spike", "P2", "triage", "worker", map[string]string{"description": "RSS growing 50MB/min, no GC pressure reduction"}},
	{"Connection Pool Exhausted", "All DB connections in use", "metric_spike", "P2", "triage", "database", map[string]string{"description": "pgbouncer pool 100/100 busy, new connections queued"}},
	{"HTTP Latency Spike P99", "API P99 latency jumped from 200ms to 8s", "metric_spike", "P2", "triage", "api", map[string]string{"description": "Sudden latency jump 3 minutes ago, 5xx rate still <1%"}},
	{"Elasticsearch Heap Pressure", "ES heap at 90%, GC pausing queries", "metric_spike", "P2", "triage", "search", map[string]string{"description": "Heap 18GB/20GB, GC overhead limit approaching"}},
	{"Rate Limit Cascade", "Rate limiter triggering across all services", "service_down", "P2", "triage", "api-gateway", map[string]string{"description": "Downstream rate limits causing cascading 429s"}},
	{"Cassandra Read Timeout", "Reads timing out due to tombstone accumulation", "metric_spike", "P2", "triage", "database", map[string]string{"description": "ReadTimeoutException: 1000 tombstones scanned per query"}},
	{"Flapping Deployment", "Rolling deployment causing repeated crashes", "service_down", "P2", "triage", "worker", map[string]string{"description": "Deployment v2.3.1 crashing, v2.3.0 rolling back"}},

	// ── Routes to RCA agent ───────────────────────────────────────────────────
	{"Recurring DB Deadlock", "Same DB deadlock pattern third time this week", "log_analysis", "P2", "rca", "database", map[string]string{"description": "Deadlock on orders table, same 3 queries as Mon/Tue"}},
	{"Mysterious Latency Spike Every 6h", "API latency spikes on a regular schedule", "metric_spike", "P2", "rca", "api", map[string]string{"description": "Spike exactly at 00:00, 06:00, 12:00, 18:00 UTC"}},
	{"Service Restarts After Deploy", "Service crashes within 2 minutes of any deploy", "service_down", "P2", "rca", "api", map[string]string{"description": "3 deploys today, all crashed within 2min with same SIGSEGV"}},
	{"Memory Growth After Traffic Spike", "RSS grows during traffic then never releases", "oom", "P2", "rca", "worker", map[string]string{"description": "Memory grew during lunch traffic, still high at 2am"}},
	{"Error Rate Correlated With Config Change", "5xx rate spiked after config push 2h ago", "log_analysis", "P2", "rca", "api", map[string]string{"description": "Errors started exactly at config deploy 14:23 UTC"}},
	{"Third Party SLA Miss Pattern", "Stripe API timeouts every day at 16:00 UTC", "service_down", "P2", "rca", "payments", map[string]string{"description": "Stripe returns 503 between 16:00-16:05 UTC daily"}},
	{"Kafka Lag Growing After Code Push", "Consumer lag started growing after v3.2.0 release", "metric_spike", "P2", "rca", "messaging", map[string]string{"description": "Lag started growing at 11:30, v3.2.0 deployed at 11:28"}},
	{"Node Evictions After K8s Upgrade", "Pods being evicted after K8s 1.31 upgrade", "service_down", "P3", "rca", "k8s-node", map[string]string{"description": "Evictions started after K8s upgrade last night"}},
	{"Auth Failures Only From Mobile", "JWT failures only on iOS/Android, not web", "log_analysis", "P2", "rca", "auth", map[string]string{"description": "100% auth failures from User-Agent: PaladinMobile, 0% on web"}},
	{"Database CPU Spike Every Monday", "RDS CPU hits 90% every Monday at 09:00", "metric_spike", "P3", "rca", "database", map[string]string{"description": "Weekly pattern, Monday 09:00 UTC, 30min duration"}},
	{"OOM Only In EU Region", "OOM kills only in eu-west-1, us-east-1 fine", "oom", "P2", "rca", "worker", map[string]string{"description": "OOM in eu-west-1 only, same code, same config"}},
	{"Latency Worsens With User Count", "P99 degrades as concurrent users exceed 1000", "metric_spike", "P2", "rca", "api", map[string]string{"description": "Linear latency increase above 1000 concurrent users"}},
	{"Error Pattern Matches Specific Tenant", "5xx only for tenant-b, tenant-a fine", "log_analysis", "P2", "rca", "api", map[string]string{"description": "100% of errors have X-Tenant-ID: tenant-b header"}},
	{"GC Pause Increase After JVM Config Change", "JVM GC pause doubled after heap increase", "metric_spike", "P2", "rca", "java-service", map[string]string{"description": "GC pause went from 200ms to 400ms after heap 4G→8G"}},
	{"Disk Latency Spike Matches Snapshot Schedule", "Disk I/O spikes match EBS snapshot schedule", "metric_spike", "P3", "rca", "k8s-node", map[string]string{"description": "I/O spike at 03:00 UTC daily matches snapshot window"}},

	// ── Routes to RUNBOOK agent ───────────────────────────────────────────────
	{"Known Postgres Failover Procedure", "Primary DB down, need to promote replica", "service_down", "P1", "runbook", "database", map[string]string{"description": "Standard failover: promote replica, update DNS, verify replication"}},
	{"Redis Cache Flush Required", "Stale cache causing data inconsistency", "service_down", "P2", "runbook", "cache", map[string]string{"description": "Cache has stale user sessions, documented flush procedure needed"}},
	{"Scale Out Worker Fleet", "Worker queue depth high, scale horizontally", "metric_spike", "P2", "runbook", "worker", map[string]string{"description": "Queue 50k jobs, 4 workers, runbook says scale to 16"}},
	{"Certificate Renewal Runbook", "TLS cert expiring in 4h, documented renewal", "service_down", "P2", "runbook", "ingress", map[string]string{"description": "cert-manager renewal runbook: delete secret, re-issue"}},
	{"Kafka Topic Rebalance Procedure", "Consumer rebalance stuck, documented fix", "service_down", "P2", "runbook", "messaging", map[string]string{"description": "Reset consumer group offset using documented kafka-consumer-groups.sh procedure"}},
	{"Drain and Cordon Node", "Node needs maintenance, drain workloads first", "service_down", "P2", "runbook", "k8s-node", map[string]string{"description": "kubectl drain + cordon before node maintenance window"}},
	{"Clear Elasticsearch Dead Indices", "Disk pressure from old indices, documented cleanup", "metric_spike", "P2", "runbook", "search", map[string]string{"description": "Run curator to delete indices older than 30d per runbook"}},
	{"Postgres Vacuum Freeze", "Table bloat causing performance degradation", "metric_spike", "P2", "runbook", "database", map[string]string{"description": "VACUUM FREEZE on orders table per documented maintenance procedure"}},
	{"Rollback Deployment", "Bad deploy causing errors, rollback available", "service_down", "P2", "runbook", "api", map[string]string{"description": "kubectl rollout undo deployment/api, previous version known good"}},
	{"Restart Service with Graceful Drain", "Memory leak, service needs restart with traffic drain", "oom", "P2", "runbook", "worker", map[string]string{"description": "Documented: drain traffic via LB, wait, restart, verify, restore"}},
	{"NATS Stream Purge", "Poison pill messages blocking consumer, purge needed", "service_down", "P2", "runbook", "messaging", map[string]string{"description": "Known bad messages in NATS stream, documented purge + republish"}},
	{"Rotate Database Credentials", "DB password rotation required per security policy", "service_down", "P3", "runbook", "database", map[string]string{"description": "90-day credential rotation runbook: Vault rotate + service restart"}},
	{"Enable Read Replica Routing", "Primary under load, failover reads to replica", "metric_spike", "P2", "runbook", "database", map[string]string{"description": "Toggle READ_REPLICA_ENABLED=true per documented load runbook"}},
	{"Clear Nginx Cache", "Stale content being served, nginx cache clear needed", "service_down", "P3", "runbook", "api-gateway", map[string]string{"description": "rm -rf /var/cache/nginx/* and reload per content runbook"}},
	{"Increase Rate Limit Temporarily", "Legitimate traffic spike, temp rate limit increase", "metric_spike", "P3", "runbook", "api-gateway", map[string]string{"description": "Update Valkey rate config per traffic runbook, review in 1h"}},
}

func main() {
	var cases []supervisorCase
	id := 1
	namespaces := []string{"prod", "staging", "prod", "prod", "canary"} // weight prod higher

	for i, tmpl := range superTemplates {
		for _, ns := range namespaces {
			sev := tmpl.severity
			if ns != "prod" && sev == "P1" {
				sev = "P2"
			}
			status := "firing"
			if id%11 == 0 {
				status = "resolved"
			}
			cluster := []string{"us-east-1", "eu-west-1", "ap-southeast-1"}[i%3]
			cases = append(cases, supervisorCase{
				ID:       fmt.Sprintf("sup-%03d", id),
				Category: "supervisor_routing",
				Description: fmt.Sprintf("[%s/%s] %s", ns, cluster, tmpl.description),
				Alert: supervisorAlert{
					Title:    tmpl.title,
					Severity: sev,
					Status:   status,
					Labels: map[string]string{
						"namespace": ns,
						"job":       tmpl.service,
						"cluster":   cluster,
						"service":   tmpl.service,
					},
					Annotations: tmpl.notes,
				},
				ExpectedAgentType: tmpl.agentType,
				ExpectedIntent:    tmpl.intent,
				ExpectedSeverity:  sev,
			})
			id++
			if len(cases) >= 200 {
				break
			}
		}
		if len(cases) >= 200 {
			break
		}
	}

	f, err := os.Create("test/fixtures/supervisor_routing.jsonl")
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
	fmt.Printf("Wrote %d supervisor routing cases\n", len(cases))
}
