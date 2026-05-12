//go:build ignore

// generate_tool_selection.go generates tool_selection.jsonl (300+ cases).
// Run: go run test/fixtures/generate_tool_selection.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type alertInput struct {
	Title       string            `json:"title"`
	Severity    string            `json:"severity"`
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Description string            `json:"description"`
}

type testCase struct {
	ID            string     `json:"id"`
	Category      string     `json:"category"`
	Description   string     `json:"description"`
	Alert         alertInput `json:"alert"`
	ExpectedTools []string   `json:"expected_tools"`
}

type template struct {
	titleFmt string
	descFmt  string
	job      string
	service  string
	severity string
	tools    []string
	labels   map[string]string
}

var services = []string{"payments-api", "orders-service", "auth-service", "user-service", "catalog-service", "inventory-service"}
var namespaces = []string{"prod", "staging", "infra", "data-pipeline"}
var clusters = []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1", "us-central-1"}

var templates = []template{
	// HTTP 5xx
	{
		titleFmt: "%s 5xx Error Rate > 50%%",
		descFmt:  "HTTP 5xx error rate for %s exceeds 50%% for 3 minutes",
		job:      "http", severity: "P1",
		tools:  []string{"get_pod_logs", "get_error_rate", "get_runbook", "get_recent_deployments"},
		labels: map[string]string{"reason": "http_5xx"},
	},
	// High latency
	{
		titleFmt: "%s p99 Latency > 2s",
		descFmt:  "p99 latency for %s exceeded 2 seconds SLO",
		job:      "http", severity: "P2",
		tools:  []string{"get_metrics", "get_recent_deployments", "get_runbook"},
		labels: map[string]string{"reason": "high_latency"},
	},
	// OOMKill
	{
		titleFmt: "OOMKilled %s pods",
		descFmt:  "%s pods are being OOMKilled in rapid succession",
		job:      "worker", severity: "P1",
		tools:  []string{"get_pod_logs", "get_metrics", "get_runbook"},
		labels: map[string]string{"reason": "oomkill"},
	},
	// Deployment failure
	{
		titleFmt: "%s deployment rollout stuck",
		descFmt:  "%s deployment rollout has been stuck for >10 minutes",
		job:      "deploy", severity: "P2",
		tools:  []string{"get_pod_logs", "describe_deployment", "get_recent_deployments"},
		labels: map[string]string{"reason": "deploy_stuck"},
	},
	// Pod crashloop
	{
		titleFmt: "%s CrashLoopBackOff",
		descFmt:  "%s has been in CrashLoopBackOff for 5 minutes",
		job:      "kubelet", severity: "P1",
		tools:  []string{"get_pod_logs", "describe_pod", "get_runbook"},
		labels: map[string]string{"reason": "crashloop"},
	},
	// DB slow queries
	{
		titleFmt: "Slow queries on %s database",
		descFmt:  "Slow query threshold exceeded for %s; 20+ queries >5s",
		job:      "postgres", severity: "P2",
		tools:  []string{"get_db_slow_queries", "get_metrics"},
		labels: map[string]string{"reason": "slow_queries"},
	},
	// DB connections exhausted
	{
		titleFmt: "%s DB connection pool exhausted",
		descFmt:  "%s reached 100%% connection pool utilisation",
		job:      "postgres", severity: "P1",
		tools:  []string{"get_metrics", "get_runbook", "get_pod_logs"},
		labels: map[string]string{"reason": "conn_pool"},
	},
	// Node NotReady
	{
		titleFmt: "Node NotReady in %s cluster",
		descFmt:  "Kubernetes node in %s is in NotReady state for 5 minutes",
		job:      "kubelet", severity: "P1",
		tools:  []string{"describe_node", "get_pod_logs", "get_metrics", "get_runbook"},
		labels: map[string]string{"reason": "node_notready"},
	},
	// Disk pressure
	{
		titleFmt: "Disk pressure on %s node",
		descFmt:  "%s cluster node disk utilisation > 90%%",
		job:      "node-exporter", severity: "P2",
		tools:  []string{"get_metrics", "get_runbook"},
		labels: map[string]string{"reason": "disk_pressure"},
	},
	// Memory pressure
	{
		titleFmt: "Memory pressure on %s",
		descFmt:  "%s memory utilisation > 90%% on cluster node",
		job:      "node-exporter", severity: "P2",
		tools:  []string{"get_metrics", "get_runbook"},
		labels: map[string]string{"reason": "memory_pressure"},
	},
	// Certificate expiry
	{
		titleFmt: "TLS certificate expiring for %s",
		descFmt:  "TLS certificate for %s expires in < 7 days",
		job:      "cert-manager", severity: "P3",
		tools:  []string{"get_runbook"},
		labels: map[string]string{"reason": "cert_expiry"},
	},
	// Kafka lag
	{
		titleFmt: "%s consumer lag > 10000",
		descFmt:  "Kafka consumer group for %s has lag > 10000 messages",
		job:      "kafka", severity: "P2",
		tools:  []string{"get_metrics", "get_pod_logs", "get_runbook"},
		labels: map[string]string{"reason": "kafka_lag"},
	},
	// Redis evictions
	{
		titleFmt: "Redis evictions spiking for %s",
		descFmt:  "Redis eviction rate for %s cache exceeded 1000/s",
		job:      "redis", severity: "P2",
		tools:  []string{"get_metrics", "get_runbook"},
		labels: map[string]string{"reason": "redis_eviction"},
	},
	// CPU throttling
	{
		titleFmt: "CPU throttling on %s",
		descFmt:  "%s pods throttled >50%% CPU — hitting cgroup limits",
		job:      "cadvisor", severity: "P3",
		tools:  []string{"get_metrics", "get_pod_logs"},
		labels: map[string]string{"reason": "cpu_throttle"},
	},
	// NATS stream full
	{
		titleFmt: "NATS stream full for %s",
		descFmt:  "NATS JetStream retention limit reached for %s consumer",
		job:      "nats", severity: "P2",
		tools:  []string{"get_metrics", "get_runbook"},
		labels: map[string]string{"reason": "nats_full"},
	},
}

