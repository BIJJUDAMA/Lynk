-- 000023_transactional_outbox_and_audit.down.sql
-- Rollback Transactional Outbox enhancements and Audit Logging Schema
-- 100% pure 7-bit ASCII

DROP TRIGGER IF EXISTS trg_audit_logs_immutable ON audit_logs;
DROP FUNCTION IF EXISTS prevent_audit_log_modification();
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP INDEX IF EXISTS idx_outbox_aggregate;
ALTER TABLE outbox_events DROP COLUMN IF EXISTS headers;
