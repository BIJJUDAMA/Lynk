package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lynk/backend/internal/ai/models"
	"github.com/lynk/backend/internal/ai/orchestrator"
	"github.com/lynk/backend/internal/application"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httputil"
	"github.com/lynk/backend/internal/job"
	"github.com/lynk/backend/internal/middleware"
	"github.com/lynk/backend/internal/review"
	"github.com/lynk/backend/internal/user"
)

// ApplicationRepository defines the subset of application repository operations needed by AIHandler.
type ApplicationRepository interface {
	ListApplicationsByJob(ctx context.Context, jobID uuid.UUID, limit, offset int) ([]*application.ApplicationWithDetails, error)
}

// JobSearchResult represents a job with associated AI relevance score and matched skills.
type JobSearchResult struct {
	*job.Job
	Score         float64  `json:"score,omitempty"`
	Snippet       string   `json:"snippet,omitempty"`
	MatchedSkills []string `json:"matched_skills,omitempty"`
}

// PeopleSearchResult represents a member profile with AI relevance score and matched skills.
type PeopleSearchResult struct {
	*user.Profile
	Score         float64  `json:"score,omitempty"`
	Snippet       string   `json:"snippet,omitempty"`
	MatchedSkills []string `json:"matched_skills,omitempty"`
}

// AIHandler handles AI-powered search, recommendations, and analytics endpoints.
type AIHandler struct {
	orchestrator *orchestrator.Orchestrator
	db           *pgxpool.Pool
	jobRepo      job.JobRepository
	userRepo     user.UserRepository
	appRepo      ApplicationRepository
	reviewRepo   ReviewRepository
}

// ReviewRepository abstracts review fetching for AI aspect extraction.
type ReviewRepository interface {
	GetUserReviewsWithSummary(ctx context.Context, userID string) (*review.UserReviewSummary, error)
}

// NewHandler creates a new AIHandler with provided dependencies.
func NewHandler(
	orch *orchestrator.Orchestrator,
	db *pgxpool.Pool,
	jobRepo job.JobRepository,
	userRepo user.UserRepository,
	appRepos ...ApplicationRepository,
) *AIHandler {
	var appRepo ApplicationRepository
	if len(appRepos) > 0 {
		appRepo = appRepos[0]
	}
	return &AIHandler{
		orchestrator: orch,
		db:           db,
		jobRepo:      jobRepo,
		userRepo:     userRepo,
		appRepo:      appRepo,
	}
}

// WithApplicationRepo sets the application repository on the handler.
func (h *AIHandler) WithApplicationRepo(appRepo ApplicationRepository) *AIHandler {
	h.appRepo = appRepo
	return h
}

// WithReviewRepo sets the review repository on the handler.
func (h *AIHandler) WithReviewRepo(reviewRepo ReviewRepository) *AIHandler {
	h.reviewRepo = reviewRepo
	return h
}

// RegisterRoutes mounts public and protected search endpoints onto the provided chi router.
func (h *AIHandler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/search", func(sr chi.Router) {
		sr.Get("/jobs", h.SearchJobs)
		sr.Group(func(pr chi.Router) {
			if authMiddleware != nil {
				pr.Use(authMiddleware)
			}
			pr.Use(middleware.RequireVerifiedEmail())
			pr.Get("/people", h.SearchPeople)
		})
	})
}

// SearchJobs handles GET /api/v1/search/jobs?q=...
// Performs AI hybrid search for open jobs with automatic graceful fallback to standard SQL search.
func (h *AIHandler) SearchJobs(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := 20
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if parsed, err := strconv.Atoi(lStr); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	if q == "" {
		if h.jobRepo != nil {
			jobs, err := h.jobRepo.ListJobs(r.Context(), job.JobFilter{Status: "open", Limit: limit})
			if err == nil {
				results := make([]JobSearchResult, 0, len(jobs))
				for _, j := range jobs {
					results = append(results, JobSearchResult{Job: j})
				}
				httputil.WriteSuccess(w, http.StatusOK, results)
				return
			}
		}
		httputil.WriteSuccess(w, http.StatusOK, []JobSearchResult{})
		return
	}

	// 1. Attempt AI hybrid search
	if h.orchestrator != nil {
		req := models.HybridSearchRequest{
			Query:      q,
			EntityType: "job",
			Limit:      limit,
		}
		resp, err := h.orchestrator.HybridSearch(r.Context(), req)
		if err == nil && resp != nil && len(resp.Results) > 0 {
			results := h.resolveJobs(r.Context(), resp.Results)
			if len(results) > 0 {
				httputil.WriteSuccess(w, http.StatusOK, results)
				return
			}
		}
	}

	// 2. Graceful Fallback: Standard SQL keyword search
	fallbackJobs := h.fallbackJobSearch(r.Context(), q, limit)
	httputil.WriteSuccess(w, http.StatusOK, fallbackJobs)
}

