package ai_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/ai"
	"github.com/lynk/backend/internal/httputil"
)

// mockProjectEmbeddingRepo implements ai.ProjectEmbeddingRepository in-memory for testing.
type mockProjectEmbeddingRepo struct {
	mu         sync.Mutex
	embeddings map[uuid.UUID][]float32
	hashes     map[uuid.UUID]string
	projects   map[uuid.UUID]ai.ProjectMatch
}

func newMockProjectEmbeddingRepo() *mockProjectEmbeddingRepo {
	return &mockProjectEmbeddingRepo{
		embeddings: make(map[uuid.UUID][]float32),
		hashes:     make(map[uuid.UUID]string),
		projects:   make(map[uuid.UUID]ai.ProjectMatch),
	}
}

func (m *mockProjectEmbeddingRepo) UpsertProjectEmbedding(ctx context.Context, projectID uuid.UUID, embedding []float32, contentHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.embeddings[projectID] = embedding
	m.hashes[projectID] = contentHash
	return nil
}

func (m *mockProjectEmbeddingRepo) GetProjectEmbedding(ctx context.Context, projectID uuid.UUID) ([]float32, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hash, ok := m.hashes[projectID]
	if !ok {
		return nil, "", ai.ErrEmbeddingNotFound
	}
	return m.embeddings[projectID], hash, nil
}

func (m *mockProjectEmbeddingRepo) SearchSimilarProjects(ctx context.Context, queryVector []float32, limit int, visibility string) ([]ai.ProjectMatch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var results []ai.ProjectMatch
	for _, p := range m.projects {
		if visibility != "" && p.Visibility != visibility {
			continue
		}
		results = append(results, p)
		if limit > 0 && len(results) >= limit {
			break
		}
	}
	if results == nil {
		results = []ai.ProjectMatch{}
	}
	return results, nil
}

// mockEmbeddingGenerator implements ai.EmbeddingGenerator for deterministic unit testing.
type mockEmbeddingGenerator struct {
	mu         sync.Mutex
	callCount  int
	lastInput  string
	vectorSize int
}

func newMockEmbeddingGenerator(vectorSize int) *mockEmbeddingGenerator {
	return &mockEmbeddingGenerator{
		vectorSize: vectorSize,
	}
}

func (m *mockEmbeddingGenerator) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	m.lastInput = text

	vec := make([]float32, m.vectorSize)
	for i := range vec {
		val := float32(math.Sin(float64(i+1) * float64(len(text))))
		vec[i] = val
	}
	return vec, nil
}

func (m *mockEmbeddingGenerator) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func TestProjectEmbeddings_FormatContent(t *testing.T) {
	text := ai.FormatProjectForEmbedding(
		"Campus Autonomous Shuttle",
		"Robotics",
		"Autonomous shuttle for campus transit with obstacle detection.",
	)
	expected := "Title: Campus Autonomous Shuttle\nCategory: Robotics\nDescription: Autonomous shuttle for campus transit with obstacle detection."
	if text != expected {
		t.Fatalf("expected %q, got %q", expected, text)
	}

	// Test whitespace trimming
	textTrim := ai.FormatProjectForEmbedding(
		"   Campus Autonomous Shuttle  ",
		"  Robotics  ",
		"  Autonomous shuttle for campus transit with obstacle detection.   ",
	)
	if textTrim != expected {
		t.Fatalf("expected trimmed output %q, got %q", expected, textTrim)
	}
}

func TestProjectEmbeddings_ComputeContentHash(t *testing.T) {
	contentA := "Title: Lynk\nCategory: Platform\nDescription: High-trust student project collaboration platform."
	hashA1 := ai.ComputeContentHash(contentA)
	hashA2 := ai.ComputeContentHash("  " + contentA + "  \n")

	if len(hashA1) != 64 {
		t.Fatalf("expected 64-char hex hash, got length %d (%q)", len(hashA1), hashA1)
	}
	if hashA1 != hashA2 {
		t.Fatalf("expected identical hashes after trimming whitespace, got %q vs %q", hashA1, hashA2)
	}

	contentB := "Title: Different\nCategory: Platform\nDescription: Different text."
	hashB := ai.ComputeContentHash(contentB)
	if hashA1 == hashB {
		t.Fatalf("expected distinct hashes for distinct contents, got %q", hashA1)
	}
}

func TestProjectEmbeddings_FormatAndParseVector(t *testing.T) {
	original := []float32{0.123456, -0.654321, 0.0, 1.0}
	str := ai.FormatVector(original)
	expectedPrefix := "["
	if len(str) == 0 || str[0] != expectedPrefix[0] || str[len(str)-1] != ']' {
		t.Fatalf("expected bracket-wrapped vector, got %q", str)
	}

	parsed, err := ai.ParseVector(str)
	if err != nil {
		t.Fatalf("ParseVector failed: %v", err)
	}
	if len(parsed) != len(original) {
		t.Fatalf("expected vector length %d, got %d", len(original), len(parsed))
	}
	for i := range original {
		if math.Abs(float64(parsed[i]-original[i])) > 1e-5 {
			t.Fatalf("element %d mismatch: expected %f, got %f", i, original[i], parsed[i])
		}
	}
}

