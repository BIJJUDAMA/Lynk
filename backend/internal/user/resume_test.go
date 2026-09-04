package user_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func setupResumeTestRouter(repo user.UserRepository, s3 storage.Client) (*user.Handler, chi.Router) {
	svc := user.NewService(repo)
	h := user.NewHandler(svc, repo, s3)
	r := h.Routes(func(next http.Handler) http.Handler {
		return next
	})
	return h, r
}

func TestResume_UploadSuccess(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	_, router := setupResumeTestRouter(repo, s3Client)

	studentUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        studentUUID.String(),
		Email:         "jane@student.mit.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	// Pre-populate student profile
	_ = repo.UpsertUser(context.Background(), &user.User{
		ID:    studentUUID,
		Email: claims.Email,
		Role:  "student",
	})
	_ = repo.UpsertStudentProfile(context.Background(), &user.StudentProfile{
		UserID:    studentUUID,
		FirstName: "Jane",
		LastName:  "Doe",
	})

	pdfContent := []byte("%PDF-1.4 sample pdf content for resume test")
	req, err := createMultipartRequest("/profile/student/resume", "resume", "jane_doe_cv.pdf", pdfContent)
	if err != nil {
		t.Fatalf("failed to create multipart request: %v", err)
	}
	req = req.WithContext(auth.WithUserContext(req.Context(), claims))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var env responseEnvelope
	if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !env.Success || env.Error != nil {
		t.Fatalf("expected success envelope, got: %+v", env)
	}

	var respData map[string]interface{}
	if err := json.Unmarshal(env.Data, &respData); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}

	if respData["filename"] != "jane_doe_cv.pdf" {
		t.Errorf("expected filename jane_doe_cv.pdf, got %v", respData["filename"])
	}

	// Verify PostgreSQL metadata update
	profile, err := repo.GetStudentProfile(context.Background(), studentUUID)
	if err != nil || profile == nil {
		t.Fatalf("failed to get updated profile: %v", err)
	}
	if profile.ResumeKey == nil || *profile.ResumeKey == "" {
		t.Errorf("expected resume_key to be set")
	}
	if profile.ResumeFilename == nil || *profile.ResumeFilename != "jane_doe_cv.pdf" {
		t.Errorf("expected resume_filename to be jane_doe_cv.pdf, got %v", profile.ResumeFilename)
	}
	if profile.ResumeByteSize != int64(len(pdfContent)) {
		t.Errorf("expected byte size %d, got %d", len(pdfContent), profile.ResumeByteSize)
	}

	// Verify MinIO upload and canonical content-type
	s3Client.mu.RLock()
	uploadedCount := len(s3Client.uploads)
	contentType := s3Client.contentTypes[*profile.ResumeKey]
	s3Client.mu.RUnlock()
	if uploadedCount != 1 {
		t.Fatalf("expected 1 upload to S3, got %d", uploadedCount)
	}
	if contentType != "application/pdf" {
		t.Errorf("expected canonical content-type application/pdf, got %s", contentType)
	}
}

func TestResume_UploadDocxAndCaseInsensitive(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	_, router := setupResumeTestRouter(repo, s3Client)

	studentUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        studentUUID.String(),
		Email:         "alex@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	_ = repo.UpsertUser(context.Background(), &user.User{
		ID:    studentUUID,
		Email: claims.Email,
		Role:  "student",
	})

	t.Run("DOCX upload sets correct content-type", func(t *testing.T) {
		docxContent := []byte("docx test content")
		req, err := createMultipartRequest("/profile/student/resume", "resume", "alex_resume.docx", docxContent)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		profile, _ := repo.GetStudentProfile(context.Background(), studentUUID)
		s3Client.mu.RLock()
		docxContentType := s3Client.contentTypes[*profile.ResumeKey]
		s3Client.mu.RUnlock()
		expectedType := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		if docxContentType != expectedType {
			t.Errorf("expected docx content-type %s, got %s", expectedType, docxContentType)
		}
	})

	t.Run("Uppercase .PDF extension is accepted", func(t *testing.T) {
		pdfContent := []byte("pdf content")
		req, err := createMultipartRequest("/profile/student/resume", "resume", "UPPERCASE.PDF", pdfContent)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for .PDF, got %d. Body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("Uppercase .DOCX extension is accepted", func(t *testing.T) {
		docxContent := []byte("docx content")
		req, err := createMultipartRequest("/profile/student/resume", "resume", "DOCUMENT.DOCX", docxContent)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for .DOCX, got %d. Body: %s", rr.Code, rr.Body.String())
		}
	})
}