// SearchPeople handles GET /api/v1/search/people?q=...
// Requires an active session and verified institutional .edu email address.
func (h *AIHandler) SearchPeople(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims == nil || claims.UserID == "" {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Active session required", nil)
		return
	}

	if !claims.EmailVerified {
		httputil.WriteError(w, r, http.StatusForbidden, "EMAIL_NOT_VERIFIED", auth.CampusVerificationPendingMsg, nil)
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := 20
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if parsed, err := strconv.Atoi(lStr); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	if q == "" {
		httputil.WriteSuccess(w, http.StatusOK, []PeopleSearchResult{})
		return
	}

	// 1. Attempt AI hybrid search
	if h.orchestrator != nil {
		req := models.HybridSearchRequest{
			Query:      q,
			EntityType: "profile",
			Limit:      limit,
		}
		resp, err := h.orchestrator.HybridSearch(r.Context(), req)
		if err == nil && resp != nil && len(resp.Results) > 0 {
			results := h.resolveProfiles(r.Context(), resp.Results)
			if len(results) > 0 {
				httputil.WriteSuccess(w, http.StatusOK, results)
				return
			}
		}
	}

	// 2. Graceful Fallback: SQL search over profiles
	fallbackPeople := h.fallbackPeopleSearch(r.Context(), q, limit)
	httputil.WriteSuccess(w, http.StatusOK, fallbackPeople)
}

func (h *AIHandler) resolveJobs(ctx context.Context, items []models.SearchResultItem) []JobSearchResult {
	if len(items) == 0 {
		return []JobSearchResult{}
	}

	uuids := make([]uuid.UUID, 0, len(items))
	itemMap := make(map[uuid.UUID]models.SearchResultItem, len(items))
	for _, item := range items {
		jobUUID, err := uuid.Parse(item.EntityID)
		if err == nil {
			uuids = append(uuids, jobUUID)
			itemMap[jobUUID] = item
		}
	}

	if len(uuids) == 0 {
		return []JobSearchResult{}
	}

	if h.db != nil {
		sql := `
			SELECT id, created_by, title, description, required_skills, department, deadline, status, created_at, updated_at
			FROM jobs
			WHERE id = ANY($1) AND status = 'open';
		`
		rows, err := h.db.Query(ctx, sql, uuids)
		if err == nil {
			jobMap := make(map[uuid.UUID]*job.Job, len(uuids))
			for rows.Next() {
				var j job.Job
				if scanErr := rows.Scan(
					&j.ID, &j.CreatedBy, &j.Title, &j.Description, &j.RequiredSkills,
					&j.Department, &j.Deadline, &j.Status, &j.CreatedAt, &j.UpdatedAt,
				); scanErr == nil {
					jobMap[j.ID] = &j
				}
			}
			rows.Close()

			results := make([]JobSearchResult, 0, len(uuids))
			for _, u := range uuids {
				if j, found := jobMap[u]; found {
					item := itemMap[u]
					results = append(results, JobSearchResult{
						Job:           j,
						Score:         item.Score,
						Snippet:       item.Snippet,
						MatchedSkills: item.MatchedSkills,
					})
				}
			}
			return results
		}
	}

	// Fallback to jobRepo iterative lookup if db is nil or query fails
	results := make([]JobSearchResult, 0, len(uuids))
	for _, u := range uuids {
		if h.jobRepo != nil {
			j, err := h.jobRepo.GetJobByID(ctx, u)
			if err == nil && j != nil && j.Status == "open" {
				item := itemMap[u]
				results = append(results, JobSearchResult{
					Job:           j,
					Score:         item.Score,
					Snippet:       item.Snippet,
					MatchedSkills: item.MatchedSkills,
				})
			}
		}
	}
	return results
}

func (h *AIHandler) fallbackJobSearch(ctx context.Context, query string, limit int) []JobSearchResult {
	if h.jobRepo != nil {
		jobs, err := h.jobRepo.ListJobs(ctx, job.JobFilter{
			Search: query,
			Status: "open",
			Limit:  limit,
		})
		if err == nil {
			results := make([]JobSearchResult, 0, len(jobs))
			for _, j := range jobs {
				results = append(results, JobSearchResult{
					Job:           j,
					Score:         1.0,
					MatchedSkills: []string{},
				})
			}
			return results
		}
	}

	if h.db != nil {
		sql := `
			SELECT id, created_by, title, description, required_skills, department, deadline, status, created_at, updated_at
			FROM jobs
			WHERE status = 'open' AND (title ILIKE $1 OR description ILIKE $1)
			LIMIT $2;
		`
		pattern := "%" + query + "%"
		rows, err := h.db.Query(ctx, sql, pattern, limit)
		if err == nil {
			defer rows.Close()
			var results []JobSearchResult
			for rows.Next() {
				var j job.Job
				if scanErr := rows.Scan(
					&j.ID, &j.CreatedBy, &j.Title, &j.Description, &j.RequiredSkills,
					&j.Department, &j.Deadline, &j.Status, &j.CreatedAt, &j.UpdatedAt,
				); scanErr == nil {
					results = append(results, JobSearchResult{
						Job:           &j,
						Score:         1.0,
						MatchedSkills: []string{},
					})
				}
			}
			return results
		}
	}

	return []JobSearchResult{}
}

func (h *AIHandler) resolveProfiles(ctx context.Context, items []models.SearchResultItem) []PeopleSearchResult {
	results := make([]PeopleSearchResult, 0, len(items))
	for _, item := range items {
		if h.userRepo != nil {
			p, err := h.userRepo.GetProfileByID(ctx, item.EntityID)
			if err != nil || p == nil {
				p, err = h.userRepo.GetProfile(ctx, item.EntityID)
			}
			if err == nil && p != nil {
				results = append(results, PeopleSearchResult{
					Profile:       p,
					Score:         item.Score,
					Snippet:       item.Snippet,
					MatchedSkills: item.MatchedSkills,
				})
			}
		}
	}
	return results
}

// profileSearcher is an optional interface UserRepository implementations can provide for keyword profile search.
type profileSearcher interface {
	SearchProfiles(ctx context.Context, query string, limit int) ([]*user.Profile, error)
}

func (h *AIHandler) fallbackPeopleSearch(ctx context.Context, query string, limit int) []PeopleSearchResult {
	if searcher, ok := h.userRepo.(profileSearcher); ok {
		profiles, err := searcher.SearchProfiles(ctx, query, limit)
		if err == nil && len(profiles) > 0 {
			results := make([]PeopleSearchResult, 0, len(profiles))
			for _, p := range profiles {
				results = append(results, PeopleSearchResult{
					Profile:       p,
					Score:         1.0,
					MatchedSkills: []string{},
				})
			}
			return results
		}
	}

	if h.db != nil {
		sql := `
			SELECT id, user_id, first_name, last_name, bio, department, graduation_year, skills, updated_at
			FROM profiles
			WHERE first_name ILIKE $1 
			   OR last_name ILIKE $1 
			   OR bio ILIKE $1 
			   OR department ILIKE $1
			   OR array_to_string(skills, ' ') ILIKE $1
			LIMIT $2;
		`
		pattern := "%" + query + "%"
		rows, err := h.db.Query(ctx, sql, pattern, limit)
		if err == nil {
			defer rows.Close()
			var results []PeopleSearchResult
			for rows.Next() {
				var p user.Profile
				if scanErr := rows.Scan(
					&p.ID, &p.UserID, &p.FirstName, &p.LastName, &p.Bio, &p.Department,
					&p.GraduationYear, &p.Skills, &p.UpdatedAt,
				); scanErr == nil {
					results = append(results, PeopleSearchResult{
						Profile:       &p,
						Score:         1.0,
						MatchedSkills: []string{},
					})
				}
			}
			return results
		}
	}

	return []PeopleSearchResult{}
}

// GetMyRecommendations handles GET /api/v1/profile/recommendations.
// Requires an active session and verified institutional .edu email address.
func (h *AIHandler) GetMyRecommendations(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetSessionUser(r.Context())
	if err != nil || claims == nil || claims.UserID == "" {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Active session required", nil)
		return
	}

	if !claims.EmailVerified {
		httputil.WriteError(w, r, http.StatusForbidden, "EMAIL_NOT_VERIFIED", auth.CampusVerificationPendingMsg, nil)
		return
	}

	var p *user.Profile
	if h.userRepo != nil {
		p, err = h.userRepo.GetProfile(r.Context(), claims.UserID)
		if err != nil || p == nil {
			p, _ = h.userRepo.GetProfileByID(r.Context(), claims.UserID)
		}
	}

	req := models.ProfileRecommendationsRequest{
		UserID: claims.UserID,
		Limit:  10,
	}
	if p != nil {
		req.CurrentSkills = p.Skills
		req.Department = p.Department
		req.Bio = p.Bio
		req.PortfolioLinks = p.PortfolioLinks
	}

	// 1. Attempt AI orchestrator call
	if h.orchestrator != nil {
		resp, err := h.orchestrator.GetProfileRecommendations(r.Context(), req)
		if err == nil && resp != nil && len(resp.Recommendations) > 0 {
			httputil.WriteSuccess(w, http.StatusOK, resp)
			return
		}
	}

	// 2. Graceful Fallback: Heuristic recommendations
	fallback := h.fallbackRecommendations(claims.UserID, p)
	httputil.WriteSuccess(w, http.StatusOK, fallback)
}

func (h *AIHandler) fallbackRecommendations(userID string, p *user.Profile) *models.ProfileRecommendationsResponse {
	var items []models.RecommendationItem

	var bio string
	var skills []string
	var links []string
	var department string

	if p != nil {
		bio = strings.TrimSpace(p.Bio)
		skills = p.Skills
		links = p.PortfolioLinks
		department = strings.TrimSpace(p.Department)
	}

	if bio == "" || len(bio) < 50 {
		items = append(items, models.RecommendationItem{
			Type:       "profile",
			Title:      "Complete your campus bio",
			Reason:     "Profiles with a bio describing academic background and project interests receive 3x more gig inquiries.",
			Confidence: 0.95,
			Metadata: map[string]any{
				"field":  "bio",
				"action": "add_bio",
			},
		})
	}

	if len(skills) == 0 {
		items = append(items, models.RecommendationItem{
			Type:       "profile",
			Title:      "Add your technical skills",
			Reason:     "Listing at least 3 skills allows our matching system to surface relevant campus gigs and projects.",
			Confidence: 0.98,
			Metadata: map[string]any{
				"field":  "skills",
				"action": "add_skills",
			},
		})
	} else if len(skills) < 3 {
		items = append(items, models.RecommendationItem{
			Type:       "profile",
			Title:      "Add more skills to your profile",
			Reason:     "Campus members with 3 or more skills appear in 5x more search queries.",
			Confidence: 0.85,
			Metadata: map[string]any{
				"field":  "skills",
				"action": "expand_skills",
			},
		})
	}

	hasLinks := len(links) > 0 || (bio != "" && (strings.Contains(strings.ToLower(bio), "github.com") || strings.Contains(strings.ToLower(bio), "http")))
	if !hasLinks {
		items = append(items, models.RecommendationItem{
			Type:       "profile",
			Title:      "Add portfolio or GitHub links",
			Reason:     "Showing code repositories or a live design portfolio gives project leaders direct evidence of your work.",
			Confidence: 0.88,
			Metadata: map[string]any{
				"field":  "portfolio_links",
				"action": "add_portfolio",
			},
		})
	}

	if department == "" {
		items = append(items, models.RecommendationItem{
			Type:       "profile",
			Title:      "Specify your academic department",
			Reason:     "Helps department-specific campus recruiters find you for research and teaching assistant gigs.",
			Confidence: 0.80,
			Metadata: map[string]any{
				"field":  "department",
				"action": "set_department",
			},
		})
	}

	return &models.ProfileRecommendationsResponse{
		Recommendations: items,
		UserID:          userID,
		ModelName:       "lynk-heuristic-fallback",
		ModelVersion:    "1.0.0",
	}
}

// RankApplicants handles GET /api/v1/jobs/{id}/applicants/ranking.
// Extracts job ID from URL param chi.URLParam(r, "id").
// Requires active session (401), verified institutional email (403), job exists (404),
// and current user is the job creator (403).
// Computes candidate rankings via AI orchestrator with graceful fallback (200).
func (h *AIHandler) RankApplicants(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims == nil || claims.UserID == "" {
		claims, err = auth.GetSessionUser(r.Context())
	}
	if err != nil || claims == nil || claims.UserID == "" {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Active session required", nil)
		return
	}

	if !claims.EmailVerified {
		httputil.WriteError(w, r, http.StatusForbidden, "EMAIL_NOT_VERIFIED", auth.CampusVerificationPendingMsg, nil)
		return
	}

	jobIDStr := chi.URLParam(r, "id")
	jobUUID, err := uuid.Parse(jobIDStr)
	if err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_JOB_ID", "Invalid job UUID", nil)
		return
	}

	if h.jobRepo == nil {
		httputil.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Job repository unavailable", nil)
		return
	}

	jobObj, err := h.jobRepo.GetJobByID(r.Context(), jobUUID)
	if err != nil || jobObj == nil {
		httputil.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Job not found", nil)
		return
	}

	if jobObj.CreatedBy != claims.UserID {
		httputil.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Only the job creator can access applicant rankings", nil)
		return
	}

	// 1. Fetch applications for the job
	type rawApp struct {
		ID          uuid.UUID
		CoverLetter string
		Skills      []string
		Bio         string
		Department  string
	}

	var rawApps []rawApp

	if h.appRepo != nil {
		apps, err := h.appRepo.ListApplicationsByJob(r.Context(), jobUUID, 100, 0)
		if err == nil && len(apps) > 0 {
			for _, a := range apps {
				item := rawApp{
					ID:          a.ID,
					CoverLetter: a.CoverLetter,
				}
				if a.Applicant != nil {
					item.Skills = a.Applicant.Skills
					item.Bio = a.Applicant.Bio
					item.Department = a.Applicant.Department
				}
				rawApps = append(rawApps, item)
			}
		}
	} else if h.db != nil {
		sql := `
			SELECT a.id, a.cover_letter, coalesce(p.skills, '{}'), coalesce(p.bio, ''), coalesce(p.department, '')
			FROM applications a
			LEFT JOIN profiles p ON a.applicant_id = p.user_id
			WHERE a.job_id = $1
			ORDER BY a.created_at DESC
			LIMIT 100;
		`
		rows, err := h.db.Query(r.Context(), sql, jobUUID)
		if err == nil {
			for rows.Next() {
				var item rawApp
				if err := rows.Scan(&item.ID, &item.CoverLetter, &item.Skills, &item.Bio, &item.Department); err == nil {
					rawApps = append(rawApps, item)
				}
			}
			rows.Close()
		}
	}

	if len(rawApps) == 0 {
		httputil.WriteSuccess(w, http.StatusOK, &models.RankCandidatesResponse{
			Results:         []models.CandidateRankResult{},
			ModelName:       "lynk-candidate-ranker",
			ModelVersion:    "1.0.0",
			PipelineVersion: "ranking-v1",
		})
		return
	}

	candidates := make([]models.CandidateRankInput, 0, len(rawApps))
	for _, a := range rawApps {
		skills := a.Skills
		if skills == nil {
			skills = []string{}
		}
		candidates = append(candidates, models.CandidateRankInput{
			ApplicationID: a.ID.String(),
			Skills:        skills,
			Bio:           a.Bio,
			Department:    a.Department,
			CoverLetter:   a.CoverLetter,
		})
	}

	// 2. Attempt AI ranking
	if h.orchestrator != nil {
		req := models.RankCandidatesRequest{
			JobID:           jobUUID.String(),
			JobTitle:        jobObj.Title,
			JobDescription:  jobObj.Description,
			JobDepartment:   jobObj.Department,
			RequiredSkills:  jobObj.RequiredSkills,
			Candidates:      candidates,
			PipelineVersion: "ranking-v1",
		}
		aiResp, err := h.orchestrator.RankCandidates(r.Context(), req)
		if err == nil && aiResp != nil && len(aiResp.Results) > 0 {
			h.persistScores(r.Context(), jobUUID, aiResp.Results, aiResp.ModelVersion, aiResp.PipelineVersion)
			httputil.WriteSuccess(w, http.StatusOK, aiResp)
			return
		}
	}

	// 3. Graceful Fallback: Heuristic skill overlap scoring
	fallbackResp := h.fallbackRanking(jobObj, candidates)
	h.persistScores(r.Context(), jobUUID, fallbackResp.Results, fallbackResp.ModelVersion, fallbackResp.PipelineVersion)
	httputil.WriteSuccess(w, http.StatusOK, fallbackResp)
}

