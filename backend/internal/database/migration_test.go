package database_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lynk/backend/internal/database"
)

func TestMigration000003_UpSQL_Structure(t *testing.T) {
	contentBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000003_supertokens_identity.up.sql"))
	if err != nil {
		t.Fatalf("failed to read 000003 up migration: %v", err)
	}
	content := string(contentBytes)

	// Check drop constraints
	expectedDrops := []string{
		"ALTER TABLE profiles DROP CONSTRAINT IF EXISTS profiles_user_id_fkey",
		"ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_created_by_fkey",
		"ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_applicant_id_fkey",
		"ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_client_id_fkey",
		"ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_freelancer_id_fkey",
		"ALTER TABLE reviews DROP CONSTRAINT IF EXISTS reviews_reviewer_id_fkey",
		"ALTER TABLE reviews DROP CONSTRAINT IF EXISTS reviews_reviewee_id_fkey",
	}
	for _, drop := range expectedDrops {
		if !strings.Contains(content, drop) {
			t.Errorf("000003 up migration missing constraint drop: %s", drop)
		}
	}

	// Check column type alterations to VARCHAR(64)
	expectedAlters := []string{
		"ALTER TABLE users ALTER COLUMN id TYPE VARCHAR(64)",
		"ALTER TABLE profiles ALTER COLUMN user_id TYPE VARCHAR(64)",
		"ALTER TABLE jobs ALTER COLUMN created_by TYPE VARCHAR(64)",
		"ALTER TABLE applications ALTER COLUMN applicant_id TYPE VARCHAR(64)",
		"ALTER TABLE contracts ALTER COLUMN client_id TYPE VARCHAR(64)",
		"ALTER TABLE contracts ALTER COLUMN freelancer_id TYPE VARCHAR(64)",
		"ALTER TABLE reviews ALTER COLUMN reviewer_id TYPE VARCHAR(64)",
		"ALTER TABLE reviews ALTER COLUMN reviewee_id TYPE VARCHAR(64)",
	}
	for _, alter := range expectedAlters {
		if !strings.Contains(content, alter) {
			t.Errorf("000003 up migration missing column alter: %s", alter)
		}
	}

	// Check re-adding foreign keys with proper cascade/restrict rules
	expectedFKs := []string{
		"FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE",
		"FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE",
		"FOREIGN KEY (applicant_id) REFERENCES users(id) ON DELETE CASCADE",
		"FOREIGN KEY (client_id) REFERENCES users(id) ON DELETE RESTRICT",
		"FOREIGN KEY (freelancer_id) REFERENCES users(id) ON DELETE RESTRICT",
		"FOREIGN KEY (reviewer_id) REFERENCES users(id) ON DELETE RESTRICT",
		"FOREIGN KEY (reviewee_id) REFERENCES users(id) ON DELETE RESTRICT",
	}
	for _, fk := range expectedFKs {
		if !strings.Contains(content, fk) {
			t.Errorf("000003 up migration missing FK constraint rule: %s", fk)
		}
	}
}

