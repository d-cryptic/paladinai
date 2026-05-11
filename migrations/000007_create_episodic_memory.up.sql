-- episodic_memory stores past incident records for similarity retrieval
CREATE TABLE episodic_memory (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID        NOT NULL,
    incident_id  UUID        NOT NULL,
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
CREATE INDEX idx_episodic_memory_fingerprint ON episodic_memory (fingerprint);