func (h *AIHandler) persistScores(ctx context.Context, jobID uuid.UUID, results []models.CandidateRankResult, modelVersion, pipelineVersion string) {
	if h.db == nil || len(results) == 0 {
		return
	}

	query := `
		INSERT INTO application_ai_scores (
			application_id, job_id, score, matched_skills, missing_skills,
			reason, confidence, model_version, pipeline_version, is_stale, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, false, NOW())
		ON CONFLICT (application_id) DO UPDATE SET
			score = EXCLUDED.score,
			matched_skills = EXCLUDED.matched_skills,
			missing_skills = EXCLUDED.missing_skills,
			reason = EXCLUDED.reason,
			confidence = EXCLUDED.confidence,
			model_version = EXCLUDED.model_version,
			pipeline_version = EXCLUDED.pipeline_version,
			is_stale = false,
			updated_at = NOW();
	`

	for _, res := range results {
		appUUID, err := uuid.Parse(res.ApplicationID)
		if err != nil {
			continue
		}
		matchedSkills := res.MatchedSkills
		if matchedSkills == nil {
			matchedSkills = []string{}
		}
		missingSkills := res.MissingSkills
		if missingSkills == nil {
			missingSkills = []string{}
		}

		mVer := modelVersion
		if mVer == "" {
			mVer = "1.0.0"
		}
		pVer := pipelineVersion
		if pVer == "" {
			pVer = "ranking-v1"
		}

		_, _ = h.db.Exec(ctx, query,
			appUUID,
			jobID,
			res.Score,
			matchedSkills,
			missingSkills,
			res.Reason,
			res.Confidence,
			mVer,
			pVer,
		)
	}
}

