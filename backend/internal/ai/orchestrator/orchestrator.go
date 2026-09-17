package orchestrator

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lynk/backend/internal/ai/client"
	"github.com/lynk/backend/internal/ai/models"
)

// Standard operation timeouts recommended for AI pipelines.
const (
	DefaultSkillTimeout           = 2 * time.Second
	DefaultSearchTimeout          = 3 * time.Second
	DefaultRecommendationsTimeout = 3 * time.Second
	DefaultRankingTimeout         = 8 * time.Second
	DefaultGenerationTimeout      = 15 * time.Second
	DefaultModerationTimeout      = 3 * time.Second
	DefaultReviewTimeout          = 4 * time.Second
	DefaultSkillAnalyticsTimeout  = 5 * time.Second
)

// FeatureFlags controls activation of AI features across the application.
type FeatureFlags struct {
	AIEnabled          bool
	SkillNormalization bool
	SemanticSearch     bool
	ApplicationRanking bool
	JobGeneration      bool
	Moderation         bool
	ReviewInsights     bool
	Recommendations    bool
	SkillAnalytics     bool
}

func parseEnvBool(key string, defaultVal bool) bool {
	val, exists := os.LookupEnv(key)
	if !exists {
		return defaultVal
	}
	val = strings.TrimSpace(strings.ToLower(val))
	switch val {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return defaultVal
	}
}

// NewFeatureFlagsFromEnv loads AI feature flags from environment variables with safe defaults.
func NewFeatureFlagsFromEnv() FeatureFlags {
	return FeatureFlags{
		AIEnabled:          parseEnvBool("AI_ENABLED", true),
		SkillNormalization: parseEnvBool("AI_SKILL_NORMALIZATION", true),
		SemanticSearch:     parseEnvBool("AI_SEMANTIC_SEARCH", true),
		ApplicationRanking: parseEnvBool("AI_APPLICATION_RANKING", true),
		JobGeneration:      parseEnvBool("AI_JOB_GENERATION", true),
		Moderation:         parseEnvBool("AI_MODERATION", true),
		ReviewInsights:     parseEnvBool("AI_REVIEW_INSIGHTS", true),
		Recommendations:    parseEnvBool("AI_RECOMMENDATIONS", true),
		SkillAnalytics:     parseEnvBool("AI_SKILL_ANALYTICS", true),
	}
}

// Orchestrator coordinates AI operations with resilience, logging, and graceful fallbacks.
type Orchestrator struct {
	client *client.Client
	flags  FeatureFlags
	logger *slog.Logger
}

// NewOrchestrator creates a new AI orchestrator.
func NewOrchestrator(c *client.Client, flags FeatureFlags, logger *slog.Logger) *Orchestrator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Orchestrator{
		client: c,
		flags:  flags,
		logger: logger,
	}
}

// Flags returns the active feature flags.
func (o *Orchestrator) Flags() FeatureFlags {
	return o.flags
}

// Client returns the underlying AI client.
func (o *Orchestrator) Client() *client.Client {
	return o.client
}

// NormalizeSkill attempts to canonicalize a skill using the AI service.
// If AI is disabled, unconfigured, or fails, it safely falls back to the original raw skill string.
func (o *Orchestrator) NormalizeSkill(ctx context.Context, rawSkill string) (string, error) {
	trimmed := strings.TrimSpace(rawSkill)
	if trimmed == "" {
		return "", nil
	}

	if !o.flags.AIEnabled || !o.flags.SkillNormalization || o.client == nil {
		o.logger.DebugContext(ctx, "ai skill normalization bypassed", "skill", trimmed)
		return trimmed, nil
	}

	callCtx, cancel := context.WithTimeout(ctx, DefaultSkillTimeout)
	defer cancel()

	resp, err := o.client.NormalizeSkill(callCtx, trimmed)
	if err != nil {
		o.logger.WarnContext(ctx, "ai skill normalization failed, falling back to raw skill",
			"skill", trimmed,
			"error", err,
		)
		return trimmed, nil
	}

	if resp != nil && resp.CanonicalName != "" {
		return resp.CanonicalName, nil
	}

	return trimmed, nil
}

