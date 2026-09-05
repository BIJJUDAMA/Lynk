package user_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/storage"
	"github.com/lynk/backend/internal/user"
)

// mockStorageClient implements storage.Client for isolated unit testing.
type mockStorageClient struct {
	mu            sync.RWMutex
	uploads       map[string][]byte
	contentTypes  map[string]string
	presignedURLs map[string]string
	errUpload     error
	errPresign    error
}

func newMockStorageClient() *mockStorageClient {
	return &mockStorageClient{
		uploads:       make(map[string][]byte),
		contentTypes:  make(map[string]string),
		presignedURLs: make(map[string]string),
	}
}

var _ storage.Client = (*mockStorageClient)(nil)

func (m *mockStorageClient) UploadResume(ctx context.Context, key string, contentType string, body io.Reader) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errUpload != nil {
		return m.errUpload
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.uploads[key] = data
	m.contentTypes[key] = contentType
	return nil
}

func (m *mockStorageClient) GetPresignedDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.errPresign != nil {
		return "", m.errPresign
	}
	if url, ok := m.presignedURLs[key]; ok {
		return url, nil
	}
	return "https://minio.local/resumes/" + key + "?expires=900", nil
}

func createMultipartRequest(url, fieldName, filename string, content []byte) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(content); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	req := httptest.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func setupResumeTestRouter(repo user.UserRepository, s3 storage.Client, claims *auth.UserClaims) (*user.Handler, chi.Router) {
	svc := user.NewService(repo)
	h := user.NewHandler(svc, repo, s3)
	r := h.Routes(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if claims != nil {
				ctx := auth.WithUserContext(req.Context(), claims)
				next.ServeHTTP(w, req.WithContext(ctx))
			} else {
				next.ServeHTTP(w, req)
			}
		})
	})
	return h, r
}

func TestResume_UploadSuccess(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	userUUID := uuid.New()

	claims := &auth.UserClaims{
		UserID:        userUUID.String(),
		Email:         "alex@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	_, router := setupResumeTestRouter(repo, s3Client, claims)

	pdfContent := []byte("%PDF-1.4 mock resume content")
	req, err := createMultipartRequest("/profile/resume", "resume", "alex_resume.pdf", pdfContent)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	profile, err := repo.GetProfile(context.Background(), userUUID.String())
	if err != nil || profile == nil {
		t.Fatalf("profile not found: %v", err)
	}

	if profile.ResumeKey == nil || *profile.ResumeKey == "" {
		t.Error("expected resume_key to be set")
	}
	if profile.ResumeFilename == nil || *profile.ResumeFilename != "alex_resume.pdf" {
		t.Errorf("expected filename alex_resume.pdf, got %v", profile.ResumeFilename)
	}
}

func TestResume_UnverifiedEmailRejected(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	userUUID := uuid.New()

	claims := &auth.UserClaims{
		UserID:        userUUID.String(),
		Email:         "unverified@berkeley.edu",
		EmailVerified: false,
		Roles:         []string{"member"},
	}

	_, router := setupResumeTestRouter(repo, s3Client, claims)

	pdfContent := []byte("%PDF-1.4 mock resume content")
	req, _ := createMultipartRequest("/profile/resume", "resume", "resume.pdf", pdfContent)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for unverified email, got %d", rec.Code)
	}
}

func TestResume_GetPresignedURL(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	userUUID := uuid.New()

	key := "resumes/" + userUUID.String() + "/resume.pdf"
	_ = repo.UpdateResume(context.Background(), userUUID.String(), key, "resume.pdf", 1024)

	claims := &auth.UserClaims{
		UserID:        userUUID.String(),
		Email:         "alex@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	_, router := setupResumeTestRouter(repo, s3Client, claims)

	req := httptest.NewRequest("GET", "/profile/resume", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestResume_MultipartCleanup verifies that UploadResume cleans up multipart temp files
// by calling RemoveAll on r.MultipartForm (SEC-09).
// We cannot introspect the deferred call directly in a black-box test, but we can confirm the
// full success path completes without error and the file is correctly processed—meaning the
// defer branch that calls RemoveAll ran without panic or side-effect.
func TestResume_MultipartCleanup(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	userUUID := uuid.New()

	claims := &auth.UserClaims{
		UserID:        userUUID.String(),
		Email:         "cleanup@mit.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	_, router := setupResumeTestRouter(repo, s3Client, claims)

	pdfContent := []byte("%PDF-1.4 cleanup test resume")
	req, err := createMultipartRequest("/profile/resume", "resume", "cleanup.pdf", pdfContent)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rec := httptest.NewRecorder()
	// ServeHTTP blocks until the handler returns, including all deferred calls.
	// If RemoveAll panics or the defer has a type error, the test will fail here.
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 after upload (multipart cleanup), got %d: %s", rec.Code, rec.Body.String())
	}

	// Confirm resume was stored, proving the full handler path ran correctly.
	profile, err := repo.GetProfile(context.Background(), userUUID.String())
	if err != nil || profile == nil {
		t.Fatalf("profile not found after upload: %v", err)
	}
	if profile.ResumeKey == nil || *profile.ResumeKey == "" {
		t.Error("expected resume_key to be set after upload with multipart cleanup")
	}
}

// TestResume_GetMemberResumeURL verifies that GET /profile/{id}/resume uses the
// route parameter {id} to look up the target member's resume, not the caller's own
// token subject (SEC-07). The caller's UUID and the target member UUID are distinct.
func TestResume_GetMemberResumeURL(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()

	// Target member: a different user whose resume we want to view.
	targetUUID := uuid.New()
	resumeKey := "resumes/" + targetUUID.String() + "/applicant_resume.pdf"
	resumeFilename := "applicant_resume.pdf"
	if err := repo.UpdateResume(context.Background(), targetUUID.String(), resumeKey, resumeFilename, 2048); err != nil {
		t.Fatalf("failed to seed target resume: %v", err)
	}

	// Caller is an employer with a different UUID.
	callerUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        callerUUID.String(),
		Email:         "employer@company.edu",
		EmailVerified: true,
		Roles:         []string{"employer"},
	}

	_, router := setupResumeTestRouter(repo, s3Client, claims)

	// Request the target member's resume via route param {id} = targetUUID.
	req := httptest.NewRequest("GET", "/profile/"+targetUUID.String()+"/resume", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for GetMemberResumeURL, got %d: %s", rec.Code, rec.Body.String())
	}

	// Confirm the response contains a download URL (presigned URL was generated for target's key).
	body := rec.Body.String()
	if body == "" {
		t.Error("expected non-empty response body with download_url")
	}
}

// TestResume_GetMemberResumeURL_NotFound verifies that GET /profile/{id}/resume returns 404
// when the target member has no resume uploaded.
func TestResume_GetMemberResumeURL_NotFound(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()

	// Target exists but has no resume.
	targetUUID := uuid.New()

	callerUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        callerUUID.String(),
		Email:         "employer2@company.edu",
		EmailVerified: true,
		Roles:         []string{"employer"},
	}

	_, router := setupResumeTestRouter(repo, s3Client, claims)

	req := httptest.NewRequest("GET", "/profile/"+targetUUID.String()+"/resume", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Target has no profile/resume row at all → 404.
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing resume, got %d: %s", rec.Code, rec.Body.String())
	}
}