func (h *AIHandler) fallbackRanking(j *job.Job, candidates []models.CandidateRankInput) *models.RankCandidatesResponse {
	results := make([]models.CandidateRankResult, 0, len(candidates))

	for _, cand := range candidates {
		candSkillsMap := make(map[string]bool)
		for _, s := range cand.Skills {
			candSkillsMap[strings.ToLower(strings.TrimSpace(s))] = true
		}

		matched := make([]string, 0)
		missing := make([]string, 0)
		for _, reqSkill := range j.RequiredSkills {
			cleanReq := strings.ToLower(strings.TrimSpace(reqSkill))
			if candSkillsMap[cleanReq] {
				matched = append(matched, reqSkill)
			} else {
				missing = append(missing, reqSkill)
			}
		}

		skillOverlap := 1.0
		if len(j.RequiredSkills) > 0 {
			skillOverlap = float64(len(matched)) / float64(len(j.RequiredSkills))
		}

		deptCompat := 0.5
		if j.Department != "" && cand.Department != "" {
			if strings.EqualFold(strings.TrimSpace(j.Department), strings.TrimSpace(cand.Department)) {
				deptCompat = 1.0
			}
		} else if cand.Department == "" {
			deptCompat = 0.2
		}

		composite := 0.45*skillOverlap + 0.35*0.5 + 0.20*deptCompat
		finalScore := math.Round(composite*1000) / 10

		matchedStr := "none"
		if len(matched) > 0 {
			matchedStr = strings.Join(matched, ", ")
		}
		missingStr := "none"
		if len(missing) > 0 {
			missingStr = strings.Join(missing, ", ")
		}

		reason := fmt.Sprintf(
			"Matched skills (%d/%d): %s. Missing skills: %s. Observable breakdown: skills=%.1f%%, semantic_similarity=50.0%%, department_compatibility=%.1f%%.",
			len(matched), len(j.RequiredSkills),
			matchedStr,
			missingStr,
			skillOverlap*100,
			deptCompat*100,
		)

		results = append(results, models.CandidateRankResult{
			ApplicationID: cand.ApplicationID,
			Score:         finalScore,
			MatchedSkills: matched,
			MissingSkills: missing,
			Reason:        reason,
			Confidence:    0.70,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return &models.RankCandidatesResponse{
		Results:         results,
		ModelName:       "lynk-heuristic-fallback",
		ModelVersion:    "1.0.0",
		PipelineVersion: "heuristic-v1",
	}
}

// GetUserAIInsights returns AI-extracted collaboration aspect scores, strengths, and recurring patterns for a user.
func (h *AIHandler) GetUserAIInsights(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	targetUserID := chi.URLParam(r, "id")
	if targetUserID == "" {
		httputil.WriteError(w, r, http.StatusBadRequest, "MISSING_USER_ID", "missing target user id", nil)
		return
	}

	// 1. Verify user exists
	if h.userRepo != nil {
		prof, err := h.userRepo.GetProfile(ctx, targetUserID)
		if err != nil || prof == nil {
			httputil.WriteError(w, r, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil)
			return
		}
	}

	// 2. Fetch completed reviews
	var reviews []*review.Review
	if h.reviewRepo != nil {
		summary, err := h.reviewRepo.GetUserReviewsWithSummary(ctx, targetUserID)
		if err == nil && summary != nil {
			reviews = summary.Reviews
		}
	} else if h.db != nil {
		rows, err := h.db.Query(ctx, `SELECT rating, comment FROM reviews WHERE reviewee_id = $1 ORDER BY created_at DESC LIMIT 100`, targetUserID)
		if err == nil {
			for rows.Next() {
				var r review.Review
				if err := rows.Scan(&r.Rating, &r.Comment); err == nil {
					reviews = append(reviews, &r)
				}
			}
			rows.Close()
		}
	}

	if len(reviews) == 0 {
		httputil.WriteSuccess(w, http.StatusOK, models.AnalyzeReviewsResponse{
			UserID:          targetUserID,
			Insights:        []models.ReviewAspectInsight{},
			SampleCount:     0,
			ModelName:       "lynk-review-analyzer",
			ModelVersion:    "1.0.0",
			PipelineVersion: "reviews-v1",
		})
		return
	}

	// 3. Prepare AI request
	reqItems := make([]models.ReviewItemInput, len(reviews))
	for i, rev := range reviews {
		reqItems[i] = models.ReviewItemInput{
			Rating:  rev.Rating,
			Comment: rev.Comment,
		}
	}

	var aiResp *models.AnalyzeReviewsResponse
	if h.orchestrator != nil {
		resp, err := h.orchestrator.GetReviewInsights(ctx, models.AnalyzeReviewsRequest{
			UserID:  targetUserID,
			Reviews: reqItems,
		})
		if err == nil && resp != nil {
			aiResp = resp
		}
	}

	// 4. Graceful heuristic fallback if AI unavailable
	if aiResp == nil {
		aiResp = fallbackReviewInsights(targetUserID, reviews)
	}

	httputil.WriteSuccess(w, http.StatusOK, *aiResp)
}

// fallbackReviewInsights computes heuristic collaboration aspect ratings from review comments and scores.
func fallbackReviewInsights(userID string, reviews []*review.Review) *models.AnalyzeReviewsResponse {
	aspectKeywords := map[string][]string{
		"technical_ability": {"code", "programming", "technical", "architecture", "bug", "implementation", "feature"},
		"timeliness":        {"time", "turnaround", "speed", "fast", "deadline", "quick", "early", "schedule"},
		"communication":     {"communication", "responsive", "clear", "communicative", "update", "explained"},
		"reliability":       {"reliable", "trustworthy", "dependable", "consistent", "responsible"},
	}

	sampleCount := len(reviews)
	insights := make([]models.ReviewAspectInsight, 0)

	for aspect, keywords := range aspectKeywords {
		var aspectRatings []float64
		for _, rev := range reviews {
			textLower := strings.ToLower(rev.Comment)
			matched := false
			for _, kw := range keywords {
				if strings.Contains(textLower, kw) {
					matched = true
					break
				}
			}
			if matched {
				aspectRatings = append(aspectRatings, float64(rev.Rating))
			}
		}

		if len(aspectRatings) > 0 {
			var sum float64
			for _, r := range aspectRatings {
				sum += r
			}
			avgScore := math.Round((sum/float64(len(aspectRatings)))*10) / 10

			strengths := []string{}
			improvements := []string{}
			if avgScore >= 4.0 {
				switch aspect {
				case "timeliness":
					strengths = append(strengths, "Fast Turnaround")
				case "technical_ability":
					strengths = append(strengths, "Superb Code Quality")
				case "communication":
					strengths = append(strengths, "Proactive Communicator")
				}
			} else if avgScore <= 3.0 {
				if aspect == "timeliness" {
					improvements = append(improvements, "Ensure timely delivery against deadlines")
				}
			}

			isRecurring := sampleCount >= 3 && len(aspectRatings) >= 2

			insights = append(insights, models.ReviewAspectInsight{
				Aspect:       aspect,
				Score:        avgScore,
				Confidence:   0.75,
				SampleCount:  len(aspectRatings),
				IsRecurring:  isRecurring,
				Strengths:    strengths,
				Improvements: improvements,
			})
		}
	}

	return &models.AnalyzeReviewsResponse{
		UserID:          userID,
		Insights:        insights,
		SampleCount:     sampleCount,
		ModelName:       "lynk-heuristic-fallback",
		ModelVersion:    "1.0.0",
		PipelineVersion: "heuristic-v1",
	}
}

// GetSkillAnalytics returns skill demand snapshots and forward-looking forecasts.
// If AI is disabled or unavailable, it falls back to heuristic counts from open job postings.
func (h *AIHandler) GetSkillAnalytics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "monthly"
	}

	var jobsList []*job.Job
	if h.jobRepo != nil {
		var err error
		jobsList, err = h.jobRepo.ListJobs(ctx, job.JobFilter{Status: "open", Limit: 100})
		if err != nil {
			httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load job listings", err)
			return
		}
	}

	skillCounts := make(map[string]int)
	for _, j := range jobsList {
		for _, s := range j.RequiredSkills {
			skillCounts[s]++
		}
	}

	if h.orchestrator != nil {
		var skillsData []models.SkillHistoricalData
		for skill, count := range skillCounts {
			posters := int(math.Max(1, float64(count/2)))
			skillsData = append(skillsData, models.SkillHistoricalData{
				Skill: skill,
				Postings: []models.SkillPostingPoint{
					{
						Period:           period,
						JobCount:         count,
						ApplicationCount: count * 4,
						UniquePosters:    posters,
					},
				},
			})
		}

		// Sort deterministically
		sort.Slice(skillsData, func(i, j int) bool {
			return skillsData[i].Skill < skillsData[j].Skill
		})

		req := models.SkillDemandAnalyticsRequest{
			Period:     period,
			SkillsData: skillsData,
		}

		aiResp, err := h.orchestrator.GetSkillDemandAnalytics(ctx, req)
		if err == nil && aiResp != nil {
			httputil.WriteSuccess(w, http.StatusOK, *aiResp)
			return
		}
	}

	// Fallback to heuristic analytics from job repository
	fallback := h.fallbackSkillAnalytics(period, skillCounts)
	httputil.WriteSuccess(w, http.StatusOK, *fallback)
}

func (h *AIHandler) fallbackSkillAnalytics(period string, skillCounts map[string]int) *models.SkillDemandAnalyticsResponse {
	type skillEntry struct {
		skill string
		count int
	}
	var entries []skillEntry
	for s, c := range skillCounts {
		entries = append(entries, skillEntry{skill: s, count: c})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count == entries[j].count {
			return entries[i].skill < entries[j].skill
		}
		return entries[i].count > entries[j].count
	})

	var snapshots []models.SkillDemandSnapshot
	var forecasts []models.SkillDemandForecast

	for _, e := range entries {
		demandScore := math.Min(100.0, float64(e.count*10))
		posters := int(math.Max(1, float64(e.count/2)))
		appCount := e.count * 3
		appToJobRatio := 3.0
		growthRate := 0.0

		snapshots = append(snapshots, models.SkillDemandSnapshot{
			Skill:            e.skill,
			Period:           period,
			JobCount:         e.count,
			ApplicationCount: appCount,
			UniquePosters:    posters,
			DemandScore:      demandScore,
			GrowthRate:       growthRate,
			AppToJobRatio:    appToJobRatio,
		})

		forecasts = append(forecasts, models.SkillDemandForecast{
			Skill:              e.skill,
			GrowthRate:         growthRate,
			DemandScore:        demandScore,
			Projected30dDemand: float64(e.count),
		})
	}

	return &models.SkillDemandAnalyticsResponse{
		Period:          period,
		Snapshots:       snapshots,
		Forecasts:       forecasts,
		ModelName:       "lynk-heuristic-fallback",
		ModelVersion:    "1.0.0",
		PipelineVersion: "heuristic-v1",
	}
}

