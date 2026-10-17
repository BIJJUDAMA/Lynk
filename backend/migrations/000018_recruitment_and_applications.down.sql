-- 000018_recruitment_and_applications.down.sql
DROP TABLE IF EXISTS project_invitations CASCADE;
DROP TABLE IF EXISTS application_answers CASCADE;
DROP TABLE IF EXISTS project_applications CASCADE;
DROP TABLE IF EXISTS application_questions CASCADE;

DROP TYPE IF EXISTS invitation_status CASCADE;
DROP TYPE IF EXISTS project_application_status CASCADE;
