package review

import (
	"strings"
	"testing"
)

func TestRepository_QueriesReferenceValidColumns(t *testing.T) {
	contractQuery := getContractReviewsQuery()
	if strings.Contains(contractQuery, "u1.first_name") || strings.Contains(contractQuery, "u2.first_name") {
		t.Fatalf("contract reviews query invalidly references u1.first_name on users table")
	}
	if strings.Contains(contractQuery, "u1.last_name") || strings.Contains(contractQuery, "u2.last_name") {
		t.Fatalf("contract reviews query invalidly references u1.last_name on users table")
	}
	if !strings.Contains(contractQuery, "LEFT JOIN profiles p1 ON p1.user_id = u1.id") {
		t.Fatalf("contract reviews query missing LEFT JOIN profiles p1")
	}
	if !strings.Contains(contractQuery, "LEFT JOIN profiles p2 ON p2.user_id = u2.id") {
		t.Fatalf("contract reviews query missing LEFT JOIN profiles p2")
	}
	if !strings.Contains(contractQuery, "COALESCE(p1.first_name, '')") || !strings.Contains(contractQuery, "COALESCE(p1.last_name, '')") {
		t.Fatalf("contract reviews query missing COALESCE for reviewer profile names")
	}
	if !strings.Contains(contractQuery, "COALESCE(p2.first_name, '')") || !strings.Contains(contractQuery, "COALESCE(p2.last_name, '')") {
		t.Fatalf("contract reviews query missing COALESCE for reviewee profile names")
	}

	userQuery := getUserReviewsQuery()
	if strings.Contains(userQuery, "u.first_name") {
		t.Fatalf("user reviews query invalidly references u.first_name on users table")
	}
	if strings.Contains(userQuery, "u.last_name") {
		t.Fatalf("user reviews query invalidly references u.last_name on users table")
	}
	if !strings.Contains(userQuery, "LEFT JOIN profiles p ON p.user_id = u.id") {
		t.Fatalf("user reviews query missing LEFT JOIN profiles p")
	}
	if !strings.Contains(userQuery, "COALESCE(p.first_name, '')") || !strings.Contains(userQuery, "COALESCE(p.last_name, '')") {
		t.Fatalf("user reviews query missing COALESCE for reviewer profile names")
	}
}
