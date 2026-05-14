package cache

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestL3Key_RequiresNonEmptyFields(t *testing.T) {
	_, err := L3Key("", "mcp-k8s", map[string]string{"pod": "foo"})
	if err == nil || !strings.Contains(err.Error(), "tenantID") {
		t.Fatalf("expected tenantID error, got %v", err)
	}
	_, err = L3Key("tenant1", "", map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "toolName") {
		t.Fatalf("expected toolName error, got %v", err)
	}
}

func TestL3Key_Determinism(t *testing.T) {
	args := map[string]string{"namespace": "production", "pod": "api-server-0"}
	k1, err := L3Key("tenant1", "mcp-k8s", args)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := L3Key("tenant1", "mcp-k8s", args)
	if err != nil {
		t.Fatal(err)
	}
	if k1 != k2 {
		t.Fatalf("L3Key not deterministic: %q vs %q", k1, k2)
	}
}

func TestL3Key_TenantIsolation(t *testing.T) {
	args := map[string]string{"pod": "foo"}
	k1, _ := L3Key("tenantA", "mcp-k8s", args)
	k2, _ := L3Key("tenantB", "mcp-k8s", args)
	if k1 == k2 {
		t.Fatal("different tenants produced the same L3 key — isolation breach")
	}
}

func TestL3Key_ToolNameArgsBoundaryCollision(t *testing.T) {
	// "foo:bar" with args {} must not equal "foo" with args ":bar{}"
	// length-prefix in hash prevents this
	k1, _ := L3Key("t1", "foo:bar", map[string]any{})
	k2, _ := L3Key("t1", "foo", ":bar{}")
	if k1 == k2 {
		t.Fatal("toolName/args boundary collision detected")
	}
}

func TestL3Key_DifferentToolsSameArgs(t *testing.T) {
	args := map[string]string{"query": "up"}
	k1, _ := L3Key("t1", "mcp-prometheus", args)
	k2, _ := L3Key("t1", "mcp-loki", args)
	if k1 == k2 {
		t.Fatal("different tools with same args should produce different keys")
	}
}

func TestL3Key_NilArgs(t *testing.T) {
	// json.Marshal(nil) → "null", must not error
	k, err := L3Key("t1", "mcp-k8s", nil)
	if err != nil {
		t.Fatalf("nil args should be allowed, got %v", err)
	}
	if k == "" {
		t.Fatal("expected non-empty key for nil args")
	}
}

func TestL3Key_UnmarshalableArgs(t *testing.T) {
	// chan and func types cannot be marshalled to JSON
	_, err := L3Key("t1", "mcp-k8s", make(chan int))
	if err == nil {
		t.Fatal("expected error for unmarshalable args")
	}
}

func TestL3TTL(t *testing.T) {
	tests := []struct {
		tool string
		want time.Duration
	}{
		{"mcp-slack", 0},
		{"mcp-loki", 15 * time.Second},
		{"mcp-k8s", 60 * time.Second},
		{"mcp-pagerduty", 120 * time.Second},
		{"mcp-prometheus", 30 * time.Second},
		{"mcp-github", 60 * time.Second},
		{"mcp-github:get_file", 5 * time.Minute},
		{"mcp-github:list_prs", 60 * time.Second},
		// unknown operation falls back to server-level TTL
		{"mcp-github:unknown_op", 60 * time.Second},
		// completely unknown tool gets default
		{"mcp-unknown-tool", l3DefaultTTL},
	}
	for _, tt := range tests {
		got := L3TTL(tt.tool)
		if got != tt.want {
			t.Errorf("L3TTL(%q) = %v, want %v", tt.tool, got, tt.want)
		}
	}
}

