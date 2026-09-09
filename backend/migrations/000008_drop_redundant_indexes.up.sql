-- 000001 created single-column indexes later superseded by 000004 composites / expression indexes.
-- Safe DROP: query planner can use the remaining composite or expression indexes.

-- idx_jobs_status superseded by idx_jobs_status_created_at (status, created_at DESC)
DROP INDEX IF EXISTS idx_jobs_status;

-- idx_jobs_department superseded by idx_jobs_department_lower (LOWER(department))
DROP INDEX IF EXISTS idx_jobs_department;

-- idx_applications_job_id superseded by idx_applications_job_id_created (job_id, created_at DESC)
DROP INDEX IF EXISTS idx_applications_job_id;

-- idx_applications_applicant_id (renamed from student) superseded by idx_applications_applicant_created
DROP INDEX IF EXISTS idx_applications_applicant_id;

-- idx_contracts_client / idx_contracts_freelancer superseded by *_created composites
DROP INDEX IF EXISTS idx_contracts_client;
DROP INDEX IF EXISTS idx_contracts_freelancer;

-- idx_reviews_reviewee superseded by idx_reviews_reviewee_created
DROP INDEX IF EXISTS idx_reviews_reviewee;
