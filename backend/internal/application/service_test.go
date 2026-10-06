package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/job"
	"github.com/lynk/backend/internal/outbox"
)

type testOutboxRecorder struct {
	events []outbox.Event
}

func (r *testOutboxRecorder) Publish(ctx context.Context, e outbox.Event) error {
	r.events = append(r.events, e)
	return nil
}

func TestService_ApplyToJob_OutboxPayload_ContainsClientID(t *testing.T) {
	ctx := context.Background()
	repo := newMockApplicationRepo()
	outboxRec := &testOutboxRecorder{}

	svc := NewService(repo, repo, repo).WithOutbox(outboxRec)

	jobID := uuid.New()
	creatorID := "client-user-999"
	applicantID := "applicant-student-111"
	deadline := time.Now().UTC().Add(48 * time.Hour)

	repo.jobs[jobID] = &job.Job{
		ID:        jobID,
		Title:     "Campus Event Photographer",
		Status:    job.StatusOpen,
		CreatedBy: creatorID,
		Deadline:  &deadline,
	}

	claims := &auth.UserClaims{
		UserID:        applicantID,
		Email:         "student@campus.edu",
		EmailVerified: true,
	}

	req := ApplyRequest{
		CoverLetter: "I have 3 years of event photography experience.",
	}

	app, err := svc.ApplyToJob(ctx, claims, jobID, req)
	if err != nil {
		t.Fatalf("unexpected ApplyToJob error: %v", err)
	}
	if app == nil {
		t.Fatal("expected application, got nil")
	}

	if len(outboxRec.events) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(outboxRec.events))
	}

	evt := outboxRec.events[0]
	if evt.EventType != "application_submitted" {
		t.Errorf("expected EventType application_submitted, got %s", evt.EventType)
	}
	if evt.AggregateType != "application" {
		t.Errorf("expected AggregateType application, got %s", evt.AggregateType)
	}
	if evt.AggregateID != app.ID.String() {
		t.Errorf("expected AggregateID %s, got %s", app.ID.String(), evt.AggregateID)
	}

	payload := evt.Payload
	if payload == nil {
		t.Fatal("expected non-nil event payload")
	}

	if payload["application_id"] != app.ID.String() {
		t.Errorf("expected application_id %s, got %v", app.ID.String(), payload["application_id"])
	}
	if payload["job_id"] != jobID.String() {
		t.Errorf("expected job_id %s, got %v", jobID.String(), payload["job_id"])
	}
	if payload["applicant_id"] != applicantID {
		t.Errorf("expected applicant_id %s, got %v", applicantID, payload["applicant_id"])
	}
	if payload["client_id"] != creatorID {
		t.Errorf("expected client_id %s, got %v", creatorID, payload["client_id"])
	}
}

func TestService_ApplyToJob_NilOutbox_Succeeds(t *testing.T) {
	ctx := context.Background()
	repo := newMockApplicationRepo()

	svc := NewService(repo, repo, repo)

	jobID := uuid.New()
	creatorID := "client-user-888"
	applicantID := "applicant-student-222"
	deadline := time.Now().UTC().Add(24 * time.Hour)

	repo.jobs[jobID] = &job.Job{
		ID:        jobID,
		Title:     "Web Developer",
		Status:    job.StatusOpen,
		CreatedBy: creatorID,
		Deadline:  &deadline,
	}

	claims := &auth.UserClaims{
		UserID:        applicantID,
		Email:         "coder@campus.edu",
		EmailVerified: true,
	}

	req := ApplyRequest{
		CoverLetter: "Full stack developer interested in campus projects.",
	}

	app, err := svc.ApplyToJob(ctx, claims, jobID, req)
	if err != nil {
		t.Fatalf("unexpected ApplyToJob error: %v", err)
	}
	if app == nil {
		t.Fatal("expected application, got nil")
	}
}

func TestService_ApplyToJob_SanitizesCoverLetter(t *testing.T) {
	ctx := context.Background()
	repo := newMockApplicationRepo()
	svc := NewService(repo, repo, repo)

	jobID := uuid.New()
	creatorID := "client-user-777"
	applicantID := "applicant-student-333"
	deadline := time.Now().UTC().Add(24 * time.Hour)

	repo.jobs[jobID] = &job.Job{
		ID:        jobID,
		Title:     "Backend Engineer",
		Status:    job.StatusOpen,
		CreatedBy: creatorID,
		Deadline:  &deadline,
	}

	claims := &auth.UserClaims{
		UserID:        applicantID,
		Email:         "coder@campus.edu",
		EmailVerified: true,
	}

	req := ApplyRequest{
		CoverLetter: "Hello <script>alert('xss')</script> I am <b>qualified</b> for this job.",
	}

	app, err := svc.ApplyToJob(ctx, claims, jobID, req)
	if err != nil {
		t.Fatalf("unexpected ApplyToJob error: %v", err)
	}
	if app == nil {
		t.Fatal("expected application, got nil")
	}

	expected := "Hello I am qualified for this job."
	if app.CoverLetter != expected {
		t.Fatalf("expected sanitized cover letter %q, got %q", expected, app.CoverLetter)
	}
}
