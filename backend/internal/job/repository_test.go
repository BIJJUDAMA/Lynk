package job

import (
	"strings"
	"testing"
)

func TestPublicListingDeadlineCondition_SkippedForOwnerListings(t *testing.T) {
	owner := "usr_owner"
	filter := JobFilter{CreatedBy: &owner}

	if cond := publicListingDeadlineCondition(filter); cond != "" {
		t.Fatalf("owner listing must not apply deadline filter; got %q", cond)
	}
}

func TestPublicListingDeadlineCondition_AppliedForPublicBrowse(t *testing.T) {
	for _, filter := range []JobFilter{{}, {Status: StatusOpen}} {
		cond := publicListingDeadlineCondition(filter)
		if !strings.Contains(cond, "j.deadline") {
			t.Fatalf("public browse should hide expired deadlines; filter=%+v cond=%q", filter, cond)
		}
	}
}

func TestListJobs_SkillFilterUsesArrayContains(t *testing.T) {
	q := skillFilterSQL(1)
	if !strings.Contains(q, "@>") {
		t.Fatalf("expected GIN-friendly @> ARRAY predicate, got %s", q)
	}
	if strings.Contains(q, "= ANY(") {
		t.Fatalf("ANY(skill) prevents GIN array index usage: %s", q)
	}
}

func TestListJobs_SearchUsesTrigramFriendlyPattern(t *testing.T) {
	q := searchFilterSQL(1)
	if !strings.Contains(strings.ToUpper(q), "ILIKE") && !strings.Contains(q, "%") {
		t.Fatalf("expected ILIKE search predicate, got %s", q)
	}
}
