package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func validPaladinYAML() paladinYAML {
	var cfg paladinYAML
	cfg.APIVersion = "paladin.io/v2"
	cfg.Kind = "Config"
	cfg.Metadata.Tenant = "acme-corp"
	cfg.Spec.Region = "us-east-1"
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
	cfg.Spec.Integrations = []Integration{{Name: "", Version: "1.0", Enabled: true}}
	errs := validateConfig(cfg)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "name")
}

func TestValidateConfig_IntegrationMissingVersion_Error(t *testing.T) {
	cfg := validPaladinYAML()
	cfg.Spec.Integrations = []Integration{{Name: "datadog", Version: "", Enabled: true}}
	errs := validateConfig(cfg)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "version")
}

func TestValidateConfig_MultipleErrors(t *testing.T) {
	var cfg paladinYAML // all zero values
	errs := validateConfig(cfg)
	assert.GreaterOrEqual(t, len(errs), 3, "should report apiVersion, kind, tenant, region errors")
}

func TestValidateConfig_ValidIntegrations(t *testing.T) {
	cfg := validPaladinYAML()
	cfg.Spec.Integrations = []Integration{
		{Name: "prometheus", Version: "2.0", Enabled: true},
		{Name: "loki", Version: "3.0", Enabled: false},
	}
	errs := validateConfig(cfg)
	assert.Empty(t, errs)
}
