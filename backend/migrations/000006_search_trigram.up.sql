CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_jobs_title_trgm
    ON jobs USING GIN (title gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_jobs_description_trgm
    ON jobs USING GIN (description gin_trgm_ops);
