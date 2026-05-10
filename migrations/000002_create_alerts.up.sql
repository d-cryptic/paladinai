-- Migration 000002: alerts table
-- Stores every ingested alert after deduplication and correlation.
-- Row-level security enforces tenant isolation.

CREATE TABLE alerts (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    fingerprint     TEXT        NOT NULL,
    correlation_id  TEXT,
    source          TEXT        NOT NULL,
    severity        TEXT        NOT NULL DEFAULT 'P4'
                                CHECK (severity IN ('P1', 'P2', 'P3', 'P4')),
    status          TEXT        NOT NULL DEFAULT 'firing'
                                CHECK (status IN ('firing', 'resolved', 'silenced')),
    title           TEXT        NOT NULL DEFAULT '',
    description     TEXT        NOT NULL DEFAULT '',
    labels          JSONB       NOT NULL DEFAULT '{}',
    annotations     JSONB       NOT NULL DEFAULT '{}',
    starts_at       TIMESTAMPTZ,
    ends_at         TIMESTAMPTZ,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_ends_after_starts CHECK (ends_at IS NULL OR ends_at >= starts_at)
);

-- Deduplication: one active fingerprint per tenant (partial for performance)
CREATE UNIQUE INDEX idx_alerts_tenant_fingerprint_firing
    ON alerts (tenant_id, fingerprint)
    WHERE status = 'firing';

-- Tenant-scoped correlation lookups
CREATE INDEX idx_alerts_tenant_correlation
    ON alerts (tenant_id, correlation_id)
    WHERE correlation_id IS NOT NULL;

-- Composite indexes for common access patterns (leftmost = tenant_id always)
CREATE INDEX idx_alerts_severity    ON alerts (tenant_id, severity);
CREATE INDEX idx_alerts_received_at ON alerts (tenant_id, received_at DESC);

-- Row-level security: 'app.tenant_id' is set to '' (empty) when no tenant ctx,
-- causing the policy to evaluate FALSE → zero rows visible (safe default).
ALTER TABLE alerts ENABLE ROW LEVEL SECURITY;
ALTER TABLE alerts FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON alerts
    USING (tenant_id::text = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id::text = current_setting('app.tenant_id', true));

-- paladin_service role bypasses RLS for migrations and admin operations
CREATE POLICY service_bypass ON alerts
    TO paladin_service
    USING (true)
    WITH CHECK (true);
