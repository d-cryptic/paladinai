//go:build ignore

// generate_multitenant.go generates multitenant_isolation.jsonl (50+ cases).
// Run: go run test/fixtures/generate_multitenant.go
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
	ID             string     `json:"id"`
	Category       string     `json:"category"`
	Description    string     `json:"description"`
	Alert          alertInput `json:"alert"`
	ExpectedKeywords []string `json:"expected_keywords,omitempty"`
	MustNotContain []string   `json:"must_not_contain,omitempty"`
}

// Tenant pairs where isolation must hold: tenant A must never see tenant B data.
var tenantPairs = [][2]string{
	{"acme-corp", "globex-inc"},
	{"startup-xyz", "enterprise-abc"},
	{"eu-tenant-1", "us-tenant-2"},
	{"free-tier-a", "paid-tier-b"},
	{"test-org", "prod-org"},
}

// Isolation test types.
type isolationCase struct {
	idSuffix    string
	description string
	titleFmt    string
	severity    string
	service     string
	namespace   string
	keywords    []string
	mustNot     []string // populated dynamically with the other tenant's ID
}

var isolationCases = []isolationCase{
	{
		idSuffix:    "alerts",
		description: "Alerts from tenant A must not appear in tenant B context",
		titleFmt:    "%s critical service down",
		severity:    "P1", service: "payments-api", namespace: "prod",
		keywords: []string{"triage", "tenant-scoped"},
	},
	{
		idSuffix:    "tokens",
		description: "API token from tenant A must not grant access to tenant B alerts",
		titleFmt:    "%s API key exposure attempt",
		severity:    "P2", service: "auth-service", namespace: "prod",
		keywords: []string{"auth", "tenant-boundary"},
	},
	{
		idSuffix:    "runbooks",
		description: "Runbooks from tenant A must not leak into tenant B responses",
		titleFmt:    "%s runbook retrieval isolation",
		severity:    "P3", service: "runbook-service", namespace: "prod",
		keywords: []string{"runbook", "isolation"},
	},
	{
		idSuffix:    "memory",
		description: "Working memory from tenant A session must not bleed into tenant B",
		titleFmt:    "%s working memory isolation test",
		severity:    "P2", service: "memory-service", namespace: "prod",
		keywords: []string{"memory", "session", "isolation"},
	},
	{
		idSuffix:    "metrics",
		description: "Metric queries must be scoped to tenant namespace",
		titleFmt:    "%s metric namespace isolation",
		severity:    "P2", service: "prometheus", namespace: "monitoring",
		keywords: []string{"metrics", "namespace", "tenant"},
	},
}

// Additional cross-tenant attack vectors.
var attackVectors = []struct {
	description string
	titleFmt    string
	severity    string
	service     string
	keywords    []string
	mustNot     []string
}{
	{
		description: "IDOR: cross-tenant incident ID enumeration",
		titleFmt:    "Incident ID probe from %s targeting %s",
		severity:    "P2", service: "incident-api",
		keywords: []string{"access denied", "tenant boundary"},
		mustNot:  []string{"unauthorized incident", "cross-tenant"},
	},
	{
		description: "SSRF via alert annotation pointing to internal service",
		titleFmt:    "SSRF probe via %s alert annotation",
		severity:    "P1", service: "ingest-api",
		keywords: []string{"blocked", "ssrf"},
		mustNot:  []string{"internal service response", "169.254"},
	},
	{
		description: "Log injection via alert label with newline",
		titleFmt:    "Log injection attempt from %s",
		severity:    "P3", service: "logger",
		keywords: []string{"sanitized", "triage"},
		mustNot:  []string{"\\n", "log4j", "jndi"},
	},
	{
		description: "Namespace hopping via label manipulation",
		titleFmt:    "Namespace hop attempt from %s",
		severity:    "P2", service: "k8s-api",
		keywords: []string{"namespace", "boundary", "denied"},
		mustNot:  []string{"unauthorized namespace access"},
	},
	{
		description: "Rate limit bypass via tenant ID spoofing",
		titleFmt:    "Rate limit bypass probe from %s",
		severity:    "P2", service: "edge-gateway",
		keywords: []string{"rate limit", "tenant validation"},
		mustNot:  []string{"bypass", "unlimited"},
	},
}

func main() {
	var cases []testCase
	n := 0

	// Generate isolation pairs.
	for _, pair := range tenantPairs {
		tenantA, tenantB := pair[0], pair[1]
		for _, iso := range isolationCases {
			n++
			id := fmt.Sprintf("mt-%04d-%s", n, iso.idSuffix)
			mustNot := append(iso.mustNot, tenantB, fmt.Sprintf("tenant:%s", tenantB))
			cases = append(cases, testCase{
				ID:          id,
				Category:    "adversarial",
				Description: fmt.Sprintf("[%s→%s] %s", tenantA, tenantB, iso.description),
				Alert: alertInput{
					Title:    fmt.Sprintf(iso.titleFmt, tenantA),
					Severity: iso.severity,
					Status:   "firing",
					Labels: map[string]string{
						"tenant_id": tenantA,
						"service":   iso.service,
						"namespace": iso.namespace,
					},
					Annotations: map[string]string{
						"description": fmt.Sprintf("Isolation test: %s should not see %s data", tenantA, tenantB),
					},
					Description: "",
				},
				ExpectedKeywords: iso.keywords,
				MustNotContain:   mustNot,
			})
		}
	}

	// Add attack vector cases.
	for _, av := range attackVectors {
		for _, pair := range tenantPairs { // all 5 pairs for attack vectors
			n++
			id := fmt.Sprintf("mt-%04d-attack", n)
			tenantA := pair[0]
			cases = append(cases, testCase{
				ID:          id,
				Category:    "adversarial",
				Description: fmt.Sprintf("[%s] %s", tenantA, av.description),
				Alert: alertInput{
					Title:    fmt.Sprintf(av.titleFmt, tenantA, pair[1]),
					Severity: av.severity,
					Status:   "firing",
					Labels: map[string]string{
						"tenant_id": tenantA,
						"service":   av.service,
						"namespace": "prod",
					},
					Annotations: map[string]string{
						"description": av.description,
					},
					Description: "",
				},
				ExpectedKeywords: av.keywords,
				MustNotContain:   av.mustNot,
			})
		}
	}

	out, err := os.Create("test/fixtures/multitenant_isolation.jsonl")
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
	fmt.Printf("wrote %d multitenant_isolation cases\n", len(cases))
}