func TestMigration000003_DownSQL_Structure(t *testing.T) {
	contentBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000003_supertokens_identity.down.sql"))
	if err != nil {
		t.Fatalf("failed to read 000003 down migration: %v", err)
	}
	content := string(contentBytes)

	// Check drop constraints
	expectedDrops := []string{
		"ALTER TABLE profiles DROP CONSTRAINT IF EXISTS profiles_user_id_fkey",
		"ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_created_by_fkey",
		"ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_applicant_id_fkey",
		"ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_client_id_fkey",
		"ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_freelancer_id_fkey",
		"ALTER TABLE reviews DROP CONSTRAINT IF EXISTS reviews_reviewer_id_fkey",
		"ALTER TABLE reviews DROP CONSTRAINT IF EXISTS reviews_reviewee_id_fkey",
	}
	for _, drop := range expectedDrops {
		if !strings.Contains(content, drop) {
			t.Errorf("000003 down migration missing constraint drop: %s", drop)
		}
	}

	// Check reverting column types back to UUID
	expectedReverts := []string{
		"ALTER TABLE users ALTER COLUMN id TYPE UUID USING id::uuid",
		"ALTER TABLE profiles ALTER COLUMN user_id TYPE UUID USING user_id::uuid",
		"ALTER TABLE jobs ALTER COLUMN created_by TYPE UUID USING created_by::uuid",
		"ALTER TABLE applications ALTER COLUMN applicant_id TYPE UUID USING applicant_id::uuid",
		"ALTER TABLE contracts ALTER COLUMN client_id TYPE UUID USING client_id::uuid",
		"ALTER TABLE contracts ALTER COLUMN freelancer_id TYPE UUID USING freelancer_id::uuid",
		"ALTER TABLE reviews ALTER COLUMN reviewer_id TYPE UUID USING reviewer_id::uuid",
		"ALTER TABLE reviews ALTER COLUMN reviewee_id TYPE UUID USING reviewee_id::uuid",
	}
	for _, revert := range expectedReverts {
		if !strings.Contains(content, revert) {
			t.Errorf("000003 down migration missing type revert: %s", revert)
		}
	}

	// Check re-adding foreign keys in down migration
	expectedFKs := []string{
		"FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE",
		"FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE",
		"FOREIGN KEY (applicant_id) REFERENCES users(id) ON DELETE CASCADE",
		"FOREIGN KEY (client_id) REFERENCES users(id) ON DELETE RESTRICT",
		"FOREIGN KEY (freelancer_id) REFERENCES users(id) ON DELETE RESTRICT",
		"FOREIGN KEY (reviewer_id) REFERENCES users(id) ON DELETE RESTRICT",
		"FOREIGN KEY (reviewee_id) REFERENCES users(id) ON DELETE RESTRICT",
	}
	for _, fk := range expectedFKs {
		if !strings.Contains(content, fk) {
			t.Errorf("000003 down migration missing FK constraint rule: %s", fk)
		}
	}
}

