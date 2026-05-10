// Package registry defines the MCP server registration model and domain logic.
package registry

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var validIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// MCPServer represents a registered MCP (Model Context Protocol) server.
// Agents discover servers via the registry to find available tools.
type MCPServer struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	Endpoint     string    `json:"endpoint"`
	Capabilities []string  `json:"capabilities"` // tool names exposed by this server
	Healthy      bool      `json:"healthy"`
	RegisteredAt time.Time `json:"registered_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

// RegisterRequest is the payload for registering a new MCP server.
type RegisterRequest struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Endpoint     string   `json:"endpoint"`
	Capabilities []string `json:"capabilities"`
}

// Validate checks the request for required fields and format constraints.
func (r *RegisterRequest) Validate() error {
	if !validIDPattern.MatchString(r.ID) {
		return fmt.Errorf("id must match [a-zA-Z0-9_-]{1,64}")
	}
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if len(r.Name) > 128 {
		return fmt.Errorf("name too long (max 128)")
	}
	u, err := url.Parse(r.Endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("endpoint must be a valid http/https URL")
	}
	if len(r.Capabilities) == 0 {
		return fmt.Errorf("at least one capability is required")
	}
	if len(r.Capabilities) > 100 {
		return fmt.Errorf("too many capabilities (max 100)")
	}
	for _, c := range r.Capabilities {
		if strings.TrimSpace(c) == "" || len(c) > 128 {
			return fmt.Errorf("capability %q is invalid (must be 1-128 non-blank chars)", c)
		}
	}
	return nil
}

// ToServer converts a validated RegisterRequest into an MCPServer.
func (r *RegisterRequest) ToServer(tenantID string, now time.Time) *MCPServer {
	caps := make([]string, len(r.Capabilities))
	copy(caps, r.Capabilities)
	return &MCPServer{
		ID:           r.ID,
		TenantID:     tenantID,
		Name:         r.Name,
		Description:  r.Description,
		Endpoint:     r.Endpoint,
		Capabilities: caps,
		Healthy:      true,
		RegisteredAt: now,
		LastSeenAt:   now,
	}
}
