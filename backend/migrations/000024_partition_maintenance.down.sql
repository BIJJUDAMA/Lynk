-- 000024_partition_maintenance.down.sql
DROP INDEX IF EXISTS idx_audit_logs_created_at_brin;
DROP INDEX IF EXISTS idx_outbox_events_created_at_brin;
