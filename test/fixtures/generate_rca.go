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
	ID                   string     `json:"id"`
	Category             string     `json:"category"`
	Description          string     `json:"description"`
	Alert                alertInput `json:"alert"`
	ExpectedRootCause    string     `json:"expected_root_cause"`
	PredictedRootCause   string     `json:"predicted_root_cause"`
	ExpectedBlastRadius  []string   `json:"expected_blast_radius"`
	PredictedBlastRadius []string   `json:"predicted_blast_radius"`
}

type rcaTemplate struct {
	titleFmt    string
	rootCause   string
	rootCauseID string
	blastRadius []string
	severity    string
	service     string
	namespace   string
}

var rcaTemplates = []rcaTemplate{
	{
		titleFmt:    "Payments API 5xx surge after deploy %s",
		rootCause:   "bad deployment introduced regression in %s",
		rootCauseID: "deployment_regression",
		blastRadius: []string{"payments-api", "checkout", "gateway"},
		severity:    "P1", service: "payments-api", namespace: "prod",
	},
	{
		titleFmt:    "Orders DB connection exhaustion on %s",
		rootCause:   "connection pool leak in %s after ORM upgrade",
		rootCauseID: "db_connection_pool_leak",
		blastRadius: []string{"orders-db", "orders-api", "reporting"},
		severity:    "P1", service: "orders-db", namespace: "prod",
	},
	{
		titleFmt:    "Auth service OOMKill loop on %s",
		rootCause:   "memory leak in token cache on %s",
		rootCauseID: "auth_token_cache_memory_leak",
		blastRadius: []string{"auth-service", "gateway", "user-api"},
		severity:    "P1", service: "auth-service", namespace: "prod",
	},
	{
		titleFmt:    "Kafka consumer lag spike on %s",
		rootCause:   "consumer group rebalance storm after rolling restart in %s",
		rootCauseID: "consumer_rebalance_storm",
		blastRadius: []string{"event-consumer", "billing-worker", "analytics-sink"},
		severity:    "P2", service: "event-consumer", namespace: "data-pipeline",
	},
	{
		titleFmt:    "Kubernetes node NotReady in %s",
		rootCause:   "kubelet crash due to containerd bug on %s",
		rootCauseID: "kubelet_containerd_crash",
		blastRadius: []string{"kubelet", "node-pool-a", "workload-scheduler"},
		severity:    "P1", service: "kubelet", namespace: "kube-system",
	},
	{
		titleFmt:    "Redis eviction storm on %s",
		rootCause:   "maxmemory policy set to allkeys-lru without TTL tuning on %s",
		rootCauseID: "redis_eviction_policy_misconfig",
		blastRadius: []string{"cache-service", "session-api", "cart-api"},
		severity:    "P2", service: "cache-service", namespace: "prod",
	},
	{
		titleFmt:    "gRPC timeout cascade on %s",
		rootCause:   "missing deadline propagation in internal RPC chain on %s",
		rootCauseID: "grpc_deadline_propagation_missing",
		blastRadius: []string{"grpc-gateway", "orders-api", "payments-api"},
		severity:    "P1", service: "grpc-gateway", namespace: "prod",
	},
	{
		titleFmt:    "Ingress 502 spike on %s",
		rootCause:   "backend pod scale-down during traffic peak on %s",
		rootCauseID: "backend_scale_down_during_peak",
		blastRadius: []string{"ingress-nginx", "frontend", "api-gateway"},
		severity:    "P2", service: "ingress-nginx", namespace: "prod",
	},
	{
		titleFmt:    "CronJob failing on %s",
		rootCause:   "permission denied on PVC mount after RBAC update on %s",
		rootCauseID: "rbac_pvc_mount_permission_denied",
		blastRadius: []string{"cleanup-cronjob", "backup-job", "artifact-pruner"},
		severity:    "P3", service: "cleanup-cronjob", namespace: "prod",
	},
	{
		titleFmt:    "Etcd high latency on %s",
		rootCause:   "disk I/O saturation on etcd node due to noisy neighbour on %s",
		rootCauseID: "etcd_disk_io_saturation",
		blastRadius: []string{"etcd", "kube-apiserver", "controller-manager"},
		severity:    "P1", service: "etcd", namespace: "kube-system",
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
			expectedBlastRadius := scopedBlastRadius(tmpl.blastRadius, cluster)
			cases = append(cases, testCase{
				ID:       id,
				Category: "rca_correctness",
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
				ExpectedRootCause:    tmpl.rootCauseID,
				PredictedRootCause:   tmpl.rootCauseID,
				ExpectedBlastRadius:  expectedBlastRadius,
				PredictedBlastRadius: append(expectedBlastRadius, cluster+"-noise"),
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

func scopedBlastRadius(services []string, cluster string) []string {
	out := make([]string, 0, len(services))
	for _, service := range services {
		out = append(out, cluster+"/"+service)
	}
	return out
}
