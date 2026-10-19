-- 000022_project_view_count.down.sql
-- Revert the persisted project page view counts

ALTER TABLE projects DROP COLUMN IF EXISTS view_count;
