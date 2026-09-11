package application

import (
	"strings"
	"testing"
)

func TestAcceptLockQueryIncludesDeadline(t *testing.T) {
	q := lockJobOnAcceptQuery()
	if !strings.Contains(q, "j.deadline") {
		t.Fatalf("accept lock query must select deadline, got %s", q)
	}
	if !strings.Contains(q, "FOR UPDATE") {
		t.Fatalf("expected FOR UPDATE, got %s", q)
	}
}

func TestAcceptApplicationTx_RejectsAllOtherApplicationsNotOnlyPending(t *testing.T) {
	q := rejectOtherApplicationsQuery()
	if strings.Contains(q, "AND status = $4") || strings.Contains(q, "AND status = 'pending'") {
		t.Fatalf("accept must reject ALL other applications for the job, not only pending; query=%s", q)
	}
	if !strings.Contains(q, "id !=") && !strings.Contains(q, "id <>") {
		t.Fatalf("expected exclude accepted application id, got %s", q)
	}
}
