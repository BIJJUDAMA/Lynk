// Package search tests.
// 100% pure 7-bit ASCII
package search

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestMeiliClient_Health(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"available"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL, APIKey: "test-key"})
	if !client.Health(context.Background()) {
		t.Fatal("expected client.Health to return true")
	}
}

func TestMeiliClient_SearchAndIndex(t *testing.T) {
	var receivedDocs []map[string]any
	var searchReceived bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing or invalid Authorization header: %s", r.Header.Get("Authorization"))
		}

		if r.URL.Path == "/indexes/projects/documents" && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &receivedDocs)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"taskUid":1,"status":"enqueued"}`))
			return
		}

		if r.URL.Path == "/indexes/projects/search" && r.Method == http.MethodPost {
			searchReceived = true
			res := SearchResult{
				Hits: []ProjectDocument{
					{
						ID:    "p1",
						Slug:  "autonomous-rover",
						Title: "Autonomous Rover",
					},
				},
				EstimatedHits: 1,
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(res)
			return
		}

		if r.URL.Path == "/indexes/projects/documents/p1" && r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL, APIKey: "test-key"})
	ctx := context.Background()

	// 1. Index
	doc := ProjectDocument{
		ID:    "p1",
		Slug:  "autonomous-rover",
		Title: "Autonomous Rover",
	}
	if err := client.IndexDocuments(ctx, "projects", []ProjectDocument{doc}); err != nil {
		t.Fatalf("IndexDocuments failed: %v", err)
	}
	if len(receivedDocs) != 1 {
		t.Fatalf("expected 1 received doc, got %d", len(receivedDocs))
	}

	// 2. Search
	searchRes, err := client.Search(ctx, "projects", SearchRequest{Query: "rover"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if !searchReceived || len(searchRes.Hits) != 1 || searchRes.Hits[0].Slug != "autonomous-rover" {
		t.Fatalf("unexpected search response: %+v", searchRes)
	}

	// 3. Delete
	if err := client.DeleteDocument(ctx, "projects", "p1"); err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}
}

func TestSearchService_FallbackToDatabase(t *testing.T) {
	// Meilisearch server returning 500 error
	brokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer brokenServer.Close()

	client := NewClient(Config{BaseURL: brokenServer.URL})
	fallbackCalled := false

	fallback := func(ctx context.Context, query string, limit, offset int, category string) ([]ProjectDocument, int, error) {
		fallbackCalled = true
		return []ProjectDocument{
			{
				ID:    "fallback-1",
				Title: "Database Match",
			},
		}, 1, nil
	}

	svc := NewService(client, fallback, nil)
	hits, total, err := svc.SearchProjects(context.Background(), "query", 10, 0, "")
	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}
	if !fallbackCalled {
		t.Fatal("expected fallback function to be called on search failure")
	}
	if total != 1 || len(hits) != 1 || hits[0].ID != "fallback-1" {
		t.Fatalf("unexpected fallback hits: %+v", hits)
	}
}

func TestProcessDelivery_ConsumerIndexAndDelete(t *testing.T) {
	indexed := false
	deleted := false

	mockSvc := &mockSearchService{
		indexFn: func(doc ProjectDocument) error {
			if doc.ID == "p1" && doc.Slug == "test-slug" {
				indexed = true
			}
			return nil
		},
		deleteFn: func(id string) error {
			if id == "p1" {
				deleted = true
			}
			return nil
		},
	}

	// Test Index Delivery
	indexPayload, _ := json.Marshal(IndexEventPayload{
		ID:     "p1",
		Slug:   "test-slug",
		Title:  "Test Project",
		Action: "create",
	})
	delivery := amqp.Delivery{
		Body:       indexPayload,
		RoutingKey: "project.created",
	}

	if err := ProcessDelivery(context.Background(), mockSvc, delivery, nil); err != nil {
		t.Fatalf("ProcessDelivery index error: %v", err)
	}
	if !indexed {
		t.Fatal("expected project to be indexed")
	}

	// Test Delete Delivery
	deletePayload, _ := json.Marshal(IndexEventPayload{
		ID:     "p1",
		Action: "delete",
	})
	deleteDelivery := amqp.Delivery{
		Body:       deletePayload,
		RoutingKey: "project.deleted",
	}

	if err := ProcessDelivery(context.Background(), mockSvc, deleteDelivery, nil); err != nil {
		t.Fatalf("ProcessDelivery delete error: %v", err)
	}
	if !deleted {
		t.Fatal("expected project to be deleted")
	}
}

type mockSearchService struct {
	indexFn  func(doc ProjectDocument) error
	deleteFn func(id string) error
}

func (m *mockSearchService) SearchProjects(ctx context.Context, query string, limit, offset int, category string) ([]ProjectDocument, int, error) {
	return nil, 0, nil
}

func (m *mockSearchService) IndexProject(ctx context.Context, doc ProjectDocument) error {
	if m.indexFn != nil {
		return m.indexFn(doc)
	}
	return nil
}

func (m *mockSearchService) DeleteProject(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(id)
	}
	return nil
}

func (m *mockSearchService) IsHealthy(ctx context.Context) bool {
	return true
}
