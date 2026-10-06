package database_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigration000015_ValidSchema(t *testing.T) {
	upPath := filepath.Clean(filepath.Join("..", "..", "migrations", "000015_phase1_5_hardening.up.sql"))
	downPath := filepath.Clean(filepath.Join("..", "..", "migrations", "000015_phase1_5_hardening.down.sql"))

	// #nosec G304 -- test file path is fixed and internal
	upBytes, err := os.ReadFile(upPath) //nolint:gosec
	if err != nil {
		t.Fatalf("failed to read 000015 up migration: %v", err)
	}
	upContent := string(upBytes)

	// #nosec G304 -- test file path is fixed and internal
	downBytes, err := os.ReadFile(downPath) //nolint:gosec
	if err != nil {
		t.Fatalf("failed to read 000015 down migration: %v", err)
	}
	downContent := string(downBytes)

	// 1. Migration 000015 does not reference idx_jobs_embedding or idx_profiles_embedding
	forbiddenTokens := []string{
		"idx_jobs_embedding",
		"idx_profiles_embedding",
	}
	for _, token := range forbiddenTokens {
		if strings.Contains(upContent, token) {
			t.Errorf("000015 up migration should not reference %s", token)
		}
		if strings.Contains(downContent, token) {
			t.Errorf("000015 down migration should not reference %s", token)
		}
	}

	// 2. PRIMARY KEY (content_hash, model_name) is defined on embedding_cache
	expectedPK := "PRIMARY KEY (content_hash, model_name)"
	if !strings.Contains(upContent, expectedPK) {
		t.Errorf("000015 up migration missing composite primary key: %s", expectedPK)
	}

	// 3. idx_outbox_pending_events covers (next_retry_at, created_at)
	expectedOutboxIndex := "ON outbox_events (next_retry_at, created_at)"
	if !strings.Contains(upContent, expectedOutboxIndex) {
		t.Errorf("000015 up migration missing index coverage: %s", expectedOutboxIndex)
	}

	expectedOutboxFilter := "WHERE status IN ('pending', 'failed')"
	if !strings.Contains(upContent, expectedOutboxFilter) {
		t.Errorf("000015 up migration missing index filter: %s", expectedOutboxFilter)
	}
}
