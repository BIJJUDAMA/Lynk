package recruitment_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httpx"
	"github.com/lynk/backend/internal/project"
	"github.com/lynk/backend/internal/recruitment"
	"github.com/lynk/backend/internal/team"
)

// mockRecruitmentRepo provides an in-memory repository for recruitment tests.
type mockRecruitmentRepo struct {
	mu           sync.RWMutex
	applications map[uuid.UUID]*recruitment.Application
	questions    map[uuid.UUID]*recruitment.ApplicationQuestion
	answers      map[uuid.UUID][]recruitment.ApplicationAnswer
	members      map[string]*team.ProjectMember
}

func newMockRecruitmentRepo() *mockRecruitmentRepo {
	return &mockRecruitmentRepo{
		applications: make(map[uuid.UUID]*recruitment.Application),
		questions:    make(map[uuid.UUID]*recruitment.ApplicationQuestion),
		answers:      make(map[uuid.UUID][]recruitment.ApplicationAnswer),
		members:      make(map[string]*team.ProjectMember),
	}
}

func (m *mockRecruitmentRepo) memberKey(projectID uuid.UUID, memberID string) string {
	return fmt.Sprintf("%s:%s", projectID.String(), memberID)
}

func (m *mockRecruitmentRepo) CreateApplication(ctx context.Context, app *recruitment.Application, answers []recruitment.ApplicationAnswer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.applications {
		if existing.ProjectID == app.ProjectID && existing.ApplicantID == app.ApplicantID {
			return recruitment.ErrAlreadyApplied
		}
	}

	appCopy := *app
	m.applications[app.ID] = &appCopy
	m.answers[app.ID] = append([]recruitment.ApplicationAnswer{}, answers...)
	return nil
}

func (m *mockRecruitmentRepo) GetApplicationByID(ctx context.Context, id uuid.UUID) (*recruitment.Application, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	app, ok := m.applications[id]
	if !ok {
		return nil, recruitment.ErrApplicationNotFound
	}
	appCopy := *app
	appCopy.Answers = append([]recruitment.ApplicationAnswer{}, m.answers[id]...)
	return &appCopy, nil
}

func (m *mockRecruitmentRepo) ListApplications(ctx context.Context, projectID uuid.UUID, statusFilter *recruitment.ApplicationStatus) ([]recruitment.Application, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make([]recruitment.Application, 0)
	for _, app := range m.applications {
		if app.ProjectID == projectID {
			if statusFilter == nil || *statusFilter == "" || app.Status == *statusFilter {
				appCopy := *app
				appCopy.Answers = append([]recruitment.ApplicationAnswer{}, m.answers[app.ID]...)
				res = append(res, appCopy)
			}
		}
	}
	return res, nil
}

func (m *mockRecruitmentRepo) ListMyApplications(ctx context.Context, applicantID string) ([]recruitment.Application, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make([]recruitment.Application, 0)
	for _, app := range m.applications {
		if app.ApplicantID == applicantID {
			appCopy := *app
			appCopy.Answers = append([]recruitment.ApplicationAnswer{}, m.answers[app.ID]...)
			res = append(res, appCopy)
		}
	}
	return res, nil
}

