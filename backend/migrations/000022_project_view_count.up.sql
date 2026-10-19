-- 000022_project_view_count.up.sql
-- Persist batched project page view counts

ALTER TABLE projects ADD COLUMN IF NOT EXISTS view_count BIGINT NOT NULL DEFAULT 0;
