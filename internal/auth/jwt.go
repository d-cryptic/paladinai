// Package auth provides shared JWT primitives for all PaladinAI services.
package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrWeakSecret is returned when a JWT secret is shorter than 32 bytes.
var ErrWeakSecret = errors.New("jwt: secret must be at least 32 bytes")

// ErrInvalidToken is returned for any token validation failure.
var ErrInvalidToken = errors.New("jwt: invalid or expired token")

// Unexported struct context keys prevent collisions with other packages that might
// use the same string ("tenant_id", "user_id") via context.WithValue.
type (
	tenantIDCtxKey struct{}
	userIDCtxKey   struct{}
	rolesCtxKey    struct{}
)

// ContextKeyTenantID, ContextKeyUserID, ContextKeyRoles are the typed context
// keys used by JWTMiddleware and the From*Context helpers.
var (
	ContextKeyTenantID = tenantIDCtxKey{}
	ContextKeyUserID   = userIDCtxKey{}
	ContextKeyRoles    = rolesCtxKey{}
)

const issuer = "paladin-auth"

// Claims are the JWT payload fields PaladinAI issues and validates.
type Claims struct {
	TenantID string   `json:"tenant_id"`
	Roles    []string `json:"roles"`
	Scopes   []string `json:"scopes"`
	jwt.RegisteredClaims
}

// TenantIDFromContext retrieves the validated tenant ID injected by JWTMiddleware.
func TenantIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ContextKeyTenantID).(string)
	return v, ok && v != ""
}

// UserIDFromContext retrieves the user ID from context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ContextKeyUserID).(string)
	return v, ok && v != ""
}

// RolesFromContext retrieves roles from context.
// Returns a defensive copy so callers cannot mutate the stored slice.
func RolesFromContext(ctx context.Context) []string {
	v, _ := ctx.Value(ContextKeyRoles).([]string)
	if v == nil {
		return nil
	}
	cp := make([]string, len(v))
	copy(cp, v)
	return cp
}

// ValidateSecret fails fast if the secret is too weak to be safe.
func ValidateSecret(secret []byte) error {
	if len(secret) < 32 {
		return ErrWeakSecret
	}
	return nil
}

// IssueToken mints a short-lived HS256 JWT for a tenant user.
// Pass nil for scopes to issue a token with no scope restrictions.
func IssueToken(secret []byte, tenantID, userID string, roles, scopes []string, ttl time.Duration) (string, error) {
	if err := ValidateSecret(secret); err != nil {
		return "", err
	}
	now := time.Now().UTC()
	claims := Claims{
		TenantID: tenantID,
		Roles:    roles,
		Scopes:   scopes,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   userID,
			Audience:  jwt.ClaimStrings{"paladin"},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(secret)
}

// ParseClaims validates an HS256 JWT and returns the embedded claims.
// Returns ErrInvalidToken for any validation failure; never returns nil, nil.
func ParseClaims(tokenStr string, secret []byte) (*Claims, error) {
	if err := ValidateSecret(secret); err != nil {
		return nil, err
	}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithIssuer(issuer),
		jwt.WithAudience("paladin"),
	)
	claims := &Claims{}
	tok, err := parser.ParseWithClaims(tokenStr, claims, func(_ *jwt.Token) (any, error) {
		return secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if !tok.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// WithTenantID stores the tenant ID in the context.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ContextKeyTenantID, tenantID)
}

// WithUserID stores the user ID in the context.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ContextKeyUserID, userID)
}

// WithRoles stores roles in the context.
func WithRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, ContextKeyRoles, roles)
}
