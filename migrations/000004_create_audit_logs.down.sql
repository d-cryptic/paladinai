DROP TRIGGER IF EXISTS alerts_updated_at ON alerts;
DROP TRIGGER IF EXISTS incidents_updated_at ON incidents;
DROP TRIGGER IF EXISTS tenants_updated_at ON tenants;
DROP FUNCTION IF EXISTS set_updated_at();
DROP TABLE IF EXISTS audit_logs;