func (m *mockRecruitmentRepo) UpdateApplicationStatus(ctx context.Context, id uuid.UUID, status recruitment.ApplicationStatus, reviewerNotes string, decidedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.applications[id]
	if !ok {
		return recruitment.ErrApplicationNotFound
	}
	if app.Status == recruitment.StatusAccepted || app.Status == recruitment.StatusRejected || app.Status == recruitment.StatusWithdrawn {
		return recruitment.ErrAlreadyDecided
	}

	app.Status = status
	if reviewerNotes != "" {
		app.ReviewerNotes = reviewerNotes
	}
	if status == recruitment.StatusRejected || status == recruitment.StatusAccepted {
		now := time.Now().UTC()
		app.DecidedAt = &now
		if decidedBy != "" {
			app.DecidedBy = &decidedBy
		}
	}
	app.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *mockRecruitmentRepo) WithdrawApplication(ctx context.Context, id uuid.UUID, applicantID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.applications[id]
	if !ok {
		return recruitment.ErrApplicationNotFound
	}
	if app.ApplicantID != applicantID {
		return recruitment.ErrForbidden
	}
	if app.Status != recruitment.StatusSubmitted && app.Status != recruitment.StatusShortlisted {
		return recruitment.ErrCannotWithdraw
	}
	app.Status = recruitment.StatusWithdrawn
	app.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *mockRecruitmentRepo) AcceptApplicationTx(ctx context.Context, appID uuid.UUID, reviewerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.applications[appID]
	if !ok {
		return recruitment.ErrApplicationNotFound
	}
	if app.Status != recruitment.StatusSubmitted && app.Status != recruitment.StatusShortlisted {
		return recruitment.ErrAlreadyDecided
	}

	now := time.Now().UTC()
	app.Status = recruitment.StatusAccepted
	app.DecidedBy = &reviewerID
	app.DecidedAt = &now
	app.UpdatedAt = now

	// Atomically add to project members (Role: member, Status: active)
	m.members[m.memberKey(app.ProjectID, app.ApplicantID)] = &team.ProjectMember{
		ID:        uuid.New(),
		ProjectID: app.ProjectID,
		MemberID:  app.ApplicantID,
		Role:      team.RoleMember,
		Status:    "active",
		JoinedAt:  now,
	}
	return nil
}

func (m *mockRecruitmentRepo) ListQuestions(ctx context.Context, projectID uuid.UUID) ([]recruitment.ApplicationQuestion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make([]recruitment.ApplicationQuestion, 0)
	for _, q := range m.questions {
		if q.ProjectID == projectID {
			res = append(res, *q)
		}
	}
	return res, nil
}

func (m *mockRecruitmentRepo) CreateQuestion(ctx context.Context, q *recruitment.ApplicationQuestion) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	qCopy := *q
	m.questions[q.ID] = &qCopy
	return nil
}

type mockProjectReader struct {
	mu       sync.RWMutex
	projects map[uuid.UUID]*project.Project
}

func newMockProjectReader() *mockProjectReader {
	return &mockProjectReader{
		projects: make(map[uuid.UUID]*project.Project),
	}
}

func (m *mockProjectReader) GetProjectByID(ctx context.Context, id uuid.UUID) (*project.Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.projects[id]
	if !ok {
		return nil, project.ErrProjectNotFound
	}
	copy := *p
	return &copy, nil
}

type mockMembershipReader struct {
	mu      sync.RWMutex
	members map[string]*team.ProjectMember
}

func newMockMembershipReader() *mockMembershipReader {
	return &mockMembershipReader{
		members: make(map[string]*team.ProjectMember),
	}
}

func (m *mockMembershipReader) memberKey(projectID uuid.UUID, memberID string) string {
	return fmt.Sprintf("%s:%s", projectID.String(), memberID)
}

func (m *mockMembershipReader) GetMember(ctx context.Context, projectID uuid.UUID, memberID string) (*team.ProjectMember, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mem, ok := m.members[m.memberKey(projectID, memberID)]
	if !ok {
		return nil, team.ErrMemberNotFound
	}
	copy := *mem
	return &copy, nil
}

func TestApplicationStatusValidation(t *testing.T) {
	cases := []struct {
		status recruitment.ApplicationStatus
		valid  bool
	}{
		{recruitment.StatusSubmitted, true},
		{recruitment.StatusShortlisted, true},
		{recruitment.StatusAccepted, true},
		{recruitment.StatusRejected, true},
		{recruitment.StatusWithdrawn, true},
		{recruitment.ApplicationStatus("pending"), false},
		{recruitment.ApplicationStatus("hired"), false},
		{recruitment.ApplicationStatus(""), false},
	}

	for _, tc := range cases {
		if got := tc.status.IsValid(); got != tc.valid {
			t.Errorf("status %q IsValid() = %v, expected %v", tc.status, got, tc.valid)
		}
	}
}

