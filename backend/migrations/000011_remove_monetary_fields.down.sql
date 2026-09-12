-- Revert migration 000011: Re-add monetary fields with safe defaults
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS budget NUMERIC(10, 2) NOT NULL DEFAULT 0.00 CHECK (budget >= 0);
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS pay_type VARCHAR(50) NOT NULL DEFAULT 'fixed' CHECK (pay_type IN ('fixed', 'hourly'));
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS agreed_budget NUMERIC(10, 2) NOT NULL DEFAULT 0.00 CHECK (agreed_budget >= 0);
