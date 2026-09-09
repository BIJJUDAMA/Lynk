DROP INDEX IF EXISTS idx_jobs_title_trgm;
DROP INDEX IF EXISTS idx_jobs_description_trgm;
-- Keep pg_trgm installed; dropping extensions that other objects may need is unsafe.