func TestRunMigrations_AppliesInitOnEmptyDatabase(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := database.NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var versionType string
	err = pool.QueryRow(ctx, `
		SELECT data_type FROM information_schema.columns
		WHERE table_schema='public' AND table_name='schema_migrations' AND column_name='version'`).Scan(&versionType)
	if err == nil && (versionType == "bigint" || versionType == "integer") {
		t.Fatal("TEST_DATABASE_URL is golang-migrate managed; CI must leave lynk_db to the app migrator")
	}

	var usersExists bool
	if qerr := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'users'
		)`).Scan(&usersExists); qerr != nil {
		t.Fatalf("check users table: %v", qerr)
	}
	if usersExists && (versionType == "character varying" || versionType == "varchar") {
		t.Skip("schema already VARCHAR-migrated by another test in this package")
	}

	if err := database.RunMigrations(ctx, pool, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
}

func TestRunMigrations_000003_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration migration test")
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	defer pool.Close()

	// 1. Run all migrations up
	if err := database.RunMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// 2. Verify all identity columns are VARCHAR(64)
	columnsToCheck := []struct {
		table  string
		column string
	}{
		{"users", "id"},
		{"profiles", "user_id"},
		{"jobs", "created_by"},
		{"applications", "applicant_id"},
		{"contracts", "client_id"},
		{"contracts", "freelancer_id"},
		{"reviews", "reviewer_id"},
		{"reviews", "reviewee_id"},
	}

	for _, col := range columnsToCheck {
		var dataType string
		var charMaxLen *int
		err := pool.QueryRow(ctx, `
			SELECT data_type, character_maximum_length
			FROM information_schema.columns
			WHERE table_name = $1 AND column_name = $2
		`, col.table, col.column).Scan(&dataType, &charMaxLen)
		if err != nil {
			t.Fatalf("failed to query column info for %s.%s: %v", col.table, col.column, err)
		}
		if dataType != "character varying" {
			t.Errorf("expected %s.%s data_type 'character varying', got '%s'", col.table, col.column, dataType)
		}
		if charMaxLen == nil || *charMaxLen != 64 {
			t.Errorf("expected %s.%s character_maximum_length 64, got %v", col.table, col.column, charMaxLen)
		}
	}

	// Rolling back 000003 to UUID is destructive against later migrations and
	// golang-migrate-managed schemas; up-path VARCHAR(64) checks above are enough.
	var versionType string
	err = pool.QueryRow(ctx, `
		SELECT data_type FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'schema_migrations' AND column_name = 'version'
	`).Scan(&versionType)
	if err == nil && (versionType == "bigint" || versionType == "integer") {
		t.Log("skipping 000003 down/re-up cycle under golang-migrate schema_migrations")
		return
	}

	var laterMigrationExists bool
	_ = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'schema_migrations'
		) AND EXISTS (
			SELECT 1 FROM schema_migrations
			WHERE version > '000003'
		)`).Scan(&laterMigrationExists)
	if laterMigrationExists {
		t.Log("skipping 000003 down/re-up cycle: later migrations applied")
		return
	}

	// 3. Test rollback of 000003
	downSQL, err := os.ReadFile("../../migrations/000003_supertokens_identity.down.sql")
	if err != nil {
		t.Fatalf("failed to read 000003 down migration: %v", err)
	}

	if _, err := pool.Exec(ctx, string(downSQL)); err != nil {
		t.Fatalf("failed to execute 000003 down migration: %v", err)
	}

	// 4. Verify columns reverted to UUID
	for _, col := range columnsToCheck {
		var dataType string
		err := pool.QueryRow(ctx, `
			SELECT data_type
			FROM information_schema.columns
			WHERE table_name = $1 AND column_name = $2
		`, col.table, col.column).Scan(&dataType)
		if err != nil {
			t.Fatalf("failed to query column info after down migration for %s.%s: %v", col.table, col.column, err)
		}
		if dataType != "uuid" {
			t.Errorf("expected %s.%s data_type 'uuid' after rollback, got '%s'", col.table, col.column, dataType)
		}
	}

	// 5. Clean up schema_migrations and re-apply
	if _, err := pool.Exec(ctx, "DELETE FROM schema_migrations WHERE version = '000003_supertokens_identity.up.sql'"); err != nil {
		t.Fatalf("failed to clear migration version: %v", err)
	}
	if err := database.RunMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatalf("failed to re-apply 000003 up migration after rollback: %v", err)
	}
}

func TestMigration000004_UpSQL_Structure(t *testing.T) {
	contentBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000004_performance_and_contract_fixes.up.sql"))
	if err != nil {
		t.Fatalf("failed to read 000004 up migration: %v", err)
	}
	content := string(contentBytes)

	expectedFragments := []string{
		"ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_job_id_key;",
		"CREATE UNIQUE INDEX IF NOT EXISTS uq_contracts_active_job",
		"ON contracts(job_id)",
		"WHERE status NOT IN ('cancelled');",
		"CREATE INDEX IF NOT EXISTS idx_jobs_status_created_at ON jobs(status, created_at DESC);",
		"CREATE INDEX IF NOT EXISTS idx_jobs_department_lower ON jobs(LOWER(department));",
		"CREATE INDEX IF NOT EXISTS idx_jobs_required_skills_gin ON jobs USING GIN(required_skills);",
		"CREATE INDEX IF NOT EXISTS idx_applications_job_id_created ON applications(job_id, created_at DESC);",
		"CREATE INDEX IF NOT EXISTS idx_applications_applicant_created ON applications(applicant_id, created_at DESC);",
		"CREATE INDEX IF NOT EXISTS idx_contracts_client_created ON contracts(client_id, created_at DESC);",
		"CREATE INDEX IF NOT EXISTS idx_contracts_freelancer_created ON contracts(freelancer_id, created_at DESC);",
		"CREATE INDEX IF NOT EXISTS idx_reviews_reviewee_created ON reviews(reviewee_id, created_at DESC);",
		"ALTER TABLE contracts ADD CONSTRAINT chk_contracts_different_parties CHECK (client_id != freelancer_id);",
		"ALTER TABLE reviews ADD CONSTRAINT chk_reviews_different_parties CHECK (reviewer_id != reviewee_id);",
	}

	for _, fragment := range expectedFragments {
		if !strings.Contains(content, fragment) {
			t.Errorf("000004 up migration missing fragment: %s", fragment)
		}
	}
}

