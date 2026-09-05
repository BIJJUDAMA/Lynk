-- Update users role constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
UPDATE users SET role = 'member' WHERE role IN ('student', 'employer');
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('member', 'admin'));
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'member';

-- Create unified profiles table
CREATE TABLE IF NOT EXISTS profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    first_name VARCHAR(100) NOT NULL DEFAULT '',
    last_name VARCHAR(100) NOT NULL DEFAULT '',
    bio TEXT NOT NULL DEFAULT '',
    department VARCHAR(100) NOT NULL DEFAULT '',
    graduation_year INT NOT NULL DEFAULT 0,
    skills TEXT[] NOT NULL DEFAULT '{}',
    portfolio_links JSONB NOT NULL DEFAULT '[]'::jsonb,
    resume_key VARCHAR(512),
    resume_filename VARCHAR(255),
    resume_byte_size BIGINT DEFAULT 0,
    organization VARCHAR(200) NOT NULL DEFAULT '',
    organization_website VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Migrate data from student_profiles and employer_profiles if existing
INSERT INTO profiles (user_id, first_name, last_name, bio, department, graduation_year, skills, portfolio_links, resume_key, resume_filename, resume_byte_size, created_at, updated_at)
SELECT user_id, first_name, last_name, bio, department, graduation_year, skills, portfolio_links, resume_key, resume_filename, resume_byte_size, created_at, updated_at
FROM student_profiles
ON CONFLICT (user_id) DO NOTHING;

UPDATE profiles p
SET organization = ep.company_or_org,
    organization_website = ep.website
FROM employer_profiles ep
WHERE p.user_id = ep.user_id;

INSERT INTO profiles (user_id, organization, organization_website, created_at, updated_at)
SELECT user_id, company_or_org, website, created_at, updated_at
FROM employer_profiles
ON CONFLICT (user_id) DO UPDATE
SET organization = EXCLUDED.organization,
    organization_website = EXCLUDED.organization_website;

DROP TABLE IF EXISTS student_profiles CASCADE;
DROP TABLE IF EXISTS employer_profiles CASCADE;

-- Rename jobs.employer_id to jobs.created_by
ALTER TABLE jobs RENAME COLUMN employer_id TO created_by;
CREATE INDEX IF NOT EXISTS idx_jobs_created_by ON jobs(created_by);

-- Rename applications.student_id to applications.applicant_id
ALTER TABLE applications RENAME COLUMN student_id TO applicant_id;
ALTER TABLE applications DROP CONSTRAINT IF EXISTS uq_job_student_application;
ALTER TABLE applications ADD CONSTRAINT uq_job_applicant UNIQUE (job_id, applicant_id);
ALTER INDEX IF EXISTS idx_applications_student_id RENAME TO idx_applications_applicant_id;

-- Rename contracts.employer_id to contracts.client_id, and student_id to freelancer_id
ALTER TABLE contracts RENAME COLUMN employer_id TO client_id;
ALTER TABLE contracts RENAME COLUMN student_id TO freelancer_id;
ALTER INDEX IF EXISTS idx_contracts_employer RENAME TO idx_contracts_client;
ALTER INDEX IF EXISTS idx_contracts_student RENAME TO idx_contracts_freelancer;
