package cmd

import (
	"testing"

	"github.com/paladinai/paladinai/internal/projectconfig"
	"github.com/stretchr/testify/assert"
)

func validPaladinYAML() projectconfig.Config {
	var cfg projectconfig.Config
	cfg.APIVersion = "paladin.io/v2"
	cfg.Kind = "Config"
	cfg.Metadata.Tenant = "acme-corp"
	cfg.Spec.Region = "us-east-1"
	cfg.Spec.Routing.P1 = projectconfig.SeverityRoute{LLMTier: "C", ApprovalPolicy: "auto"}
	return cfg
}

func TestValidateConfig_ValidConfig_NoErrors(t *testing.T) {
	errs := validateConfig(validPaladinYAML())
	assert.Empty(t, errs)
}

func TestValidateConfig_WrongAPIVersion_Error(t *testing.T) {
	cfg := validPaladinYAML()
	cfg.APIVersion = "v1"
	errs := validateConfig(cfg)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "apiVersion")
}

func TestValidateConfig_WrongKind_Error(t *testing.T) {
	cfg := validPaladinYAML()
	cfg.Kind = "Service"
	errs := validateConfig(cfg)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "kind")
}

func TestValidateConfig_EmptyTenant_Error(t *testing.T) {
	cfg := validPaladinYAML()
	cfg.Metadata.Tenant = ""
	errs := validateConfig(cfg)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "tenant")
}

func TestValidateConfig_EmptyRegion_Error(t *testing.T) {
	cfg := validPaladinYAML()
	cfg.Spec.Region = ""
	errs := validateConfig(cfg)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "region")
}

func TestValidateConfig_InvalidTier_Error(t *testing.T) {
	cfg := validPaladinYAML()
	cfg.Metadata.Tier = "enterprise"
	errs := validateConfig(cfg)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "tier")
}

func TestValidateConfig_ValidTiers_NoError(t *testing.T) {
	for _, tier := range []string{"pool", "bridge", "silo", ""} {
		cfg := validPaladinYAML()
		cfg.Metadata.Tier = tier
		errs := validateConfig(cfg)
		assert.Empty(t, errs, "tier=%q should be valid", tier)
	}
}

func TestValidateConfig_IntegrationMissingName_Error(t *testing.T) {
	cfg := validPaladinYAML()
	cfg.Spec.Integrations = []projectconfig.Integration{{Name: "", Version: "1.0", Enabled: true}}
	errs := validateConfig(cfg)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "name")
}

func TestValidateConfig_IntegrationMissingVersion_Error(t *testing.T) {
	cfg := validPaladinYAML()
	cfg.Spec.Integrations = []projectconfig.Integration{{Name: "datadog", Version: "", Enabled: true}}
	errs := validateConfig(cfg)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "version")
}

func TestValidateConfig_MultipleErrors(t *testing.T) {
	var cfg projectconfig.Config // all zero values
	errs := validateConfig(cfg)
	assert.GreaterOrEqual(t, len(errs), 3, "should report apiVersion, kind, tenant, region errors")
}

func TestValidateConfig_ValidIntegrations(t *testing.T) {
	cfg := validPaladinYAML()
	cfg.Spec.Integrations = []projectconfig.Integration{
		{Name: "prometheus", Version: "2.0", Enabled: true},
		{Name: "loki", Version: "3.0", Enabled: false},
	}
	errs := validateConfig(cfg)
	assert.Empty(t, errs)
}

func TestValidateConfig_InvalidTenantCharacters_Error(t *testing.T) {
	cfg := validPaladinYAML()
	cfg.Metadata.Tenant = "tenant\r\nx-bad: value"
	errs := validateConfig(cfg)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "invalid characters")
}

func TestValidateConfig_InvalidRoutingTier_Error(t *testing.T) {
	cfg := validPaladinYAML()
	cfg.Spec.Routing.P2 = projectconfig.SeverityRoute{LLMTier: "D", ApprovalPolicy: "auto"}
	errs := validateConfig(cfg)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "spec.routing.p2.llm_tier")
}

func TestValidateConfig_InvalidRoutingPolicy_Error(t *testing.T) {
	cfg := validPaladinYAML()
	cfg.Spec.Routing.P2 = projectconfig.SeverityRoute{LLMTier: "B", ApprovalPolicy: "always"}
	errs := validateConfig(cfg)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "spec.routing.p2.approval_policy")
}
