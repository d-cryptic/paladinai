package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEpisodicMemoryMigrationEnforcesTenantIsolation(t *testing.T) {
	sql := readMigration(t, "000007_create_episodic_memory.up.sql")

	assert.Contains(t, sql, "tenant_id    UUID        NOT NULL REFERENCES tenants")
	assert.Contains(t, sql, "incident_id  UUID        NOT NULL REFERENCES incidents")
	assert.Contains(t, sql, "ALTER TABLE episodic_memory ENABLE ROW LEVEL SECURITY")
	assert.Contains(t, sql, "ALTER TABLE episodic_memory FORCE ROW LEVEL SECURITY")
	assert.Contains(t, sql, "CREATE POLICY tenant_isolation ON episodic_memory")
	assert.Contains(t, sql, "current_setting('app.tenant_id', true)")
	assert.Contains(t, sql, "CREATE POLICY service_bypass ON episodic_memory")
	assert.NotContains(t, sql, "idx_episodic_memory_fingerprint ON episodic_memory (fingerprint)")
}

func readMigration(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "migrations", name)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.TrimSpace(string(data))
}
