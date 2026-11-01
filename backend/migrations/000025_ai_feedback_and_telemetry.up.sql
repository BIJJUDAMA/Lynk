-- Migration: 000025_ai_feedback_and_telemetry.up.sql
-- Description: AI feedback schema and token telemetry columns for LLMOps evaluation

-- 1. Add token consumption and cost estimation columns to ai_runs
ALTER TABLE ai_runs ADD COLUMN IF NOT EXISTS prompt_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE ai_runs ADD COLUMN IF NOT EXISTS completion_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE ai_runs ADD COLUMN IF NOT EXISTS total_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE ai_runs ADD COLUMN IF NOT EXISTS estimated_cost_usd NUMERIC(10, 6) NOT NULL DEFAULT 0.0;

-- 2. Create AI feedback collection table
CREATE TABLE IF NOT EXISTS ai_feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID REFERENCES ai_runs(id) ON DELETE SET NULL,
    target_type VARCHAR(50) NOT NULL,
    target_id VARCHAR(128) NOT NULL,
    feedback_type VARCHAR(50) NOT NULL,
    rating DOUBLE PRECISION,
    comment TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    user_id VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Covering indexes for feedback analytics
CREATE INDEX IF NOT EXISTS idx_ai_feedback_target ON ai_feedback(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_ai_feedback_run_id ON ai_feedback(run_id);
CREATE INDEX IF NOT EXISTS idx_ai_feedback_created_at ON ai_feedback(created_at DESC);