func TestRecruitmentService_InstitutionalEmailGate(t *testing.T) {
	ctx := context.Background()
	repo := newMockRecruitmentRepo()
	projReader := newMockProjectReader()
	memReader := newMockMembershipReader()
	svc := recruitment.NewService(repo, projReader, memReader)

	projectID := uuid.New()
	projReader.projects[projectID] = &project.Project{
		ID:                projectID,
		Title:             "Autonomous Rover",
		RecruitmentStatus: project.RecruitmentOpen,
		OwnerMemberID:     "lead-1",
	}

	// 1. Unverified campus email rejected
	unverifiedClaims := &auth.UserClaims{
		UserID:        "student-unverified",
		Email:         "unverified@stanford.edu",
		EmailVerified: false,
	}

	input := recruitment.SubmitApplicationInput{
		Statement: "I want to contribute to the telemetry system.",
	}

	_, err := svc.SubmitApplication(ctx, unverifiedClaims, projectID, input)
	if err == nil || err != recruitment.ErrEmailNotVerified {
		t.Fatalf("expected ErrEmailNotVerified for unverified student, got: %v", err)
	}

	// 2. Verified campus email accepted
	verifiedClaims := &auth.UserClaims{
		UserID:        "student-verified",
		Email:         "verified@stanford.edu",
		EmailVerified: true,
	}

	app, err := svc.SubmitApplication(ctx, verifiedClaims, projectID, input)
	if err != nil {
		t.Fatalf("expected successful application submission, got: %v", err)
	}
	if app.Status != recruitment.StatusSubmitted {
		t.Errorf("expected status 'submitted', got: %s", app.Status)
	}
	if app.ApplicantID != "student-verified" {
		t.Errorf("expected applicant ID 'student-verified', got: %s", app.ApplicantID)
	}
}

func TestRecruitmentService_RecruitmentStatusEnforcement(t *testing.T) {
	ctx := context.Background()
	repo := newMockRecruitmentRepo()
	projReader := newMockProjectReader()
	memReader := newMockMembershipReader()
	svc := recruitment.NewService(repo, projReader, memReader)

	closedProjectID := uuid.New()
	projReader.projects[closedProjectID] = &project.Project{
		ID:                closedProjectID,
		Title:             "Closed Team",
		RecruitmentStatus: project.RecruitmentClosed,
		OwnerMemberID:     "lead-1",
	}

	claims := &auth.UserClaims{
		UserID:        "applicant-1",
		Email:         "applicant@mit.edu",
		EmailVerified: true,
	}

	input := recruitment.SubmitApplicationInput{
		Statement: "I want to join this closed team.",
	}

	_, err := svc.SubmitApplication(ctx, claims, closedProjectID, input)
	if err == nil || err != recruitment.ErrRecruitmentClosed {
		t.Fatalf("expected ErrRecruitmentClosed when recruitment status is closed, got: %v", err)
	}
}