// GenerateJobDraft handles POST /api/v1/jobs/generate.
// Generates a structured job draft for user review prior to posting.
// Requires active session and verified institutional email. Never mutates the jobs table directly.
func (h *AIHandler) GenerateJobDraft(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetSessionUser(r.Context())
	if err != nil || claims == nil || claims.UserID == "" {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Active session required", nil)
		return
	}

	if !claims.EmailVerified {
		httputil.WriteError(w, r, http.StatusForbidden, "EMAIL_NOT_VERIFIED", auth.CampusVerificationPendingMsg, nil)
		return
	}

	var req models.GenerateJobDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body", err)
		return
	}

	req.Idea = strings.TrimSpace(req.Idea)
	if req.Idea == "" {
		httputil.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Job idea must not be empty", nil)
		return
	}

	if h.orchestrator != nil {
		resp, err := h.orchestrator.GenerateJobDraft(r.Context(), req)
		if err == nil && resp != nil {
			httputil.WriteSuccess(w, http.StatusOK, *resp)
			return
		}
	}

	// Fallback to deterministic heuristic generator
	fallback := h.fallbackJobDraft(req)
	httputil.WriteSuccess(w, http.StatusOK, *fallback)
}

func (h *AIHandler) fallbackJobDraft(req models.GenerateJobDraftRequest) *models.GeneratedJobDraftResponse {
	idea := req.Idea
	ideaLower := strings.ToLower(idea)

	title := "Campus Project Developer"
	skills := []string{"General Development", "Problem Solving"}

	if strings.Contains(ideaLower, "ios") || strings.Contains(ideaLower, "swift") {
		title = "iOS Application Developer"
		skills = []string{"Swift", "SwiftUI", "iOS"}
	} else if strings.Contains(ideaLower, "react") && strings.Contains(ideaLower, "dashboard") {
		title = "React Dashboard Developer"
		skills = []string{"React", "TypeScript", "Tailwind CSS"}
	} else if strings.Contains(ideaLower, "machine learning") || strings.Contains(ideaLower, "pytorch") || strings.Contains(ideaLower, "classifier") {
		title = "Machine Learning Engineer"
		skills = []string{"Python", "PyTorch", "Machine Learning"}
	} else if strings.Contains(ideaLower, "next.js") || strings.Contains(ideaLower, "tailwind") {
		title = "Frontend Web Developer"
		skills = []string{"Next.js", "Tailwind CSS", "TypeScript"}
	}

	dept := req.Department
	if dept == "" {
		dept = "Computer Science"
	}

	description := fmt.Sprintf("We are seeking a student contributor to help develop: %s. Key responsibilities include designing components, writing clean and tested code, and delivering working deliverables on schedule.", idea)

	return &models.GeneratedJobDraftResponse{
		Draft: models.GeneratedJobDraft{
			Title:          title,
			Description:    description,
			RequiredSkills: skills,
			Department:     dept,
		},
		ModelName:       "lynk-heuristic-fallback",
		ModelVersion:    "1.0.0",
		PromptVersion:   "heuristic-v1",
		PipelineVersion: "generation-v1",
	}
}
