package cache

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	l3KeyPrefix = "tool:l3:"
	// l3DefaultTTL is used when the tool name does not appear in the TTL table.
	l3DefaultTTL = 30 * time.Second
	// l3MaxBytes is the maximum serialised result size that will be cached.
	// Oversized results are returned to the caller but not written to L3.
	l3MaxBytes = 256 * 1024
)

// l3TTLTable maps tool names to configured cache TTLs.
// Keys are either fully-qualified names ("mcp-github:get_file") or server names
// ("mcp-k8s"). Lookup tries the fully-qualified name first, then the server prefix.
// A zero TTL means the tool must not be cached.
var l3TTLTable = map[string]time.Duration{
	"mcp-prometheus":      30 * time.Second,
	"mcp-loki":            15 * time.Second,
	"mcp-k8s":             60 * time.Second,
	"mcp-pagerduty":       120 * time.Second,
	"mcp-slack":           0, // never cache — actions, not reads
	"mcp-github":          60 * time.Second,
	"mcp-github:get_file": 5 * time.Minute,
	"mcp-github:list_prs": 60 * time.Second,
}

// L3TTL returns the configured TTL for the given tool name.
// It checks the full name first, then the server prefix (part before ":").
// A zero return means the tool must not be cached.
func L3TTL(toolName string) time.Duration {
	if ttl, ok := l3TTLTable[toolName]; ok {
		return ttl
	}
	// fall back to server-level TTL (e.g. "mcp-github:unknown_op" → "mcp-github")
	if idx := indexByte(toolName, ':'); idx > 0 {
		if ttl, ok := l3TTLTable[toolName[:idx]]; ok {
			return ttl
		}
	}
	return l3DefaultTTL
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// secretPatterns detects secret-like values in tool results.
// Results matching any pattern are returned to the caller but not written to L3.
// Note: base64-encoded secrets will not be caught here — callers should redact
// sensitive content before passing tool results to the cache.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_\-]?key|secret|token|password|credential)["'\s:=]+[A-Za-z0-9+/=_\-]{16,}`),
	regexp.MustCompile(`sk-[A-Za-z0-9]{32,}`),
	regexp.MustCompile(`xoxb-[0-9]+-[A-Za-z0-9\-]+`),
	regexp.MustCompile(`ghp_[A-Za-z0-9]{32,}`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`eyJ[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+`),
	regexp.MustCompile(`sk_live_[a-zA-Z0-9]{24,}`),
	// SSH/TLS private keys
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	// Postgres DSNs with embedded credentials
	regexp.MustCompile(`postgres://[^:]+:[^@\s]{3,}@`),
	// Bearer tokens: require at least 20 chars to avoid matching doc strings
	regexp.MustCompile(`Bearer\s+[a-zA-Z0-9._~+/\-]{20,}=*`),
}

// ContainsSecret reports whether data appears to contain a secret value.
// Used to prevent sensitive tool results from being cached.
func ContainsSecret(data []byte) bool {
	for _, p := range secretPatterns {
		if p.Match(data) {
			return true
		}
	}
	return false
}

// bypassKey is the context key for the L3 cache bypass flag.
type bypassKey struct{}

// BypassMode describes how the cache bypass should behave for a request.
type BypassMode int

const (
	// BypassNone is the zero value — normal caching behaviour.
	// This MUST remain the zero value; bypassKey absence in ctx maps to BypassNone.
	BypassNone  BypassMode = iota
	BypassRead             // skip read, still write-back (Cache-Control: no-cache)
	BypassStore            // skip both read and write (Cache-Control: no-store)
)

// WithL3Bypass injects the bypass mode into ctx.
func WithL3Bypass(ctx context.Context, mode BypassMode) context.Context {
	return context.WithValue(ctx, bypassKey{}, mode)
}

// l3Bypass reads the bypass mode from ctx, defaulting to BypassNone.
func l3Bypass(ctx context.Context) BypassMode {
	if m, ok := ctx.Value(bypassKey{}).(BypassMode); ok {
		return m
	}
	return BypassNone
}