func TestRecruitmentService_QuestionManagementAndRequiredValidation(t *testing.T) {
	ctx := context.Background()
	repo := newMockRecruitmentRepo()
	projReader := newMockProjectReader()
	memReader := newMockMembershipReader()
	svc := recruitment.NewService(repo, projReader, memReader)

	projectID := uuid.New()
	leadID := "lead-dev"
	memReader.members[memReader.memberKey(projectID, leadID)] = &team.ProjectMember{
		ProjectID: projectID,
		MemberID:  leadID,
		Role:      team.RoleLead,
		Status:    "active",
	}
	projReader.projects[projectID] = &project.Project{
		ID:                projectID,
		Title:             "Quantum Computing Sim",
		RecruitmentStatus: project.RecruitmentOpen,
		OwnerMemberID:     leadID,
	}

	// 1. Unauthorized non-lead cannot create questions
	nonLeadClaims := &auth.UserClaims{
		UserID:        "random-user",
		Email:         "random@mit.edu",
		EmailVerified: true,
	}
	_, err := svc.CreateQuestion(ctx, nonLeadClaims, projectID, recruitment.CreateQuestionInput{
		QuestionText: "How many hours per week can you commit?",
		Required:     true,
	})
	if err != recruitment.ErrForbidden {
		t.Fatalf("expected ErrForbidden for non-lead creating questions, got: %v", err)
	}

	// 2. Lead creates required question
	leadClaims := &auth.UserClaims{
		UserID:        leadID,
		Email:         "lead@mit.edu",
		EmailVerified: true,
	}
	q1, err := svc.CreateQuestion(ctx, leadClaims, projectID, recruitment.CreateQuestionInput{
		QuestionText: "How many hours per week can you commit?",
		Required:     true,
		Position:     1,
	})
	if err != nil {
		t.Fatalf("failed to create question: %v", err)
	}
	if q1.ID == uuid.Nil {
		t.Fatalf("expected generated UUID for question, got nil")
	}

	// 3. Applicant submits without answering required question
	applicantClaims := &auth.UserClaims{
		UserID:        "applicant-hours",
		Email:         "applicant@mit.edu",
		EmailVerified: true,
	}
	_, err = svc.SubmitApplication(ctx, applicantClaims, projectID, recruitment.SubmitApplicationInput{
		Statement: "Strong Go background.",
		Answers:   []recruitment.SubmitAnswerInput{},
	})
	if err != recruitment.ErrMissingRequiredAnswer {
		t.Fatalf("expected ErrMissingRequiredAnswer when required answer is missing, got: %v", err)
	}

	// 4. Applicant submits with answering required question
	app, err := svc.SubmitApplication(ctx, applicantClaims, projectID, recruitment.SubmitApplicationInput{
		Statement: "Strong Go background.",
		Answers: []recruitment.SubmitAnswerInput{
			{QuestionID: q1.ID, AnswerText: "15 hours"},
		},
	})
	if err != nil {
		t.Fatalf("failed to submit application with required answer: %v", err)
	}
	if len(app.Answers) != 1 || app.Answers[0].AnswerText != "15 hours" {
		t.Fatalf("expected 1 answer with '15 hours', got: %+v", app.Answers)
	}
}

func TestRecruitmentService_ReviewerAuthorization(t *testing.T) {
	ctx := context.Background()
	repo := newMockRecruitmentRepo()
	projReader := newMockProjectReader()
	memReader := newMockMembershipReader()
	svc := recruitment.NewService(repo, projReader, memReader)

	projectID := uuid.New()
	leadID := "lead-user"
	maintainerID := "maint-user"
	viewerID := "viewer-user"
	applicantID := "applicant-user"

	memReader.members[memReader.memberKey(projectID, leadID)] = &team.ProjectMember{
		ProjectID: projectID,
		MemberID:  leadID,
		Role:      team.RoleLead,
		Status:    "active",
	}
	memReader.members[memReader.memberKey(projectID, maintainerID)] = &team.ProjectMember{
		ProjectID: projectID,
		MemberID:  maintainerID,
		Role:      team.RoleMaintainer,
		Status:    "active",
	}
	memReader.members[memReader.memberKey(projectID, viewerID)] = &team.ProjectMember{
		ProjectID: projectID,
		MemberID:  viewerID,
		Role:      team.RoleViewer,
		Status:    "active",
	}

	projReader.projects[projectID] = &project.Project{
		ID:                projectID,
		Title:             "Campus Navigation",
		RecruitmentStatus: project.RecruitmentOpen,
		OwnerMemberID:     leadID,
	}

	// Applicant submits application
	appClaims := &auth.UserClaims{UserID: applicantID, Email: "app@stanford.edu", EmailVerified: true}
	app, err := svc.SubmitApplication(ctx, appClaims, projectID, recruitment.SubmitApplicationInput{
		Statement: "Interested in pathfinding algorithms.",
	})
	if err != nil {
		t.Fatalf("failed to submit application: %v", err)
	}

	// Viewer cannot list applications
	viewerClaims := &auth.UserClaims{UserID: viewerID, Email: "viewer@stanford.edu", EmailVerified: true}
	_, err = svc.ListProjectApplications(ctx, viewerClaims, projectID, nil)
	if err != recruitment.ErrForbidden {
		t.Fatalf("expected ErrForbidden for team viewer, got: %v", err)
	}

	// Maintainer can list applications
	maintClaims := &auth.UserClaims{UserID: maintainerID, Email: "maint@stanford.edu", EmailVerified: true}
	apps, err := svc.ListProjectApplications(ctx, maintClaims, projectID, nil)
	if err != nil {
		t.Fatalf("maintainer failed to list applications: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 application, got %d", len(apps))
	}

	// Applicant can view their own application
	gotApp, err := svc.GetApplicationByID(ctx, appClaims, app.ID)
	if err != nil {
		t.Fatalf("applicant failed to get own application: %v", err)
	}
	if gotApp.ID != app.ID {
		t.Fatalf("expected app ID %s, got %s", app.ID, gotApp.ID)
	}

	// Unrelated student cannot view applicant's application
	strangerClaims := &auth.UserClaims{UserID: "stranger", Email: "stranger@stanford.edu", EmailVerified: true}
	_, err = svc.GetApplicationByID(ctx, strangerClaims, app.ID)
	if err != recruitment.ErrForbidden {
		t.Fatalf("expected ErrForbidden for stranger viewing application, got: %v", err)
	}
}

