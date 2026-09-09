package database_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lynk/backend/internal/database"
)

func TestMigrationFilesExist(t *testing.T) {
	dir := "../../migrations"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	var foundUp1, foundDown1, foundUp2, foundDown2, foundUp3, foundDown3, foundUp4, foundDown4 bool
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".sql" {
			switch filepath.Base(e.Name()) {
			case "000001_init_schema.up.sql":
				foundUp1 = true
			case "000001_init_schema.down.sql":
				foundDown1 = true
			case "000002_campus_member_unification.up.sql":
				foundUp2 = true
			case "000002_campus_member_unification.down.sql":
				foundDown2 = true
			case "000003_supertokens_identity.up.sql":
				foundUp3 = true
			case "000003_supertokens_identity.down.sql":
				foundDown3 = true
			case "000004_performance_and_contract_fixes.up.sql":
				foundUp4 = true
			case "000004_performance_and_contract_fixes.down.sql":
				foundDown4 = true
			}
		}
	}

	if !foundUp1 {
		t.Errorf("expected 000001_init_schema.up.sql to exist")
	}
	if !foundDown1 {
		t.Errorf("expected 000001_init_schema.down.sql to exist")
	}
	if !foundUp2 {
		t.Errorf("expected 000002_campus_member_unification.up.sql to exist")
	}
	if !foundDown2 {
		t.Errorf("expected 000002_campus_member_unification.down.sql to exist")
	}
	if !foundUp3 {
		t.Errorf("expected 000003_supertokens_identity.up.sql to exist")
	}
	if !foundDown3 {
		t.Errorf("expected 000003_supertokens_identity.down.sql to exist")
	}
	if !foundUp4 {
		t.Errorf("expected 000004_performance_and_contract_fixes.up.sql to exist")
	}
	if !foundDown4 {
		t.Errorf("expected 000004_performance_and_contract_fixes.down.sql to exist")
	}
}

func TestNewPool_InvalidURL(t *testing.T) {
	ctx := context.Background()
	_, err := database.NewPool(ctx, "postgres://invalid uri with spaces")
	if err == nil {
		t.Error("expected error for invalid database URL, got nil")
	}
}

func TestRunMigrations_Integration(t *testing.T) {
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

	// 1. Run migrations up
	if err := database.RunMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// 2. Verify schema after up migration
	var profilesExists bool
	err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'profiles')").Scan(&profilesExists)
	if err != nil || !profilesExists {
		t.Fatalf("expected 'profiles' table to exist after up migration, err: %v", err)
	}

	var createdByExists bool
	err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = 'jobs' AND column_name = 'created_by')").Scan(&createdByExists)
	if err != nil || !createdByExists {
		t.Fatalf("expected jobs.created_by column to exist, err: %v", err)
	}

	var applicantIdExists bool
	err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = 'applications' AND column_name = 'applicant_id')").Scan(&applicantIdExists)
	if err != nil || !applicantIdExists {
		t.Fatalf("expected applications.applicant_id column to exist, err: %v", err)
	}

	var clientIdExists, freelancerIdExists bool
	err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = 'contracts' AND column_name = 'client_id')").Scan(&clientIdExists)
	if err != nil || !clientIdExists {
		t.Fatalf("expected contracts.client_id column to exist, err: %v", err)
	}
	err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = 'contracts' AND column_name = 'freelancer_id')").Scan(&freelancerIdExists)
	if err != nil || !freelancerIdExists {
		t.Fatalf("expected contracts.freelancer_id column to exist, err: %v", err)
	}

	// 3. Test rollback (down migration)
	downSQL, err := os.ReadFile("../../migrations/000002_campus_member_unification.down.sql")
	if err != nil {
		t.Fatalf("failed to read down migration: %v", err)
	}

	if _, err := pool.Exec(ctx, string(downSQL)); err != nil {
		t.Fatalf("failed to execute down migration: %v", err)
	}

	// 4. Verify schema after rollback
	err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'profiles')").Scan(&profilesExists)
	if err != nil || profilesExists {
		t.Fatalf("expected 'profiles' table to NOT exist after down migration, err: %v", err)
	}

	var studentProfilesExists, employerProfilesExists bool
	err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'student_profiles')").Scan(&studentProfilesExists)
	if err != nil || !studentProfilesExists {
		t.Fatalf("expected 'student_profiles' table to exist after down migration, err: %v", err)
	}
	err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'employer_profiles')").Scan(&employerProfilesExists)
	if err != nil || !employerProfilesExists {
		t.Fatalf("expected 'employer_profiles' table to exist after down migration, err: %v", err)
	}

	// 5. Clean up down migration record from schema_migrations and re-apply up migration
	if _, err := pool.Exec(ctx, "DELETE FROM schema_migrations WHERE version = '000002_campus_member_unification.up.sql'"); err != nil {
		t.Fatalf("failed to clear migration version: %v", err)
	}
	if err := database.RunMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatalf("failed to re-apply up migration after rollback: %v", err)
	}
}