// ExtractSkills extracts and normalizes skills from text, falling back to an empty slice on failure.
func (o *Orchestrator) ExtractSkills(ctx context.Context, text string) ([]models.NormalizeSkillResponse, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, nil
	}

	if !o.flags.AIEnabled || !o.flags.SkillNormalization || o.client == nil {
		o.logger.DebugContext(ctx, "ai skill extraction bypassed", "text_len", len(trimmed))
		return nil, nil
	}

	callCtx, cancel := context.WithTimeout(ctx, DefaultSkillTimeout)
	defer cancel()

	resp, err := o.client.ExtractSkills(callCtx, trimmed)
	if err != nil {
		o.logger.WarnContext(ctx, "ai skill extraction failed, falling back to empty list",
			"error", err,
		)
		return nil, nil
	}

	if resp == nil {
		return nil, nil
	}

	return resp.ExtractedSkills, nil
}

// ErrAIBypassed is returned when AI operations are disabled by configuration or feature flags.
var ErrAIBypassed = errors.New("ai service bypassed or disabled")

// HybridSearch coordinates hybrid semantic/keyword search via the AI service.
// If AI is disabled, unconfigured, or fails, it returns an error allowing the caller to gracefully fall back.
func (o *Orchestrator) HybridSearch(ctx context.Context, req models.HybridSearchRequest) (*models.HybridSearchResponse, error) {
	if !o.flags.AIEnabled || !o.flags.SemanticSearch || o.client == nil {
		o.logger.DebugContext(ctx, "ai hybrid search bypassed", "query", req.Query)
		return nil, ErrAIBypassed
	}

	callCtx, cancel := context.WithTimeout(ctx, DefaultSearchTimeout)
	defer cancel()

	resp, err := o.client.HybridSearch(callCtx, req)
	if err != nil {
		o.logger.WarnContext(ctx, "ai hybrid search failed",
			"query", req.Query,
			"entity_type", req.EntityType,
			"error", err,
		)
		return nil, err
	}

	return resp, nil
}

// GetProfileRecommendations coordinates personalized profile recommendations via the AI service.
// If AI is disabled, unconfigured, or fails, it returns an error allowing the caller to gracefully fall back.
func (o *Orchestrator) GetProfileRecommendations(ctx context.Context, req models.ProfileRecommendationsRequest) (*models.ProfileRecommendationsResponse, error) {
	if !o.flags.AIEnabled || !o.flags.Recommendations || o.client == nil {
		o.logger.DebugContext(ctx, "ai profile recommendations bypassed", "user_id", req.UserID)
		return nil, ErrAIBypassed
	}

	callCtx, cancel := context.WithTimeout(ctx, DefaultRecommendationsTimeout)
	defer cancel()

	resp, err := o.client.GetProfileRecommendations(callCtx, req)
	if err != nil {
		o.logger.WarnContext(ctx, "ai profile recommendations failed",
			"user_id", req.UserID,
			"error", err,
		)
		return nil, err
	}

	return resp, nil
}

// RankCandidates coordinates applicant ranking and advisory scoring via the AI service.
// If AI is disabled, unconfigured, or fails, it returns an error allowing the caller to gracefully fall back.
func (o *Orchestrator) RankCandidates(ctx context.Context, req models.RankCandidatesRequest) (*models.RankCandidatesResponse, error) {
	if !o.flags.AIEnabled || !o.flags.ApplicationRanking || o.client == nil {
		o.logger.DebugContext(ctx, "ai application ranking bypassed", "job_id", req.JobID)
		return nil, ErrAIBypassed
	}

	callCtx, cancel := context.WithTimeout(ctx, DefaultRankingTimeout)
	defer cancel()

	resp, err := o.client.RankCandidates(callCtx, req)
	if err != nil {
		o.logger.WarnContext(ctx, "ai application ranking failed",
			"job_id", req.JobID,
			"error", err,
		)
		return nil, err
	}

	return resp, nil
}

