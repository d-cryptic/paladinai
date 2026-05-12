// credentials.go implements Stage 11 §16: CI/CD environment variable credential
// resolution for integrations.
//
// In CI environments (GitHub Actions, ArgoCD, Terraform) the OS keychain is
// unavailable. Integration secrets are instead passed as environment variables
// following the pattern:
//
//	PALADIN_{INTEGRATION}_{KEY_NAME}
//
// Examples:
//
//	PALADIN_DATADOG_API_KEY
//	PALADIN_PAGERDUTY_API_KEY
//	PALADIN_SLACK_BOT_TOKEN
//	PALADIN_PROMETHEUS_BEARER_TOKEN
//	PALADIN_GRAFANA_API_KEY
//
// The CLI reads env vars before falling back to the keychain.
package integrationpkg

import (
	"fmt"
	"os"
	"strings"
)

// CredentialKey returns the environment variable name for a given integration
// and key. Example: CredentialKey("datadog", "api_key") → "PALADIN_DATADOG_API_KEY".
func CredentialKey(integration, key string) string {
	integ := strings.ToUpper(strings.ReplaceAll(integration, "-", "_"))
	k := strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
	return fmt.Sprintf("PALADIN_%s_%s", integ, k)
}

// ResolveCredential returns the secret for the given integration and key name.
// It checks the environment variable PALADIN_{INTEGRATION}_{KEY} first, then
// returns (value, true) if found or ("", false) if not set.
func ResolveCredential(integration, key string) (string, bool) {
	envKey := CredentialKey(integration, key)
	val := os.Getenv(envKey)
	if val != "" {
		return val, true
	}
	return "", false
}

// MustResolveCredential returns the credential value or returns an error if
// the environment variable is not set. Suitable for startup validation.
func MustResolveCredential(integration, key string) (string, error) {
	val, ok := ResolveCredential(integration, key)
	if !ok {
		return "", fmt.Errorf("missing credential: set %s environment variable", CredentialKey(integration, key))
	}
	return val, nil
}

// ResolveAll returns a map of key → value for all environment variables matching
// the PALADIN_{INTEGRATION}_ prefix. Keys are lowercased without the prefix.
// This allows a caller to discover all credentials for an integration at once.
func ResolveAll(integration string) map[string]string {
	prefix := fmt.Sprintf("PALADIN_%s_", strings.ToUpper(strings.ReplaceAll(integration, "-", "_")))
	result := make(map[string]string)
	for _, env := range os.Environ() {
		k, v, found := strings.Cut(env, "=")
		if !found {
			continue
		}
		if strings.HasPrefix(k, prefix) {
			shortKey := strings.ToLower(k[len(prefix):])
			result[shortKey] = v
		}
	}
	return result
}
