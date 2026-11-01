-- Migration: 000025_ai_feedback_and_telemetry.down.sql
-- Description: Revert AI feedback schema and token telemetry columns

DROP TABLE IF EXISTS ai_feedback;
ALTER TABLE ai_runs DROP COLUMN IF EXISTS estimated_cost_usd;
ALTER TABLE ai_runs DROP COLUMN IF EXISTS total_tokens;
ALTER TABLE ai_runs DROP COLUMN IF EXISTS completion_tokens;
ALTER TABLE ai_runs DROP COLUMN IF EXISTS prompt_tokens;
