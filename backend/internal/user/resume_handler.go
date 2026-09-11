package user

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httputil"
	"github.com/lynk/backend/internal/storage"
)

const (
	// MaxResumeSize defines maximum allowed resume file size (5MB).
	MaxResumeSize = 5 * 1024 * 1024
	// PresignedURLExpiry defines the expiration duration for generated presigned download URLs (15 minutes).
	PresignedURLExpiry = 15 * time.Minute
	// MaxMultipartMemory defines the memory buffer before spilling to disk (64KB).
	MaxMultipartMemory = 64 * 1024
)

// UploadResume handles POST /api/v1/profile/resume.
// Enforces 5MB max body size (64KB memory threshold), parses multipart payload,
// and delegates business logic, validation, S3 upload, and metadata persistence to user.Service.
func (h *Handler) UploadResume(_ ...storage.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.service == nil {
			httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "User service not configured", nil)
			return
		}

		claims, err := auth.GetUserContext(r.Context())
		if err != nil || claims == nil || claims.UserID == "" {
			httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
			return
		}

		// Max request body size: 5MB payload with 1MB multipart framing headroom
		const maxMultipartBody = MaxResumeSize + 1024*1024
		r.Body = http.MaxBytesReader(w, r.Body, maxMultipartBody)
		if err := r.ParseMultipartForm(MaxMultipartMemory); err != nil {
			httputil.WriteError(w, r, http.StatusBadRequest, "FILE_TOO_LARGE", "Resume must not exceed 5MB", err)
			return
		}
		defer func() {
			if r.MultipartForm != nil {
				_ = r.MultipartForm.RemoveAll()
			}
		}()

		file, header, err := r.FormFile("resume")
		if err != nil {
			httputil.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Form field 'resume' is required", err)
			return
		}
		defer func() { _ = file.Close() }()

		profile, err := h.service.UploadResume(r.Context(), claims, header.Filename, header.Size, header.Header.Get("Content-Type"), file)
		if err != nil {
			if errors.Is(err, ErrUnauthorized) {
				httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
				return
			}
			if errors.Is(err, ErrForbidden) || errors.Is(err, auth.ErrEmailNotVerified) {
				httputil.WriteError(w, r, http.StatusForbidden, "EMAIL_NOT_VERIFIED", auth.CampusVerificationPendingMsg, nil)
				return
			}
			if errors.Is(err, ErrFileTooLarge) {
				httputil.WriteError(w, r, http.StatusBadRequest, "FILE_TOO_LARGE", "Resume must not exceed 5MB", err)
				return
			}
			if errors.Is(err, ErrInvalidFileType) {
				httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_FILE_TYPE", "Resume must be a PDF or DOCX file", err)
				return
			}
			if errors.Is(err, ErrInvalidInput) {
				httputil.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), err)
				return
			}
			if errors.Is(err, ErrStorageNotConfigured) {
				httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Storage client not configured", err)
				return
			}
			httputil.WriteError(w, r, http.StatusInternalServerError, "STORAGE_ERROR", "Failed to upload resume to storage", err)
			return
		}

		var key string
		if profile != nil && profile.ResumeKey != nil {
			key = *profile.ResumeKey
		}
		var filename string
		if profile != nil && profile.ResumeFilename != nil {
			filename = *profile.ResumeFilename
		}

		httputil.WriteSuccess(w, http.StatusOK, map[string]interface{}{
			"message":    "Resume uploaded successfully",
			"filename":   filename,
			"byte_size":  header.Size,
			"resume_key": key,
		})
	}
}

// GetResume handles GET /api/v1/profile/resume and GET /api/v1/profile/{id}/resume.
// If {id} URL parameter is provided, it retrieves that member's resume download URL;
// otherwise, it retrieves the authenticated caller's own resume download URL.
func (h *Handler) GetResume(_ ...storage.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.service == nil {
			httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "User service not configured", nil)
			return
		}

		claims, err := auth.GetUserContext(r.Context())
		if err != nil || claims == nil || claims.UserID == "" {
			httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
			return
		}

		targetUserID := chi.URLParam(r, "id")
		if targetUserID == "" {
			targetUserID = claims.UserID
		}

		result, err := h.service.GetResumeDownloadResult(r.Context(), claims, targetUserID)
		if err != nil {
			h.handleResumeDownloadError(w, r, err)
			return
		}

		httputil.WriteSuccess(w, http.StatusOK, map[string]string{
			"download_url": result.DownloadURL,
			"url":          result.DownloadURL,
			"filename":     result.Filename,
		})
	}
}

// GetMyResumeURL handles GET /api/v1/profile/resume.
func (h *Handler) GetMyResumeURL(s3Client ...storage.Client) http.HandlerFunc {
	return h.GetResume(s3Client...)
}

// GetResumeURL is an alias for GetMyResumeURL.
func (h *Handler) GetResumeURL(s3Client ...storage.Client) http.HandlerFunc {
	return h.GetResume(s3Client...)
}

// GetMemberResumeURL handles GET /api/v1/profile/{id}/resume.
func (h *Handler) GetMemberResumeURL(s3Client ...storage.Client) http.HandlerFunc {
	return h.GetResume(s3Client...)
}

// GetStudentResumeURL is an alias for GetMemberResumeURL.
func (h *Handler) GetStudentResumeURL(s3Client ...storage.Client) http.HandlerFunc {
	return h.GetResume(s3Client...)
}

func (h *Handler) handleResumeDownloadError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrUnauthorized) {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Access denied to resume", nil)
		return
	}
	if errors.Is(err, ErrInvalidInput) {
		httputil.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error(), err)
		return
	}
	if errors.Is(err, ErrProfileNotFound) {
		httputil.WriteError(w, r, http.StatusNotFound, "PROFILE_NOT_FOUND", "Profile not found", nil)
		return
	}
	if errors.Is(err, ErrResumeNotFound) {
		httputil.WriteError(w, r, http.StatusNotFound, "RESUME_NOT_FOUND", "No resume found for this profile", nil)
		return
	}
	if errors.Is(err, ErrStorageNotConfigured) {
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Storage client not configured", err)
		return
	}
	httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve resume download URL", err)
}