func TestResume_UploadSizeLimitRejection(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	_, router := setupResumeTestRouter(repo, s3Client)

	studentUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        studentUUID.String(),
		Email:         "large@mit.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	_ = repo.UpsertUser(context.Background(), &user.User{
		ID:    studentUUID,
		Email: claims.Email,
		Role:  "student",
	})

	// Exceed 5MB limit: 5MB + 1024 bytes
	largeContent := bytes.Repeat([]byte("A"), user.MaxResumeSize+1024)
	req, err := createMultipartRequest("/profile/student/resume", "resume", "too_large.pdf", largeContent)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req = req.WithContext(auth.WithUserContext(req.Context(), claims))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for file > 5MB, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var env responseEnvelope
	if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if env.Success || env.Error == nil || env.Error.Code != "FILE_TOO_LARGE" {
		t.Fatalf("expected FILE_TOO_LARGE error code, got: %+v", env)
	}
}

func TestResume_UploadInvalidFileType(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	_, router := setupResumeTestRouter(repo, s3Client)

	studentUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        studentUUID.String(),
		Email:         "student@mit.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	_ = repo.UpsertUser(context.Background(), &user.User{
		ID:    studentUUID,
		Email: claims.Email,
		Role:  "student",
	})

	disallowedFiles := []string{
		"malicious.exe",
		"avatar.png",
		"photo.jpg",
		"script.sh",
		"archive.zip",
		"doc.txt",
	}

	for _, filename := range disallowedFiles {
		t.Run("Rejects "+filename, func(t *testing.T) {
			req, err := createMultipartRequest("/profile/student/resume", "resume", filename, []byte("content"))
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req = req.WithContext(auth.WithUserContext(req.Context(), claims))
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 Bad Request for %s, got %d. Body: %s", filename, rr.Code, rr.Body.String())
			}

			var env responseEnvelope
			_ = json.NewDecoder(rr.Body).Decode(&env)
			if env.Error == nil || env.Error.Code != "INVALID_FILE_TYPE" {
				t.Errorf("expected INVALID_FILE_TYPE, got: %+v", env.Error)
			}
		})
	}
}

func TestResume_UploadValidationEdgeCases(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	_, router := setupResumeTestRouter(repo, s3Client)

	studentUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        studentUUID.String(),
		Email:         "valid@univ.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	_ = repo.UpsertUser(context.Background(), &user.User{
		ID:    studentUUID,
		Email: claims.Email,
		Role:  "student",
	})

	t.Run("Missing resume form field returns 400 BAD_REQUEST", func(t *testing.T) {
		req, err := createMultipartRequest("/profile/student/resume", "other_field", "file.pdf", []byte("data"))
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", rr.Code)
		}

		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if env.Error == nil || env.Error.Code != "BAD_REQUEST" {
			t.Errorf("expected BAD_REQUEST, got %+v", env.Error)
		}
	})

	t.Run("Empty 0-byte file returns 400 BAD_REQUEST", func(t *testing.T) {
		req, err := createMultipartRequest("/profile/student/resume", "resume", "empty.pdf", []byte{})
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for 0-byte file, got %d", rr.Code)
		}

		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if env.Error == nil || env.Error.Code != "BAD_REQUEST" {
			t.Errorf("expected BAD_REQUEST, got %+v", env.Error)
		}
	})
}

