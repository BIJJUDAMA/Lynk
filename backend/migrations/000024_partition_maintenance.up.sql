-- 000024_partition_maintenance.up.sql
-- 100% pure 7-bit ASCII
-- Creates indexes and triggers for partition boundary maintenance

CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at_brin 
ON audit_logs USING brin (created_at);

CREATE INDEX IF NOT EXISTS idx_outbox_events_created_at_brin 
ON outbox_events USING brin (created_at);
