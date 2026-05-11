// White-box tests: package db gives access to tenantIDKey.
package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithTenantID_ValueIsAccessible(t *testing.T) {
	t.Parallel()
	ctx := WithTenantID(context.Background(), "acme-corp")
	got, ok := ctx.Value(tenantIDKey{}).(string)
	assert.True(t, ok, "value should be a string")
	assert.Equal(t, "acme-corp", got)
}

func TestWithTenantID_EmptyID_IsStored(t *testing.T) {
	t.Parallel()
	// An empty tenant ID is a valid (if unusual) value; the caller controls validity.
	ctx := WithTenantID(context.Background(), "")
	got, ok := ctx.Value(tenantIDKey{}).(string)
	require.True(t, ok, "value must be present even when empty")
	assert.Equal(t, "", got)
}

func TestWithTenantID_OverridesParentValue(t *testing.T) {
	t.Parallel()
	parent := WithTenantID(context.Background(), "tenant-a")
	child := WithTenantID(parent, "tenant-b")
	got, _ := child.Value(tenantIDKey{}).(string)
	assert.Equal(t, "tenant-b", got, "child context should shadow the parent value")
}

func TestWithTenantID_ParentUnmodified(t *testing.T) {
	t.Parallel()
	parent := WithTenantID(context.Background(), "tenant-a")
	_ = WithTenantID(parent, "tenant-b")
	got, _ := parent.Value(tenantIDKey{}).(string)
	assert.Equal(t, "tenant-a", got, "creating a child context must not modify the parent")
}

func TestWithTenantID_BaseContextLacksValue(t *testing.T) {
	t.Parallel()
	// Verify that Background() has no tenant ID — the test above's parent check depends on this.
	_, ok := context.Background().Value(tenantIDKey{}).(string)
	assert.False(t, ok, "base context must not carry a tenant ID before WithTenantID is called")
}