// CheckModeration coordinates automated content moderation via the AI service.
// If AI is disabled or fails, it gracefully falls back to decision: "allow" with RiskScore: 0.0,
// logging a warning so that user submissions are not blocked during AI service degradation.
func (o *Orchestrator) CheckModeration(ctx context.Context, req models.ModerationCheckRequest) (*models.ModerationCheckResponse, error) {
	if !o.flags.AIEnabled || !o.flags.Moderation || o.client == nil {
		o.logger.DebugContext(ctx, "ai moderation bypassed", "entity_id", req.EntityID)
		return &models.ModerationCheckResponse{
			RiskScore:       0.0,
			Decision:        "allow",
			Signals:         []string{},
			Confidence:      1.0,
			PipelineVersion: "moderation-fallback",
		}, nil
	}

	callCtx, cancel := context.WithTimeout(ctx, DefaultModerationTimeout)
	defer cancel()

	resp, err := o.client.CheckModeration(callCtx, req)
	if err != nil {
		o.logger.WarnContext(ctx, "ai moderation failed, falling back to allow",
			"entity_id", req.EntityID,
			"error", err,
		)
		return &models.ModerationCheckResponse{
			RiskScore:       0.0,
			Decision:        "allow",
			Signals:         []string{},
			Confidence:      0.5,
			PipelineVersion: "moderation-fallback",
		}, nil
	}

	return resp, nil
}

// GetReviewInsights coordinates aspect extraction and recurring reputation analysis via the AI service.
// If AI is disabled or fails, it returns ErrAIBypassed or the error allowing the caller to fall back gracefully.
func (o *Orchestrator) GetReviewInsights(ctx context.Context, req models.AnalyzeReviewsRequest) (*models.AnalyzeReviewsResponse, error) {
	if !o.flags.AIEnabled || !o.flags.ReviewInsights || o.client == nil {
		o.logger.DebugContext(ctx, "ai review insights bypassed", "user_id", req.UserID)
		return nil, ErrAIBypassed
	}

	callCtx, cancel := context.WithTimeout(ctx, DefaultReviewTimeout)
	defer cancel()

	resp, err := o.client.AnalyzeReviews(callCtx, req)
	if err != nil {
		o.logger.WarnContext(ctx, "ai review insights failed",
			"user_id", req.UserID,
			"error", err,
		)
		return nil, err
	}

	return resp, nil
}

// GetSkillDemandAnalytics coordinates skill demand snapshot aggregation and forecasting via AI service.
func (o *Orchestrator) GetSkillDemandAnalytics(ctx context.Context, req models.SkillDemandAnalyticsRequest) (*models.SkillDemandAnalyticsResponse, error) {
	if !o.flags.AIEnabled || !o.flags.SkillAnalytics || o.client == nil {
		o.logger.DebugContext(ctx, "ai skill demand analytics bypassed", "period", req.Period)
		return nil, ErrAIBypassed
	}

	callCtx, cancel := context.WithTimeout(ctx, DefaultSkillAnalyticsTimeout)
	defer cancel()

	resp, err := o.client.GetSkillDemandAnalytics(callCtx, req)
	if err != nil {
		o.logger.WarnContext(ctx, "ai skill demand analytics failed",
			"period", req.Period,
			"error", err,
		)
		return nil, err
	}

	return resp, nil
}

// GenerateJobDraft coordinates AI-assisted job posting draft generation.
func (o *Orchestrator) GenerateJobDraft(ctx context.Context, req models.GenerateJobDraftRequest) (*models.GeneratedJobDraftResponse, error) {
	if !o.flags.AIEnabled || !o.flags.JobGeneration || o.client == nil {
		o.logger.DebugContext(ctx, "ai job generation bypassed", "idea", req.Idea)
		return nil, ErrAIBypassed
	}

	callCtx, cancel := context.WithTimeout(ctx, DefaultGenerationTimeout)
	defer cancel()

	resp, err := o.client.GenerateJobDraft(callCtx, req)
	if err != nil {
		o.logger.WarnContext(ctx, "ai job generation failed",
			"error", err,
		)
		return nil, err
	}

	return resp, nil
}



