package auth_test

import (
	"context"
	"encoding/base64"
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

func b64url(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
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
	tok := issueTestToken(t, testSecret, "acme-corp", "user-1", nil, -time.Minute)

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
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestParseClaims_WeakSecret(t *testing.T) {
	_, err := auth.ParseClaims("anything", []byte(weakSecret))
	assert.ErrorIs(t, err, auth.ErrWeakSecret)
}

// TestParseClaims_AlgNoneRejected ensures tokens claiming "alg":"none" are rejected.
// This defends against the JWT algorithm-confusion attack.
func TestParseClaims_AlgNoneRejected(t *testing.T) {
	header := b64url(`{"alg":"none","typ":"JWT"}`)
	payload := b64url(`{"sub":"u1","tenant_id":"t1","exp":9999999999,"iss":"paladin-auth","aud":["paladin"]}`)
	tok := header + "." + payload + "." // unsigned

	_, err := auth.ParseClaims(tok, []byte(testSecret))
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

// TestParseClaims_WrongAlgorithmRejected ensures tokens claiming a non-HS256 algorithm
// are rejected even when the rest of the token looks structurally valid.
func TestParseClaims_WrongAlgorithmRejected(t *testing.T) {
	valid := issueTestToken(t, testSecret, "t1", "u1", nil, time.Hour)
	parts := strings.Split(valid, ".")

	// Swap header to claim HS384; original signature is over the old header so it will be invalid.
	wrongAlgHeader := b64url(`{"alg":"HS384","typ":"JWT"}`)
	tamperedTok := wrongAlgHeader + "." + parts[1] + "." + parts[2]

	_, err := auth.ParseClaims(tamperedTok, []byte(testSecret))
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

// TestParseClaims_TamperedPayload ensures a token with a modified payload but the
// original (now-invalid) signature is rejected.
func TestParseClaims_TamperedPayload(t *testing.T) {
	valid := issueTestToken(t, testSecret, "acme-corp", "u1", nil, time.Hour)
	parts := strings.Split(valid, ".")

	// Flip tenant_id in the payload while keeping the original signature.
	evilPayload := b64url(`{"sub":"u1","tenant_id":"evil-corp","exp":9999999999,"iss":"paladin-auth","aud":["paladin"]}`)
	tamperedTok := parts[0] + "." + evilPayload + "." + parts[2]

	_, err := auth.ParseClaims(tamperedTok, []byte(testSecret))
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
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

// TestRolesFromContext_MutationSafe ensures callers cannot mutate the context's
// role slice through the returned reference.
func TestRolesFromContext_MutationSafe(t *testing.T) {
	ctx := auth.WithRoles(context.Background(), []string{"admin", "viewer"})
	roles := auth.RolesFromContext(ctx)
	roles[0] = "tampered"
	again := auth.RolesFromContext(ctx)
	assert.Equal(t, "admin", again[0], "mutating returned slice must not affect context")
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
