-- episodic_memory stores past incident records for similarity retrieval
CREATE TABLE episodic_memory (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    incident_id  UUID        NOT NULL REFERENCES incidents (id) ON DELETE CASCADE,
    fingerprint  TEXT        NOT NULL,
    summary      TEXT        NOT NULL,
    root_cause   TEXT,
    resolution   TEXT,
    severity     TEXT        NOT NULL,
    labels       JSONB       NOT NULL DEFAULT '{}',
    valid_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_episodic_memory_tenant_valid ON episodic_memory (tenant_id, valid_at DESC);
CREATE INDEX idx_episodic_memory_tenant_fingerprint ON episodic_memory (tenant_id, fingerprint);

ALTER TABLE episodic_memory ENABLE ROW LEVEL SECURITY;
ALTER TABLE episodic_memory FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON episodic_memory
    USING (tenant_id::text = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id::text = current_setting('app.tenant_id', true));

CREATE POLICY service_bypass ON episodic_memory
    TO paladin_service
    USING (true)
    WITH CHECK (true);
