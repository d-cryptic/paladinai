package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newLLMKeyTestCmd() *cobra.Command {
	c := &cobra.Command{Use: "llm-key"}
	c.PersistentFlags().String("tenant", "tenant-a", "")
	c.PersistentFlags().String("output", "table", "")
	c.PersistentFlags().Bool("ci", false, "")
	return c
}

func TestNormalizeLLMProvider(t *testing.T) {
	got, err := normalizeLLMProvider(" OpenRouter ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "openrouter" {
		t.Fatalf("provider = %q, want openrouter", got)
	}
	if _, err := normalizeLLMProvider("bedrock"); err == nil {
		t.Fatal("expected unsupported provider error")
	}
}

func TestLLMKeySetStoresSecretWithoutPrintingIt(t *testing.T) {
	store := &fakeSecretStore{}
	withSecretStore(t, store)
	cmd := newLLMKeyTestCmd()
	cmd.RunE = llmKeySetCmd.RunE
	cmd.Flags().String("key", "", "")
	if err := cmd.Flags().Set("key", "sk-secret"); err != nil {
		t.Fatal(err)
	}

	stdout := captureStdout(t, func() {
		if err := cmd.RunE(cmd, []string{"openrouter"}); err != nil {
			t.Fatal(err)
		}
	})

	if strings.Contains(stdout, "sk-secret") {
		t.Fatalf("stdout exposed secret: %s", stdout)
	}
	got, err := paladinSecretStore.Get(tokenStoreService, llmKeyAccount("tenant-a", "openrouter"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "sk-secret" {
		t.Fatalf("stored key = %q, want sk-secret", got)
	}
}

func TestLLMKeySetCIModeRequiresKeyFlag(t *testing.T) {
	cmd := newLLMKeyTestCmd()
	cmd.RunE = llmKeySetCmd.RunE
	cmd.Flags().String("key", "", "")
	if err := cmd.PersistentFlags().Set("ci", "true"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, []string{"anthropic"})
	if err == nil {
		t.Fatal("expected missing key error")
	}
	if !strings.Contains(err.Error(), "--key") {
		t.Fatalf("error = %q, want --key hint", err.Error())
	}
}

func TestLLMKeyStatusJSON(t *testing.T) {
	store := &fakeSecretStore{}
	withSecretStore(t, store)
	if err := saveLLMKey("tenant-a", "openrouter", "sk-secret"); err != nil {
		t.Fatal(err)
	}
	cmd := newLLMKeyTestCmd()
	if err := cmd.PersistentFlags().Set("output", "json"); err != nil {
		t.Fatal(err)
	}

	stdout := captureStdout(t, func() {
		status, err := llmKeyStatus("tenant-a")
		if err != nil {
			t.Fatal(err)
		}
		if err := writeLLMKeyStatus(cmd, llmKeyStatusResult{TenantID: "tenant-a", Providers: status}); err != nil {
			t.Fatal(err)
		}
	})

	var result llmKeyStatusResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if result.TenantID != "tenant-a" {
		t.Fatalf("tenant = %q, want tenant-a", result.TenantID)
	}
	found := false
	for _, provider := range result.Providers {
		if provider.Provider == "openrouter" {
			found = true
			if !provider.Configured {
				t.Fatal("openrouter should be configured")
			}
		}
	}
	if !found {
		t.Fatal("openrouter status missing")
	}
	if strings.Contains(stdout, "sk-secret") {
		t.Fatalf("status exposed secret: %s", stdout)
	}
}

func TestDeleteLLMKeyRemovesSecret(t *testing.T) {
	store := &fakeSecretStore{}
	withSecretStore(t, store)
	if err := saveLLMKey("tenant-a", "anthropic", "sk-secret"); err != nil {
		t.Fatal(err)
	}
	if err := deleteLLMKey("tenant-a", "anthropic"); err != nil {
		t.Fatal(err)
	}
	status, err := llmKeyStatus("tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	for _, provider := range status {
		if provider.Provider == "anthropic" && provider.Configured {
			t.Fatal("anthropic key should be deleted")
		}
	}
}
