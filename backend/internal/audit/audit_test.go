package audit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type mockAuditRepo struct {
	entries []*Entry
}

func (m *mockAuditRepo) Insert(ctx context.Context, entry *Entry) error {
	m.entries = append(m.entries, entry)
	return nil
}

func (m *mockAuditRepo) Query(ctx context.Context, filter Filter) ([]*Entry, error) {
	var res []*Entry
	for _, e := range m.entries {
		if filter.ActorID != "" && e.ActorID != filter.ActorID {
			continue
		}
		if filter.Action != "" && e.Action != filter.Action {
			continue
		}
		res = append(res, e)
	}
	return res, nil
}

func TestService_RecordValidation(t *testing.T) {
	repo := &mockAuditRepo{}
	svc := NewService(repo, nil)
	ctx := context.Background()

	// Missing actor_id
	if err := svc.Record(ctx, &Entry{Action: "act", TargetType: "job", TargetID: "j1"}); err != ErrInvalidActorID {
		t.Errorf("expected ErrInvalidActorID, got %v", err)
	}

	// Missing action
	if err := svc.Record(ctx, &Entry{ActorID: "usr", TargetType: "job", TargetID: "j1"}); err != ErrInvalidAction {
		t.Errorf("expected ErrInvalidAction, got %v", err)
	}

	// Missing target_type
	if err := svc.Record(ctx, &Entry{ActorID: "usr", Action: "act", TargetID: "j1"}); err != ErrInvalidTargetType {
		t.Errorf("expected ErrInvalidTargetType, got %v", err)
	}

	// Missing target_id
	if err := svc.Record(ctx, &Entry{ActorID: "usr", Action: "act", TargetType: "job"}); err != ErrInvalidTargetID {
		t.Errorf("expected ErrInvalidTargetID, got %v", err)
	}

	// Valid entry
	entry := &Entry{
		ActorID:    "u1",
		Action:     "project.created",
		TargetType: "project",
		TargetID:   "p1",
		Metadata:   map[string]any{"title": "Test Rover"},
	}
	if err := svc.Record(ctx, entry); err != nil {
		t.Errorf("unexpected error on valid record: %v", err)
	}
	if len(repo.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(repo.entries))
	}
}

func TestConsumer_ProcessDelivery(t *testing.T) {
	repo := &mockAuditRepo{}
	svc := NewService(repo, nil)
	ctx := context.Background()

	body := []byte(`{"actor_id":"u456","action":"user.login","target_type":"user","target_id":"u456"}`)
	headers := amqp.Table{
		"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	}

	d := amqp.Delivery{
		Body:    body,
		Headers: headers,
	}

	if err := ProcessDelivery(ctx, svc, d, nil); err != nil {
		t.Fatalf("ProcessDelivery failed: %v", err)
	}

	if len(repo.entries) != 1 {
		t.Fatalf("expected 1 entry in repo, got %d", len(repo.entries))
	}

	if repo.entries[0].TraceID != "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01" {
		t.Errorf("expected trace id populated, got %s", repo.entries[0].TraceID)
	}
}

func TestHandler_QueryLogs(t *testing.T) {
	repo := &mockAuditRepo{
		entries: []*Entry{
			{
				ID:         uuid.New(),
				ActorID:    "admin-1",
				Action:     "contract.signed",
				TargetType: "contract",
				TargetID:   "c-999",
				CreatedAt:  time.Now(),
			},
		},
	}
	svc := NewService(repo, nil)
	handler := NewHandler(svc)

	req := httptest.NewRequest("GET", "/api/v1/audit/logs?actor_id=admin-1", nil)
	rec := httptest.NewRecorder()

	handler.QueryLogs(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", rec.Code)
	}
}
