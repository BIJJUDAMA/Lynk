-- Drop existing foreign key constraints referencing users(id)
ALTER TABLE profiles DROP CONSTRAINT IF EXISTS profiles_user_id_fkey;

ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_created_by_fkey;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_employer_id_fkey;

ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_applicant_id_fkey;
ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_student_id_fkey;

ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_client_id_fkey;
ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_employer_id_fkey;
ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_freelancer_id_fkey;
ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_student_id_fkey;

ALTER TABLE reviews DROP CONSTRAINT IF EXISTS reviews_reviewer_id_fkey;
ALTER TABLE reviews DROP CONSTRAINT IF EXISTS reviews_reviewee_id_fkey;

-- Alter users.id column type to VARCHAR(64)
ALTER TABLE users ALTER COLUMN id TYPE VARCHAR(64) USING id::text;

-- Alter referencing columns to VARCHAR(64)
ALTER TABLE profiles ALTER COLUMN user_id TYPE VARCHAR(64) USING user_id::text;
ALTER TABLE jobs ALTER COLUMN created_by TYPE VARCHAR(64) USING created_by::text;
ALTER TABLE applications ALTER COLUMN applicant_id TYPE VARCHAR(64) USING applicant_id::text;
ALTER TABLE contracts ALTER COLUMN client_id TYPE VARCHAR(64) USING client_id::text;
ALTER TABLE contracts ALTER COLUMN freelancer_id TYPE VARCHAR(64) USING freelancer_id::text;
ALTER TABLE reviews ALTER COLUMN reviewer_id TYPE VARCHAR(64) USING reviewer_id::text;
ALTER TABLE reviews ALTER COLUMN reviewee_id TYPE VARCHAR(64) USING reviewee_id::text;

-- Re-add foreign key constraints preserving cascade and restrict rules
ALTER TABLE profiles
    ADD CONSTRAINT profiles_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE jobs
    ADD CONSTRAINT jobs_created_by_fkey
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE applications
    ADD CONSTRAINT applications_applicant_id_fkey
    FOREIGN KEY (applicant_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE contracts
    ADD CONSTRAINT contracts_client_id_fkey
    FOREIGN KEY (client_id) REFERENCES users(id) ON DELETE RESTRICT;

ALTER TABLE contracts
    ADD CONSTRAINT contracts_freelancer_id_fkey
    FOREIGN KEY (freelancer_id) REFERENCES users(id) ON DELETE RESTRICT;

ALTER TABLE reviews
    ADD CONSTRAINT reviews_reviewer_id_fkey
    FOREIGN KEY (reviewer_id) REFERENCES users(id) ON DELETE RESTRICT;

ALTER TABLE reviews
    ADD CONSTRAINT reviews_reviewee_id_fkey
    FOREIGN KEY (reviewee_id) REFERENCES users(id) ON DELETE RESTRICT;
