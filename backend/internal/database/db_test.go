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

	var foundUp, foundDown bool
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".sql" {
			if filepath.Base(e.Name()) == "000001_init_schema.up.sql" {
				foundUp = true
			}
			if filepath.Base(e.Name()) == "000001_init_schema.down.sql" {
				foundDown = true
			}
		}
	}

	if !foundUp {
		t.Errorf("expected 000001_init_schema.up.sql to exist")
	}
	if !foundDown {
		t.Errorf("expected 000001_init_schema.down.sql to exist")
	}
}

func TestNewPool_InvalidURL(t *testing.T) {
	ctx := context.Background()
	_, err := database.NewPool(ctx, "postgres://invalid uri with spaces")
	if err == nil {
		t.Error("expected error for invalid database URL, got nil")
	}
}
