-- 000019_project_vector_search.up.sql
-- Semantic Search Embeddings for Projects (384-dimensional all-MiniLM-L6-v2)

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS project_embeddings (
    project_id UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    embedding vector(384) NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- HNSW index for sub-5ms cosine similarity semantic search
CREATE INDEX IF NOT EXISTS idx_project_embeddings_hnsw 
ON project_embeddings USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);
