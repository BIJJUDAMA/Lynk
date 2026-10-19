package project

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidCursor = errors.New("invalid pagination cursor")

// EncodeCursor serializes a timestamp and UUID into a URL-safe base64 string.
func EncodeCursor(t time.Time, id uuid.UUID) string {
	raw := fmt.Sprintf("%d,%s", t.UTC().UnixNano(), id.String())
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// ValidateCursor reports whether a pagination cursor is acceptable. An empty
// cursor is valid (it means "start from the first page"); anything else must
// decode cleanly.
func ValidateCursor(cursor string) error {
	if strings.TrimSpace(cursor) == "" {
		return nil
	}
	_, _, err := DecodeCursor(cursor)
	return err
}

// NextCursor returns the cursor that continues a keyset page after the given
// projects. It returns an empty string when there are no projects to continue
// from; otherwise it encodes the last project's creation time and ID.
func NextCursor(projects []Project, limit int) string {
	if len(projects) == 0 {
		return ""
	}
	last := projects[len(projects)-1]
	return EncodeCursor(last.CreatedAt, last.ID)
}

// DecodeCursor parses a base64 pagination cursor into timestamp and UUID.
func DecodeCursor(cursor string) (time.Time, uuid.UUID, error) {
	trimmed := strings.TrimSpace(cursor)
	if trimmed == "" {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}

	data, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: base64 decode failed", ErrInvalidCursor)
	}

	parts := strings.Split(string(data), ",")
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: malformed structure", ErrInvalidCursor)
	}

	var nano int64
	if _, err := fmt.Sscanf(parts[0], "%d", &nano); err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: invalid timestamp", ErrInvalidCursor)
	}

	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: invalid uuid", ErrInvalidCursor)
	}

	return time.Unix(0, nano).UTC(), id, nil
}
