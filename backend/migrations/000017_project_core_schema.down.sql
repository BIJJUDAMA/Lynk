-- 000017_project_core_schema.down.sql
DROP TABLE IF EXISTS project_activity CASCADE;
DROP TABLE IF EXISTS project_resources CASCADE;
DROP TABLE IF EXISTS project_sections CASCADE;
DROP TABLE IF EXISTS project_members CASCADE;
DROP TABLE IF EXISTS projects CASCADE;

DROP TYPE IF EXISTS project_member_role CASCADE;
DROP TYPE IF EXISTS project_lifecycle_status CASCADE;
DROP TYPE IF EXISTS project_recruitment_status CASCADE;
DROP TYPE IF EXISTS project_visibility CASCADE;
