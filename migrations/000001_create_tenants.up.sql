-- Migration 000001: tenants table
-- Tracks all registered tenants and their lifecycle state.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'paladin_service') THEN
        CREATE ROLE paladin_service;
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS tenants (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    state       TEXT        NOT NULL DEFAULT 'active'
                            CHECK (state IN ('active', 'suspended')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- slug must be globally unique (enforced at app layer too, but belt-and-suspenders)
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenants_slug ON tenants (slug);

-- Fast lookup by state for billing and auth middleware
CREATE INDEX IF NOT EXISTS idx_tenants_state ON tenants (state);
