-- 000020_project_feed_covering_indexes.down.sql
-- Revert compound covering indexes for project discovery feed and member lookups

DROP INDEX IF EXISTS idx_project_members_lookup;
DROP INDEX IF EXISTS idx_projects_active_feed;
DROP INDEX IF EXISTS idx_projects_feed_covering;
