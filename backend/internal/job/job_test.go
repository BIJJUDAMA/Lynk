package job_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/job"
	"github.com/lynk/backend/internal/middleware"
)

// mockJobRepository implements job.JobRepository in-memory for unit and HTTP tests.
type mockJobRepository struct {
	mu   sync.RWMutex
	jobs map[uuid.UUID]*job.Job

	errCreateJob error
	errGetJob    error
	errUpdateJob error
	errDeleteJob error
	errListJobs  error
}

func newMockJobRepository() *mockJobRepository {
	return &mockJobRepository{
		jobs: make(map[uuid.UUID]*job.Job),
	}
}

var _ job.JobRepository = (*mockJobRepository)(nil)

func (m *mockJobRepository) CreateJob(ctx context.Context, j *job.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errCreateJob != nil {
		return m.errCreateJob
	}
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	now := time.Now().UTC()
	j.CreatedAt = now
	j.UpdatedAt = now
	if j.RequiredSkills == nil {
		j.RequiredSkills = []string{}
	}
	if j.Status == "" {
		j.Status = job.StatusOpen
	}

	copied := *j
	copied.RequiredSkills = append([]string(nil), j.RequiredSkills...)
	m.jobs[j.ID] = &copied
	return nil
}

func (m *mockJobRepository) GetJobByID(ctx context.Context, id uuid.UUID) (*job.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.errGetJob != nil {
		return nil, m.errGetJob
	}
	j, ok := m.jobs[id]
	if !ok {
		return nil, nil
	}
	copied := *j
	copied.RequiredSkills = append([]string(nil), j.RequiredSkills...)
	return &copied, nil
}

func (m *mockJobRepository) UpdateJob(ctx context.Context, j *job.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errUpdateJob != nil {
		return m.errUpdateJob
	}
	if _, ok := m.jobs[j.ID]; !ok {
		return errors.New("job not found in mock store")
	}
	j.UpdatedAt = time.Now().UTC()
	copied := *j
	copied.RequiredSkills = append([]string(nil), j.RequiredSkills...)
	m.jobs[j.ID] = &copied
	return nil
}

func (m *mockJobRepository) DeleteJob(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errDeleteJob != nil {
		return m.errDeleteJob
	}
	if j, ok := m.jobs[id]; ok {
		j.Status = job.StatusCancelled
		j.UpdatedAt = time.Now().UTC()
	}
	return nil
}

func (m *mockJobRepository) ListJobs(ctx context.Context, filter job.JobFilter) ([]*job.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.errListJobs != nil {
		return nil, m.errListJobs
	}

	result := make([]*job.Job, 0)
	for _, j := range m.jobs {
		// Filter Search (title or description)
		if filter.Search != "" {
			term := strings.ToLower(filter.Search)
			if !strings.Contains(strings.ToLower(j.Title), term) &&
				!strings.Contains(strings.ToLower(j.Description), term) {
				continue
			}
		}

		// Filter Department
		if filter.Department != "" {
			if !strings.EqualFold(j.Department, filter.Department) {
				continue
			}
		}

		// Filter Skill
		if filter.Skill != "" {
			skillFound := false
			for _, s := range j.RequiredSkills {
				if strings.EqualFold(s, filter.Skill) {
					skillFound = true
					break
				}
			}
			if !skillFound {
				continue
			}
		}

		// Filter Skills
		if len(filter.Skills) > 0 {
			allSkillsFound := true
			for _, required := range filter.Skills {
				found := false
				for _, s := range j.RequiredSkills {
					if strings.EqualFold(s, required) {
						found = true
						break
					}
				}
				if !found {
					allSkillsFound = false
					break
				}
			}
			if !allSkillsFound {
				continue
			}
		}

		// Filter MinBudget
		if filter.MinBudget != nil && j.Budget < *filter.MinBudget {
			continue
		}

		// Filter MaxBudget
		if filter.MaxBudget != nil && j.Budget > *filter.MaxBudget {
			continue
		}

		// Filter PayType
		if filter.PayType != "" && !strings.EqualFold(j.PayType, filter.PayType) {
			continue
		}

		// Filter Status
		if filter.Status != "" && !strings.EqualFold(j.Status, filter.Status) {
			continue
		}

		// Filter EmployerID
		if filter.EmployerID != nil && j.EmployerID != *filter.EmployerID {
			continue
		}

		copied := *j
		copied.RequiredSkills = append([]string(nil), j.RequiredSkills...)
		result = append(result, &copied)
	}

	// Pagination
	if filter.Offset > 0 {
		if filter.Offset >= len(result) {
			return make([]*job.Job, 0), nil
		}
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result, nil
}

// mockValidator implements auth.TokenValidator for middleware tests
type mockValidator struct {
	claims *auth.UserClaims
	err    error
}

func (m *mockValidator) ValidateToken(ctx context.Context, tokenStr string) (*auth.UserClaims, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.claims, nil
}

func parseJSONResponse(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var res map[string]interface{}
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("failed to parse JSON response: %v, raw body: %s", err, string(body))
	}
	return res
}

