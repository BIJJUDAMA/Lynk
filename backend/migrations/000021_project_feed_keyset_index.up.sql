-- 000021_project_feed_keyset_index.up.sql
-- Keyset pagination index for the project discovery feed (stable created_at, id order)

CREATE INDEX IF NOT EXISTS idx_projects_feed_keyset
ON projects (visibility, recruitment_status, category, created_at DESC, id DESC);
