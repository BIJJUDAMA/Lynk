-- Migration: 000012_ai_subsystem_init.up.sql
-- Description: AI subsystem schema, pgvector extension, and relational tables

CREATE EXTENSION IF NOT EXISTS vector;

-- 1. Skills taxonomy table
CREATE TABLE IF NOT EXISTS skills (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    canonical_name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    category VARCHAR(50),
    parent_skill_id UUID REFERENCES skills(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Skill aliases mapping
CREATE TABLE IF NOT EXISTS skill_aliases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    alias VARCHAR(100) NOT NULL UNIQUE,
    source VARCHAR(50) NOT NULL DEFAULT 'manual',
    confidence DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Profile skills relationship
CREATE TABLE IF NOT EXISTS profile_skills (
    user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    source VARCHAR(50) NOT NULL DEFAULT 'user',
    verified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, skill_id)
);

-- 4. Job skills relationship
CREATE TABLE IF NOT EXISTS job_skills (
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    importance VARCHAR(20) NOT NULL DEFAULT 'required',
    required BOOLEAN NOT NULL DEFAULT true,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (job_id, skill_id)
);

-- 5. AI Embeddings table (pgvector 384 dimensions)
CREATE TABLE IF NOT EXISTS ai_embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(64) NOT NULL,
    embedding_type VARCHAR(50) NOT NULL DEFAULT 'semantic',
    model_name VARCHAR(100) NOT NULL,
    model_version VARCHAR(50) NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    embedding vector(384) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_type, entity_id, embedding_type, model_name, model_version)
);

-- 6. AI Runs audit log
CREATE TABLE IF NOT EXISTS ai_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    feature VARCHAR(50) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(64) NOT NULL,
    model_name VARCHAR(100) NOT NULL,
    model_version VARCHAR(50) NOT NULL,
    prompt_version VARCHAR(50),
    pipeline_version VARCHAR(50) NOT NULL,
    input_hash VARCHAR(64) NOT NULL,
    output_json JSONB NOT NULL DEFAULT '{}',
    confidence DOUBLE PRECISION,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'success',
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 7. AI Recommendations
CREATE TABLE IF NOT EXISTS ai_recommendations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    reason TEXT NOT NULL,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    metadata JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 8. Application AI Scores
CREATE TABLE IF NOT EXISTS application_ai_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    score DOUBLE PRECISION NOT NULL,
    matched_skills TEXT[] NOT NULL DEFAULT '{}',
    missing_skills TEXT[] NOT NULL DEFAULT '{}',
    reason TEXT NOT NULL,
    confidence DOUBLE PRECISION NOT NULL,
    job_version INTEGER NOT NULL DEFAULT 1,
    profile_version INTEGER NOT NULL DEFAULT 1,
    model_version VARCHAR(50) NOT NULL,
    pipeline_version VARCHAR(50) NOT NULL,
    is_stale BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(application_id)
);

-- 9. Moderation Events
CREATE TABLE IF NOT EXISTS moderation_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(64) NOT NULL,
    risk_score DOUBLE PRECISION NOT NULL,
    decision VARCHAR(20) NOT NULL DEFAULT 'allow',
    signals TEXT[] NOT NULL DEFAULT '{}',
    confidence DOUBLE PRECISION NOT NULL,
    pipeline_version VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 10. Review Insights
CREATE TABLE IF NOT EXISTS review_insights (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    aspect VARCHAR(50) NOT NULL,
    score DOUBLE PRECISION NOT NULL,
    confidence DOUBLE PRECISION NOT NULL,
    sample_count INTEGER NOT NULL DEFAULT 1,
    is_recurring BOOLEAN NOT NULL DEFAULT false,
    strengths TEXT[] NOT NULL DEFAULT '{}',
    improvements TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, aspect)
);

-- 11. Skill Demand Snapshots
CREATE TABLE IF NOT EXISTS skill_demand_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    period VARCHAR(20) NOT NULL,
    job_count INTEGER NOT NULL DEFAULT 0,
    application_count INTEGER NOT NULL DEFAULT 0,
    unique_posters INTEGER NOT NULL DEFAULT 0,
    demand_score DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    growth_rate DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(skill_id, period)
);

-- 12. Asynchronous AI Background Jobs
CREATE TABLE IF NOT EXISTS ai_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_type VARCHAR(50) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(64) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    error TEXT,
    content_hash VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

-- Indexes

-- HNSW Vector index on 384-dimension embedding
CREATE INDEX IF NOT EXISTS idx_ai_embeddings_embedding ON ai_embeddings USING hnsw (embedding vector_cosine_ops);
CREATE INDEX IF NOT EXISTS idx_ai_embeddings_entity ON ai_embeddings(entity_type, entity_id);

-- Skills indexes
CREATE INDEX IF NOT EXISTS idx_skills_parent_skill_id ON skills(parent_skill_id);
CREATE INDEX IF NOT EXISTS idx_skills_category ON skills(category);
CREATE INDEX IF NOT EXISTS idx_skill_aliases_skill_id ON skill_aliases(skill_id);
CREATE INDEX IF NOT EXISTS idx_profile_skills_skill_id ON profile_skills(skill_id);
CREATE INDEX IF NOT EXISTS idx_job_skills_skill_id ON job_skills(skill_id);

-- AI Runs indexes
CREATE INDEX IF NOT EXISTS idx_ai_runs_entity ON ai_runs(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_ai_runs_feature ON ai_runs(feature);
CREATE INDEX IF NOT EXISTS idx_ai_runs_status ON ai_runs(status);
CREATE INDEX IF NOT EXISTS idx_ai_runs_created_at ON ai_runs(created_at DESC);

-- AI Recommendations indexes
CREATE INDEX IF NOT EXISTS idx_ai_recommendations_user_id ON ai_recommendations(user_id);
CREATE INDEX IF NOT EXISTS idx_ai_recommendations_status ON ai_recommendations(status);
CREATE INDEX IF NOT EXISTS idx_ai_recommendations_type ON ai_recommendations(type);

-- Application AI Scores indexes
CREATE INDEX IF NOT EXISTS idx_application_ai_scores_job_id ON application_ai_scores(job_id);
CREATE INDEX IF NOT EXISTS idx_application_ai_scores_is_stale ON application_ai_scores(is_stale);

-- Moderation Events indexes
CREATE INDEX IF NOT EXISTS idx_moderation_events_entity ON moderation_events(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_moderation_events_decision ON moderation_events(decision);
CREATE INDEX IF NOT EXISTS idx_moderation_events_created_at ON moderation_events(created_at DESC);

-- Review Insights indexes
CREATE INDEX IF NOT EXISTS idx_review_insights_user_id ON review_insights(user_id);

-- Skill Demand Snapshots indexes
CREATE INDEX IF NOT EXISTS idx_skill_demand_snapshots_skill_id ON skill_demand_snapshots(skill_id);
CREATE INDEX IF NOT EXISTS idx_skill_demand_snapshots_period ON skill_demand_snapshots(period);

-- AI Jobs queue indexes (for SKIP LOCKED queries and status lookups)
CREATE INDEX IF NOT EXISTS idx_ai_jobs_status_created_at ON ai_jobs(status, created_at);
CREATE INDEX IF NOT EXISTS idx_ai_jobs_entity ON ai_jobs(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_ai_jobs_job_type ON ai_jobs(job_type);
