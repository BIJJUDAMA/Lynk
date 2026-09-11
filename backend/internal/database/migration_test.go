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