func TestMigration000004_DownSQL_Structure(t *testing.T) {
	contentBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000004_performance_and_contract_fixes.down.sql"))
	if err != nil {
		t.Fatalf("failed to read 000004 down migration: %v", err)
	}
	content := string(contentBytes)

	expectedFragments := []string{
		"ALTER TABLE reviews DROP CONSTRAINT IF EXISTS chk_reviews_different_parties;",
		"ALTER TABLE contracts DROP CONSTRAINT IF EXISTS chk_contracts_different_parties;",
		"DROP INDEX IF EXISTS idx_reviews_reviewee_created;",
		"DROP INDEX IF EXISTS idx_contracts_freelancer_created;",
		"DROP INDEX IF EXISTS idx_contracts_client_created;",
		"DROP INDEX IF EXISTS idx_applications_applicant_created;",
		"DROP INDEX IF EXISTS idx_applications_job_id_created;",
		"DROP INDEX IF EXISTS idx_jobs_required_skills_gin;",
		"DROP INDEX IF EXISTS idx_jobs_department_lower;",
		"DROP INDEX IF EXISTS idx_jobs_status_created_at;",
		"DROP INDEX IF EXISTS uq_contracts_active_job;",
		"ALTER TABLE contracts ADD CONSTRAINT contracts_job_id_key UNIQUE (job_id);",
	}

	for _, fragment := range expectedFragments {
		if !strings.Contains(content, fragment) {
			t.Errorf("000004 down migration missing fragment: %s", fragment)
		}
	}
}

func TestMigration000009_UpSQL_Structure(t *testing.T) {
	contentBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000009_foreign_key_indexes_and_restrict.up.sql"))
	if err != nil {
		t.Fatalf("failed to read 000009 up migration: %v", err)
	}
	content := string(contentBytes)

	expectedFragments := []string{
		"CREATE INDEX IF NOT EXISTS idx_contracts_job_id ON contracts(job_id);",
		"CREATE INDEX IF NOT EXISTS idx_contracts_application_id ON contracts(application_id);",
		"CREATE INDEX IF NOT EXISTS idx_reviews_reviewer_id ON reviews(reviewer_id);",
		"ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_job_id_fkey;",
		"ALTER TABLE applications ADD CONSTRAINT applications_job_id_fkey",
		"FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE RESTRICT;",
	}

	for _, fragment := range expectedFragments {
		if !strings.Contains(content, fragment) {
			t.Errorf("000009 up migration missing fragment: %s", fragment)
		}
	}
}

func TestMigration000009_DownSQL_Structure(t *testing.T) {
	contentBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000009_foreign_key_indexes_and_restrict.down.sql"))
	if err != nil {
		t.Fatalf("failed to read 000009 down migration: %v", err)
	}
	content := string(contentBytes)

	expectedFragments := []string{
		"DROP INDEX IF EXISTS idx_contracts_job_id;",
		"DROP INDEX IF EXISTS idx_contracts_application_id;",
		"DROP INDEX IF EXISTS idx_reviews_reviewer_id;",
		"ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_job_id_fkey;",
		"ALTER TABLE applications ADD CONSTRAINT applications_job_id_fkey",
		"FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;",
	}

	for _, fragment := range expectedFragments {
		if !strings.Contains(content, fragment) {
			t.Errorf("000009 down migration missing fragment: %s", fragment)
		}
	}
}

