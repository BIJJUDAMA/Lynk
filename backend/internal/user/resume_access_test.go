package user

import (
	"strings"
	"testing"
)

func TestResumeOwnerAccessQuery_LimitsStatus(t *testing.T) {
	q := jobOwnerResumeAccessQuery()
	if !strings.Contains(q, "pending") || !strings.Contains(q, "accepted") {
		t.Fatalf("expected pending/accepted filter, got %s", q)
	}
	if !strings.Contains(q, "in_progress") {
		t.Fatalf("expected job status filter, got %s", q)
	}
}
