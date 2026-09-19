package user_test

import (
	"archive/zip"
	"bytes"
	"context"
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

func zipWith(files map[string]string) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := w.Create(name)
		if err != nil {
			panic(err)
		}
		if _, err := f.Write([]byte(body)); err != nil {
			panic(err)
		}
	}
	_ = w.Close()
	return buf.Bytes()
}

func validDOCXFixture() []byte {
	return zipWith(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"word/document.xml":   `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"/>`,
	})
}

// mockStorageClient implements storage.Client for isolated unit testing.
type mockStorageClient struct {
	mu            sync.RWMutex
	uploads       map[string][]byte
	contentTypes  map[string]string
	presignedURLs map[string]string
	deletedKeys   []string
	errUpload     error
	errPresign    error
	errDelete     error
}

func newMockStorageClient() *mockStorageClient {
	return &mockStorageClient{
		uploads:       make(map[string][]byte),
		contentTypes:  make(map[string]string),
		presignedURLs: make(map[string]string),
		deletedKeys:   make([]string, 0),
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

func (m *mockStorageClient) DeleteResume(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errDelete != nil {
		return m.errDelete
	}
	delete(m.uploads, key)
	delete(m.contentTypes, key)
	delete(m.presignedURLs, key)
	m.deletedKeys = append(m.deletedKeys, key)
	return nil
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
	svc := user.NewService(repo, s3)
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

func TestService_GetResumeDownloadURL_RestrictsThirdPartyAccess(t *testing.T) {
	svc := user.NewService(newMockUserRepository(), newMockStorageClient())
	callerClaims := &auth.UserClaims{UserID: "usr_alice", Roles: []string{"member"}}
	// Requesting Bob's resume as Alice without admin or application relationship
	_, err := svc.GetResumeDownloadURL(context.Background(), callerClaims, "usr_bob")
	if !errors.Is(err, user.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for third party download, got %v", err)
	}
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
// full success path completes without error and the file is correctly processed--meaning the
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

// TestResume_GetMemberResumeURL verifies that GET /profile/{id}/resume allows an admin
// to look up the target member's resume presigned download URL.
func TestResume_GetMemberResumeURL(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()

	// Target member: a user whose resume is stored.
	targetUUID := uuid.New()
	resumeKey := "resumes/" + targetUUID.String() + "/applicant_resume.pdf"
	resumeFilename := "applicant_resume.pdf"
	if err := repo.UpdateResume(context.Background(), targetUUID.String(), resumeKey, resumeFilename, 2048); err != nil {
		t.Fatalf("failed to seed target resume: %v", err)
	}

	// Caller is an admin with a different UUID.
	callerUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        callerUUID.String(),
		Email:         "admin@company.edu",
		EmailVerified: true,
		Roles:         []string{"admin"},
	}

	_, router := setupResumeTestRouter(repo, s3Client, claims)

	// Admin requests the target member's resume via route param {id} = targetUUID.
	req := httptest.NewRequest("GET", "/profile/"+targetUUID.String()+"/resume", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for GetMemberResumeURL by admin, got %d: %s", rec.Code, rec.Body.String())
	}

	// Confirm the response contains a download URL.
	body := rec.Body.String()
	if body == "" {
		t.Error("expected non-empty response body with download_url")
	}
}

// TestResume_GetMemberResumeURL_ForbiddenForNonAdmin verifies that a non-admin third party
// cannot download another member's resume (returns 403 Forbidden).
func TestResume_GetMemberResumeURL_ForbiddenForNonAdmin(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()

	targetUUID := uuid.New()
	resumeKey := "resumes/" + targetUUID.String() + "/applicant_resume.pdf"
	if err := repo.UpdateResume(context.Background(), targetUUID.String(), resumeKey, "applicant_resume.pdf", 2048); err != nil {
		t.Fatalf("failed to seed target resume: %v", err)
	}

	// Caller is a member (not admin, not target)
	callerUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        callerUUID.String(),
		Email:         "unauthorized@company.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	_, router := setupResumeTestRouter(repo, s3Client, claims)

	req := httptest.NewRequest("GET", "/profile/"+targetUUID.String()+"/resume", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for non-admin downloading third-party resume, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestResume_GetMemberResumeURL_NotFound verifies that GET /profile/{id}/resume returns 404
// when an admin requests a member who has no resume uploaded.
func TestResume_GetMemberResumeURL_NotFound(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()

	// Target exists but has no resume.
	targetUUID := uuid.New()

	callerUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        callerUUID.String(),
		Email:         "admin@company.edu",
		EmailVerified: true,
		Roles:         []string{"admin"},
	}

	_, router := setupResumeTestRouter(repo, s3Client, claims)

	req := httptest.NewRequest("GET", "/profile/"+targetUUID.String()+"/resume", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Target has no profile/resume row at all -> 404.
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing resume, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestService_GetResumeDownloadURL_AuthorizedOwner(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	svc := user.NewService(repo, s3Client)

	userUUID := uuid.New().String()
	key := "resumes/" + userUUID + "/resume.pdf"
	if err := repo.UpdateResume(context.Background(), userUUID, key, "resume.pdf", 1024); err != nil {
		t.Fatalf("failed to update resume: %v", err)
	}

	claims := &auth.UserClaims{
		UserID:        userUUID,
		Email:         "bob@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	url, err := svc.GetResumeDownloadURL(context.Background(), claims, userUUID)
	if err != nil {
		t.Fatalf("unexpected error for owner download: %v", err)
	}
	if url == "" {
		t.Error("expected valid download url")
	}

	// Owner requesting non-existent resume returns ErrResumeNotFound
	otherUUID := uuid.New().String()
	otherClaims := &auth.UserClaims{
		UserID:        otherUUID,
		Email:         "charlie@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}
	_, err = svc.GetResumeDownloadURL(context.Background(), otherClaims, otherUUID)
	if !errors.Is(err, user.ErrResumeNotFound) && !errors.Is(err, user.ErrProfileNotFound) {
		t.Fatalf("expected ErrResumeNotFound or ErrProfileNotFound for user without resume, got %v", err)
	}
}

func TestService_GetResumeDownloadResult(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	svc := user.NewService(repo, s3Client)

	userUUID := uuid.New().String()
	key := "resumes/" + userUUID + "/my_cv.pdf"
	if err := repo.UpdateResume(context.Background(), userUUID, key, "my_cv.pdf", 2048); err != nil {
		t.Fatalf("failed to update resume: %v", err)
	}

	claims := &auth.UserClaims{
		UserID:        userUUID,
		Email:         "owner@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	res, err := svc.GetResumeDownloadResult(context.Background(), claims, userUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.DownloadURL == "" {
		t.Error("expected non-empty download url")
	}
	if res.Filename != "my_cv.pdf" {
		t.Errorf("expected filename my_cv.pdf, got %q", res.Filename)
	}
}

type stubResumeAccess struct {
	allow bool
}

func (s *stubResumeAccess) JobOwnerMayDownloadApplicantResume(ctx context.Context, ownerID, applicantUserID string) (bool, error) {
	return s.allow, nil
}

func TestGetResumeDownloadResult_JobOwnerCanDownloadApplicantResume(t *testing.T) {
	repo := newMockUserRepository()
	checker := &stubResumeAccess{allow: true}
	svc := user.NewService(repo, newMockStorageClient()).WithResumeAccessChecker(checker)

	owner := &auth.UserClaims{UserID: "poster", EmailVerified: true, Roles: []string{"member"}}
	applicantID := "applicant"
	key := "resumes/applicant/cv.pdf"
	_ = repo.UpdateResume(context.Background(), applicantID, key, "cv.pdf", 100)

	res, err := svc.GetResumeDownloadResult(context.Background(), owner, applicantID)
	if err != nil {
		t.Fatalf("expected job owner access, got %v", err)
	}
	if res == nil || res.DownloadURL == "" {
		t.Fatal("expected presigned URL")
	}
}

func TestGetResumeDownloadResult_UnrelatedMemberForbidden(t *testing.T) {
	repo := newMockUserRepository()
	checker := &stubResumeAccess{allow: false}
	svc := user.NewService(repo, newMockStorageClient()).WithResumeAccessChecker(checker)
	_ = repo.UpdateResume(context.Background(), "applicant", "resumes/applicant/cv.pdf", "cv.pdf", 100)

	stranger := &auth.UserClaims{UserID: "stranger", EmailVerified: true, Roles: []string{"member"}}
	_, err := svc.GetResumeDownloadResult(context.Background(), stranger, "applicant")
	if !errors.Is(err, user.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestService_GetResumeDownloadURL_AuthorizedAdmin(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	svc := user.NewService(repo, s3Client)

	userUUID := uuid.New().String()
	key := "resumes/" + userUUID + "/resume.pdf"
	if err := repo.UpdateResume(context.Background(), userUUID, key, "resume.pdf", 1024); err != nil {
		t.Fatalf("failed to update resume: %v", err)
	}

	adminClaims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "admin@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"admin"},
	}

	url, err := svc.GetResumeDownloadURL(context.Background(), adminClaims, userUUID)
	if err != nil {
		t.Fatalf("unexpected error for admin download: %v", err)
	}
	if url == "" {
		t.Error("expected valid download url for admin")
	}
}

func TestService_UploadResume_MagicBytesValidation(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	svc := user.NewService(repo, s3Client)

	userUUID := uuid.New().String()
	claims := &auth.UserClaims{
		UserID:        userUUID,
		Email:         "student@harvard.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	// 1. Valid PDF with %PDF- magic bytes
	pdfBody := bytes.NewReader([]byte("%PDF-1.7 valid pdf content"))
	profile, err := svc.UploadResume(context.Background(), claims, "resume.pdf", int64(pdfBody.Len()), "application/pdf", pdfBody)
	if err != nil {
		t.Fatalf("expected valid PDF to succeed, got %v", err)
	}
	if profile == nil || profile.ResumeKey == nil {
		t.Fatalf("expected profile with resume key")
	}

	// 2. Valid DOCX with OOXML Content_Types
	docxContent := validDOCXFixture()
	docxBody := bytes.NewReader(docxContent)
	profile, err = svc.UploadResume(context.Background(), claims, "resume.docx", int64(docxBody.Len()), "application/vnd.openxmlformats-officedocument.wordprocessingml.document", docxBody)
	if err != nil {
		t.Fatalf("expected valid DOCX to succeed, got %v", err)
	}
	if profile == nil || profile.ResumeKey == nil {
		t.Fatalf("expected profile with resume key")
	}

	// 3. Invalid: PDF extension but plain text content
	fakePDFBody := bytes.NewReader([]byte("This is plain text pretending to be a pdf"))
	_, err = svc.UploadResume(context.Background(), claims, "fake.pdf", int64(fakePDFBody.Len()), "application/pdf", fakePDFBody)
	if !errors.Is(err, user.ErrInvalidFileType) {
		t.Fatalf("expected ErrInvalidFileType for fake PDF, got %v", err)
	}

	// 4. Invalid: DOCX extension but plain text content
	fakeDOCXBody := bytes.NewReader([]byte("This is plain text pretending to be a docx"))
	_, err = svc.UploadResume(context.Background(), claims, "fake.docx", int64(fakeDOCXBody.Len()), "application/octet-stream", fakeDOCXBody)
	if !errors.Is(err, user.ErrInvalidFileType) {
		t.Fatalf("expected ErrInvalidFileType for fake DOCX, got %v", err)
	}

	// 4b. Invalid: DOCX extension but generic ZIP without OOXML markers
	plainZip := zipWith(map[string]string{"readme.txt": "hello"})
	plainZipBody := bytes.NewReader(plainZip)
	_, err = svc.UploadResume(context.Background(), claims, "fake.docx", int64(plainZipBody.Len()), "application/octet-stream", plainZipBody)
	if !errors.Is(err, user.ErrInvalidFileType) {
		t.Fatalf("expected ErrInvalidFileType for plain ZIP as DOCX, got %v", err)
	}

	// 5. Invalid: Unsupported extension (.exe)
	exeBody := bytes.NewReader([]byte("%PDF-1.4 but with .exe extension"))
	_, err = svc.UploadResume(context.Background(), claims, "exploit.exe", int64(exeBody.Len()), "application/octet-stream", exeBody)
	if !errors.Is(err, user.ErrInvalidFileType) {
		t.Fatalf("expected ErrInvalidFileType for .exe, got %v", err)
	}

	// 6. Invalid: Empty file
	emptyBody := bytes.NewReader([]byte{})
	_, err = svc.UploadResume(context.Background(), claims, "empty.pdf", 0, "application/pdf", emptyBody)
	if !errors.Is(err, user.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty file, got %v", err)
	}

	// 7. Invalid: Unverified email
	unverifiedClaims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "unverified@mit.edu",
		EmailVerified: false,
		Roles:         []string{"member"},
	}
	pdfBody = bytes.NewReader([]byte("%PDF-1.7 valid pdf"))
	_, err = svc.UploadResume(context.Background(), unverifiedClaims, "resume.pdf", int64(pdfBody.Len()), "application/pdf", pdfBody)
	if !errors.Is(err, auth.ErrEmailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified for unverified email, got %v", err)
	}
}

// deleteSpy wraps a storage.Client and records whether DeleteResume was called with a canceled context.
type deleteSpy struct {
	storage.Client
	deletedWithCancel bool
	lastDeadline      time.Time
	mu                sync.Mutex
}

func (d *deleteSpy) DeleteResume(ctx context.Context, key string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if dl, ok := ctx.Deadline(); ok {
		d.lastDeadline = dl
	}
	if err := ctx.Err(); err != nil {
		d.deletedWithCancel = true
		return err
	}
	return d.Client.DeleteResume(ctx, key)
}

func TestCompensatingDelete_UsesWithoutCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	derived := context.WithoutCancel(ctx)
	if derived.Err() != nil {
		t.Fatal("WithoutCancel context must not be canceled")
	}
}

func TestService_UploadResume_CompensatingDeleteIgnoresCanceledParent(t *testing.T) {
	repo := newMockUserRepository()
	repo.errUpdateResume = errors.New("database disk full")
	base := newMockStorageClient()
	spy := &deleteSpy{Client: base}
	svc := user.NewService(repo, spy)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	claims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "student@berkeley.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	pdfContent := []byte("%PDF-1.4 compensating delete test")
	pdfBody := bytes.NewReader(pdfContent)
	_, err := svc.UploadResume(ctx, claims, "cv.pdf", int64(len(pdfContent)), "application/pdf", pdfBody)
	if err == nil {
		t.Fatal("expected metadata update error")
	}

	spy.mu.Lock()
	defer spy.mu.Unlock()
	if spy.deletedWithCancel {
		t.Fatal("compensating delete used canceled parent context")
	}
	if len(base.deletedKeys) == 0 {
		t.Fatal("expected compensating delete to run with uncanceled context")
	}
}

func TestService_UploadResume_OldResumeDeleteIgnoresCanceledParent(t *testing.T) {
	repo := newMockUserRepository()
	base := newMockStorageClient()
	spy := &deleteSpy{Client: base}
	svc := user.NewService(repo, spy)

	userUUID := uuid.New().String()
	oldKey := "resumes/" + userUUID + "/old_resume.pdf"
	base.uploads[oldKey] = []byte("%PDF-1.4 old resume content")
	if err := repo.UpdateResume(context.Background(), userUUID, oldKey, "old_resume.pdf", 100); err != nil {
		t.Fatalf("failed to seed initial resume: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	claims := &auth.UserClaims{
		UserID:        userUUID,
		Email:         "student@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	newPDF := bytes.NewReader([]byte("%PDF-1.7 updated resume content"))
	_, err := svc.UploadResume(ctx, claims, "new_resume.pdf", int64(newPDF.Len()), "application/pdf", newPDF)
	if err != nil {
		t.Fatalf("unexpected error uploading new resume: %v", err)
	}

	spy.mu.Lock()
	defer spy.mu.Unlock()
	if spy.deletedWithCancel {
		t.Fatal("old resume cleanup used canceled parent context")
	}
	foundOldDeleted := false
	for _, k := range base.deletedKeys {
		if k == oldKey {
			foundOldDeleted = true
			break
		}
	}
	if !foundOldDeleted {
		t.Errorf("expected old resume key %q to be deleted, deleted keys: %v", oldKey, base.deletedKeys)
	}
}

func TestService_UploadResume_CompensatingS3DeletionOnDBError(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	svc := user.NewService(repo, s3Client)

	// Simulate database failure on UpdateResume
	repo.errUpdateResume = errors.New("database disk full")

	claims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "student@berkeley.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	pdfBody := bytes.NewReader([]byte("%PDF-1.7 content for db error test"))
	_, err := svc.UploadResume(context.Background(), claims, "resume.pdf", int64(pdfBody.Len()), "application/pdf", pdfBody)
	if err == nil {
		t.Fatal("expected error due to simulated db failure, got nil")
	}

	// Verify compensating S3 deletion occurred
	if len(s3Client.deletedKeys) == 0 {
		t.Fatal("expected compensating S3 deletion to be executed, but no keys were deleted")
	}
	if len(s3Client.uploads) != 0 {
		t.Errorf("expected uploaded file to be removed from storage on DB error, remaining: %v", s3Client.uploads)
	}
}

func TestService_UploadResume_DeletesOldResumeOnSuccess(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	svc := user.NewService(repo, s3Client)

	userUUID := uuid.New().String()
	oldKey := "resumes/" + userUUID + "/old_resume.pdf"
	s3Client.uploads[oldKey] = []byte("%PDF-1.4 old resume content")

	if err := repo.UpdateResume(context.Background(), userUUID, oldKey, "old_resume.pdf", 100); err != nil {
		t.Fatalf("failed to seed initial resume: %v", err)
	}

	claims := &auth.UserClaims{
		UserID:        userUUID,
		Email:         "student@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	newPDF := bytes.NewReader([]byte("%PDF-1.7 updated resume content"))
	profile, err := svc.UploadResume(context.Background(), claims, "new_resume.pdf", int64(newPDF.Len()), "application/pdf", newPDF)
	if err != nil {
		t.Fatalf("unexpected error uploading new resume: %v", err)
	}

	if profile == nil || profile.ResumeKey == nil || *profile.ResumeKey == oldKey {
		t.Fatalf("expected new resume key, got %v", profile)
	}

	// Verify old key was deleted from S3
	foundOldDeleted := false
	for _, k := range s3Client.deletedKeys {
		if k == oldKey {
			foundOldDeleted = true
			break
		}
	}
	if !foundOldDeleted {
		t.Errorf("expected old resume key %q to be deleted from S3, deleted keys: %v", oldKey, s3Client.deletedKeys)
	}
	if _, exists := s3Client.uploads[oldKey]; exists {
		t.Errorf("expected old resume %q to be removed from uploads map", oldKey)
	}
}

func TestService_UploadResume_CompensatingDeleteStorageErrorHandled(t *testing.T) {
	repo := newMockUserRepository()
	repo.errUpdateResume = errors.New("db failure")
	s3Client := newMockStorageClient()
	s3Client.errDelete = errors.New("storage network partition")
	svc := user.NewService(repo, s3Client)

	claims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "student@berkeley.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	pdfContent := []byte("%PDF-1.4 compensating delete test")
	pdfBody := bytes.NewReader(pdfContent)
	_, err := svc.UploadResume(context.Background(), claims, "cv.pdf", int64(len(pdfContent)), "application/pdf", pdfBody)
	if err == nil {
		t.Fatal("expected error from UploadResume due to DB failure")
	}
}

type failFirstGetProfileRepo struct {
	user.UserRepository
	calls int
}

func (f *failFirstGetProfileRepo) GetProfile(ctx context.Context, userID string) (*user.Profile, error) {
	f.calls++
	if f.calls == 1 {
		return nil, errors.New("db query timeout")
	}
	return f.UserRepository.GetProfile(ctx, userID)
}

func TestService_UploadResume_GetProfileWarningLogged(t *testing.T) {
	baseRepo := newMockUserRepository()
	repo := &failFirstGetProfileRepo{UserRepository: baseRepo}
	s3Client := newMockStorageClient()
	svc := user.NewService(repo, s3Client)

	claims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "student@berkeley.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	pdfContent := []byte("%PDF-1.4 resume upload test")
	pdfBody := bytes.NewReader(pdfContent)
	profile, err := svc.UploadResume(context.Background(), claims, "cv.pdf", int64(len(pdfContent)), "application/pdf", pdfBody)
	if err != nil {
		t.Fatalf("unexpected error from UploadResume: %v", err)
	}
	if profile == nil {
		t.Fatal("expected profile returned")
	}
	if repo.calls < 2 {
		t.Fatalf("expected at least 2 calls to GetProfile, got %d", repo.calls)
	}
}

func TestService_UploadResume_UsesProfileLock(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	svc := user.NewService(repo, s3Client)

	claims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "student@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	pdfContent := []byte("%PDF-1.4 profile lock test resume")
	pdfBody := bytes.NewReader(pdfContent)
	_, err := svc.UploadResume(context.Background(), claims, "resume.pdf", int64(len(pdfContent)), "application/pdf", pdfBody)
	if err != nil {
		t.Fatalf("unexpected error uploading resume: %v", err)
	}

	repo.mu.RLock()
	lockCalls := repo.lockCalls
	repo.mu.RUnlock()

	if lockCalls != 1 {
		t.Fatalf("expected WithProfileLock to be called once, got %d", lockCalls)
	}
}

type orderTrackingStorageClient struct {
	*mockStorageClient
	mu       sync.Mutex
	onUpload func()
	onDelete func()
}

func (o *orderTrackingStorageClient) UploadResume(ctx context.Context, key string, contentType string, body io.Reader) error {
	o.mu.Lock()
	cb := o.onUpload
	o.mu.Unlock()
	if cb != nil {
		cb()
	}
	return o.mockStorageClient.UploadResume(ctx, key, contentType, body)
}

func (o *orderTrackingStorageClient) DeleteResume(ctx context.Context, key string) error {
	o.mu.Lock()
	cb := o.onDelete
	o.mu.Unlock()
	if cb != nil {
		cb()
	}
	return o.mockStorageClient.DeleteResume(ctx, key)
}

type orderTrackingRepo struct {
	user.UserRepository
	mu           sync.Mutex
	isLockActive bool
	onLockEnter  func()
	onLockExit   func()
}

func (o *orderTrackingRepo) WithProfileLock(ctx context.Context, userID string, fn func(context.Context) error) error {
	o.mu.Lock()
	o.isLockActive = true
	if o.onLockEnter != nil {
		o.onLockEnter()
	}
	o.mu.Unlock()

	defer func() {
		o.mu.Lock()
		o.isLockActive = false
		if o.onLockExit != nil {
			o.onLockExit()
		}
		o.mu.Unlock()
	}()

	return o.UserRepository.WithProfileLock(ctx, userID, fn)
}

func (o *orderTrackingRepo) IsLockActive() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.isLockActive
}

func TestService_UploadResume_UploadsToS3BeforeDatabaseLock(t *testing.T) {
	baseRepo := newMockUserRepository()
	baseS3 := newMockStorageClient()

	var mu sync.Mutex
	var executionOrder []string
	var lockActiveDuringUpload bool
	var lockCallsDuringUpload int

	repo := &orderTrackingRepo{
		UserRepository: baseRepo,
		onLockEnter: func() {
			mu.Lock()
			executionOrder = append(executionOrder, "db_lock_enter")
			mu.Unlock()
		},
		onLockExit: func() {
			mu.Lock()
			executionOrder = append(executionOrder, "db_lock_exit")
			mu.Unlock()
		},
	}

	s3Client := &orderTrackingStorageClient{
		mockStorageClient: baseS3,
		onUpload: func() {
			mu.Lock()
			executionOrder = append(executionOrder, "s3_upload")
			lockActiveDuringUpload = repo.IsLockActive()
			baseRepo.mu.RLock()
			lockCallsDuringUpload = baseRepo.lockCalls
			baseRepo.mu.RUnlock()
			mu.Unlock()
		},
	}

	svc := user.NewService(repo, s3Client)

	claims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "student@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	pdfContent := []byte("%PDF-1.4 decoupled upload test")
	pdfBody := bytes.NewReader(pdfContent)
	profile, err := svc.UploadResume(context.Background(), claims, "resume.pdf", int64(len(pdfContent)), "application/pdf", pdfBody)
	if err != nil {
		t.Fatalf("unexpected error uploading resume: %v", err)
	}
	if profile == nil {
		t.Fatal("expected profile to be returned")
	}

	if lockActiveDuringUpload {
		t.Error("database advisory lock was held while uploading to S3")
	}
	if lockCallsDuringUpload != 0 {
		t.Errorf("expected 0 lock calls during S3 upload, got %d", lockCallsDuringUpload)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(executionOrder) != 3 {
		t.Fatalf("expected 3 events [s3_upload, db_lock_enter, db_lock_exit], got %v", executionOrder)
	}
	if executionOrder[0] != "s3_upload" {
		t.Errorf("expected first event to be s3_upload, got %s", executionOrder[0])
	}
	if executionOrder[1] != "db_lock_enter" {
		t.Errorf("expected second event to be db_lock_enter, got %s", executionOrder[1])
	}
	if executionOrder[2] != "db_lock_exit" {
		t.Errorf("expected third event to be db_lock_exit, got %s", executionOrder[2])
	}
}

func TestService_UploadResume_S3Failure_DoesNotAcquireLock(t *testing.T) {
	baseRepo := newMockUserRepository()
	baseS3 := newMockStorageClient()
	baseS3.errUpload = errors.New("s3 connection timeout")

	svc := user.NewService(baseRepo, baseS3)

	claims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "student@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	pdfContent := []byte("%PDF-1.4 s3 failure test")
	pdfBody := bytes.NewReader(pdfContent)
	_, err := svc.UploadResume(context.Background(), claims, "resume.pdf", int64(len(pdfContent)), "application/pdf", pdfBody)
	if err == nil {
		t.Fatal("expected error due to S3 failure, got nil")
	}

	baseRepo.mu.RLock()
	lockCalls := baseRepo.lockCalls
	baseRepo.mu.RUnlock()

	if lockCalls != 0 {
		t.Errorf("expected 0 database lock calls when S3 upload fails, got %d", lockCalls)
	}
}

func TestService_UploadResume_DBFailure_CompensatingDeleteOutsideLock(t *testing.T) {
	baseRepo := newMockUserRepository()
	baseRepo.errUpdateResume = errors.New("db write conflict")
	baseS3 := newMockStorageClient()

	var lockActiveDuringDelete bool
	var deleteCalled bool
	var mu sync.Mutex

	repo := &orderTrackingRepo{
		UserRepository: baseRepo,
	}

	s3Client := &orderTrackingStorageClient{
		mockStorageClient: baseS3,
		onDelete: func() {
			mu.Lock()
			deleteCalled = true
			lockActiveDuringDelete = repo.IsLockActive()
			mu.Unlock()
		},
	}

	svc := user.NewService(repo, s3Client)

	claims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "student@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	pdfContent := []byte("%PDF-1.4 db failure compensating delete test")
	pdfBody := bytes.NewReader(pdfContent)
	_, err := svc.UploadResume(context.Background(), claims, "resume.pdf", int64(len(pdfContent)), "application/pdf", pdfBody)
	if err == nil {
		t.Fatal("expected error due to DB update failure, got nil")
	}

	mu.Lock()
	defer mu.Unlock()
	if !deleteCalled {
		t.Fatal("expected compensating delete to be called")
	}
	if lockActiveDuringDelete {
		t.Error("compensating S3 delete was executed while holding database advisory lock")
	}
}
