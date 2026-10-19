package database_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigration000017_FilesExistAndSyntaxValid(t *testing.T) {
	upPath := filepath.Clean(filepath.Join("..", "..", "migrations", "000017_project_core_schema.up.sql"))
	downPath := filepath.Clean(filepath.Join("..", "..", "migrations", "000017_project_core_schema.down.sql"))

	// #nosec G304 -- test file path is fixed and internal
	upBytes, err := os.ReadFile(upPath) //nolint:gosec
	if err != nil {
		t.Fatalf("failed to read 000017 up migration: %v", err)
	}
	upContent := string(upBytes)
	if len(strings.TrimSpace(upContent)) == 0 {
		t.Fatalf("000017 up migration file is empty")
	}

	// #nosec G304 -- test file path is fixed and internal
	downBytes, err := os.ReadFile(downPath) //nolint:gosec
	if err != nil {
		t.Fatalf("failed to read 000017 down migration: %v", err)
	}
	downContent := string(downBytes)
	if len(strings.TrimSpace(downContent)) == 0 {
		t.Fatalf("000017 down migration file is empty")
	}

	expectedTypes := []string{
		"project_visibility",
		"project_recruitment_status",
		"project_lifecycle_status",
		"project_member_role",
	}
	for _, typ := range expectedTypes {
		if !strings.Contains(upContent, typ) {
			t.Errorf("000017 up migration missing type: %s", typ)
		}
		if !strings.Contains(downContent, typ) {
			t.Errorf("000017 down migration missing drop for type: %s", typ)
		}
	}

	expectedTables := []string{
		"projects",
		"project_members",
		"project_sections",
		"project_resources",
		"project_activity",
	}
	for _, tbl := range expectedTables {
		if !strings.Contains(upContent, "CREATE TABLE IF NOT EXISTS "+tbl) {
			t.Errorf("000017 up migration missing CREATE TABLE for: %s", tbl)
		}
		if !strings.Contains(downContent, "DROP TABLE IF EXISTS "+tbl) {
			t.Errorf("000017 down migration missing DROP TABLE for: %s", tbl)
		}
	}

	expectedConstraints := []string{
		"uq_project_member UNIQUE (project_id, member_id)",
		"uq_project_section_type UNIQUE (project_id, section_type)",
		"REFERENCES users(id) ON DELETE RESTRICT",
		"REFERENCES users(id) ON DELETE CASCADE",
		"REFERENCES projects(id) ON DELETE CASCADE",
	}
	for _, c := range expectedConstraints {
		if !strings.Contains(upContent, c) {
			t.Errorf("000017 up migration missing constraint: %s", c)
		}
	}

	expectedIndexes := []string{
		"idx_projects_visibility",
		"idx_projects_recruitment",
		"idx_projects_lifecycle",
		"idx_projects_owner",
		"idx_projects_category",
		"idx_project_members_project",
		"idx_project_members_member",
		"idx_project_members_role",
		"idx_project_sections_order",
		"idx_project_resources_project",
		"idx_project_activity_project",
	}
	for _, idx := range expectedIndexes {
		if !strings.Contains(upContent, idx) {
			t.Errorf("000017 up migration missing index: %s", idx)
		}
	}
}

func TestMigration000018_FilesExistAndSyntaxValid(t *testing.T) {
	upPath := filepath.Clean(filepath.Join("..", "..", "migrations", "000018_recruitment_and_applications.up.sql"))
	downPath := filepath.Clean(filepath.Join("..", "..", "migrations", "000018_recruitment_and_applications.down.sql"))

	// #nosec G304 -- test file path is fixed and internal
	upBytes, err := os.ReadFile(upPath) //nolint:gosec
	if err != nil {
		t.Fatalf("failed to read 000018 up migration: %v", err)
	}
	upContent := string(upBytes)
	if len(strings.TrimSpace(upContent)) == 0 {
		t.Fatalf("000018 up migration file is empty")
	}

	// #nosec G304 -- test file path is fixed and internal
	downBytes, err := os.ReadFile(downPath) //nolint:gosec
	if err != nil {
		t.Fatalf("failed to read 000018 down migration: %v", err)
	}
	downContent := string(downBytes)
	if len(strings.TrimSpace(downContent)) == 0 {
		t.Fatalf("000018 down migration file is empty")
	}

	expectedTypes := []string{
		"project_application_status",
		"invitation_status",
	}
	for _, typ := range expectedTypes {
		if !strings.Contains(upContent, typ) {
			t.Errorf("000018 up migration missing type: %s", typ)
		}
		if !strings.Contains(downContent, typ) {
			t.Errorf("000018 down migration missing drop for type: %s", typ)
		}
	}

	expectedTables := []string{
		"application_questions",
		"project_applications",
		"application_answers",
		"project_invitations",
	}
	for _, tbl := range expectedTables {
		if !strings.Contains(upContent, "CREATE TABLE IF NOT EXISTS "+tbl) {
			t.Errorf("000018 up migration missing CREATE TABLE for: %s", tbl)
		}
		if !strings.Contains(downContent, "DROP TABLE IF EXISTS "+tbl) {
			t.Errorf("000018 down migration missing DROP TABLE for: %s", tbl)
		}
	}

	expectedConstraints := []string{
		"uq_project_applicant UNIQUE (project_id, applicant_id)",
		"uq_application_question_answer UNIQUE (application_id, question_id)",
		"uq_project_invitee UNIQUE (project_id, invitee_id)",
		"REFERENCES projects(id) ON DELETE CASCADE",
		"REFERENCES users(id) ON DELETE CASCADE",
		"REFERENCES users(id) ON DELETE RESTRICT",
		"REFERENCES users(id) ON DELETE SET NULL",
		"REFERENCES application_questions(id) ON DELETE CASCADE",
		"REFERENCES project_applications(id) ON DELETE CASCADE",
	}
	for _, c := range expectedConstraints {
		if !strings.Contains(upContent, c) {
			t.Errorf("000018 up migration missing constraint: %s", c)
		}
	}

	expectedIndexes := []string{
		"idx_application_questions_project",
		"idx_project_applications_project",
		"idx_project_applications_applicant",
		"idx_application_answers_app",
		"idx_project_invitations_invitee",
	}
	for _, idx := range expectedIndexes {
		if !strings.Contains(upContent, idx) {
			t.Errorf("000018 up migration missing index: %s", idx)
		}
	}
}

