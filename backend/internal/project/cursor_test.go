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