func setupTestRouter(repo *mockJobRepository) (http.Handler, *job.Service, *job.Handler) {
	svc := job.NewService(repo)
	h := job.NewHandler(svc, repo)
	// We return router without authMiddleware wrapper, but handlers themselves check context
	return h.Routes(nil), svc, h
}

func TestJob_CreateJobValidation(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	ctx := context.Background()

	employerID := uuid.New()

	tests := []struct {
		name    string
		req     job.CreateJobRequest
		wantErr bool
		errCode string
	}{
		{
			name: "valid fixed pay job",
			req: job.CreateJobRequest{
				Title:          "Frontend Developer",
				Description:    "Build UI with Next.js",
				Budget:         500.00,
				PayType:        job.PayTypeFixed,
				RequiredSkills: []string{"React", "TypeScript"},
				Department:     "Computer Science",
			},
			wantErr: false,
		},
		{
			name: "valid hourly pay job",
			req: job.CreateJobRequest{
				Title:          "Lab Assistant",
				Description:    "Assist with chemistry lab setup",
				Budget:         20.00,
				PayType:        job.PayTypeHourly,
				RequiredSkills: []string{"Chemistry"},
				Department:     "Chemistry",
			},
			wantErr: false,
		},
		{
			name: "zero budget is allowed",
			req: job.CreateJobRequest{
				Title:          "Volunteer Tutor",
				Description:    "Help campus peers in math",
				Budget:         0,
				PayType:        job.PayTypeFixed,
				RequiredSkills: []string{"Math"},
				Department:     "Mathematics",
			},
			wantErr: false,
		},
		{
			name: "empty title rejected",
			req: job.CreateJobRequest{
				Title:       "   ",
				Description: "Some desc",
				Budget:      100,
				PayType:     job.PayTypeFixed,
			},
			wantErr: true,
		},
		{
			name: "title exceeding 200 chars rejected",
			req: job.CreateJobRequest{
				Title:       strings.Repeat("a", 201),
				Description: "Some desc",
				Budget:      100,
				PayType:     job.PayTypeFixed,
			},
			wantErr: true,
		},
		{
			name: "empty description rejected",
			req: job.CreateJobRequest{
				Title:       "Valid Title",
				Description: "   ",
				Budget:      100,
				PayType:     job.PayTypeFixed,
			},
			wantErr: true,
		},
		{
			name: "negative budget rejected",
			req: job.CreateJobRequest{
				Title:       "Valid Title",
				Description: "Valid desc",
				Budget:      -50.00,
				PayType:     job.PayTypeFixed,
			},
			wantErr: true,
		},
		{
			name: "invalid pay_type rejected",
			req: job.CreateJobRequest{
				Title:       "Valid Title",
				Description: "Valid desc",
				Budget:      100,
				PayType:     "monthly",
			},
			wantErr: true,
		},
		{
			name: "department exceeding 100 chars rejected",
			req: job.CreateJobRequest{
				Title:       "Valid Title",
				Description: "Valid desc",
				Budget:      100,
				PayType:     job.PayTypeFixed,
				Department:  strings.Repeat("d", 101),
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			created, err := svc.CreateJob(ctx, employerID, tc.req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, job.ErrInvalidInput) {
					t.Errorf("expected ErrInvalidInput, got: %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if created == nil {
					t.Fatalf("expected non-nil job")
				}
				if created.EmployerID != employerID {
					t.Errorf("expected employerID %v, got %v", employerID, created.EmployerID)
				}
				if created.Status != job.StatusOpen {
					t.Errorf("expected default status open, got %s", created.Status)
				}
				if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
					t.Errorf("expected timestamps to be populated")
				}
			}
		})
	}
}

