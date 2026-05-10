package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/services/paladin-auth/config"
)

func clearAuthEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"JWT_SECRET", "ADMIN_SECRET", "TOKEN_TTL"} {
		t.Setenv(k, "")
	}
}

func TestLoad_DevFallbackSecretWhenJWTSecretUnset(t *testing.T) {
	clearAuthEnv(t)

	c, err := config.Load()
	require.NoError(t, err)
	// Fallback is a dev-only default; it must still satisfy the ≥32-byte requirement.
	assert.GreaterOrEqual(t, len(c.JWTSecret), 32, "dev fallback secret must be ≥32 bytes")
}

func TestLoad_JWTSecretFromEnv(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("JWT_SECRET", "production-secret-that-is-32bytes!!")

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "production-secret-that-is-32bytes!!", c.JWTSecret)
}

func TestLoad_DefaultTokenTTL(t *testing.T) {
	clearAuthEnv(t)

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, c.TokenTTL)
}

func TestLoad_TokenTTLFromEnv(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("TOKEN_TTL", "1h30m")

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 90*time.Minute, c.TokenTTL)
}

func TestLoad_InvalidTokenTTLReturnsError(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("TOKEN_TTL", "not-a-duration")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TOKEN_TTL")
}

func TestLoad_AdminSecretFromEnv(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("ADMIN_SECRET", "super-secret-admin")

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "super-secret-admin", c.AdminSecret)
}

func TestLoad_AdminSecretEmptyByDefault(t *testing.T) {
	clearAuthEnv(t)

	c, err := config.Load()
	require.NoError(t, err)
	assert.Empty(t, c.AdminSecret)
}

func TestLoad_ServerPortIsFixed(t *testing.T) {
	clearAuthEnv(t)

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 9003, c.Server.Port)
}

func TestLoad_ServerTimeoutsNonZero(t *testing.T) {
	clearAuthEnv(t)

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, c.Server.ReadTimeout)
	assert.Equal(t, 10*time.Second, c.Server.WriteTimeout)
	assert.Equal(t, 60*time.Second, c.Server.IdleTimeout)
	assert.Equal(t, 10*time.Second, c.Server.ShutdownTimeout)
}
