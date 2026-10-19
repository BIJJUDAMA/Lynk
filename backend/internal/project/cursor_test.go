package project

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCursor_EncodeDecode(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	id := uuid.New()

	cursor := EncodeCursor(now, id)
	if cursor == "" {
		t.Fatal("expected non-empty cursor string")
	}

	decodedTime, decodedID, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("failed to decode cursor: %v", err)
	}

	if !decodedTime.Equal(now) {
		t.Fatalf("time mismatch: expected %v, got %v", now, decodedTime)
	}
	if decodedID != id {
		t.Fatalf("id mismatch: expected %v, got %v", id, decodedID)
	}
}

func TestCursor_DecodeInvalid(t *testing.T) {
	_, _, err := DecodeCursor("invalid-base64-string!@#$")
	if err == nil {
		t.Fatal("expected error on malformed cursor")
	}

	_, _, err = DecodeCursor("")
	if err == nil {
		t.Fatal("expected error on empty cursor")
	}
}

func TestValidateCursor(t *testing.T) {
	if err := ValidateCursor(""); err != nil {
		t.Fatalf("empty cursor must be valid, got: %v", err)
	}
	if err := ValidateCursor("   "); err != nil {
		t.Fatalf("whitespace cursor must be valid, got: %v", err)
	}

	valid := EncodeCursor(time.Now().UTC(), uuid.New())
	if err := ValidateCursor(valid); err != nil {
		t.Fatalf("well-formed cursor must be valid, got: %v", err)
	}

	if err := ValidateCursor("not-a-cursor!!"); err == nil {
		t.Fatal("expected error on malformed cursor")
	}
	if err := ValidateCursor("Zm9vYmFy"); err == nil {
		t.Fatal("expected error on structurally invalid cursor")
	}
}

func TestNextCursor(t *testing.T) {
	if got := NextCursor(nil, 20); got != "" {
		t.Fatalf("expected empty cursor for nil slice, got %q", got)
	}
	if got := NextCursor([]Project{}, 20); got != "" {
		t.Fatalf("expected empty cursor for empty slice, got %q", got)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	last := Project{ID: uuid.New(), CreatedAt: now}

	smaller := []Project{
		{ID: uuid.New(), CreatedAt: now.Add(-time.Minute)},
		last,
	}

	larger := make([]Project, 0, 25)
	for i := 0; i < 24; i++ {
		larger = append(larger, Project{ID: uuid.New(), CreatedAt: now.Add(-time.Duration(i) * time.Minute)})
	}
	larger = append(larger, last)

	for _, tc := range []struct {
		name     string
		projects []Project
		limit    int
	}{
		{name: "smaller than limit", projects: smaller, limit: 20},
		{name: "larger than limit", projects: larger, limit: 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := NextCursor(tc.projects, tc.limit)
			if got == "" {
				t.Fatal("expected non-empty next cursor")
			}
			decodedTime, decodedID, err := DecodeCursor(got)
			if err != nil {
				t.Fatalf("failed to decode next cursor: %v", err)
			}
			if !decodedTime.Equal(now) {
				t.Fatalf("cursor time mismatch: want %v, got %v", now, decodedTime)
			}
			if decodedID != last.ID {
				t.Fatalf("cursor id mismatch: want %v, got %v", last.ID, decodedID)
			}
		})
	}
}