func TestJob_JobDateJSON(t *testing.T) {
	// Tests date unmarshaling for both YYYY-MM-DD and RFC3339
	isoJSON := `{"title": "Test", "deadline": "2026-11-15"}`
	var reqISO job.CreateJobRequest
	if err := json.Unmarshal([]byte(isoJSON), &reqISO); err != nil {
		t.Fatalf("failed to unmarshal ISO date: %v", err)
	}
	if reqISO.Deadline == nil {
		t.Fatalf("expected non-nil deadline")
	}
	tm := reqISO.Deadline.Time()
	if tm.Year() != 2026 || tm.Month() != 11 || tm.Day() != 15 {
		t.Errorf("unexpected date parsed: %v", tm)
	}

	rfcJSON := `{"title": "Test", "deadline": "2026-12-31T23:59:59Z"}`
	var reqRFC job.CreateJobRequest
	if err := json.Unmarshal([]byte(rfcJSON), &reqRFC); err != nil {
		t.Fatalf("failed to unmarshal RFC3339 date: %v", err)
	}
	if reqRFC.Deadline == nil {
		t.Fatalf("expected non-nil deadline")
	}
	tmRFC := reqRFC.Deadline.Time()
	if tmRFC.Year() != 2026 || tmRFC.Month() != 12 || tmRFC.Day() != 31 {
		t.Errorf("unexpected RFC date parsed: %v", tmRFC)
	}
}

func TestJob_JobDateZero(t *testing.T) {
	var jd job.JobDate
	if jd.Time() != nil {
		t.Errorf("expected nil for zero JobDate, got %v", jd.Time())
	}
}

