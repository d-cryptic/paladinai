# PaladinAI authorization policy — Stage 8 (day-1 multi-tenancy).
#
# OPA runs as a sidecar of paladin-edge. paladin-edge sends:
#   input.token  = { tenant_id, roles: [...] }
#   input.action = "incidents:read" | "incidents:write" | "runbooks:read" | ...
#   input.resource = { tenant_id }
#
# Evaluation: deny rules take precedence over allow. paladin-edge checks
# `data.paladin.authz.allow == true && count(data.paladin.authz.deny) == 0`.
package paladin.authz

import rego.v1

# Deny by default.
default allow := false

# Admins can do anything.
allow if {
	"admin" in input.token.roles
}

# Viewers may read incidents and runbooks.
allow if {
	input.action in {"incidents:read", "runbooks:read"}
	"viewer" in input.token.roles
}

# Deny cross-tenant access (mandatory boundary — evaluated before allow).
deny contains msg if {
	input.resource.tenant_id != input.token.tenant_id
	msg := "cross-tenant access denied"
}
