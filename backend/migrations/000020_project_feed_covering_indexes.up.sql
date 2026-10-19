-- 000020_project_feed_covering_indexes.up.sql
-- Compound covering indexes for project discovery feed and member lookups

CREATE INDEX IF NOT EXISTS idx_projects_feed_covering
ON projects (visibility, recruitment_status, category, created_at DESC)
INCLUDE (id, slug, title, description, owner_member_id, lifecycle_status);

CREATE INDEX IF NOT EXISTS idx_projects_active_feed
ON projects (visibility, created_at DESC)
WHERE visibility = 'public';

CREATE INDEX IF NOT EXISTS idx_project_members_lookup
ON project_members (project_id, role, status);