func TestRecruitmentService_ShortlistRejectAcceptWorkflow(t *testing.T) {
	ctx := context.Background()
	repo := newMockRecruitmentRepo()
	projReader := newMockProjectReader()
	memReader := newMockMembershipReader()
	svc := recruitment.NewService(repo, projReader, memReader)

	projectID := uuid.New()
	leadID := "lead-boss"
	applicantID := "applicant-star"

	memReader.members[memReader.memberKey(projectID, leadID)] = &team.ProjectMember{
		ProjectID: projectID,
		MemberID:  leadID,
		Role:      team.RoleLead,
		Status:    "active",
	}
	projReader.projects[projectID] = &project.Project{
		ID:                projectID,
		Title:             "Solar Car Telemetry",
		RecruitmentStatus: project.RecruitmentOpen,
		OwnerMemberID:     leadID,
	}

	appClaims := &auth.UserClaims{UserID: applicantID, Email: "star@stanford.edu", EmailVerified: true}
	leadClaims := &auth.UserClaims{UserID: leadID, Email: "lead@stanford.edu", EmailVerified: true}

	app, err := svc.SubmitApplication(ctx, appClaims, projectID, recruitment.SubmitApplicationInput{
		Statement: "Experienced with CAN bus and embedded C.",
	})
	if err != nil {
		t.Fatalf("submit application failed: %v", err)
	}

	// 1. Lead shortlists applicant
	err = svc.ShortlistApplication(ctx, leadClaims, app.ID, "Strong resume, schedule interview")
	if err != nil {
		t.Fatalf("shortlist failed: %v", err)
	}

	shortlistedApp, err := svc.GetApplicationByID(ctx, leadClaims, app.ID)
	if err != nil {
		t.Fatalf("get app failed: %v", err)
	}
	if shortlistedApp.Status != recruitment.StatusShortlisted {
		t.Fatalf("expected status 'shortlisted', got: %s", shortlistedApp.Status)
	}
	if shortlistedApp.ReviewerNotes != "Strong resume, schedule interview" {
		t.Fatalf("expected reviewer notes updated, got: %s", shortlistedApp.ReviewerNotes)
	}

	// 2. Lead accepts applicant -> atomic transaction creates membership
	err = svc.AcceptApplication(ctx, leadClaims, app.ID)
	if err != nil {
		t.Fatalf("accept failed: %v", err)
	}

	acceptedApp, err := svc.GetApplicationByID(ctx, leadClaims, app.ID)
	if err != nil {
		t.Fatalf("get app failed: %v", err)
	}
	if acceptedApp.Status != recruitment.StatusAccepted {
		t.Fatalf("expected status 'accepted', got: %s", acceptedApp.Status)
	}
	if acceptedApp.DecidedBy == nil || *acceptedApp.DecidedBy != leadID {
		t.Fatalf("expected decidedBy %s, got: %v", leadID, acceptedApp.DecidedBy)
	}

	// Verify atomic team membership row was created with Role 'member' and Status 'active'
	member, ok := repo.members[repo.memberKey(projectID, applicantID)]
	if !ok {
		t.Fatalf("expected applicant to be added to project_members")
	}
	if member.Role != team.RoleMember {
		t.Errorf("expected role 'member', got: %s", member.Role)
	}
	if member.Status != "active" {
		t.Errorf("expected status 'active', got: %s", member.Status)
	}

	// 3. Second decision fails with ErrAlreadyDecided
	err = svc.RejectApplication(ctx, leadClaims, app.ID, "too late")
	if err != recruitment.ErrAlreadyDecided {
		t.Fatalf("expected ErrAlreadyDecided for rejecting accepted application, got: %v", err)
	}
}