func TestJob_HTTP_CreateJob(t *testing.T) {
	repo := newMockJobRepository()
	router, _, _ := setupTestRouter(repo)

	employerID := uuid.New()
	studentID := uuid.New()

	t.Run("success as employer", func(t *testing.T) {
		body := map[string]interface{}{
			"title":           "Research Assistant in ML",
			"description":     "Assist with training PyTorch models",
			"budget":          1200.50,
			"pay_type":        "fixed",
			"required_skills": []string{"Python", "PyTorch", "Git"},
			"department":      "Computer Science",
			"deadline":        "2026-10-31",
		}
		rawBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/api/v1/jobs", bytes.NewReader(rawBody))
		req.Header.Set("Content-Type", "application/json")
		claims := &auth.UserClaims{
			UserID:        employerID.String(),
			Email:         "prof@univ.edu",
			EmailVerified: true,
			Roles:         []string{"employer"},
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}

		res := parseJSONResponse(t, rec.Body.Bytes())
		if res["success"] != true {
			t.Errorf("expected success true, got %v", res["success"])
		}
		if res["error"] != nil {
			t.Errorf("expected nil error, got %v", res["error"])
		}

		data := res["data"].(map[string]interface{})
		if data["title"] != "Research Assistant in ML" {
			t.Errorf("unexpected title: %v", data["title"])
		}
		if data["employer_id"] != employerID.String() {
			t.Errorf("unexpected employer_id: %v", data["employer_id"])
		}
		if data["status"] != "open" {
			t.Errorf("expected status open, got %v", data["status"])
		}
	})

	t.Run("forbidden for student", func(t *testing.T) {
		body := map[string]interface{}{
			"title":       "Illegal Job",
			"description": "Student cannot post",
			"budget":      100,
			"pay_type":    "fixed",
		}
		rawBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/api/v1/jobs", bytes.NewReader(rawBody))
		claims := &auth.UserClaims{
			UserID:        studentID.String(),
			Email:         "student@univ.edu",
			EmailVerified: true,
			Roles:         []string{"student"},
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d: %s", rec.Code, rec.Body.String())
		}
		res := parseJSONResponse(t, rec.Body.Bytes())
		if res["success"] != false {
			t.Errorf("expected success false")
		}
		errObj := res["error"].(map[string]interface{})
		if errObj["code"] != "FORBIDDEN" {
			t.Errorf("expected code FORBIDDEN, got %v", errObj["code"])
		}
	})

	t.Run("unauthorized when missing credentials", func(t *testing.T) {
		body := map[string]interface{}{
			"title":       "No Auth Job",
			"description": "Missing claims",
			"budget":      100,
			"pay_type":    "fixed",
		}
		rawBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/api/v1/jobs", bytes.NewReader(rawBody))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
		res := parseJSONResponse(t, rec.Body.Bytes())
		errObj := res["error"].(map[string]interface{})
		if errObj["code"] != "UNAUTHORIZED" {
			t.Errorf("expected code UNAUTHORIZED, got %v", errObj["code"])
		}
	})

	t.Run("bad request on invalid validation", func(t *testing.T) {
		body := map[string]interface{}{
			"title":       "",
			"description": "Desc",
			"budget":      -10,
			"pay_type":    "random",
		}
		rawBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/api/v1/jobs", bytes.NewReader(rawBody))
		claims := &auth.UserClaims{
			UserID: employerID.String(),
			Roles:  []string{"employer"},
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
		res := parseJSONResponse(t, rec.Body.Bytes())
		errObj := res["error"].(map[string]interface{})
		if errObj["code"] != "INVALID_INPUT" {
			t.Errorf("expected code INVALID_INPUT, got %v", errObj["code"])
		}
	})

	t.Run("bad request on malformed json", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/jobs", strings.NewReader("not-json"))
		claims := &auth.UserClaims{
			UserID: employerID.String(),
			Roles:  []string{"employer"},
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
		res := parseJSONResponse(t, rec.Body.Bytes())
		errObj := res["error"].(map[string]interface{})
		if errObj["code"] != "BAD_REQUEST" {
			t.Errorf("expected code BAD_REQUEST, got %v", errObj["code"])
		}
	})
}

func TestJob_HTTP_GetJobByID(t *testing.T) {
	repo := newMockJobRepository()
	router, _, _ := setupTestRouter(repo)

	employerID := uuid.New()
	j := &job.Job{
		ID:             uuid.New(),
		EmployerID:     employerID,
		Title:          "Robotics Software Engineer",
		Description:    "Work on ROS2 navigation stack",
		Budget:         2500,
		PayType:        job.PayTypeFixed,
		RequiredSkills: []string{"C++", "ROS2", "Python"},
		Department:     "Robotics",
		Status:         job.StatusOpen,
		Employer: &job.EmployerInfo{
			ID:           employerID,
			Email:        "robotics-lab@univ.edu",
			CompanyOrOrg: "Autonomous Systems Lab",
			ContactName:  "Dr. Alan Turing",
			Website:      "https://robotics.univ.edu",
		},
	}
	_ = repo.CreateJob(context.Background(), j)

	t.Run("public access without auth header succeeds", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/jobs/%s", j.ID.String()), nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		res := parseJSONResponse(t, rec.Body.Bytes())
		if res["success"] != true {
			t.Errorf("expected success true")
		}
		data := res["data"].(map[string]interface{})
		if data["id"] != j.ID.String() {
			t.Errorf("expected id %s, got %v", j.ID, data["id"])
		}
		if data["title"] != "Robotics Software Engineer" {
			t.Errorf("unexpected title: %v", data["title"])
		}
	})

	t.Run("not found for non-existent id", func(t *testing.T) {
		randomID := uuid.New()
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/jobs/%s", randomID.String()), nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
		res := parseJSONResponse(t, rec.Body.Bytes())
		errObj := res["error"].(map[string]interface{})
		if errObj["code"] != "JOB_NOT_FOUND" {
			t.Errorf("expected code JOB_NOT_FOUND, got %v", errObj["code"])
		}
	})

	t.Run("bad request for invalid uuid", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/jobs/not-a-uuid", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
		res := parseJSONResponse(t, rec.Body.Bytes())
		errObj := res["error"].(map[string]interface{})
		if errObj["code"] != "INVALID_ID" {
			t.Errorf("expected code INVALID_ID, got %v", errObj["code"])
		}
	})
}

