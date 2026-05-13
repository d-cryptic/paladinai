package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/paladinai/paladinai/cmd/paladin/client"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

const tokenStoreService = "paladin"

type secretStore interface {
	Get(service, user string) (string, error)
	Set(service, user, secret string) error
	Delete(service, user string) error
}

type keyringSecretStore struct{}

func (keyringSecretStore) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (keyringSecretStore) Set(service, user, secret string) error {
	return keyring.Set(service, user, secret)
}

func (keyringSecretStore) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

var paladinSecretStore secretStore = keyringSecretStore{}

func tokenStoreAccount(authEndpoint string) string {
	endpoint := strings.TrimSpace(strings.TrimRight(authEndpoint, "/"))
	if endpoint == "" {
		endpoint = "default"
	}
	return "tokens/" + endpoint
}

func loadStoredToken(cfg *PaladinConfig, authEndpoint string) (string, error) {
	token, err := paladinSecretStore.Get(tokenStoreService, tokenStoreAccount(authEndpoint))
	if err == nil {
		return token, nil
	}
	if !errors.Is(err, keyring.ErrNotFound) {
		return "", fmt.Errorf("read token from keychain: %w", err)
	}
	if cfg != nil && cfg.Token != "" {
		return cfg.Token, nil
	}
	return "", nil
}

func saveStoredToken(authEndpoint, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	if err := paladinSecretStore.Set(tokenStoreService, tokenStoreAccount(authEndpoint), token); err != nil {
		return fmt.Errorf("save token to keychain: %w", err)
	}
	return nil
}

func deleteStoredToken(authEndpoint string) error {
	if err := paladinSecretStore.Delete(tokenStoreService, tokenStoreAccount(authEndpoint)); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("delete token from keychain: %w", err)
	}
	return nil
}

func commandToken(cmd *cobra.Command) (string, error) {
	if token := optToken(cmd); token != "" {
		return token, nil
	}
	if token := os.Getenv("PALADIN_TOKEN"); token != "" {
		return token, nil
	}
	cfg, err := loadConfig()
	if err != nil {
		return "", fmt.Errorf("load config for token: %w", err)
	}
	if cfg == nil {
		return "", nil
	}
	return loadStoredToken(cfg, cfg.AuthEndpoint)
}

func commandOptions(cmd *cobra.Command, tenantID string) (client.Options, error) {
	token, err := commandToken(cmd)
	if err != nil {
		return client.Options{}, err
	}
	return client.Options{TenantID: tenantID, Token: token}, nil
}
