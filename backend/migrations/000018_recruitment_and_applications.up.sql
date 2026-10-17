-- 000018_recruitment_and_applications.up.sql
-- Application Lifecycle, Questions, Answers, and Team Invitations

DO $$ BEGIN
    CREATE TYPE project_application_status AS ENUM ('submitted', 'shortlisted', 'accepted', 'rejected', 'withdrawn');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE invitation_status AS ENUM ('pending', 'accepted', 'declined', 'expired');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS application_questions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    question_text TEXT NOT NULL,
    help_text TEXT NOT NULL DEFAULT '',
    required BOOLEAN NOT NULL DEFAULT TRUE,
    position INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_application_questions_project ON application_questions(project_id, position);

CREATE TABLE IF NOT EXISTS project_applications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    applicant_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status project_application_status NOT NULL DEFAULT 'submitted',
    statement TEXT NOT NULL,
    resume_asset_key VARCHAR(512),
    reviewer_notes TEXT NOT NULL DEFAULT '',
    decided_by VARCHAR(64) REFERENCES users(id) ON DELETE SET NULL,
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_project_applicant UNIQUE (project_id, applicant_id)
);

CREATE INDEX IF NOT EXISTS idx_project_applications_project ON project_applications(project_id, status);
CREATE INDEX IF NOT EXISTS idx_project_applications_applicant ON project_applications(applicant_id);

CREATE TABLE IF NOT EXISTS application_answers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    application_id UUID NOT NULL REFERENCES project_applications(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES application_questions(id) ON DELETE CASCADE,
    answer_text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_application_question_answer UNIQUE (application_id, question_id)
);

CREATE INDEX IF NOT EXISTS idx_application_answers_app ON application_answers(application_id);

CREATE TABLE IF NOT EXISTS project_invitations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    inviter_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    invitee_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    proposed_role project_member_role NOT NULL DEFAULT 'member',
    status invitation_status NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '14 days'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_project_invitee UNIQUE (project_id, invitee_id)
);

CREATE INDEX IF NOT EXISTS idx_project_invitations_invitee ON project_invitations(invitee_id, status);