// L3Cache is the interface for the MCP tool result cache.
type L3Cache interface {
	// Get returns the cached tool result, or (nil, nil) on a miss or bypass.
	Get(ctx context.Context, tenantID, toolName string, args any) ([]byte, error)
	// Set stores result under the L3 key. It is a no-op if:
	//   - toolName maps to zero TTL (explicitly uncacheable)
	//   - result contains a secret pattern
	//   - result exceeds l3MaxBytes
	//   - bypass mode is BypassStore
	Set(ctx context.Context, tenantID, toolName string, args any, result []byte) error
}

// L3Key returns the canonical L3 cache key.
// The hash covers a length-prefixed toolName and canonical JSON args to avoid
// ambiguity between toolName "foo:bar" with args {} and toolName "foo" with args ":bar".
// Returns an error if tenantID or toolName is empty, or if args cannot be marshalled.
func L3Key(tenantID, toolName string, args any) (string, error) {
	if tenantID == "" {
		return "", errors.New("l3: tenantID must not be empty")
	}
	if toolName == "" {
		return "", errors.New("l3: toolName must not be empty")
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return "", err
	}
	// Length-prefix toolName so "foo:bar" with "{}" != "foo" with ":bar{}"
	h := sha256.New()
	var lb [4]byte
	binary.BigEndian.PutUint32(lb[:], uint32(len(toolName)))
	h.Write(lb[:])
	h.Write([]byte(toolName))
	h.Write(argsJSON)
	return l3KeyPrefix + tenantID + ":" + hex.EncodeToString(h.Sum(nil)), nil
}

// shouldSkipWrite returns true if the result must not be written to cache.
func shouldSkipWrite(ctx context.Context, toolName string, result []byte) bool {
	if l3Bypass(ctx) == BypassStore {
		return true
	}
	if L3TTL(toolName) == 0 {
		return true
	}
	if len(result) > l3MaxBytes {
		return true
	}
	return ContainsSecret(result)
}

// ValkeyL3 is the production Valkey-backed L3 tool result cache.
type ValkeyL3 struct {
	rdb *redis.Client
}

// NewValkeyL3 wraps an existing go-redis client.
func NewValkeyL3(rdb *redis.Client) *ValkeyL3 {
	return &ValkeyL3{rdb: rdb}
}

func (c *ValkeyL3) Get(ctx context.Context, tenantID, toolName string, args any) ([]byte, error) {
	if l3Bypass(ctx) != BypassNone {
		return nil, nil
	}
	key, err := L3Key(tenantID, toolName, args)
	if err != nil {
		return nil, err
	}
	val, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	return val, err
}

func (c *ValkeyL3) Set(ctx context.Context, tenantID, toolName string, args any, result []byte) error {
	if shouldSkipWrite(ctx, toolName, result) {
		return nil
	}
	key, err := L3Key(tenantID, toolName, args)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, result, L3TTL(toolName)).Err()
}

// MemL3 is a thread-safe in-memory L3 cache for tests.
// Expired entries are evicted on Get only (no background sweeper).
// Not suitable for production use.
type MemL3 struct {
	mu      sync.RWMutex
	entries map[string]memEntry
}

// NewMemL3 creates a fresh in-memory L3 cache.
func NewMemL3() *MemL3 {
	return &MemL3{entries: make(map[string]memEntry)}
}

func (c *MemL3) Get(ctx context.Context, tenantID, toolName string, args any) ([]byte, error) {
	if l3Bypass(ctx) != BypassNone {
		return nil, nil
	}
	key, err := L3Key(tenantID, toolName, args)
	if err != nil {
		return nil, err
	}
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, nil
	}
	return append([]byte(nil), e.value...), nil
}

func (c *MemL3) Set(ctx context.Context, tenantID, toolName string, args any, result []byte) error {
	if shouldSkipWrite(ctx, toolName, result) {
		return nil
	}
	key, err := L3Key(tenantID, toolName, args)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.entries[key] = memEntry{value: append([]byte(nil), result...), expiresAt: time.Now().Add(L3TTL(toolName))}
	c.mu.Unlock()
	return nil
}