func TestJob_HTTP_ListJobsSearchAndFilter(t *testing.T) {
	repo := newMockJobRepository()
	router, _, _ := setupTestRouter(repo)
	ctx := context.Background()

	emp1 := uuid.New()
	emp2 := uuid.New()

	jobsData := []*job.Job{
		{
			ID:             uuid.New(),
			EmployerID:     emp1,
			Title:          "Go Backend Developer",
			Description:    "Build high concurrency microservices with Golang and Postgres",
			Budget:         800,
			PayType:        job.PayTypeFixed,
			RequiredSkills: []string{"Go", "PostgreSQL", "Docker"},
			Department:     "Computer Science",
			Status:         job.StatusOpen,
		},
		{
			ID:             uuid.New(),
			EmployerID:     emp1,
			Title:          "React Native Mobile App",
			Description:    "Develop cross-platform campus navigation mobile application",
			Budget:         1500,
			PayType:        job.PayTypeFixed,
			RequiredSkills: []string{"React Native", "TypeScript"},
			Department:     "Computer Science",
			Status:         job.StatusOpen,
		},
		{
			ID:             uuid.New(),
			EmployerID:     emp2,
			Title:          "Chemistry Data Analyst",
			Description:    "Analyze spectroscopy experiment data using Python and Pandas",
			Budget:         25,
			PayType:        job.PayTypeHourly,
			RequiredSkills: []string{"Python", "Pandas", "Statistics"},
			Department:     "Chemistry",
			Status:         job.StatusOpen,
		},
		{
			ID:             uuid.New(),
			EmployerID:     emp2,
			Title:          "Closed Lab Project",
			Description:    "Legacy research job already completed",
			Budget:         300,
			PayType:        job.PayTypeFixed,
			RequiredSkills: []string{"Python"},
			Department:     "Chemistry",
			Status:         job.StatusClosed,
		},
	}

	for _, j := range jobsData {
		if err := repo.CreateJob(ctx, j); err != nil {
			t.Fatalf("failed to seed job: %v", err)
		}
	}

	tests := []struct {
		name          string
		queryString   string
		expectedCount int
		checkTitles   []string
	}{
		{
			name:          "list all jobs without filter",
			queryString:   "",
			expectedCount: 4,
		},
		{
			name:          "filter by search text in title",
			queryString:   "?search=backend",
			expectedCount: 1,
			checkTitles:   []string{"Go Backend Developer"},
		},
		{
			name:          "filter by search text in description",
			queryString:   "?search=spectroscopy",
			expectedCount: 1,
			checkTitles:   []string{"Chemistry Data Analyst"},
		},
		{
			name:          "filter by department",
			queryString:   "?department=Chemistry",
			expectedCount: 2,
		},
		{
			name:          "filter by skill Go",
			queryString:   "?skill=Go",
			expectedCount: 1,
			checkTitles:   []string{"Go Backend Developer"},
		},
		{
			name:          "filter by skill Python case-insensitive",
			queryString:   "?skill=python",
			expectedCount: 2,
		},
		{
			name:          "filter by min_budget",
			queryString:   "?min_budget=800",
			expectedCount: 2, // 800 and 1500
		},
		{
			name:          "filter by max_budget",
			queryString:   "?max_budget=100",
			expectedCount: 1, // 25
			checkTitles:   []string{"Chemistry Data Analyst"},
		},
		{
			name:          "filter by budget range",
			queryString:   "?min_budget=500&max_budget=1000",
			expectedCount: 1, // 800
			checkTitles:   []string{"Go Backend Developer"},
		},
		{
			name:          "filter by pay_type hourly",
			queryString:   "?pay_type=hourly",
			expectedCount: 1,
			checkTitles:   []string{"Chemistry Data Analyst"},
		},
		{
			name:          "filter by status closed",
			queryString:   "?status=closed",
			expectedCount: 1,
			checkTitles:   []string{"Closed Lab Project"},
		},
		{
			name:          "combined filters: department + skill + status",
			queryString:   "?department=Chemistry&skill=Python&status=open",
			expectedCount: 1,
			checkTitles:   []string{"Chemistry Data Analyst"},
		},
		{
			name:          "empty results return non-nil empty array",
			queryString:   "?department=Astronomy",
			expectedCount: 0,
		},
		{
			name:          "limit capped at 100",
			queryString:   "?limit=200",
			expectedCount: 4,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/jobs"+tc.queryString, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
			}

			res := parseJSONResponse(t, rec.Body.Bytes())
			if res["success"] != true {
				t.Errorf("expected success true")
			}

			dataArr := res["data"].([]interface{})
			if len(dataArr) != tc.expectedCount {
				t.Fatalf("expected %d jobs, got %d", tc.expectedCount, len(dataArr))
			}

			if len(tc.checkTitles) > 0 {
				foundTitles := make(map[string]bool)
				for _, item := range dataArr {
					m := item.(map[string]interface{})
					foundTitles[m["title"].(string)] = true
				}
				for _, expTitle := range tc.checkTitles {
					if !foundTitles[expTitle] {
						t.Errorf("expected to find job title %q in results, got %v", expTitle, foundTitles)
					}
				}
			}
		})
	}
}