func TestResume_UploadEmailVerificationGate(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	_, router := setupResumeTestRouter(repo, s3Client)

	studentUUID := uuid.New()
	// Unverified student email (email_verified: false)
	claims := &auth.UserClaims{
		UserID:        studentUUID.String(),
		Email:         "unverified@mit.edu",
		EmailVerified: false,
		Roles:         []string{"student"},
	}

	_ = repo.UpsertUser(context.Background(), &user.User{
		ID:    studentUUID,
		Email: claims.Email,
		Role:  "student",
	})

	req, err := createMultipartRequest("/profile/student/resume", "resume", "resume.pdf", []byte("pdf content"))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req = req.WithContext(auth.WithUserContext(req.Context(), claims))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for unverified student email, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var env responseEnvelope
	_ = json.NewDecoder(rr.Body).Decode(&env)
	if env.Error == nil || env.Error.Code != "EMAIL_NOT_VERIFIED" {
		t.Errorf("expected EMAIL_NOT_VERIFIED, got %+v", env.Error)
	}
}

func TestResume_UploadRoleAuthorization(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	_, router := setupResumeTestRouter(repo, s3Client)

	employerUUID := uuid.New()
	employerClaims := &auth.UserClaims{
		UserID:        employerUUID.String(),
		Email:         "boss@corp.com",
		EmailVerified: true,
		Roles:         []string{"employer"},
	}

	_ = repo.UpsertUser(context.Background(), &user.User{
		ID:    employerUUID,
		Email: employerClaims.Email,
		Role:  "employer",
	})

	req, err := createMultipartRequest("/profile/student/resume", "resume", "resume.pdf", []byte("pdf content"))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req = req.WithContext(auth.WithUserContext(req.Context(), employerClaims))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when employer attempts to upload student resume, got %d", rr.Code)
	}

	var env responseEnvelope
	_ = json.NewDecoder(rr.Body).Decode(&env)
	if env.Error == nil || env.Error.Code != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN code, got %+v", env.Error)
	}
}

func TestResume_UploadErrors(t *testing.T) {
	studentUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        studentUUID.String(),
		Email:         "error_test@univ.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	t.Run("Storage failure returns 500 STORAGE_ERROR", func(t *testing.T) {
		repo := newMockUserRepository()
		s3Client := newMockStorageClient()
		s3Client.errUpload = errors.New("simulated S3 connection timeout")

		_, router := setupResumeTestRouter(repo, s3Client)
		_ = repo.UpsertUser(context.Background(), &user.User{
			ID:    studentUUID,
			Email: claims.Email,
			Role:  "student",
		})

		req, _ := createMultipartRequest("/profile/student/resume", "resume", "resume.pdf", []byte("pdf content"))
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 Internal Server Error, got %d", rr.Code)
		}

		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if env.Error == nil || env.Error.Code != "STORAGE_ERROR" {
			t.Errorf("expected STORAGE_ERROR, got %+v", env.Error)
		}
	})

	t.Run("Database update failure returns 500 DB_ERROR", func(t *testing.T) {
		repo := newMockUserRepository()
		repo.errUpdateStudentResume = errors.New("simulated DB constraint failure")
		s3Client := newMockStorageClient()

		_, router := setupResumeTestRouter(repo, s3Client)
		_ = repo.UpsertUser(context.Background(), &user.User{
			ID:    studentUUID,
			Email: claims.Email,
			Role:  "student",
		})

		req, _ := createMultipartRequest("/profile/student/resume", "resume", "resume.pdf", []byte("pdf content"))
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 Internal Server Error, got %d", rr.Code)
		}

		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if env.Error == nil || env.Error.Code != "DB_ERROR" {
			t.Errorf("expected DB_ERROR, got %+v", env.Error)
		}
	})
}

