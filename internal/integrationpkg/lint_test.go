package integrationpkg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLint_BundledIntegrationsPass(t *testing.T) {
	all, err := LoadAll(integrationsDir(t))
	require.NoError(t, err)

	var issues []LintIssue
	for _, integ := range all {
		issues = append(issues, Lint(integ)...)
	}

	assert.False(t, HasLintErrors(issues), "issues: %#v", issues)
}

func TestLint_RejectsInvalidIntegration(t *testing.T) {
	issues := Lint(Integration{
		Name:        "Bad_Name",
		Version:     "1.0.0",
		Description: "anytime",
		Receiver: ReceiverConfig{
			Type: "webhook",
			Path: "missing-slash",
		},
		Auth: AuthConfig{
			Type: "unsupported",
			Fields: []AuthField{
				{Name: "Bad Field"},
			},
		},
		ConfigSchema: map[string]Field{
			"BadKey": {Type: "mystery"},
		},
		Tools:   []string{"query_metrics", "query_metrics", "Bad Tool"},
		DocsURL: "not a url",
	})

	require.True(t, HasLintErrors(issues))
	assertContainsLint(t, issues, "name")
	assertContainsLint(t, issues, "description")
	assertContainsLint(t, issues, "receiver.path")
	assertContainsLint(t, issues, "auth.type")
	assertContainsLint(t, issues, "auth.fields[0].name")
	assertContainsLint(t, issues, "config_schema.BadKey")
	assertContainsLint(t, issues, "tools[1]")
	assertContainsLint(t, issues, "tools[2]")
	assertContainsLint(t, issues, "docs_url")
}

func TestLint_AllowsReceiverNoneWithoutPath(t *testing.T) {
	issues := Lint(Integration{
		Name:        "kubernetes",
		Version:     "1.0.0",
		Description: "Kubernetes cluster state lookup for incident response",
		Receiver:    ReceiverConfig{Type: "none"},
		Auth:        AuthConfig{Type: "kubeconfig"},
		Tools:       []string{"get_pod_logs"},
		DocsURL:     "https://kubernetes.io/docs/reference/kubectl/",
	})

	assert.False(t, HasLintErrors(issues), "issues: %#v", issues)
}

func assertContainsLint(t *testing.T, issues []LintIssue, field string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Field == field {
			return
		}
	}
	t.Fatalf("expected issue for field %q in %#v", field, issues)
}
