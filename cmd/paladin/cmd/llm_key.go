package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

var llmKeyCmd = &cobra.Command{
	Use:   "llm-key",
	Short: "Manage BYOK LLM provider keys",
}

var llmKeySetCmd = &cobra.Command{
	Use:   "set <provider>",
	Short: "Store an LLM provider API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, err := normalizeLLMProvider(args[0])
		if err != nil {
			return err
		}
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}
		key, _ := cmd.Flags().GetString("key")
		if key == "" {
			if isCIMode(cmd) {
				return fmt.Errorf("key is required in CI mode — set --key")
			}
			fmt.Fprintf(os.Stdout, "%s API key: ", provider)
			read, readErr := readLine(bufio.NewReader(os.Stdin))
			if readErr != nil {
				return fmt.Errorf("read llm key: %w", readErr)
			}
			key = read
		}
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("key is required")
		}
		if err := saveLLMKey(tenant, provider, key); err != nil {
			return err
		}
		return writeLLMKeyResult(cmd, llmKeyResult{
			TenantID:   tenant,
			Provider:   provider,
			Configured: true,
			Status:     "stored",
		})
	},
}

var llmKeyStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show BYOK LLM provider key status",
	RunE: func(cmd *cobra.Command, _ []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}
		status, err := llmKeyStatus(tenant)
		if err != nil {
			return err
		}
		return writeLLMKeyStatus(cmd, llmKeyStatusResult{TenantID: tenant, Providers: status})
	},
}

var llmKeyDeleteCmd = &cobra.Command{
	Use:   "delete <provider>",
	Short: "Delete an LLM provider API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, err := normalizeLLMProvider(args[0])
		if err != nil {
			return err
		}
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}
		if err := deleteLLMKey(tenant, provider); err != nil {
			return err
		}
		return writeLLMKeyResult(cmd, llmKeyResult{
			TenantID:   tenant,
			Provider:   provider,
			Configured: false,
			Status:     "deleted",
		})
	},
}

type llmKeyResult struct {
	TenantID   string `json:"tenant_id"`
	Provider   string `json:"provider"`
	Configured bool   `json:"configured"`
	Status     string `json:"status"`
}

type llmProviderStatus struct {
	Provider   string `json:"provider"`
	Configured bool   `json:"configured"`
}

type llmKeyStatusResult struct {
	TenantID  string              `json:"tenant_id"`
	Providers []llmProviderStatus `json:"providers"`
}

func normalizeLLMProvider(provider string) (string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "anthropic", "openrouter", "openai":
		return provider, nil
	default:
		return "", fmt.Errorf("unsupported LLM provider %q — use anthropic, openrouter, or openai", provider)
	}
}

func llmKeyAccount(tenant, provider string) string {
	return "llm-keys/" + tenant + "/" + provider
}

func saveLLMKey(tenant, provider, key string) error {
	if err := paladinSecretStore.Set(tokenStoreService, llmKeyAccount(tenant, provider), strings.TrimSpace(key)); err != nil {
		return fmt.Errorf("save llm key to keychain: %w", err)
	}
	return nil
}

func deleteLLMKey(tenant, provider string) error {
	if err := paladinSecretStore.Delete(tokenStoreService, llmKeyAccount(tenant, provider)); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("delete llm key from keychain: %w", err)
	}
	return nil
}

func llmKeyStatus(tenant string) ([]llmProviderStatus, error) {
	providers := []string{"anthropic", "openrouter", "openai"}
	sort.Strings(providers)
	status := make([]llmProviderStatus, 0, len(providers))
	for _, provider := range providers {
		_, err := paladinSecretStore.Get(tokenStoreService, llmKeyAccount(tenant, provider))
		if err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return nil, fmt.Errorf("read llm key from keychain: %w", err)
		}
		status = append(status, llmProviderStatus{
			Provider:   provider,
			Configured: err == nil,
		})
	}
	return status, nil
}

func writeLLMKeyResult(cmd *cobra.Command, result llmKeyResult) error {
	if outputFormat(cmd) == "json" {
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return fmt.Errorf("write llm key result: %w", err)
		}
		return nil
	}
	fmt.Fprintf(os.Stdout, "%s key %s for tenant %s.\n", result.Provider, result.Status, result.TenantID)
	return nil
}

func writeLLMKeyStatus(cmd *cobra.Command, result llmKeyStatusResult) error {
	if outputFormat(cmd) == "json" {
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return fmt.Errorf("write llm key status: %w", err)
		}
		return nil
	}
	fmt.Fprintf(os.Stdout, "LLM keys for tenant %s\n", result.TenantID)
	for _, provider := range result.Providers {
		state := "missing"
		if provider.Configured {
			state = "configured"
		}
		fmt.Fprintf(os.Stdout, "  %-10s %s\n", provider.Provider, state)
	}
	return nil
}

func init() {
	llmKeySetCmd.Flags().String("key", "", "Provider API key; omit to read interactively")
	llmKeyCmd.AddCommand(llmKeySetCmd, llmKeyStatusCmd, llmKeyDeleteCmd)
	authCmd.AddCommand(llmKeyCmd)
}