func TestRecruitmentService_WithdrawalRules(t *testing.T) {
	ctx := context.Background()
	repo := newMockRecruitmentRepo()
	projReader := newMockProjectReader()
	memReader := newMockMembershipReader()
	svc := recruitment.NewService(repo, projReader, memReader)

	projectID := uuid.New()
	leadID := "lead-boss"
	applicantID := "applicant-coder"
	strangerID := "stranger-user"

	memReader.members[memReader.memberKey(projectID, leadID)] = &team.ProjectMember{
		ProjectID: projectID,
		MemberID:  leadID,
		Role:      team.RoleLead,
		Status:    "active",
	}
	projReader.projects[projectID] = &project.Project{
		ID:                projectID,
		Title:             "Campus Pantry App",
		RecruitmentStatus: project.RecruitmentOpen,
		OwnerMemberID:     leadID,
	}

	appClaims := &auth.UserClaims{UserID: applicantID, Email: "coder@stanford.edu", EmailVerified: true}
	strangerClaims := &auth.UserClaims{UserID: strangerID, Email: "stranger@stanford.edu", EmailVerified: true}
	leadClaims := &auth.UserClaims{UserID: leadID, Email: "lead@stanford.edu", EmailVerified: true}

	app, err := svc.SubmitApplication(ctx, appClaims, projectID, recruitment.SubmitApplicationInput{
		Statement: "Want to help food security.",
	})
	if err != nil {
		t.Fatalf("failed to submit application: %v", err)
	}

	// 1. Stranger cannot withdraw applicant's application
	err = svc.WithdrawApplication(ctx, strangerClaims, app.ID)
	if err != recruitment.ErrForbidden {
		t.Fatalf("expected ErrForbidden for non-applicant withdrawing, got: %v", err)
	}

	// 2. Applicant can withdraw submitted application
	err = svc.WithdrawApplication(ctx, appClaims, app.ID)
	if err != nil {
		t.Fatalf("applicant failed to withdraw submitted application: %v", err)
	}

	withdrawnApp, err := svc.GetApplicationByID(ctx, appClaims, app.ID)
	if err != nil {
		t.Fatalf("failed to get app: %v", err)
	}
	if withdrawnApp.Status != recruitment.StatusWithdrawn {
		t.Fatalf("expected status 'withdrawn', got: %s", withdrawnApp.Status)
	}

	// 3. Cannot withdraw already withdrawn application
	err = svc.WithdrawApplication(ctx, appClaims, app.ID)
	if err != recruitment.ErrCannotWithdraw {
		t.Fatalf("expected ErrCannotWithdraw, got: %v", err)
	}

	// 4. Cannot accept or reject withdrawn application
	err = svc.AcceptApplication(ctx, leadClaims, app.ID)
	if err != recruitment.ErrAlreadyDecided {
		t.Fatalf("expected ErrAlreadyDecided, got: %v", err)
	}
}

