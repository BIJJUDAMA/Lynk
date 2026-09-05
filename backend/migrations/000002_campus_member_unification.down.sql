-- Revert contracts columns and indexes
ALTER TABLE contracts RENAME COLUMN freelancer_id TO student_id;
ALTER TABLE contracts RENAME COLUMN client_id TO employer_id;
ALTER INDEX IF EXISTS idx_contracts_client RENAME TO idx_contracts_employer;
ALTER INDEX IF EXISTS idx_contracts_freelancer RENAME TO idx_contracts_student;

-- Revert applications columns, constraints, and indexes
ALTER TABLE applications DROP CONSTRAINT IF EXISTS uq_job_applicant;
ALTER TABLE applications RENAME COLUMN applicant_id TO student_id;
ALTER TABLE applications ADD CONSTRAINT uq_job_student_application UNIQUE (job_id, student_id);
ALTER INDEX IF EXISTS idx_applications_applicant_id RENAME TO idx_applications_student_id;

-- Revert jobs column and index
DROP INDEX IF EXISTS idx_jobs_created_by;
ALTER TABLE jobs RENAME COLUMN created_by TO employer_id;

-- Recreate student_profiles table
CREATE TABLE IF NOT EXISTS student_profiles (
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
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Recreate employer_profiles table
CREATE TABLE IF NOT EXISTS employer_profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_or_org VARCHAR(200) NOT NULL DEFAULT '',
    contact_name VARCHAR(100) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    website VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Migrate data back from profiles
INSERT INTO student_profiles (id, user_id, first_name, last_name, bio, department, graduation_year, skills, portfolio_links, resume_key, resume_filename, resume_byte_size, created_at, updated_at)
SELECT id, user_id, first_name, last_name, bio, department, graduation_year, skills, portfolio_links, resume_key, resume_filename, resume_byte_size, created_at, updated_at
FROM profiles
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO employer_profiles (user_id, company_or_org, contact_name, description, website, created_at, updated_at)
SELECT user_id, organization, TRIM(first_name || ' ' || last_name), bio, organization_website, created_at, updated_at
FROM profiles
WHERE organization <> '' OR organization_website <> ''
ON CONFLICT (user_id) DO NOTHING;

-- Drop profiles table
DROP TABLE IF EXISTS profiles CASCADE;

-- Revert users role check constraint and default
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ALTER COLUMN role DROP DEFAULT;
UPDATE users SET role = 'student' WHERE role = 'member';
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('student', 'employer', 'admin'));
