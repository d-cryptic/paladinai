# authz.rego — PaladinAI RBAC + tenant-boundary policy
# See: docs/plans/08.multitenancy-stage8.md §4.2
#
# OPA runs as a sidecar of paladin-edge.
# Input shape:
#   input.token.roles     []string  — JWT roles claim
#   input.token.tenant_id string    — JWT tenant claim
#   input.action          string    — e.g. "incidents:read"
#   input.resource.tenant_id string — tenant owning the requested resource
package paladin.authz

import future.keywords.in

default allow = false

# Admins can do anything within their tenant.
allow {
	"admin" in input.token.roles
	not cross_tenant
}

# Viewers can read incidents and runbooks within their tenant.
allow {
	input.action in {"incidents:read", "runbooks:read", "alerts:read"}
	"viewer" in input.token.roles
	not cross_tenant
}

# Operators can read and act on incidents and runbooks within their tenant.
allow {
	input.action in {
		"incidents:read", "incidents:update",
		"runbooks:read", "runbooks:execute",
		"alerts:read",
	}
	"operator" in input.token.roles
	not cross_tenant
}

# Cross-tenant boundary check — tenant IDs must always match.
# This is enforced regardless of role to prevent confused-deputy attacks.
cross_tenant {
	input.resource.tenant_id != input.token.tenant_id
}

deny[msg] {
	cross_tenant
	msg := sprintf("cross-tenant access denied: token tenant=%v resource tenant=%v",
		[input.token.tenant_id, input.resource.tenant_id])
}
