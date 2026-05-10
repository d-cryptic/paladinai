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
	// DefaultTimeout is the max time to wait for an API response.
	DefaultTimeout = 10 * time.Second
	// MaxResponseBytes limits the response body to 10 MB to prevent OOM.
	MaxResponseBytes = 10 * 1024 * 1024
)

// HTTP is a configured client that all CLI commands should use.
var HTTP = &http.Client{Timeout: DefaultTimeout}

// Get performs a GET request with the given tenant header.
// Returns the response body or an error. Response body is capped at MaxResponseBytes.
func Get(ctx context.Context, url, tenantID string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// DoJSON performs a request with the given method, body, and tenant header.
// Returns (response body, status code, error).
func DoJSON(ctx context.Context, method, url, tenantID string, reqBody io.Reader) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := HTTP.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return body, resp.StatusCode, nil
}
