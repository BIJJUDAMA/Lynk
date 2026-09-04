package user

import (
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/storage"
)

const (
	// MaxResumeSize defines maximum allowed resume file size (5MB).
	MaxResumeSize = 5 * 1024 * 1024
	// PresignedURLExpiry defines the expiration duration for generated presigned download URLs (15 minutes).
	PresignedURLExpiry = 15 * time.Minute
)

// UploadResume handles POST /api/v1/profile/student/resume.
// Enforces 5MB max body size, validates .pdf/.docx extensions, streams directly to MinIO,
// and updates the student profile record in PostgreSQL.
func (h *Handler) UploadResume(s3Client storage.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client := s3Client
		if client == nil {
			client = h.s3Client
		}
		if client == nil {
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Storage client not configured")
			return
		}

		claims, err := auth.GetUserContext(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
			return
		}

		userUUID, err := uuid.Parse(claims.UserID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user UUID")
			return
		}

		// Email verification gate (Hard Invariant: student email must be verified)
		if !claims.EmailVerified {
			writeJSONError(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "University email must be verified before performing this action")
			return
		}

		// Affirmative student role check
		if !h.isStudent(r.Context(), claims, userUUID) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Student role required")
			return
		}

		// 1. Max request body size: 5MB payload with 1MB multipart framing headroom
		const maxMultipartBody = MaxResumeSize + 1024*1024
		r.Body = http.MaxBytesReader(w, r.Body, maxMultipartBody)
		if err := r.ParseMultipartForm(maxMultipartBody); err != nil {
			writeJSONError(w, http.StatusBadRequest, "FILE_TOO_LARGE", "Resume must not exceed 5MB")
			return
		}

		file, header, err := r.FormFile("resume")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Form field 'resume' is required")
			return
		}
		defer file.Close()

		if header.Size > MaxResumeSize {
			writeJSONError(w, http.StatusBadRequest, "FILE_TOO_LARGE", "Resume must not exceed 5MB")
			return
		}

		if header.Size <= 0 {
			writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Resume file cannot be empty")
			return
		}

		// 2. File extension: .pdf or .docx (case-insensitive)
		ext := strings.ToLower(filepath.Ext(header.Filename))
		if ext != ".pdf" && ext != ".docx" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_FILE_TYPE", "Resume must be a PDF or DOCX file")
			return
		}

		// Canonical Content-Type strictly normalized based on validated extension
		var contentType string
		if ext == ".pdf" {
			contentType = "application/pdf"
		} else if ext == ".docx" {
			contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		}

		filename := filepath.Base(header.Filename)

		// 3. Generate sanitized object key via storage.GenerateResumeKey
		key := storage.GenerateResumeKey(userUUID.String(), filename)

		// 4. Stream directly to s3Client.UploadResume
		if err := client.UploadResume(r.Context(), key, contentType, file); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "STORAGE_ERROR", "Failed to upload resume to storage")
			return
		}

		// 5. Record resume_key, resume_filename, resume_byte_size via repo.UpdateStudentResume
		if err := h.repo.UpdateStudentResume(r.Context(), userUUID, key, filename, header.Size); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to update profile resume metadata")
			return
		}

		writeJSONSuccess(w, http.StatusOK, map[string]interface{}{
			"message":     "Resume uploaded successfully",
			"filename":    filename,
			"byte_size":   header.Size,
			"resume_key":  key,
		})
	}
}

// GetMyResumeURL handles GET /api/v1/profile/student/resume.
// Retrieves the authenticated student's resume and generates a 15-minute presigned download URL.
func (h *Handler) GetMyResumeURL(s3Client storage.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client := s3Client
		if client == nil {
			client = h.s3Client
		}
		if client == nil {
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Storage client not configured")
			return
		}

		claims, err := auth.GetUserContext(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
			return
		}

		userUUID, err := uuid.Parse(claims.UserID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user UUID")
			return
		}

		if !h.isStudent(r.Context(), claims, userUUID) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Student role required")
			return
		}

		profile, err := h.repo.GetStudentProfile(r.Context(), userUUID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve student profile")
			return
		}

		if profile == nil || profile.ResumeKey == nil || *profile.ResumeKey == "" {
			writeJSONError(w, http.StatusNotFound, "RESUME_NOT_FOUND", "No resume found for this profile")
			return
		}

		url, err := client.GetPresignedDownloadURL(r.Context(), *profile.ResumeKey, PresignedURLExpiry)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "PRESIGN_ERROR", "Failed to generate presigned URL")
			return
		}

		filename := ""
		if profile.ResumeFilename != nil {
			filename = *profile.ResumeFilename
		}

		writeJSONSuccess(w, http.StatusOK, map[string]string{
			"download_url": url,
			"url":          url,
			"filename":     filename,
		})
	}
}

// GetResumeURL is an alias for GetMyResumeURL for backwards compatibility with the original plan.
func (h *Handler) GetResumeURL(s3Client storage.Client) http.HandlerFunc {
	return h.GetMyResumeURL(s3Client)
}

// GetStudentResumeURL handles GET /api/v1/profile/student/{id}/resume.
// Retrieves a student's resume by ID parameter and generates a 15-minute presigned download URL.
// Accessible to employers, admins, or the student resume owner.
func (h *Handler) GetStudentResumeURL(s3Client storage.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client := s3Client
		if client == nil {
			client = h.s3Client
		}
		if client == nil {
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Storage client not configured")
			return
		}

		claims, err := auth.GetUserContext(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
			return
		}

		callerUUID, err := uuid.Parse(claims.UserID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user UUID")
			return
		}

		idStr := chi.URLParam(r, "id")
		targetID, err := uuid.Parse(idStr)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid student ID")
			return
		}

		profile, err := h.repo.GetStudentProfileByID(r.Context(), targetID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve student profile")
			return
		}

		if profile == nil {
			writeJSONError(w, http.StatusNotFound, "PROFILE_NOT_FOUND", "Student profile not found")
			return
		}

		if profile.ResumeKey == nil || *profile.ResumeKey == "" {
			writeJSONError(w, http.StatusNotFound, "RESUME_NOT_FOUND", "No resume found for this profile")
			return
		}

		// Authorization: employer, admin, or the student owner themselves
		isOwner := profile.UserID == callerUUID || profile.ID == callerUUID
		isEmp := h.isEmployer(r.Context(), claims, callerUUID)
		isAdmin := claims.HasRole("admin")

		if !isOwner && !isEmp && !isAdmin {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Access denied: employer, admin, or profile owner role required")
			return
		}

		url, err := client.GetPresignedDownloadURL(r.Context(), *profile.ResumeKey, PresignedURLExpiry)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "PRESIGN_ERROR", "Failed to generate presigned URL")
			return
		}

		filename := ""
		if profile.ResumeFilename != nil {
			filename = *profile.ResumeFilename
		}

		writeJSONSuccess(w, http.StatusOK, map[string]string{
			"download_url": url,
			"url":          url,
			"filename":     filename,
		})
	}
}
