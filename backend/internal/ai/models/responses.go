package models

// NormalizeSkillResponse represents the canonical normalization of a skill string.
type NormalizeSkillResponse struct {
	OriginalSkill string  `json:"original_skill"`
	CanonicalName string  `json:"canonical_name"`
	Confidence    float64 `json:"confidence"`
	MatchMethod   string  `json:"match_method"`
	Category      string  `json:"category,omitempty"`
	SkillID       string  `json:"skill_id,omitempty"`
}

// ExtractSkillsResponse contains the list of canonical skills extracted from text.
type ExtractSkillsResponse struct {
	ExtractedSkills []NormalizeSkillResponse `json:"extracted_skills"`
}

// SearchResultItem represents an individual entity matched in hybrid search.
type SearchResultItem struct {
	EntityID      string   `json:"entity_id"`
	Score         float64  `json:"score"`
	Snippet       string   `json:"snippet"`
	MatchedSkills []string `json:"matched_skills,omitempty"`
}

// HybridSearchResponse contains matched items from a hybrid semantic/lexical search query.
type HybridSearchResponse struct {
	Results      []SearchResultItem `json:"results"`
	Total        int                `json:"total,omitempty"`
	ModelName    string             `json:"model_name,omitempty"`
	ModelVersion string             `json:"model_version,omitempty"`
}

// CandidateRankResult represents an AI-ranked candidate evaluation for a job posting.
type CandidateRankResult struct {
	ApplicationID string   `json:"application_id"`
	Score         float64  `json:"score"`
	MatchedSkills []string `json:"matched_skills"`
	MissingSkills []string `json:"missing_skills"`
	Reason        string   `json:"reason"`
	Confidence    float64  `json:"confidence"`
}

// RankCandidatesResponse contains scored and ranked candidate evaluations.
type RankCandidatesResponse struct {
	Results         []CandidateRankResult `json:"results"`
	ModelName       string                `json:"model_name"`
	ModelVersion    string                `json:"model_version"`
	PipelineVersion string                `json:"pipeline_version,omitempty"`
}

// GeneratedJobDraft represents a structured job posting draft generated from a prompt.
type GeneratedJobDraft struct {
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	RequiredSkills []string `json:"required_skills"`
	Department     string   `json:"department"`
}

// GeneratedJobDraftResponse represents the response containing the AI-generated job draft.
type GeneratedJobDraftResponse struct {
	Draft           GeneratedJobDraft `json:"draft"`
	ModelName       string            `json:"model_name,omitempty"`
	ModelVersion    string            `json:"model_version,omitempty"`
	PromptVersion   string            `json:"prompt_version,omitempty"`
	PipelineVersion string            `json:"pipeline_version,omitempty"`
}

// HealthResponse represents the health status of the AI service.
type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// RecommendationItem represents an individual recommendation for a campus member.
type RecommendationItem struct {
	Type       string         `json:"type"`
	Title      string         `json:"title"`
	Reason     string         `json:"reason"`
	Confidence float64        `json:"confidence"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// ProfileRecommendationsResponse contains generated recommendations and metadata for a member.
type ProfileRecommendationsResponse struct {
	Recommendations []RecommendationItem `json:"recommendations"`
	UserID          string               `json:"user_id"`
	ModelName       string               `json:"model_name"`
	ModelVersion    string               `json:"model_version"`
}

// ModerationCheckResponse represents the automated moderation evaluation result.
type ModerationCheckResponse struct {
	RiskScore       float64  `json:"risk_score"`
	Decision        string   `json:"decision"`
	Signals         []string `json:"signals"`
	Confidence      float64  `json:"confidence"`
	PipelineVersion string   `json:"pipeline_version"`
}

// ReviewAspectInsight represents an extracted collaboration aspect insight.
type ReviewAspectInsight struct {
	Aspect       string   `json:"aspect"`
	Score        float64  `json:"score"`
	Confidence   float64  `json:"confidence"`
	SampleCount  int      `json:"sample_count"`
	IsRecurring  bool     `json:"is_recurring"`
	Strengths    []string `json:"strengths"`
	Improvements []string `json:"improvements"`
}

// AnalyzeReviewsResponse represents the aggregated aspect insights for a campus member.
type AnalyzeReviewsResponse struct {
	UserID          string                `json:"user_id"`
	Insights        []ReviewAspectInsight `json:"insights"`
	SampleCount     int                   `json:"sample_count"`
	ModelName       string                `json:"model_name"`
	ModelVersion    string                `json:"model_version"`
	PipelineVersion string                `json:"pipeline_version"`
}

// SkillDemandSnapshot represents an aggregated demand metric snapshot for a skill.
type SkillDemandSnapshot struct {
	Skill            string  `json:"skill"`
	Period           string  `json:"period"`
	JobCount         int     `json:"job_count"`
	ApplicationCount int     `json:"application_count"`
	UniquePosters    int     `json:"unique_posters"`
	DemandScore      float64 `json:"demand_score"`
	GrowthRate       float64 `json:"growth_rate"`
	AppToJobRatio    float64 `json:"app_to_job_ratio"`
}

// SkillDemandForecast represents forward-looking demand projection for a skill.
type SkillDemandForecast struct {
	Skill              string  `json:"skill"`
	GrowthRate         float64 `json:"growth_rate"`
	DemandScore        float64 `json:"demand_score"`
	Projected30dDemand float64 `json:"projected_30d_demand"`
}

// SkillDemandAnalyticsResponse represents the response containing demand snapshots and forecasts.
type SkillDemandAnalyticsResponse struct {
	Period          string                `json:"period"`
	Snapshots       []SkillDemandSnapshot `json:"snapshots"`
	Forecasts       []SkillDemandForecast `json:"forecasts"`
	ModelName       string                `json:"model_name,omitempty"`
	ModelVersion    string                `json:"model_version,omitempty"`
	PipelineVersion string                `json:"pipeline_version,omitempty"`
}