func TestJob_HTTP_GetMyJobs(t *testing.T) {
	repo := newMockJobRepository()
	router, _, _ := setupTestRouter(repo)
	ctx := context.Background()

	emp1 := uuid.New()
	emp2 := uuid.New()

	j1 := &job.Job{
		ID:          uuid.New(),
		EmployerID:  emp1,
		Title:       "Employer 1 Job A",
		Description: "Desc",
		Budget:      100,
		PayType:     job.PayTypeFixed,
		Status:      job.StatusOpen,
	}
	j2 := &job.Job{
		ID:          uuid.New(),
		EmployerID:  emp1,
		Title:       "Employer 1 Job B",
		Description: "Desc",
		Budget:      200,
		PayType:     job.PayTypeFixed,
		Status:      job.StatusOpen,
	}
	j3 := &job.Job{
		ID:          uuid.New(),
		EmployerID:  emp2,
		Title:       "Employer 2 Job",
		Description: "Desc",
		Budget:      300,
		PayType:     job.PayTypeFixed,
		Status:      job.StatusOpen,
	}

	_ = repo.CreateJob(ctx, j1)
	_ = repo.CreateJob(ctx, j2)
	_ = repo.CreateJob(ctx, j3)

	t.Run("employer 1 sees only their jobs", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/jobs/mine", nil)
		claims := &auth.UserClaims{
			UserID: emp1.String(),
			Roles:  []string{"employer"},
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		res := parseJSONResponse(t, rec.Body.Bytes())
		dataArr := res["data"].([]interface{})
		if len(dataArr) != 2 {
			t.Fatalf("expected 2 jobs for emp1, got %d", len(dataArr))
		}
	})

	t.Run("/mine is not masked by /{id}", func(t *testing.T) {
		// Ensure that request to /mine does not trigger GetJobByID which would return INVALID_ID
		req := httptest.NewRequest("GET", "/api/v1/jobs/mine", nil)
		claims := &auth.UserClaims{
			UserID: emp2.String(),
			Roles:  []string{"employer"},
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		res := parseJSONResponse(t, rec.Body.Bytes())
		dataArr := res["data"].([]interface{})
		if len(dataArr) != 1 {
			t.Fatalf("expected 1 job for emp2, got %d", len(dataArr))
		}
	})

	t.Run("forbidden for student", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/jobs/mine", nil)
		claims := &auth.UserClaims{
			UserID: uuid.New().String(),
			Roles:  []string{"student"},
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}
	})
}

