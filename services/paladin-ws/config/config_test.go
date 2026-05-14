package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/services/paladin-ws/config"
)

func clearWSEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"JWT_SECRET", "NATS_ALERTS_SUBJECT", "NATS_CONSUMER_NAME", "PALADIN_WS_PORT", "PALADIN_WS_ADMIN_PORT"} {
		t.Setenv(k, "")
	}
}

func TestLoad_MissingJWTSecretReturnsError(t *testing.T) {
	clearWSEnv(t)

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestLoad_JWTSecretSetReturnsConfig(t *testing.T) {
	clearWSEnv(t)
	t.Setenv("JWT_SECRET", "ws-secret-at-least-32-bytes!!!!!")

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, []byte("ws-secret-at-least-32-bytes!!!!!"), c.JWTSecret)
}

func TestLoad_ShortJWTSecretReturnsError(t *testing.T) {
	clearWSEnv(t)
	t.Setenv("JWT_SECRET", "tooshort") // < 32 bytes

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestLoad_DefaultNATSSubjectAndConsumer(t *testing.T) {
	clearWSEnv(t)
	t.Setenv("JWT_SECRET", "ws-secret-at-least-32-bytes!!!!!")

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "paladin.alerts.processed", c.NATSSubject)
	assert.Equal(t, "paladin-ws", c.NATSConsumer)
}

func TestLoad_NATSEnvOverrides(t *testing.T) {
	clearWSEnv(t)
	t.Setenv("JWT_SECRET", "ws-secret-at-least-32-bytes!!!!!")
	t.Setenv("NATS_ALERTS_SUBJECT", "custom.alerts")
	t.Setenv("NATS_CONSUMER_NAME", "my-consumer")

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "custom.alerts", c.NATSSubject)
	assert.Equal(t, "my-consumer", c.NATSConsumer)
}

func TestLoad_DefaultPort(t *testing.T) {
	clearWSEnv(t)
	t.Setenv("JWT_SECRET", "ws-secret-at-least-32-bytes!!!!!")

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 9007, c.Server.Port)
}

func TestLoad_DefaultAdminPortKeepsAdminRoutesOnPublicServer(t *testing.T) {
	clearWSEnv(t)
	t.Setenv("JWT_SECRET", "ws-secret-at-least-32-bytes!!!!!")

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 0, c.AdminPort)
}

func TestLoad_AdminPortOverride(t *testing.T) {
	clearWSEnv(t)
	t.Setenv("JWT_SECRET", "ws-secret-at-least-32-bytes!!!!!")
	t.Setenv("PALADIN_WS_ADMIN_PORT", "9107")

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 9107, c.AdminPort)
}

func TestLoad_InvalidAdminPortReturnsError(t *testing.T) {
	clearWSEnv(t)
	t.Setenv("JWT_SECRET", "ws-secret-at-least-32-bytes!!!!!")
	t.Setenv("PALADIN_WS_ADMIN_PORT", "not-a-port")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PALADIN_WS_ADMIN_PORT")
}

func TestLoad_OutOfRangeAdminPortReturnsError(t *testing.T) {
	clearWSEnv(t)
	t.Setenv("JWT_SECRET", "ws-secret-at-least-32-bytes!!!!!")
	t.Setenv("PALADIN_WS_ADMIN_PORT", "70000")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PALADIN_WS_ADMIN_PORT")
}
