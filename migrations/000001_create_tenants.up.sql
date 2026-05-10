-- Migration 000001: tenants table
-- Tracks all registered tenants and their lifecycle state.

CREATE TABLE tenants (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    state       TEXT        NOT NULL DEFAULT 'active'
                            CHECK (state IN ('active', 'suspended')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- slug must be globally unique (enforced at app layer too, but belt-and-suspenders)
CREATE UNIQUE INDEX idx_tenants_slug ON tenants (slug);

-- Fast lookup by state for billing and auth middleware
CREATE INDEX idx_tenants_state ON tenants (state);