func TestContainsSecret(t *testing.T) {
	cases := []struct {
		data      string
		hasSecret bool
	}{
		{`{"result": "ok", "value": 42}`, false},
		{`{"status": "healthy"}`, false},
		{`{"api_key": "abc123456789012345678901234"}`, true},
		{`{"token": "sk-abc123456789012345678901234567890123"}`, true},
		{`AKIAIOSFODNN7EXAMPLE`, true},
		{`ghp_abcdefghijklmnopqrstuvwxyz123456789`, true},
		{`-----BEGIN RSA PRIVATE KEY-----`, true},
		{`postgres://admin:password@db.example.com/mydb`, true},
		{`{"Authorization": "Bearer abcdefghijklmnopqrstuvwxyz"}`, true},
		{`Use Bearer abc for authentication`, false}, // too short
		// Constructed at runtime so GitHub push protection doesn't flag the file.
		{strings.Join([]string{"xoxb", "00000000", "AAAAAAAAAAAAAAAA"}, "-"), true},
		{strings.Join([]string{"sk", "live", "AAAAAAAAAAAAAAAAAAAAAAAAA"}, "_"), true},
	}
	for _, c := range cases {
		got := ContainsSecret([]byte(c.data))
		if got != c.hasSecret {
			t.Errorf("ContainsSecret(%q) = %v, want %v", c.data, got, c.hasSecret)
		}
	}
}

func TestMemL3_SetAndGet(t *testing.T) {
	c := NewMemL3()
	ctx := context.Background()
	args := map[string]string{"pod": "api"}

	// miss
	v, err := c.Get(ctx, "t1", "mcp-k8s", args)
	if err != nil || v != nil {
		t.Fatalf("expected miss, got %v %v", v, err)
	}

	if err := c.Set(ctx, "t1", "mcp-k8s", args, []byte(`{"cpu": "50m"}`)); err != nil {
		t.Fatal(err)
	}

	// hit
	v, err = c.Get(ctx, "t1", "mcp-k8s", args)
	if err != nil || string(v) != `{"cpu": "50m"}` {
		t.Fatalf("expected hit, got %q %v", v, err)
	}
}

