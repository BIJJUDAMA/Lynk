-- Drop existing foreign key constraints referencing users(id)
ALTER TABLE profiles DROP CONSTRAINT IF EXISTS profiles_user_id_fkey;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_created_by_fkey;
ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_applicant_id_fkey;
ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_client_id_fkey;
ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_freelancer_id_fkey;
ALTER TABLE reviews DROP CONSTRAINT IF EXISTS reviews_reviewer_id_fkey;
ALTER TABLE reviews DROP CONSTRAINT IF EXISTS reviews_reviewee_id_fkey;

-- Revert users.id column type to UUID
ALTER TABLE users ALTER COLUMN id TYPE UUID USING id::uuid;

-- Revert referencing columns to UUID
ALTER TABLE profiles ALTER COLUMN user_id TYPE UUID USING user_id::uuid;
ALTER TABLE jobs ALTER COLUMN created_by TYPE UUID USING created_by::uuid;
ALTER TABLE applications ALTER COLUMN applicant_id TYPE UUID USING applicant_id::uuid;
ALTER TABLE contracts ALTER COLUMN client_id TYPE UUID USING client_id::uuid;
ALTER TABLE contracts ALTER COLUMN freelancer_id TYPE UUID USING freelancer_id::uuid;
ALTER TABLE reviews ALTER COLUMN reviewer_id TYPE UUID USING reviewer_id::uuid;
ALTER TABLE reviews ALTER COLUMN reviewee_id TYPE UUID USING reviewee_id::uuid;

-- Re-add foreign key constraints
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
