package registry_test

import (
	"testing"
	"time"

	"github.com/paladinai/paladinai/services/paladin-hub/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validReq() registry.RegisterRequest {
	return registry.RegisterRequest{
		ID:           "my-mcp-server",
		Name:         "My MCP Server",
		Endpoint:     "https://mcp.example.com/api",
		Capabilities: []string{"read_logs", "execute_command"},
	}
}

func TestRegisterRequest_ValidRequest(t *testing.T) {
	req := validReq()
	assert.NoError(t, req.Validate())
}

func TestRegisterRequest_InvalidID(t *testing.T) {
	cases := []struct{ id, desc string }{
		{"", "empty"},
		{"has space", "contains space"},
		{"has/slash", "contains slash"},
		{"toolooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooong", "too long"},
	}
	for _, c := range cases {
		req := validReq()
		req.ID = c.id
		assert.Error(t, req.Validate(), "expected error for %s ID", c.desc)
	}
}

func TestRegisterRequest_InvalidEndpoint(t *testing.T) {
	cases := []string{
		"not-a-url",
		"ftp://mcp.example.com",
		"",
	}
	for _, ep := range cases {
		req := validReq()
		req.Endpoint = ep
		assert.Error(t, req.Validate(), "expected error for endpoint %q", ep)
	}
}

func TestRegisterRequest_EmptyCapabilities(t *testing.T) {
	req := validReq()
	req.Capabilities = nil
	assert.Error(t, req.Validate())
}

func TestRegisterRequest_ToServer(t *testing.T) {
	req := validReq()
	require.NoError(t, req.Validate())

	now := time.Now().UTC()
	server := req.ToServer("tenant-1", now)

	assert.Equal(t, req.ID, server.ID)
	assert.Equal(t, "tenant-1", server.TenantID)
	assert.Equal(t, req.Endpoint, server.Endpoint)
	assert.Equal(t, req.Capabilities, server.Capabilities)
	assert.True(t, server.Healthy)
	assert.Equal(t, now, server.RegisteredAt)
}