func main() {
	var cases []testCase
	n := 0
	for ti, tmpl := range templates {
		for si, svc := range services {
			for ni, ns := range namespaces {
				for ci, cluster := range clusters {
					n++
					if n > 300 {
						break
					}
					id := fmt.Sprintf("tool-%04d", n)
					lbls := map[string]string{
						"namespace": ns,
						"cluster":   cluster,
						"service":   svc,
						"job":       tmpl.job,
					}
					for k, v := range tmpl.labels {
						lbls[k] = v
					}
					cases = append(cases, testCase{
						ID:       id,
						Category: "tool_use",
						Description: fmt.Sprintf("[tmpl=%d svc=%d ns=%d cluster=%d] %s",
							ti, si, ni, ci, fmt.Sprintf(tmpl.descFmt, svc)),
						Alert: alertInput{
							Title:    fmt.Sprintf(tmpl.titleFmt, svc),
							Severity: tmpl.severity,
							Status:   "firing",
							Labels:   lbls,
							Annotations: map[string]string{
								"description": fmt.Sprintf(tmpl.descFmt, svc),
							},
							Description: "",
						},
						ExpectedTools: tmpl.tools,
					})
				}
				if n > 300 {
					break
				}
			}
			if n > 300 {
				break
			}
		}
		if n > 300 {
			break
		}
	}

	out, err := os.Create("test/fixtures/tool_selection.jsonl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	enc := json.NewEncoder(out)
	for _, tc := range cases {
		if err := enc.Encode(tc); err != nil {
			fmt.Fprintf(os.Stderr, "encode: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("wrote %d tool_selection cases\n", len(cases))
}
