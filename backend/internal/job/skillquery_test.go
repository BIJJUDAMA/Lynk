package job

import (
	"context"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestParseSkillQuery_CSVAndRepeated(t *testing.T) {
	single, many := ParseSkillQuery([]string{"React, Python"})
	if single != "" || len(many) != 2 {
		t.Fatalf("csv: single=%q many=%v", single, many)
	}
	single, many = ParseSkillQuery([]string{"Go"})
	if single != "Go" || len(many) != 0 {
		t.Fatalf("one: single=%q many=%v", single, many)
	}
	single, many = ParseSkillQuery([]string{"Go, ", " Rust ", "TypeScript,Python"})
	if single != "" || len(many) != 4 {
		t.Fatalf("mixed: single=%q many=%v", single, many)
	}
	single, many = ParseSkillQuery([]string{"", "  ", ","})
	if single != "" || len(many) != 0 {
		t.Fatalf("empty: single=%q many=%v", single, many)
	}
}

type mockFilterCaptureRepo struct {
	JobRepository
	lastFilter JobFilter
}

func (m *mockFilterCaptureRepo) ListJobs(ctx context.Context, filter JobFilter) ([]*Job, error) {
	m.lastFilter = filter
	return []*Job{}, nil
}

func TestListJobs_SkillQueryIntegration(t *testing.T) {
	repo := &mockFilterCaptureRepo{}
	svc := NewService(repo)
	h := NewHandler(svc, repo)

	t.Run("single skill", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/jobs?skill=Go", nil)
		rec := httptest.NewRecorder()
		h.ListJobs(rec, req)

		if repo.lastFilter.Skill != "Go" {
			t.Errorf("expected Skill='Go', got %q", repo.lastFilter.Skill)
		}
		if len(repo.lastFilter.Skills) != 0 {
			t.Errorf("expected Skills=nil, got %v", repo.lastFilter.Skills)
		}
	})

	t.Run("csv skills", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/jobs?skill=React,Python", nil)
		rec := httptest.NewRecorder()
		h.ListJobs(rec, req)

		if repo.lastFilter.Skill != "" {
			t.Errorf("expected Skill='', got %q", repo.lastFilter.Skill)
		}
		expected := []string{"React", "Python"}
		if !reflect.DeepEqual(repo.lastFilter.Skills, expected) {
			t.Errorf("expected Skills=%v, got %v", expected, repo.lastFilter.Skills)
		}
	})

	t.Run("repeated skills", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/jobs?skill=Go&skill=Rust", nil)
		rec := httptest.NewRecorder()
		h.ListJobs(rec, req)

		if repo.lastFilter.Skill != "" {
			t.Errorf("expected Skill='', got %q", repo.lastFilter.Skill)
		}
		expected := []string{"Go", "Rust"}
		if !reflect.DeepEqual(repo.lastFilter.Skills, expected) {
			t.Errorf("expected Skills=%v, got %v", expected, repo.lastFilter.Skills)
		}
	})
}
