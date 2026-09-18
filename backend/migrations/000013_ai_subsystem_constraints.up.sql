-- Migration: 000013_ai_subsystem_constraints.up.sql
-- Description: AI subsystem domain check constraints and recommendation uniqueness

-- 1. application_ai_scores constraints
ALTER TABLE application_ai_scores ADD CONSTRAINT chk_app_scores_score CHECK (score >= 0.0 AND score <= 100.0);
ALTER TABLE application_ai_scores ADD CONSTRAINT chk_app_scores_confidence CHECK (confidence >= 0.0 AND confidence <= 1.0);

-- 2. review_insights constraints
ALTER TABLE review_insights ADD CONSTRAINT chk_review_insights_score CHECK (score >= 1.0 AND score <= 5.0);
ALTER TABLE review_insights ADD CONSTRAINT chk_review_insights_confidence CHECK (confidence >= 0.0 AND confidence <= 1.0);
ALTER TABLE review_insights ADD CONSTRAINT chk_review_insights_sample_count CHECK (sample_count >= 1);

-- 3. ai_jobs constraints
ALTER TABLE ai_jobs ADD CONSTRAINT chk_ai_jobs_status CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'dead_letter'));
ALTER TABLE ai_jobs ADD CONSTRAINT chk_ai_jobs_attempts CHECK (attempts >= 0);
ALTER TABLE ai_jobs ADD CONSTRAINT chk_ai_jobs_max_attempts CHECK (max_attempts > 0);

-- 4. ai_recommendations uniqueness constraint
ALTER TABLE ai_recommendations ADD CONSTRAINT uq_ai_recommendations_user_type_title UNIQUE (user_id, type, title);

-- 5. moderation_events constraints
ALTER TABLE moderation_events ADD CONSTRAINT chk_moderation_events_risk_score CHECK (risk_score >= 0.0 AND risk_score <= 1.0);
ALTER TABLE moderation_events ADD CONSTRAINT chk_moderation_events_confidence CHECK (confidence >= 0.0 AND confidence <= 1.0);
