-- Partial indexes to accelerate active marketplace queries while keeping index sizes small

-- 1. Active open jobs ordered by creation date
CREATE INDEX IF NOT EXISTS idx_jobs_active_created_at
ON jobs (created_at DESC)
WHERE status = 'open';

-- 2. Pending applications per job
CREATE INDEX IF NOT EXISTS idx_applications_pending_job
ON applications (job_id, created_at DESC)
WHERE status = 'pending';

-- 3. Active and draft contracts
CREATE INDEX IF NOT EXISTS idx_contracts_active_status
ON contracts (status, updated_at DESC)
WHERE status IN ('active', 'draft');

-- 4. Active recommendations per user
CREATE INDEX IF NOT EXISTS idx_recommendations_active_user
ON ai_recommendations (user_id, confidence DESC)
WHERE status = 'active';