func TestMemL3_IsolatesCachedBytes(t *testing.T) {
	c := NewMemL3()
	ctx := context.Background()
	args := map[string]string{"pod": "api"}
	value := []byte(`{"cpu": "50m"}`)

	if err := c.Set(ctx, "t1", "mcp-k8s", args, value); err != nil {
		t.Fatal(err)
	}
	value[9] = '9'

	got, err := c.Get(ctx, "t1", "mcp-k8s", args)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"cpu": "50m"}` {
		t.Fatalf("cached value mutated through Set input: %q", got)
	}

	got[9] = '8'
	again, err := c.Get(ctx, "t1", "mcp-k8s", args)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != `{"cpu": "50m"}` {
		t.Fatalf("cached value mutated through Get result: %q", again)
	}
}

func TestMemL3_UncacheableToolIsNoop(t *testing.T) {
	c := NewMemL3()
	ctx := context.Background()
	args := map[string]string{"channel": "#incidents"}

	if err := c.Set(ctx, "t1", "mcp-slack", args, []byte(`{"ok": true}`)); err != nil {
		t.Fatal(err)
	}

	// mcp-slack TTL=0 means result is never stored
	v, err := c.Get(ctx, "t1", "mcp-slack", args)
	if err != nil || v != nil {
		t.Fatalf("mcp-slack should be uncacheable but got %q %v", v, err)
	}
}

func TestMemL3_SecretResultNotCached(t *testing.T) {
	c := NewMemL3()
	ctx := context.Background()
	args := map[string]string{"path": ".env"}

	secret := []byte(`{"content": "OPENAI_API_KEY=sk-abcdefghijklmnopqrstuvwxyz12345678901234"}`)
	if err := c.Set(ctx, "t1", "mcp-github", args, secret); err != nil {
		t.Fatal(err)
	}
	v, err := c.Get(ctx, "t1", "mcp-github", args)
	if err != nil || v != nil {
		t.Fatalf("secret result should not be cached but got %q %v", v, err)
	}
}

func TestMemL3_OversizeResultNotCached(t *testing.T) {
	c := NewMemL3()
	ctx := context.Background()
	args := map[string]string{"namespace": "prod"}

	oversize := make([]byte, l3MaxBytes+1)
	if err := c.Set(ctx, "t1", "mcp-k8s", args, oversize); err != nil {
		t.Fatal(err)
	}
	v, err := c.Get(ctx, "t1", "mcp-k8s", args)
	if err != nil || v != nil {
		t.Fatalf("oversize result should not be cached but got len=%d err=%v", len(v), err)
	}
}

func TestMemL3_TenantIsolation(t *testing.T) {
	c := NewMemL3()
	ctx := context.Background()
	args := map[string]string{"pod": "api"}

	_ = c.Set(ctx, "tenantA", "mcp-k8s", args, []byte(`{"tenantA": true}`))

	v, err := c.Get(ctx, "tenantB", "mcp-k8s", args)
	if err != nil || v != nil {
		t.Fatalf("tenant isolation failed: got %q %v", v, err)
	}
}

func TestMemL3_BypassRead(t *testing.T) {
	c := NewMemL3()
	base := context.Background()
	args := map[string]string{"pod": "api"}

	_ = c.Set(base, "t1", "mcp-k8s", args, []byte(`{"ok": true}`))

	bypass := WithL3Bypass(base, BypassRead)
	v, err := c.Get(bypass, "t1", "mcp-k8s", args)
	if err != nil || v != nil {
		t.Fatalf("BypassRead should return miss but got %q %v", v, err)
	}
}

func TestMemL3_BypassRead_StillWrites(t *testing.T) {
	c := NewMemL3()
	base := context.Background()
	args := map[string]string{"pod": "api"}

	// Write with BypassRead context — result should still be persisted
	bypass := WithL3Bypass(base, BypassRead)
	_ = c.Set(bypass, "t1", "mcp-k8s", args, []byte(`{"ok": true}`))

	// non-bypass read should see the entry
	v, err := c.Get(base, "t1", "mcp-k8s", args)
	if err != nil || string(v) != `{"ok": true}` {
		t.Fatalf("BypassRead Set should persist; got %q %v", v, err)
	}
}

func TestMemL3_BypassStore(t *testing.T) {
	c := NewMemL3()
	base := context.Background()
	args := map[string]string{"pod": "api"}

	bypass := WithL3Bypass(base, BypassStore)
	_ = c.Set(bypass, "t1", "mcp-k8s", args, []byte(`{"ok": true}`))

	v, err := c.Get(base, "t1", "mcp-k8s", args)
	if err != nil || v != nil {
		t.Fatalf("BypassStore should not persist but got %q %v", v, err)
	}
}

func TestMemL3_Expiry(t *testing.T) {
	c := NewMemL3()
	ctx := context.Background()
	args := map[string]string{"query": "up"}

	// manually insert with expired timestamp
	key, _ := L3Key("t1", "mcp-prometheus", args)
	c.mu.Lock()
	c.entries[key] = memEntry{value: []byte(`{"value": 1}`), expiresAt: time.Now().Add(-time.Millisecond)}
	c.mu.Unlock()

	v, err := c.Get(ctx, "t1", "mcp-prometheus", args)
	if err != nil || v != nil {
		t.Fatalf("expired entry should be a miss, got %q %v", v, err)
	}
}

func TestMemL3_Concurrent(t *testing.T) {
	c := NewMemL3()
	ctx := context.Background()
	args := map[string]string{"pod": "api"}
	payload := []byte(`{"cpu": "100m"}`)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = c.Set(ctx, "t1", "mcp-k8s", args, payload)
		}()
		go func() {
			defer wg.Done()
			_, _ = c.Get(ctx, "t1", "mcp-k8s", args)
		}()
	}
	wg.Wait()
}
