-- Migration: 000012_ai_subsystem_init.down.sql
-- Description: Revert AI subsystem schema and pgvector extension

DROP TABLE IF EXISTS ai_jobs;
DROP TABLE IF EXISTS skill_demand_snapshots;
DROP TABLE IF EXISTS review_insights;
DROP TABLE IF EXISTS moderation_events;
DROP TABLE IF EXISTS application_ai_scores;
DROP TABLE IF EXISTS ai_recommendations;
DROP TABLE IF EXISTS ai_runs;
DROP TABLE IF EXISTS ai_embeddings;
DROP TABLE IF EXISTS job_skills;
DROP TABLE IF EXISTS profile_skills;
DROP TABLE IF EXISTS skill_aliases;
DROP TABLE IF EXISTS skills;

DROP EXTENSION IF EXISTS vector;
