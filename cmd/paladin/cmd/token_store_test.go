package cmd

import (
	"errors"
	"os"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestMain(m *testing.M) {
	paladinSecretStore = &fakeSecretStore{}
	os.Exit(m.Run())
}

type fakeSecretStore struct {
	values map[string]string
	err    error
}

func (s *fakeSecretStore) Get(service, user string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if s.values == nil {
		return "", keyring.ErrNotFound
	}
	v, ok := s.values[service+"\x00"+user]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return v, nil
}

func (s *fakeSecretStore) Set(service, user, secret string) error {
	if s.err != nil {
		return s.err
	}
	if s.values == nil {
		s.values = make(map[string]string)
	}
	s.values[service+"\x00"+user] = secret
	return nil
}

func (s *fakeSecretStore) Delete(service, user string) error {
	if s.err != nil {
		return s.err
	}
	if s.values == nil {
		return keyring.ErrNotFound
	}
	delete(s.values, service+"\x00"+user)
	return nil
}

func withSecretStore(t *testing.T, store secretStore) {
	t.Helper()
	old := paladinSecretStore
	paladinSecretStore = store
	t.Cleanup(func() {
		paladinSecretStore = old
	})
}

func TestStoredTokenUsesKeychainBeforeLegacyConfig(t *testing.T) {
	store := &fakeSecretStore{}
	withSecretStore(t, store)

	if err := saveStoredToken("https://auth.example.com/", "keychain-token"); err != nil {
		t.Fatal(err)
	}

	got, err := loadStoredToken(&PaladinConfig{Token: "legacy-token"}, "https://auth.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != "keychain-token" {
		t.Fatalf("token = %q, want keychain-token", got)
	}
}

func TestStoredTokenFallsBackToLegacyConfig(t *testing.T) {
	withSecretStore(t, &fakeSecretStore{})

	got, err := loadStoredToken(&PaladinConfig{Token: "legacy-token"}, "https://auth.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != "legacy-token" {
		t.Fatalf("token = %q, want legacy-token", got)
	}
}

func TestStoredTokenSurfacesKeychainErrors(t *testing.T) {
	withSecretStore(t, &fakeSecretStore{err: errors.New("locked")})

	_, err := loadStoredToken(&PaladinConfig{Token: "legacy-token"}, "https://auth.example.com")
	if err == nil {
		t.Fatal("expected keychain error")
	}
}

func TestDeleteStoredTokenIgnoresMissingToken(t *testing.T) {
	withSecretStore(t, &fakeSecretStore{})

	if err := deleteStoredToken("https://auth.example.com"); err != nil {
		t.Fatal(err)
	}
}

func TestCommandTokenFallsBackToKeychain(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("PALADIN_TOKEN", "")
	store := &fakeSecretStore{}
	withSecretStore(t, store)

	cfg := &PaladinConfig{AuthEndpoint: "https://auth.example.com"}
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if err := saveStoredToken("https://auth.example.com", "stored-token"); err != nil {
		t.Fatal(err)
	}

	cmd := newTestCmd("", "", "")
	got, err := commandToken(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if got != "stored-token" {
		t.Fatalf("token = %q, want stored-token", got)
	}
}

func TestCommandOptionsOmitTenantWhenTokenConfigured(t *testing.T) {
	cmd := newTestCmd("tenant-slug", "jwt-token", "")

	opts, err := commandOptions(cmd, "tenant-slug")
	if err != nil {
		t.Fatal(err)
	}
	if opts.Token != "jwt-token" {
		t.Fatalf("Token = %q, want jwt-token", opts.Token)
	}
	if opts.TenantID != "" {
		t.Fatalf("TenantID = %q, want empty when token is configured", opts.TenantID)
	}
}

func TestCommandOptionsIncludeTenantWithoutToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PALADIN_TOKEN", "")
	withSecretStore(t, &fakeSecretStore{})
	cmd := newTestCmd("tenant-slug", "", "")

	opts, err := commandOptions(cmd, "tenant-slug")
	if err != nil {
		t.Fatal(err)
	}
	if opts.Token != "" {
		t.Fatalf("Token = %q, want empty", opts.Token)
	}
	if opts.TenantID != "tenant-slug" {
		t.Fatalf("TenantID = %q, want tenant-slug", opts.TenantID)
	}
}
