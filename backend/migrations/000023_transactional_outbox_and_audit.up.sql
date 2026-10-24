-- 000023_transactional_outbox_and_audit.up.sql
-- Production Transactional Outbox enhancements and Append-Only Audit Logging Infrastructure
-- 100% pure 7-bit ASCII

ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS headers JSONB NOT NULL DEFAULT '{}'::jsonb;
CREATE INDEX IF NOT EXISTS idx_outbox_aggregate ON outbox_events(aggregate_type, aggregate_id);

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    actor_id VARCHAR(64) NOT NULL,
    action VARCHAR(128) NOT NULL,
    target_type VARCHAR(64) NOT NULL,
    target_id VARCHAR(64) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address VARCHAR(45),
    trace_id VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_timeline ON audit_logs(created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_target ON audit_logs(target_type, target_id);

CREATE OR REPLACE FUNCTION prevent_audit_log_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'Audit log records are immutable and cannot be updated or deleted';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_audit_logs_immutable ON audit_logs;
CREATE TRIGGER trg_audit_logs_immutable
BEFORE UPDATE OR DELETE ON audit_logs
FOR EACH ROW
EXECUTE FUNCTION prevent_audit_log_modification();
