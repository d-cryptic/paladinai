-- Migration 000003: incidents table + incident_alerts junction
-- An incident groups correlated alerts into a single lifecycle object.

CREATE TABLE incidents (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    correlation_id  TEXT        NOT NULL,
    severity        TEXT        NOT NULL DEFAULT 'P4'
                                CHECK (severity IN ('P1', 'P2', 'P3', 'P4')),
    status          TEXT        NOT NULL DEFAULT 'open'
                                CHECK (status IN ('open', 'acknowledged', 'resolved', 'closed')),
    title           TEXT        NOT NULL DEFAULT '',
    summary         TEXT        NOT NULL DEFAULT '',
    likely_cause    TEXT        NOT NULL DEFAULT '',
    affected_services TEXT[]    NOT NULL DEFAULT '{}',
    needs_human     BOOLEAN     NOT NULL DEFAULT false,
    assigned_to     TEXT,
    triage_model    TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at     TIMESTAMPTZ,
    CONSTRAINT chk_resolved_after_created CHECK (resolved_at IS NULL OR resolved_at >= created_at)
);

-- One open incident per correlation group per tenant
CREATE UNIQUE INDEX idx_incidents_tenant_correlation_open
    ON incidents (tenant_id, correlation_id)
    WHERE status NOT IN ('resolved', 'closed');

CREATE INDEX idx_incidents_status    ON incidents (tenant_id, status);
CREATE INDEX idx_incidents_severity  ON incidents (tenant_id, severity);
CREATE INDEX idx_incidents_created_at ON incidents (tenant_id, created_at DESC);

-- Row-level security
ALTER TABLE incidents ENABLE ROW LEVEL SECURITY;
ALTER TABLE incidents FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON incidents
    USING (tenant_id::text = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id::text = current_setting('app.tenant_id', true));

CREATE POLICY service_bypass ON incidents
    TO paladin_service
    USING (true)
    WITH CHECK (true);

-- Junction table: incidents ↔ alerts.
-- Includes tenant_id to enable RLS and avoid cross-tenant enumeration via JOIN.
CREATE TABLE incident_alerts (
    tenant_id   UUID        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    incident_id UUID        NOT NULL REFERENCES incidents (id) ON DELETE CASCADE,
    alert_id    UUID        NOT NULL REFERENCES alerts (id) ON DELETE CASCADE,
    linked_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (incident_id, alert_id)
);

CREATE INDEX idx_incident_alerts_alert  ON incident_alerts (alert_id);
CREATE INDEX idx_incident_alerts_tenant ON incident_alerts (tenant_id);

-- RLS on junction table
ALTER TABLE incident_alerts ENABLE ROW LEVEL SECURITY;
ALTER TABLE incident_alerts FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON incident_alerts
    USING (tenant_id::text = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id::text = current_setting('app.tenant_id', true));

CREATE POLICY service_bypass ON incident_alerts
    TO paladin_service
    USING (true)
    WITH CHECK (true);