func TestResume_GetMyResumeURL(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	_, router := setupResumeTestRouter(repo, s3Client)

	studentUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        studentUUID.String(),
		Email:         "student@harvard.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	_ = repo.UpsertUser(context.Background(), &user.User{
		ID:    studentUUID,
		Email: claims.Email,
		Role:  "student",
	})

	t.Run("Missing resume returns 404 RESUME_NOT_FOUND", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/profile/student/resume", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if env.Error == nil || env.Error.Code != "RESUME_NOT_FOUND" {
			t.Errorf("expected RESUME_NOT_FOUND, got %+v", env.Error)
		}
	})

	t.Run("Happy path returns 200 OK with download_url and filename", func(t *testing.T) {
		resumeKey := "resumes/" + studentUUID.String() + "/uuid-my_cv.pdf"
		_ = repo.UpdateStudentResume(context.Background(), studentUUID, resumeKey, "my_cv.pdf", 2048)

		req := httptest.NewRequest("GET", "/profile/student/resume", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var env responseEnvelope
		if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !env.Success {
			t.Fatalf("expected success: true, got %+v", env)
		}

		var data map[string]string
		_ = json.Unmarshal(env.Data, &data)
		if data["download_url"] == "" {
			t.Errorf("expected non-empty download_url")
		}
		if data["filename"] != "my_cv.pdf" {
			t.Errorf("expected filename my_cv.pdf, got %s", data["filename"])
		}
	})

	t.Run("Employer calling student's own resume endpoint returns 403 FORBIDDEN", func(t *testing.T) {
		employerClaims := &auth.UserClaims{
			UserID: uuid.New().String(),
			Email:  "emp@inc.com",
			Roles:  []string{"employer"},
		}
		req := httptest.NewRequest("GET", "/profile/student/resume", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), employerClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", rr.Code)
		}
	})
}

func TestResume_GetStudentResumeURL(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	_, router := setupResumeTestRouter(repo, s3Client)

	studentUUID := uuid.New()
	studentClaims := &auth.UserClaims{
		UserID:        studentUUID.String(),
		Email:         "target@univ.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	_ = repo.UpsertUser(context.Background(), &user.User{
		ID:    studentUUID,
		Email: studentClaims.Email,
		Role:  "student",
	})
	resumeKey := "resumes/" + studentUUID.String() + "/uuid-target_resume.pdf"
	_ = repo.UpdateStudentResume(context.Background(), studentUUID, resumeKey, "target_resume.pdf", 4096)

	employerUUID := uuid.New()
	employerClaims := &auth.UserClaims{
		UserID: employerUUID.String(),
		Email:  "recruiter@company.com",
		Roles:  []string{"employer"},
	}
	_ = repo.UpsertUser(context.Background(), &user.User{
		ID:    employerUUID,
		Email: employerClaims.Email,
		Role:  "employer",
	})

	adminClaims := &auth.UserClaims{
		UserID: uuid.New().String(),
		Email:  "admin@lynk.internal",
		Roles:  []string{"admin"},
	}

	otherStudentClaims := &auth.UserClaims{
		UserID: uuid.New().String(),
		Email:  "peer@univ.edu",
		Roles:  []string{"student"},
	}

	t.Run("Employer can retrieve student resume by ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/profile/student/"+studentUUID.String()+"/resume", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), employerClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for employer, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		var data map[string]string
		_ = json.Unmarshal(env.Data, &data)
		if data["download_url"] == "" || data["filename"] != "target_resume.pdf" {
			t.Errorf("unexpected response data: %+v", data)
		}
	})

	t.Run("Admin can retrieve student resume by ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/profile/student/"+studentUUID.String()+"/resume", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), adminClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for admin, got %d. Body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("Student owner can retrieve their own resume by ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/profile/student/"+studentUUID.String()+"/resume", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), studentClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for owner student, got %d. Body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("Different student is forbidden from viewing other student's resume", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/profile/student/"+studentUUID.String()+"/resume", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), otherStudentClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for different student, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if env.Error == nil || env.Error.Code != "FORBIDDEN" {
			t.Errorf("expected FORBIDDEN code, got %+v", env.Error)
		}
	})

	t.Run("Non-existent student profile returns 404 PROFILE_NOT_FOUND", func(t *testing.T) {
		missingUUID := uuid.New()
		req := httptest.NewRequest("GET", "/profile/student/"+missingUUID.String()+"/resume", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), employerClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rr.Code)
		}
	})

	t.Run("Invalid student ID returns 400 BAD_REQUEST", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/profile/student/invalid-uuid/resume", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), employerClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rr.Code)
		}
	})
}
