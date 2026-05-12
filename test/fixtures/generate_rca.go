//go:build ignore

// generate_rca.go generates rca_correctness.jsonl (100+ cases).
// Run: go run test/fixtures/generate_rca.go
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
	ID               string     `json:"id"`
	Category         string     `json:"category"`
	Description      string     `json:"description"`
	Alert            alertInput `json:"alert"`
	ExpectedKeywords []string   `json:"expected_keywords"`
	MustNotContain   []string   `json:"must_not_contain,omitempty"`
}

type rcaTemplate struct {
	titleFmt       string
	rootCause      string
	keywords       []string
	mustNotContain []string
	severity       string
	service        string
	namespace      string
}

var rcaTemplates = []rcaTemplate{
	{
		titleFmt:       "Payments API 5xx surge after deploy %s",
		rootCause:      "bad deployment introduced regression in %s",
		keywords:       []string{"deploy", "regression", "rollback", "version"},
		mustNotContain: []string{"network partition", "hardware failure"},
		severity:       "P1", service: "payments-api", namespace: "prod",
	},
	{
		titleFmt:       "Orders DB connection exhaustion on %s",
		rootCause:      "connection pool leak in %s after ORM upgrade",
		keywords:       []string{"connection pool", "leak", "orm", "upgrade"},
		mustNotContain: []string{"disk full", "cpu throttle"},
		severity:       "P1", service: "orders-db", namespace: "prod",
	},
	{
		titleFmt:       "Auth service OOMKill loop on %s",
		rootCause:      "memory leak in token cache on %s",
		keywords:       []string{"memory", "leak", "cache", "token"},
		mustNotContain: []string{"disk io", "network"},
		severity:       "P1", service: "auth-service", namespace: "prod",
	},
	{
		titleFmt:       "Kafka consumer lag spike on %s",
		rootCause:      "consumer group rebalance storm after rolling restart in %s",
		keywords:       []string{"rebalance", "consumer", "lag", "rolling restart"},
		mustNotContain: []string{"broker disk", "schema registry"},
		severity:       "P2", service: "event-consumer", namespace: "data-pipeline",
	},
	{
		titleFmt:       "Kubernetes node NotReady in %s",
		rootCause:      "kubelet crash due to containerd bug on %s",
		keywords:       []string{"kubelet", "containerd", "node", "crash"},
		mustNotContain: []string{"application error", "database"},
		severity:       "P1", service: "kubelet", namespace: "kube-system",
	},
	{
		titleFmt:       "Redis eviction storm on %s",
		rootCause:      "maxmemory policy set to allkeys-lru without TTL tuning on %s",
		keywords:       []string{"maxmemory", "eviction", "lru", "ttl"},
		mustNotContain: []string{"network partition", "disk"},
		severity:       "P2", service: "cache-service", namespace: "prod",
	},
	{
		titleFmt:       "gRPC timeout cascade on %s",
		rootCause:      "missing deadline propagation in internal RPC chain on %s",
		keywords:       []string{"deadline", "timeout", "cascade", "grpc"},
		mustNotContain: []string{"disk", "network packet loss"},
		severity:       "P1", service: "grpc-gateway", namespace: "prod",
	},
	{
		titleFmt:       "Ingress 502 spike on %s",
		rootCause:      "backend pod scale-down during traffic peak on %s",
		keywords:       []string{"scale-down", "502", "backend", "pod"},
		mustNotContain: []string{"certificate", "database"},
		severity:       "P2", service: "ingress-nginx", namespace: "prod",
	},
	{
		titleFmt:       "CronJob failing on %s",
		rootCause:      "permission denied on PVC mount after RBAC update on %s",
		keywords:       []string{"rbac", "permission", "pvc", "mount"},
		mustNotContain: []string{"memory", "cpu"},
		severity:       "P3", service: "cleanup-cronjob", namespace: "prod",
	},
	{
		titleFmt:       "Etcd high latency on %s",
		rootCause:      "disk I/O saturation on etcd node due to noisy neighbour on %s",
		keywords:       []string{"etcd", "disk", "io", "latency"},
		mustNotContain: []string{"network partition", "application"},
		severity:       "P1", service: "etcd", namespace: "kube-system",
	},
}

var clusters = []string{
	"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1", "us-central-1",
	"eu-central-1", "ap-northeast-1", "sa-east-1", "ca-central-1", "af-south-1",
}

func main() {
	var cases []testCase
	n := 0
	for ti, tmpl := range rcaTemplates {
		for ci, cluster := range clusters {
			n++
			id := fmt.Sprintf("rca-%04d", n)
			cases = append(cases, testCase{
				ID:       id,
				Category: "adversarial", // rca uses adversarial category for scoring
				Description: fmt.Sprintf("[tmpl=%d cluster=%d] RCA: %s",
					ti, ci, fmt.Sprintf(tmpl.rootCause, cluster)),
				Alert: alertInput{
					Title:    fmt.Sprintf(tmpl.titleFmt, cluster),
					Severity: tmpl.severity,
					Status:   "firing",
					Labels: map[string]string{
						"cluster":   cluster,
						"service":   tmpl.service,
						"namespace": tmpl.namespace,
					},
					Annotations: map[string]string{
						"description": fmt.Sprintf(tmpl.rootCause, cluster),
					},
					Description: "",
				},
				ExpectedKeywords: tmpl.keywords,
				MustNotContain:   tmpl.mustNotContain,
			})
		}
	}

	out, err := os.Create("test/fixtures/rca_correctness.jsonl")
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
	fmt.Printf("wrote %d rca_correctness cases\n", len(cases))
}
