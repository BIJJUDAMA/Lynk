-- Unconditional foreign key indexes to prevent sequential scans
CREATE INDEX IF NOT EXISTS idx_contracts_job_id ON contracts(job_id);
CREATE INDEX IF NOT EXISTS idx_contracts_application_id ON contracts(application_id);
CREATE INDEX IF NOT EXISTS idx_reviews_reviewer_id ON reviews(reviewer_id);

-- Enforce domain invariant: do not cascade delete student applications if job is deleted
ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_job_id_fkey;
ALTER TABLE applications ADD CONSTRAINT applications_job_id_fkey 
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE RESTRICT;