func TestRecruitmentHandler_EndToEndEndpoints(t *testing.T) {
	repo := newMockRecruitmentRepo()
	projReader := newMockProjectReader()
	memReader := newMockMembershipReader()
	svc := recruitment.NewService(repo, projReader, memReader)
	handler := recruitment.NewHandler(svc)

	projectID := uuid.New()
	leadID := "lead-test"
	applicantID := "applicant-test"

	memReader.members[memReader.memberKey(projectID, leadID)] = &team.ProjectMember{
		ProjectID: projectID,
		MemberID:  leadID,
		Role:      team.RoleLead,
		Status:    "active",
	}
	projReader.projects[projectID] = &project.Project{
		ID:                projectID,
		Title:             "Campus Robotics",
		RecruitmentStatus: project.RecruitmentOpen,
		OwnerMemberID:     leadID,
	}

	leadClaims := &auth.UserClaims{UserID: leadID, Email: "lead@mit.edu", EmailVerified: true}
	applicantClaims := &auth.UserClaims{UserID: applicantID, Email: "app@mit.edu", EmailVerified: true}

	router := handler.Routes(httpx.RequireAuthFromContext())

	// 1. Lead creates a recruitment question
	createQBody := map[string]any{
		"question_text": "What is your experience with ROS2?",
		"help_text":     "List projects or courses",
		"required":      true,
		"position":      1,
	}
	bodyBytes, _ := json.Marshal(createQBody)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/projects/%s/questions", projectID.String()), bytes.NewReader(bodyBytes))
	req = req.WithContext(auth.WithUserContext(req.Context(), leadClaims))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for question creation, got %d: %s", rr.Code, rr.Body.String())
	}

	var qResp struct {
		Data recruitment.ApplicationQuestion `json:"data"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &qResp)
	qID := qResp.Data.ID

	// 2. Applicant submits application
	appBody := map[string]any{
		"statement": "I have built 2 autonomous mobile robots using ROS2 Nav2.",
		"answers": []map[string]any{
			{"question_id": qID.String(), "answer_text": "Completed CS223A and built rover"},
		},
	}
	bodyBytes, _ = json.Marshal(appBody)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/projects/%s/applications", projectID.String()), bytes.NewReader(bodyBytes))
	req = req.WithContext(auth.WithUserContext(req.Context(), applicantClaims))
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for application submission, got %d: %s", rr.Code, rr.Body.String())
	}

	var appResp struct {
		Data recruitment.Application `json:"data"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &appResp)
	createdAppID := appResp.Data.ID

	// 3. Lead lists applications for project
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/projects/%s/applications", projectID.String()), nil)
	req = req.WithContext(auth.WithUserContext(req.Context(), leadClaims))
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for list applications, got %d: %s", rr.Code, rr.Body.String())
	}

	// 4. Applicant views their submitted applications via /applications/me
	req = httptest.NewRequest(http.MethodGet, "/applications/me", nil)
	req = req.WithContext(auth.WithUserContext(req.Context(), applicantClaims))
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /applications/me, got %d: %s", rr.Code, rr.Body.String())
	}

	// 5. Lead shortlists application
	shortlistBody := map[string]string{"reviewer_notes": "Promising candidate"}
	bodyBytes, _ = json.Marshal(shortlistBody)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/applications/%s/shortlist", createdAppID.String()), bytes.NewReader(bodyBytes))
	req = req.WithContext(auth.WithUserContext(req.Context(), leadClaims))
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for shortlist, got %d: %s", rr.Code, rr.Body.String())
	}

	// 6. Lead accepts application
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/applications/%s/accept", createdAppID.String()), nil)
	req = req.WithContext(auth.WithUserContext(req.Context(), leadClaims))
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for accept, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify team membership in repo
	if _, ok := repo.members[repo.memberKey(projectID, applicantID)]; !ok {
		t.Fatalf("expected applicant to be added to project team members")
	}
}
