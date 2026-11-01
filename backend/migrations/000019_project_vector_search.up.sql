-- 000019_project_vector_search.up.sql
-- Semantic Search Embeddings for Projects (384-dimensional dense vectors stored as JSON strings)

CREATE TABLE IF NOT EXISTS project_embeddings (
    project_id UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    embedding TEXT NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
