-- Migration 000004: audit_logs table + updated_at triggers
-- Append-only compliance audit trail. RLS restricts reads to own tenant.

CREATE TABLE audit_logs (
    id          BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id   UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    actor_type  TEXT        NOT NULL CHECK (actor_type IN ('user', 'system', 'api_key')),
    actor_id    TEXT        NOT NULL,
    action      TEXT        NOT NULL,
    resource    TEXT        NOT NULL,
    resource_id TEXT        NOT NULL,
    details     JSONB       NOT NULL DEFAULT '{}',
    ip_address  INET,
    user_agent  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ON DELETE RESTRICT: tenant deletion must explicitly handle audit logs first
-- (move to archival storage or purge under GDPR erasure flow before dropping tenant)

CREATE INDEX idx_audit_tenant_created ON audit_logs (tenant_id, created_at DESC);
CREATE INDEX idx_audit_resource       ON audit_logs (tenant_id, resource, resource_id);
CREATE INDEX idx_audit_action         ON audit_logs (tenant_id, action);

-- RLS: tenants can only read their own audit logs.
-- paladin_service can read all (for compliance export, billing, etc.).
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON audit_logs
    USING (tenant_id::text = current_setting('app.tenant_id', true));
-- No WITH CHECK — audit_logs are written only by paladin_service.

CREATE POLICY service_bypass ON audit_logs
    TO paladin_service
    USING (true)
    WITH CHECK (true);

-- updated_at trigger function (applied to tables that need it)
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER tenants_updated_at
    BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER incidents_updated_at
    BEFORE UPDATE ON incidents
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER alerts_updated_at
    BEFORE UPDATE ON alerts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
