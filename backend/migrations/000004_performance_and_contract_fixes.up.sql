-- 1. Replace unconditional UNIQUE(job_id) with partial unique index on non-cancelled contracts
ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_job_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS uq_contracts_active_job 
ON contracts(job_id) 
WHERE status NOT IN ('cancelled');

-- 2. Add composite index for default marketplace feed
CREATE INDEX IF NOT EXISTS idx_jobs_status_created_at ON jobs(status, created_at DESC);

-- 3. Add expression index for case-insensitive department filters
CREATE INDEX IF NOT EXISTS idx_jobs_department_lower ON jobs(LOWER(department));

-- 4. Add GIN index for skill array matching
CREATE INDEX IF NOT EXISTS idx_jobs_required_skills_gin ON jobs USING GIN(required_skills);

-- 5. Add composite sorting indexes for applications, contracts, and reviews
CREATE INDEX IF NOT EXISTS idx_applications_job_id_created ON applications(job_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_applications_applicant_created ON applications(applicant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_contracts_client_created ON contracts(client_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_contracts_freelancer_created ON contracts(freelancer_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_reviews_reviewee_created ON reviews(reviewee_id, created_at DESC);

-- 6. Add relational check constraints preventing self-deals
ALTER TABLE contracts ADD CONSTRAINT chk_contracts_different_parties CHECK (client_id != freelancer_id);
ALTER TABLE reviews ADD CONSTRAINT chk_reviews_different_parties CHECK (reviewer_id != reviewee_id);