func TestService_SyncProjectEmbedding_CacheAsideDeduplication(t *testing.T) {
	ctx := context.Background()
	repo := newMockProjectEmbeddingRepo()
	generator := newMockEmbeddingGenerator(384)
	service := ai.NewService(repo, generator)

	projectID := uuid.New()
	title := "Solar Car Telemetry"
	category := "Engineering"
	description := "Real-time telemetry and battery thermal management dashboard."

	// 1. Initial Sync: Should generate embedding and persist
	err := service.SyncProjectEmbedding(ctx, projectID, title, category, description)
	if err != nil {
		t.Fatalf("first SyncProjectEmbedding failed: %v", err)
	}
	if generator.CallCount() != 1 {
		t.Fatalf("expected 1 generator call, got %d", generator.CallCount())
	}

	storedVec, storedHash, err := repo.GetProjectEmbedding(ctx, projectID)
	if err != nil {
		t.Fatalf("GetProjectEmbedding failed: %v", err)
	}
	if len(storedVec) != 384 {
		t.Fatalf("expected 384-dimensional vector, got %d", len(storedVec))
	}
	if storedHash == "" {
		t.Fatalf("expected non-empty stored content hash")
	}

	// 2. Second Sync with Identical Content: Should skip generator call (idempotent cache-aside)
	err = service.SyncProjectEmbedding(ctx, projectID, title, category, description)
	if err != nil {
		t.Fatalf("second SyncProjectEmbedding failed: %v", err)
	}
	if generator.CallCount() != 1 {
		t.Fatalf("expected generator call count to remain 1 after identical content, got %d", generator.CallCount())
	}

	// 3. Third Sync with Modified Description: Should detect hash change and re-embed
	updatedDescription := "Real-time telemetry and battery thermal management dashboard with CAN bus logging."
	err = service.SyncProjectEmbedding(ctx, projectID, title, category, updatedDescription)
	if err != nil {
		t.Fatalf("third SyncProjectEmbedding failed: %v", err)
	}
	if generator.CallCount() != 2 {
		t.Fatalf("expected generator call count to increment to 2 on content update, got %d", generator.CallCount())
	}

	_, newHash, err := repo.GetProjectEmbedding(ctx, projectID)
	if err != nil {
		t.Fatalf("GetProjectEmbedding after update failed: %v", err)
	}
	if newHash == storedHash {
		t.Fatalf("expected content hash to change on update, but stayed %q", newHash)
	}
}

func TestService_SearchProjectsSemantic(t *testing.T) {
	ctx := context.Background()
	repo := newMockProjectEmbeddingRepo()
	generator := newMockEmbeddingGenerator(384)
	service := ai.NewService(repo, generator)

	projID1 := uuid.New()
	projID2 := uuid.New()
	repo.projects[projID1] = ai.ProjectMatch{
		ProjectID:   projID1,
		Slug:        "campus-transit-bot",
		Title:       "Campus Transit Bot",
		Description: "Autonomous campus transport with SLAM navigation.",
		Category:    "Robotics",
		Visibility:  "public",
		Similarity:  0.92,
		Distance:    0.08,
	}
	repo.projects[projID2] = ai.ProjectMatch{
		ProjectID:   projID2,
		Slug:        "secret-drone",
		Title:       "Secret Drone Project",
		Description: "Classified robotics project.",
		Category:    "Robotics",
		Visibility:  "private",
		Similarity:  0.88,
		Distance:    0.12,
	}

	// Search public visibility
	matches, err := service.SearchProjectsSemantic(ctx, "autonomous robotics navigation", 10, "public")
	if err != nil {
		t.Fatalf("SearchProjectsSemantic failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 public match, got %d", len(matches))
	}
	if matches[0].ProjectID != projID1 {
		t.Fatalf("expected match project ID %s, got %s", projID1, matches[0].ProjectID)
	}

	// Empty query returns empty results without calling generator
	emptyMatches, err := service.SearchProjectsSemantic(ctx, "   ", 10, "public")
	if err != nil {
		t.Fatalf("expected empty query to succeed, got %v", err)
	}
	if len(emptyMatches) != 0 {
		t.Fatalf("expected 0 matches for empty query, got %d", len(emptyMatches))
	}
}

func TestAIHandler_SearchProjects(t *testing.T) {
	repo := newMockProjectEmbeddingRepo()
	generator := newMockEmbeddingGenerator(384)
	service := ai.NewService(repo, generator)

	projID := uuid.New()
	repo.projects[projID] = ai.ProjectMatch{
		ProjectID:   projID,
		Slug:        "solar-car",
		Title:       "Solar Car Team",
		Description: "High-efficiency electric vehicle powered by solar arrays.",
		Category:    "Automotive",
		Visibility:  "public",
		Similarity:  0.95,
		Distance:    0.05,
	}

	handler := ai.NewHandler(nil, nil, nil, nil).WithService(service)

	// 1. Successful query
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/projects?q=electric+vehicle&limit=10", nil)
	w := httptest.NewRecorder()
	handler.SearchProjects(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp httputil.Response[[]ai.ProjectMatch]
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success: true")
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Data))
	}
	if resp.Data[0].Slug != "solar-car" {
		t.Fatalf("expected slug solar-car, got %s", resp.Data[0].Slug)
	}

	// 2. Empty query returns empty list
	reqEmpty := httptest.NewRequest(http.MethodGet, "/api/v1/search/projects?q=", nil)
	wEmpty := httptest.NewRecorder()
	handler.SearchProjects(wEmpty, reqEmpty)

	if wEmpty.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", wEmpty.Code)
	}
	var respEmpty httputil.Response[[]ai.ProjectMatch]
	if err := json.Unmarshal(wEmpty.Body.Bytes(), &respEmpty); err != nil {
		t.Fatalf("failed to decode empty response: %v", err)
	}
	if len(respEmpty.Data) != 0 {
		t.Fatalf("expected 0 results for empty query, got %d", len(respEmpty.Data))
	}
}
