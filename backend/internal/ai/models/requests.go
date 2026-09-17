package models

// NormalizeSkillRequest contains an unstructured skill string to be normalized.
type NormalizeSkillRequest struct {
	Skill string `json:"skill"`
}

// ExtractSkillsRequest contains freeform text from which to extract canonical skills.
type ExtractSkillsRequest struct {
	Text string `json:"text"`
}

// HybridSearchRequest represents a query for semantic and hybrid search over platform entities.
type HybridSearchRequest struct {
	Query      string `json:"query"`
	Limit      int    `json:"limit"`
	EntityType string `json:"entity_type"`
}

// CandidateRankInput represents observable, non-protected input features for ranking an applicant.
type CandidateRankInput struct {
	ApplicationID string   `json:"application_id"`
	Skills        []string `json:"skills"`
	Bio           string   `json:"bio,omitempty"`
	Department    string   `json:"department,omitempty"`
	CoverLetter   string   `json:"cover_letter,omitempty"`
}

// RankCandidatesRequest represents a request to score and rank job applicants using AI pipelines.
type RankCandidatesRequest struct {
	JobID           string               `json:"job_id"`
	JobTitle        string               `json:"job_title"`
	JobDescription  string               `json:"job_description"`
	JobDepartment   string               `json:"job_department"`
	RequiredSkills  []string             `json:"required_skills"`
	Candidates      []CandidateRankInput `json:"candidates"`
	PipelineVersion string               `json:"pipeline_version,omitempty"`
}

// GenerateJobDraftRequest represents a request to generate a structured job posting draft from a rough idea.
type GenerateJobDraftRequest struct {
	Idea       string `json:"idea"`
	Department string `json:"department"`
}

// ProfileRecommendationsRequest represents a query to generate recommendations for a member profile.
type ProfileRecommendationsRequest struct {
	UserID         string   `json:"user_id"`
	CurrentSkills  []string `json:"current_skills"`
	Department     string   `json:"department,omitempty"`
	Bio            string   `json:"bio,omitempty"`
	PortfolioLinks []string `json:"portfolio_links,omitempty"`
	Limit          int      `json:"limit,omitempty"`
}

// ModerationCheckRequest represents a request to evaluate content for spam, duplicates, and policy violations.
type ModerationCheckRequest struct {
	EntityType            string   `json:"entity_type"`
	EntityID              string   `json:"entity_id"`
	Text                  string   `json:"text"`
	AuthorID              string   `json:"author_id"`
	AuthorRecentPostCount int      `json:"author_recent_post_count,omitempty"`
	RecentSubmissions     []string `json:"recent_submissions,omitempty"`
}

// ReviewItemInput represents an individual review submitted for a contract participant.
type ReviewItemInput struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

// AnalyzeReviewsRequest represents a request to extract aspect insights and recurring reputation patterns.
type AnalyzeReviewsRequest struct {
	UserID  string            `json:"user_id"`
	Reviews []ReviewItemInput `json:"reviews"`
}

// SkillPostingPoint represents a historical posting datapoint for a skill over a specific period.
type SkillPostingPoint struct {
	Period           string `json:"period"`
	JobCount         int    `json:"job_count"`
	ApplicationCount int    `json:"application_count"`
	UniquePosters    int    `json:"unique_posters"`
}

// SkillHistoricalData represents time-series posting points for a specific skill.
type SkillHistoricalData struct {
	Skill    string              `json:"skill"`
	Postings []SkillPostingPoint `json:"postings"`
}

// SkillDemandAnalyticsRequest represents a request for skill demand snapshots and forecasts.
type SkillDemandAnalyticsRequest struct {
	Period     string                `json:"period"`
	SkillsData []SkillHistoricalData `json:"skills_data"`
}



