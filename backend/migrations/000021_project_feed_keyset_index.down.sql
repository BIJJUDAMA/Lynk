-- 000021_project_feed_keyset_index.down.sql
-- Revert the project discovery feed keyset pagination index

DROP INDEX IF EXISTS idx_projects_feed_keyset;
