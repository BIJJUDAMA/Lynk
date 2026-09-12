-- Migration 000011: Remove monetary fields from jobs and contracts tables
ALTER TABLE jobs DROP COLUMN IF EXISTS budget;
ALTER TABLE jobs DROP COLUMN IF EXISTS pay_type;
ALTER TABLE contracts DROP COLUMN IF EXISTS agreed_budget;