func TestMigration000019_FilesExistAndSyntaxValid(t *testing.T) {
	upPath := filepath.Clean(filepath.Join("..", "..", "migrations", "000019_project_vector_search.up.sql"))
	downPath := filepath.Clean(filepath.Join("..", "..", "migrations", "000019_project_vector_search.down.sql"))

	// #nosec G304 -- test file path is fixed and internal
	upBytes, err := os.ReadFile(upPath) //nolint:gosec
	if err != nil {
		t.Fatalf("failed to read 000019 up migration: %v", err)
	}
	upContent := string(upBytes)
	if len(strings.TrimSpace(upContent)) == 0 {
		t.Fatalf("000019 up migration file is empty")
	}

	// #nosec G304 -- test file path is fixed and internal
	downBytes, err := os.ReadFile(downPath) //nolint:gosec
	if err != nil {
		t.Fatalf("failed to read 000019 down migration: %v", err)
	}
	downContent := string(downBytes)
	if len(strings.TrimSpace(downContent)) == 0 {
		t.Fatalf("000019 down migration file is empty")
	}

	expectedUpSubstrings := []string{
		"CREATE EXTENSION IF NOT EXISTS vector;",
		"CREATE TABLE IF NOT EXISTS project_embeddings",
		"project_id UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE",
		"embedding vector(384) NOT NULL",
		"content_hash VARCHAR(64) NOT NULL",
		"updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()",
		"CREATE INDEX IF NOT EXISTS idx_project_embeddings_hnsw",
		"USING hnsw (embedding vector_cosine_ops)",
		"WITH (m = 16, ef_construction = 64);",
	}
	for _, substr := range expectedUpSubstrings {
		if !strings.Contains(upContent, substr) {
			t.Errorf("000019 up migration missing expected substring: %s", substr)
		}
	}

	expectedDownSubstrings := []string{
		"DROP TABLE IF EXISTS project_embeddings CASCADE;",
	}
	for _, substr := range expectedDownSubstrings {
		if !strings.Contains(downContent, substr) {
			t.Errorf("000019 down migration missing expected substring: %s", substr)
		}
	}
}

func TestMigration000020_FilesExistAndSyntaxValid(t *testing.T) {
	upPath := filepath.Clean(filepath.Join("..", "..", "migrations", "000020_project_feed_covering_indexes.up.sql"))
	downPath := filepath.Clean(filepath.Join("..", "..", "migrations", "000020_project_feed_covering_indexes.down.sql"))

	// #nosec G304 -- test file path is fixed and internal
	upBytes, err := os.ReadFile(upPath) //nolint:gosec
	if err != nil {
		t.Fatalf("failed to read 000020 up migration: %v", err)
	}
	upContent := string(upBytes)
	if len(strings.TrimSpace(upContent)) == 0 {
		t.Fatalf("000020 up migration file is empty")
	}

	// #nosec G304 -- test file path is fixed and internal
	downBytes, err := os.ReadFile(downPath) //nolint:gosec
	if err != nil {
		t.Fatalf("failed to read 000020 down migration: %v", err)
	}
	downContent := string(downBytes)
	if len(strings.TrimSpace(downContent)) == 0 {
		t.Fatalf("000020 down migration file is empty")
	}

	expectedUpSubstrings := []string{
		"CREATE INDEX IF NOT EXISTS idx_projects_feed_covering",
		"ON projects (visibility, recruitment_status, category, created_at DESC)",
		"INCLUDE (id, slug, title, description, owner_member_id, lifecycle_status);",
		"CREATE INDEX IF NOT EXISTS idx_projects_active_feed",
		"ON projects (visibility, created_at DESC)",
		"WHERE visibility = 'public';",
		"CREATE INDEX IF NOT EXISTS idx_project_members_lookup",
		"ON project_members (project_id, role, status);",
	}
	for _, substr := range expectedUpSubstrings {
		if !strings.Contains(upContent, substr) {
			t.Errorf("000020 up migration missing expected substring: %s", substr)
		}
	}

	expectedDownSubstrings := []string{
		"DROP INDEX IF EXISTS idx_project_members_lookup;",
		"DROP INDEX IF EXISTS idx_projects_active_feed;",
		"DROP INDEX IF EXISTS idx_projects_feed_covering;",
	}
	for _, substr := range expectedDownSubstrings {
		if !strings.Contains(downContent, substr) {
			t.Errorf("000020 down migration missing expected substring: %s", substr)
		}
	}
}
