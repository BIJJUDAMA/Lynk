-- Migration: 000013_ai_subsystem_constraints.down.sql
-- Description: Revert AI subsystem domain check constraints and recommendation uniqueness

-- 1. application_ai_scores
ALTER TABLE application_ai_scores DROP CONSTRAINT IF EXISTS chk_app_scores_score;
ALTER TABLE application_ai_scores DROP CONSTRAINT IF EXISTS chk_app_scores_confidence;

-- 2. review_insights
ALTER TABLE review_insights DROP CONSTRAINT IF EXISTS chk_review_insights_score;
ALTER TABLE review_insights DROP CONSTRAINT IF EXISTS chk_review_insights_confidence;
ALTER TABLE review_insights DROP CONSTRAINT IF EXISTS chk_review_insights_sample_count;

-- 3. ai_jobs
ALTER TABLE ai_jobs DROP CONSTRAINT IF EXISTS chk_ai_jobs_status;
ALTER TABLE ai_jobs DROP CONSTRAINT IF EXISTS chk_ai_jobs_attempts;
ALTER TABLE ai_jobs DROP CONSTRAINT IF EXISTS chk_ai_jobs_max_attempts;

-- 4. ai_recommendations
ALTER TABLE ai_recommendations DROP CONSTRAINT IF EXISTS uq_ai_recommendations_user_type_title;

-- 5. moderation_events
ALTER TABLE moderation_events DROP CONSTRAINT IF EXISTS chk_moderation_events_risk_score;
ALTER TABLE moderation_events DROP CONSTRAINT IF EXISTS chk_moderation_events_confidence;