func TestMigration000010_UpSQL_Structure(t *testing.T) {
	contentBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000010_seed_test_campus_members.up.sql"))
	if err != nil {
		t.Fatalf("failed to read 000010 up migration: %v", err)
	}
	content := string(contentBytes)

	expectedFragments := []string{
		"INSERT INTO users (id, email, role, created_at, updated_at)",
		"'a2be7abb-a253-4b8d-b3c3-5a5f39416239', 'poster@campus.edu', 'member'",
		"'2e961701-96a1-43ef-9201-f9720bdef3d5', 'applicant@campus.edu', 'member'",
		"'e4264e59-8290-4a3f-91fe-4826d9165c89', 'unverified@campus.edu', 'member'",
		"INSERT INTO profiles (",
		"'a2be7abb-a253-4b8d-b3c3-5a5f39416239'",
		"'2e961701-96a1-43ef-9201-f9720bdef3d5'",
		"'e4264e59-8290-4a3f-91fe-4826d9165c89'",
		"CREATE EXTENSION IF NOT EXISTS dblink;",
		"INSERT INTO roles (app_id, role)",
		"INSERT INTO emailverification_verified_emails",
		"INSERT INTO user_roles",
	}

	for _, fragment := range expectedFragments {
		if !strings.Contains(content, fragment) {
			t.Errorf("000010 up migration missing fragment: %s", fragment)
		}
	}
}

func TestMigration000010_DownSQL_Structure(t *testing.T) {
	contentBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000010_seed_test_campus_members.down.sql"))
	if err != nil {
		t.Fatalf("failed to read 000010 down migration: %v", err)
	}
	content := string(contentBytes)

	expectedFragments := []string{
		"DELETE FROM profiles WHERE user_id IN (",
		"'a2be7abb-a253-4b8d-b3c3-5a5f39416239'",
		"'2e961701-96a1-43ef-9201-f9720bdef3d5'",
		"'e4264e59-8290-4a3f-91fe-4826d9165c89'",
		"DELETE FROM users WHERE id IN (",
		"DELETE FROM user_roles WHERE user_id IN",
		"DELETE FROM emailverification_verified_emails WHERE user_id IN",
		"DELETE FROM emailpassword_users WHERE user_id IN",
	}

	for _, fragment := range expectedFragments {
		if !strings.Contains(content, fragment) {
			t.Errorf("000010 down migration missing fragment: %s", fragment)
		}
	}
}

func TestMigration000011_UpSQL_Structure(t *testing.T) {
	contentBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000011_remove_monetary_fields.up.sql"))
	if err != nil {
		t.Fatalf("failed to read 000011 up migration: %v", err)
	}
	content := string(contentBytes)

	expectedFragments := []string{
		"ALTER TABLE jobs DROP COLUMN IF EXISTS budget;",
		"ALTER TABLE jobs DROP COLUMN IF EXISTS pay_type;",
		"ALTER TABLE contracts DROP COLUMN IF EXISTS agreed_budget;",
	}

	for _, fragment := range expectedFragments {
		if !strings.Contains(content, fragment) {
			t.Errorf("000011 up migration missing fragment: %s", fragment)
		}
	}
}

