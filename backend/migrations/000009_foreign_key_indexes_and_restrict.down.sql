DROP INDEX IF EXISTS idx_contracts_job_id;
DROP INDEX IF EXISTS idx_contracts_application_id;
DROP INDEX IF EXISTS idx_reviews_reviewer_id;

ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_job_id_fkey;
ALTER TABLE applications ADD CONSTRAINT applications_job_id_fkey 
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;
