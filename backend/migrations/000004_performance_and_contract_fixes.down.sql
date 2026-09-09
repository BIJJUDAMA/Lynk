ALTER TABLE reviews DROP CONSTRAINT IF EXISTS chk_reviews_different_parties;
ALTER TABLE contracts DROP CONSTRAINT IF EXISTS chk_contracts_different_parties;

DROP INDEX IF EXISTS idx_reviews_reviewee_created;
DROP INDEX IF EXISTS idx_contracts_freelancer_created;
DROP INDEX IF EXISTS idx_contracts_client_created;
DROP INDEX IF EXISTS idx_applications_applicant_created;
DROP INDEX IF EXISTS idx_applications_job_id_created;
DROP INDEX IF EXISTS idx_jobs_required_skills_gin;
DROP INDEX IF EXISTS idx_jobs_department_lower;
DROP INDEX IF EXISTS idx_jobs_status_created_at;

DROP INDEX IF EXISTS uq_contracts_active_job;
ALTER TABLE contracts ADD CONSTRAINT contracts_job_id_key UNIQUE (job_id);