func TestJob_HTTP_UpdateJob(t *testing.T) {
	repo := newMockJobRepository()
	router, _, _ := setupTestRouter(repo)
	ctx := context.Background()

	ownerID := uuid.New()
	otherEmployerID := uuid.New()

	initialJob := &job.Job{
		ID:             uuid.New(),
		EmployerID:     ownerID,
		Title:          "Initial Title",
		Description:    "Initial Description",
		Budget:         500,
		PayType:        job.PayTypeFixed,
		RequiredSkills: []string{"Go"},
		Department:     "CS",
		Status:         job.StatusOpen,
	}
	_ = repo.CreateJob(ctx, initialJob)

	t.Run("owner updates job successfully", func(t *testing.T) {
		newTitle := "Updated Title"
		newBudget := 750.0
		newStatus := job.StatusInProgress
		body := map[string]interface{}{
			"title":    newTitle,
			"budget":   newBudget,
			"status":   newStatus,
			"pay_type": "hourly",
		}
		rawBody, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/jobs/%s", initialJob.ID.String()), bytes.NewReader(rawBody))
		claims := &auth.UserClaims{
			UserID: ownerID.String(),
			Roles:  []string{"employer"},
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		res := parseJSONResponse(t, rec.Body.Bytes())
		data := res["data"].(map[string]interface{})
		if data["title"] != newTitle {
			t.Errorf("expected title %s, got %v", newTitle, data["title"])
		}
		if data["budget"] != newBudget {
			t.Errorf("expected budget %v, got %v", newBudget, data["budget"])
		}
		if data["status"] != newStatus {
			t.Errorf("expected status %s, got %v", newStatus, data["status"])
		}
		if data["pay_type"] != "hourly" {
			t.Errorf("expected pay_type hourly, got %v", data["pay_type"])
		}
	})

	t.Run("non-owner employer rejected with 403 forbidden", func(t *testing.T) {
		body := map[string]interface{}{
			"title": "Hacked Title",
		}
		rawBody, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/jobs/%s", initialJob.ID.String()), bytes.NewReader(rawBody))
		claims := &auth.UserClaims{
			UserID: otherEmployerID.String(),
			Roles:  []string{"employer"},
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d: %s", rec.Code, rec.Body.String())
		}
		res := parseJSONResponse(t, rec.Body.Bytes())
		errObj := res["error"].(map[string]interface{})
		if errObj["code"] != "FORBIDDEN" {
			t.Errorf("expected code FORBIDDEN, got %v", errObj["code"])
		}
	})

	t.Run("non-existent job returns 404", func(t *testing.T) {
		body := map[string]interface{}{"title": "New Title"}
		rawBody, _ := json.Marshal(body)

		randomID := uuid.New()
		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/jobs/%s", randomID.String()), bytes.NewReader(rawBody))
		claims := &auth.UserClaims{
			UserID: ownerID.String(),
			Roles:  []string{"employer"},
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})

	t.Run("invalid status update returns 400", func(t *testing.T) {
		body := map[string]interface{}{
			"status": "destroyed",
		}
		rawBody, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/jobs/%s", initialJob.ID.String()), bytes.NewReader(rawBody))
		claims := &auth.UserClaims{
			UserID: ownerID.String(),
			Roles:  []string{"employer"},
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})
}

func TestJob_HTTP_DeleteJob(t *testing.T) {
	repo := newMockJobRepository()
	router, _, _ := setupTestRouter(repo)
	ctx := context.Background()

	ownerID := uuid.New()
	otherEmployerID := uuid.New()

	targetJob := &job.Job{
		ID:          uuid.New(),
		EmployerID:  ownerID,
		Title:       "Job To Delete",
		Description: "Will be deleted",
		Budget:      300,
		PayType:     job.PayTypeFixed,
		Status:      job.StatusOpen,
	}
	_ = repo.CreateJob(ctx, targetJob)

	t.Run("non-owner employer cannot delete job", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/jobs/%s", targetJob.ID.String()), nil)
		claims := &auth.UserClaims{
			UserID: otherEmployerID.String(),
			Roles:  []string{"employer"},
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("owner employer can delete job", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/jobs/%s", targetJob.ID.String()), nil)
		claims := &auth.UserClaims{
			UserID: ownerID.String(),
			Roles:  []string{"employer"},
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		res := parseJSONResponse(t, rec.Body.Bytes())
		if res["success"] != true {
			t.Errorf("expected success true")
		}

		// Verify job is soft-cancelled
		getReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/jobs/%s", targetJob.ID.String()), nil)
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)

		if getRec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK after delete/cancel, got %d", getRec.Code)
		}
		getRes := parseJSONResponse(t, getRec.Body.Bytes())
		getData := getRes["data"].(map[string]interface{})
		if getData["status"] != job.StatusCancelled {
			t.Fatalf("expected status 'cancelled', got %v", getData["status"])
		}
	})

	t.Run("deleting non-existent job returns 404", func(t *testing.T) {
		randomID := uuid.New()
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/jobs/%s", randomID.String()), nil)
		claims := &auth.UserClaims{
			UserID: ownerID.String(),
			Roles:  []string{"employer"},
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})
}

func TestJob_WithAuthMiddleware(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	h := job.NewHandler(svc, repo)

	employerID := uuid.New()
	validator := &mockValidator{
		claims: &auth.UserClaims{
			UserID:        employerID.String(),
			Email:         "emp@univ.edu",
			EmailVerified: true,
			Roles:         []string{"employer"},
		},
	}
	authMw := middleware.AuthMiddleware(validator)
	router := h.Routes(authMw)

	t.Run("public GET /jobs works without token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/jobs", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK without auth, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("protected POST /jobs works with bearer token", func(t *testing.T) {
		body := map[string]interface{}{
			"title":       "Protected Job Creation",
			"description": "Created with valid Bearer token",
			"budget":      400,
			"pay_type":    "fixed",
		}
		rawBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/api/v1/jobs", bytes.NewReader(rawBody))
		req.Header.Set("Authorization", "Bearer valid-token")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("protected POST /jobs fails without token", func(t *testing.T) {
		body := map[string]interface{}{
			"title":       "Unauthenticated Job Creation",
			"description": "Should fail",
			"budget":      400,
			"pay_type":    "fixed",
		}
		rawBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/api/v1/jobs", bytes.NewReader(rawBody))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})
}
