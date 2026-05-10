package auth_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/internal/auth"
)

const (
	testSecret = "test-secret-32-bytes-for-hs256!!"
	weakSecret = "short"
)

func issueTestToken(t *testing.T, secret, tenantID, userID string, roles []string, ttl time.Duration) string {
	t.Helper()
	tok, err := auth.IssueToken([]byte(secret), tenantID, userID, roles, ttl)
	require.NoError(t, err)
	return tok
}

// ── ValidateSecret ────────────────────────────────────────────────────────────

func TestValidateSecret_TooShort(t *testing.T) {
	assert.ErrorIs(t, auth.ValidateSecret([]byte(weakSecret)), auth.ErrWeakSecret)
}

func TestValidateSecret_ExactlyMinimumLength(t *testing.T) {
	secret := strings.Repeat("x", 32)
	assert.NoError(t, auth.ValidateSecret([]byte(secret)))
}

func TestValidateSecret_LongerThanMinimum(t *testing.T) {
	secret := strings.Repeat("y", 64)
	assert.NoError(t, auth.ValidateSecret([]byte(secret)))
}

// ── IssueToken ────────────────────────────────────────────────────────────────

func TestIssueToken_Success(t *testing.T) {
	tok := issueTestToken(t, testSecret, "acme-corp", "user-1", []string{"admin"}, time.Hour)
	assert.NotEmpty(t, tok)
	// JWT format: header.payload.signature
	parts := strings.Split(tok, ".")
	assert.Len(t, parts, 3)
}

func TestIssueToken_WeakSecretReturnsError(t *testing.T) {
	_, err := auth.IssueToken([]byte(weakSecret), "t1", "u1", nil, time.Hour)
	assert.ErrorIs(t, err, auth.ErrWeakSecret)
}

// ── ParseClaims ───────────────────────────────────────────────────────────────

func TestParseClaims_ValidToken(t *testing.T) {
	tok := issueTestToken(t, testSecret, "acme-corp", "user-1", []string{"admin", "viewer"}, time.Hour)

	claims, err := auth.ParseClaims(tok, []byte(testSecret))
	require.NoError(t, err)

	assert.Equal(t, "acme-corp", claims.TenantID)
	assert.Equal(t, "user-1", claims.Subject)
	assert.Equal(t, []string{"admin", "viewer"}, claims.Roles)
}

func TestParseClaims_ExpiredToken(t *testing.T) {
	tok := issueTestToken(t, testSecret, "acme-corp", "user-1", nil, -time.Second)

	_, err := auth.ParseClaims(tok, []byte(testSecret))
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestParseClaims_WrongSecret(t *testing.T) {
	tok := issueTestToken(t, testSecret, "t1", "u1", nil, time.Hour)

	other := strings.Repeat("z", 32)
	_, err := auth.ParseClaims(tok, []byte(other))
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestParseClaims_MalformedToken(t *testing.T) {
	_, err := auth.ParseClaims("not.a.jwt", []byte(testSecret))
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestParseClaims_EmptyToken(t *testing.T) {
	_, err := auth.ParseClaims("", []byte(testSecret))
	assert.Error(t, err)
}

func TestParseClaims_WeakSecret(t *testing.T) {
	_, err := auth.ParseClaims("anything", []byte(weakSecret))
	assert.ErrorIs(t, err, auth.ErrWeakSecret)
}

// ── Context helpers ───────────────────────────────────────────────────────────

func TestTenantIDFromContext_SetAndGet(t *testing.T) {
	ctx := auth.WithTenantID(context.Background(), "acme-corp")
	got, ok := auth.TenantIDFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, "acme-corp", got)
}

func TestTenantIDFromContext_Missing(t *testing.T) {
	_, ok := auth.TenantIDFromContext(context.Background())
	assert.False(t, ok)
}

func TestTenantIDFromContext_Empty(t *testing.T) {
	ctx := auth.WithTenantID(context.Background(), "")
	_, ok := auth.TenantIDFromContext(ctx)
	assert.False(t, ok, "empty tenant ID should not be considered present")
}

func TestUserIDFromContext_SetAndGet(t *testing.T) {
	ctx := auth.WithUserID(context.Background(), "user-42")
	got, ok := auth.UserIDFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, "user-42", got)
}

func TestUserIDFromContext_Missing(t *testing.T) {
	_, ok := auth.UserIDFromContext(context.Background())
	assert.False(t, ok)
}

func TestRolesFromContext_SetAndGet(t *testing.T) {
	ctx := auth.WithRoles(context.Background(), []string{"admin", "viewer"})
	roles := auth.RolesFromContext(ctx)
	assert.Equal(t, []string{"admin", "viewer"}, roles)
}

func TestRolesFromContext_Missing(t *testing.T) {
	roles := auth.RolesFromContext(context.Background())
	assert.Nil(t, roles)
}

// ── Round-trip ────────────────────────────────────────────────────────────────

func TestIssueAndParseClaims_RoundTrip(t *testing.T) {
	tenant := "round-trip-tenant"
	user := "round-trip-user"
	roles := []string{"viewer"}

	tok := issueTestToken(t, testSecret, tenant, user, roles, time.Hour)
	claims, err := auth.ParseClaims(tok, []byte(testSecret))
	require.NoError(t, err)

	assert.Equal(t, tenant, claims.TenantID)
	assert.Equal(t, user, claims.Subject)
	assert.Equal(t, roles, claims.Roles)
	assert.False(t, claims.ExpiresAt.IsZero())
	assert.True(t, claims.ExpiresAt.After(time.Now()))
}