func TestMigration000011_DownSQL_Structure(t *testing.T) {
	contentBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000011_remove_monetary_fields.down.sql"))
	if err != nil {
		t.Fatalf("failed to read 000011 down migration: %v", err)
	}
	content := string(contentBytes)

	expectedFragments := []string{
		"ALTER TABLE jobs ADD COLUMN IF NOT EXISTS budget NUMERIC(10, 2) NOT NULL DEFAULT 0.00 CHECK (budget >= 0);",
		"ALTER TABLE jobs ADD COLUMN IF NOT EXISTS pay_type VARCHAR(50) NOT NULL DEFAULT 'fixed' CHECK (pay_type IN ('fixed', 'hourly'));",
		"ALTER TABLE contracts ADD COLUMN IF NOT EXISTS agreed_budget NUMERIC(10, 2) NOT NULL DEFAULT 0.00 CHECK (agreed_budget >= 0);",
	}

	for _, fragment := range expectedFragments {
		if !strings.Contains(content, fragment) {
			t.Errorf("000011 down migration missing fragment: %s", fragment)
		}
	}
}

func TestMigration000012_UpSQL_Structure(t *testing.T) {
	contentBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000012_ai_subsystem_init.up.sql"))
	if err != nil {
		t.Fatalf("failed to read 000012 up migration: %v", err)
	}
	content := string(contentBytes)

	expectedFragments := []string{
		"CREATE EXTENSION IF NOT EXISTS vector;",
		"CREATE TABLE IF NOT EXISTS skills (",
		"canonical_name VARCHAR(100) NOT NULL UNIQUE",
		"parent_skill_id UUID REFERENCES skills(id) ON DELETE SET NULL",
		"CREATE TABLE IF NOT EXISTS skill_aliases (",
		"skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE",
		"alias VARCHAR(100) NOT NULL UNIQUE",
		"source VARCHAR(50) NOT NULL DEFAULT 'manual'",
		"confidence DOUBLE PRECISION NOT NULL DEFAULT 1.0",
		"CREATE TABLE IF NOT EXISTS profile_skills (",
		"user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE",
		"skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE",
		"confidence DOUBLE PRECISION NOT NULL DEFAULT 1.0",
		"source VARCHAR(50) NOT NULL DEFAULT 'user'",
		"verified BOOLEAN NOT NULL DEFAULT false",
		"PRIMARY KEY (user_id, skill_id)",
		"CREATE TABLE IF NOT EXISTS job_skills (",
		"job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE",
		"skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE",
		"importance VARCHAR(20) NOT NULL DEFAULT 'required'",
		"required BOOLEAN NOT NULL DEFAULT true",
		"confidence DOUBLE PRECISION NOT NULL DEFAULT 1.0",
		"PRIMARY KEY (job_id, skill_id)",
		"CREATE TABLE IF NOT EXISTS ai_embeddings (",
		"embedding vector(384) NOT NULL",
		"UNIQUE(entity_type, entity_id, embedding_type, model_name, model_version)",
		"CREATE TABLE IF NOT EXISTS ai_runs (",
		"feature VARCHAR(50) NOT NULL",
		"pipeline_version VARCHAR(50) NOT NULL",
		"input_hash VARCHAR(64) NOT NULL",
		"output_json JSONB NOT NULL DEFAULT '{}'",
		"status VARCHAR(20) NOT NULL DEFAULT 'success'",
		"CREATE TABLE IF NOT EXISTS ai_recommendations (",
		"user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE",
		"metadata JSONB NOT NULL DEFAULT '{}'",
		"status VARCHAR(20) NOT NULL DEFAULT 'active'",
		"CREATE TABLE IF NOT EXISTS application_ai_scores (",
		"application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE",
		"job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE",
		"matched_skills TEXT[] NOT NULL DEFAULT '{}'",
		"missing_skills TEXT[] NOT NULL DEFAULT '{}'",
		"job_version INTEGER NOT NULL DEFAULT 1",
		"profile_version INTEGER NOT NULL DEFAULT 1",
		"is_stale BOOLEAN NOT NULL DEFAULT false",
		"UNIQUE(application_id)",
		"CREATE TABLE IF NOT EXISTS moderation_events (",
		"risk_score DOUBLE PRECISION NOT NULL",
		"decision VARCHAR(20) NOT NULL DEFAULT 'allow'",
		"signals TEXT[] NOT NULL DEFAULT '{}'",
		"CREATE TABLE IF NOT EXISTS review_insights (",
		"user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE",
		"aspect VARCHAR(50) NOT NULL",
		"sample_count INTEGER NOT NULL DEFAULT 1",
		"is_recurring BOOLEAN NOT NULL DEFAULT false",
		"strengths TEXT[] NOT NULL DEFAULT '{}'",
		"improvements TEXT[] NOT NULL DEFAULT '{}'",
		"UNIQUE(user_id, aspect)",
		"CREATE TABLE IF NOT EXISTS skill_demand_snapshots (",
		"skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE",
		"period VARCHAR(20) NOT NULL",
		"demand_score DOUBLE PRECISION NOT NULL DEFAULT 0.0",
		"growth_rate DOUBLE PRECISION NOT NULL DEFAULT 0.0",
		"UNIQUE(skill_id, period)",
		"CREATE TABLE IF NOT EXISTS ai_jobs (",
		"job_type VARCHAR(50) NOT NULL",
		"payload JSONB NOT NULL DEFAULT '{}'",
		"status VARCHAR(20) NOT NULL DEFAULT 'pending'",
		"attempts INTEGER NOT NULL DEFAULT 0",
		"max_attempts INTEGER NOT NULL DEFAULT 3",
		"CREATE INDEX IF NOT EXISTS idx_ai_embeddings_embedding ON ai_embeddings USING hnsw (embedding vector_cosine_ops);",
		"CREATE INDEX IF NOT EXISTS idx_ai_embeddings_entity ON ai_embeddings(entity_type, entity_id);",
		"CREATE INDEX IF NOT EXISTS idx_ai_runs_entity ON ai_runs(entity_type, entity_id);",
		"CREATE INDEX IF NOT EXISTS idx_ai_runs_feature ON ai_runs(feature);",
		"CREATE INDEX IF NOT EXISTS idx_ai_recommendations_user_id ON ai_recommendations(user_id);",
		"CREATE INDEX IF NOT EXISTS idx_application_ai_scores_job_id ON application_ai_scores(job_id);",
		"CREATE INDEX IF NOT EXISTS idx_moderation_events_entity ON moderation_events(entity_type, entity_id);",
		"CREATE INDEX IF NOT EXISTS idx_ai_jobs_status_created_at ON ai_jobs(status, created_at);",
	}

	for _, fragment := range expectedFragments {
		if !strings.Contains(content, fragment) {
			t.Errorf("000012 up migration missing fragment: %s", fragment)
		}
	}
}

func TestMigration000012_DownSQL_Structure(t *testing.T) {
	contentBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000012_ai_subsystem_init.down.sql"))
	if err != nil {
		t.Fatalf("failed to read 000012 down migration: %v", err)
	}
	content := string(contentBytes)

	expectedFragments := []string{
		"DROP TABLE IF EXISTS ai_jobs;",
		"DROP TABLE IF EXISTS skill_demand_snapshots;",
		"DROP TABLE IF EXISTS review_insights;",
		"DROP TABLE IF EXISTS moderation_events;",
		"DROP TABLE IF EXISTS application_ai_scores;",
		"DROP TABLE IF EXISTS ai_recommendations;",
		"DROP TABLE IF EXISTS ai_runs;",
		"DROP TABLE IF EXISTS ai_embeddings;",
		"DROP TABLE IF EXISTS job_skills;",
		"DROP TABLE IF EXISTS profile_skills;",
		"DROP TABLE IF EXISTS skill_aliases;",
		"DROP TABLE IF EXISTS skills;",
		"DROP EXTENSION IF EXISTS vector;",
	}

	for _, fragment := range expectedFragments {
		if !strings.Contains(content, fragment) {
			t.Errorf("000012 down migration missing fragment: %s", fragment)
		}
	}
}


