// Package client provides a shared HTTP client with timeout and response size limits
// for all paladin CLI commands.
package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	DefaultTimeout   = 10 * time.Second
	MaxResponseBytes = 10 * 1024 * 1024 // 10 MB
)

// HTTP is a configured client shared by all CLI commands.
var HTTP = &http.Client{Timeout: DefaultTimeout}

// Options controls authentication and other per-request options.
type Options struct {
	TenantID    string
	Token       string
	AdminSecret string
}

func (o Options) applyHeaders(req *http.Request) {
	if o.TenantID != "" {
		req.Header.Set("X-Tenant-ID", o.TenantID)
	}
	if o.Token != "" {
		req.Header.Set("Authorization", "Bearer "+o.Token)
	}
	if o.AdminSecret != "" {
		req.Header.Set("X-Admin-Secret", o.AdminSecret)
	}
}

// Get performs a GET request and returns the response body.
// Body is capped at MaxResponseBytes to prevent OOM.
func Get(ctx context.Context, url string, opts Options) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	opts.applyHeaders(req)

	resp, err := HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// DoJSON performs a request with the given method and body.
// Returns (response body, status code, error).
func DoJSON(ctx context.Context, method, url string, opts Options, reqBody io.Reader) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	opts.applyHeaders(req)

	resp, err := HTTP.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return body, resp.StatusCode, nil
}
