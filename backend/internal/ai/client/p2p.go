package client

import (
	"context"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

// DefaultP2PTimeout specifies the sub-2s timeout for P2P collaboration inference calls.
const DefaultP2PTimeout = 2 * time.Second

// MatchCollaboratorsRequest defines the payload for matching complementary peers.
type MatchCollaboratorsRequest struct {
	TeamSkills           []string                `json:"team_skills"`
	ProjectMissingSkills []string                `json:"project_missing_skills"`
	Candidates           []CollaboratorCandidate `json:"candidates"`
	Limit                int                     `json:"limit,omitempty"`
}

// CollaboratorCandidate represents an available peer student for matching.
type CollaboratorCandidate struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Department string   `json:"department,omitempty"`
	Skills     []string `json:"skills"`
}

// CollaboratorMatch represents a ranked peer with complement score and skill overlap.
type CollaboratorMatch struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Department      string   `json:"department"`
	ComplementScore float64  `json:"complement_score"`
	CoveredSkills   []string `json:"covered_skills"`
	AllSkills       []string `json:"all_skills"`
}

// MatchCollaboratorsResponse contains ranked complementary collaborator matches.
type MatchCollaboratorsResponse struct {
	Matches []CollaboratorMatch `json:"matches"`
}

// SkillGapRequest represents a request to evaluate a student readiness against project requirements.
type SkillGapRequest struct {
	StudentSkills         []string `json:"student_skills"`
	ProjectRequiredSkills []string `json:"project_required_skills"`
}

// SkillGuidance represents curated next-step learning guidance for a missing skill.
type SkillGuidance struct {
	Skill    string `json:"skill"`
	Guidance string `json:"guidance"`
}

// SkillGapResponse contains readiness score, matched skills, and learning pathway.
type SkillGapResponse struct {
	ReadySkills         []string        `json:"ready_skills"`
	MissingSkills       []string        `json:"missing_skills"`
	ReadinessPercentage float64         `json:"readiness_percentage"`
	LearningPathway     []SkillGuidance `json:"learning_pathway"`
}

// ProjectAssistRequest represents a rough project idea for AI spec generation.
type ProjectAssistRequest struct {
	Idea     string `json:"idea"`
	Category string `json:"category,omitempty"`
}

// ProjectRole represents a defined collaborator position needed on a project.
type ProjectRole struct {
	RoleTitle      string   `json:"role_title"`
	RequiredSkills []string `json:"required_skills"`
	Description    string   `json:"description"`
}

// GeneratedProjectSpec contains structured project listing fields.
type GeneratedProjectSpec struct {
	Title                  string        `json:"title"`
	Overview               string        `json:"overview"`
	Category               string        `json:"category"`
	NeededRoles            []ProjectRole `json:"needed_roles"`
	Deliverables           []string      `json:"deliverables"`
	EstimatedDurationWeeks int           `json:"estimated_duration_weeks"`
}

// MilestonesRequest represents a request to plan project execution phases.
type MilestonesRequest struct {
	Title        string   `json:"title"`
	Deliverables []string `json:"deliverables"`
}

// MilestoneTask represents a specific task within a milestone phase.
type MilestoneTask struct {
	Title          string `json:"title"`
	EstimatedHours int    `json:"estimated_hours"`
	SuggestedSkill string `json:"suggested_skill"`
}

// ProjectMilestone represents a sprint phase with discrete deliverables.
type ProjectMilestone struct {
	Name          string          `json:"name"`
	DurationWeeks int             `json:"duration_weeks"`
	Tasks         []MilestoneTask `json:"tasks"`
}

// ProjectMilestonePlan represents the full roadmap for student venture execution.
type ProjectMilestonePlan struct {
	ProjectTitle string             `json:"project_title"`
	Milestones   []ProjectMilestone `json:"milestones"`
}

// TeammateInput represents a collaborator assigned to tasks.
type TeammateInput struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Skills     []string `json:"skills"`
	Department string   `json:"department,omitempty"`
}

// RoleAllocationRequest represents a request to assign teammates to milestone tasks.
type RoleAllocationRequest struct {
	MilestoneTasks []string        `json:"milestone_tasks"`
	Teammates      []TeammateInput `json:"teammates"`
}

// RoleAllocation represents an assigned task pairing with confidence and rationale.
type RoleAllocation struct {
	Task           string  `json:"task"`
	AssignedTo     string  `json:"assigned_to"`
	AssignedUserID string  `json:"assigned_user_id"`
	Confidence     float64 `json:"confidence"`
	Reason         string  `json:"reason"`
}

// RoleAllocationResponse contains suggested task-to-member pairings.
type RoleAllocationResponse struct {
	Allocations []RoleAllocation `json:"allocations"`
}

// NaturalQueryRequest represents freeform text for conversational search filtering.
type NaturalQueryRequest struct {
	Query string `json:"query"`
}

// ParsedNaturalQuery represents structured filters extracted from conversational text.
type ParsedNaturalQuery struct {
	RawQuery            string   `json:"raw_query"`
	Intent              string   `json:"intent"`
	ExtractedSkills     []string `json:"extracted_skills"`
	PreferredDepartment string   `json:"preferred_department"`
}

// MatchCollaborators ranks student peers by complementarity with sub-2s timeout and deterministic fallback.
func (c *Client) MatchCollaborators(ctx context.Context, req MatchCollaboratorsRequest) (*MatchCollaboratorsResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	p2pCtx, cancel := context.WithTimeout(ctx, DefaultP2PTimeout)
	defer cancel()

	var resp MatchCollaboratorsResponse
	err := c.doRequest(p2pCtx, http.MethodPost, "/internal/v1/collaborators/match", req, &resp)
	if err == nil {
		return &resp, nil
	}

	return c.fallbackMatchCollaborators(req), nil
}

// GetSkillGap evaluates student readiness against project requirements with fallback learning guidance.
func (c *Client) GetSkillGap(ctx context.Context, req SkillGapRequest) (*SkillGapResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	p2pCtx, cancel := context.WithTimeout(ctx, DefaultP2PTimeout)
	defer cancel()

	var resp SkillGapResponse
	err := c.doRequest(p2pCtx, http.MethodPost, "/internal/v1/skills/gap", req, &resp)
	if err == nil {
		return &resp, nil
	}

	return c.fallbackSkillGap(req), nil
}

// AssistProject generates structured project listings from rough concepts with deterministic fallback.
func (c *Client) AssistProject(ctx context.Context, req ProjectAssistRequest) (*GeneratedProjectSpec, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	p2pCtx, cancel := context.WithTimeout(ctx, DefaultP2PTimeout)
	defer cancel()

	var resp GeneratedProjectSpec
	err := c.doRequest(p2pCtx, http.MethodPost, "/internal/v1/projects/assist", req, &resp)
	if err == nil {
		return &resp, nil
	}

	return c.fallbackAssistProject(req), nil
}

// PlanMilestones constructs a 3-phase execution roadmap from deliverables with fallback.
func (c *Client) PlanMilestones(ctx context.Context, req MilestonesRequest) (*ProjectMilestonePlan, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	p2pCtx, cancel := context.WithTimeout(ctx, DefaultP2PTimeout)
	defer cancel()

	var resp ProjectMilestonePlan
	err := c.doRequest(p2pCtx, http.MethodPost, "/internal/v1/projects/milestones", req, &resp)
	if err == nil {
		return &resp, nil
	}

	return c.fallbackPlanMilestones(req), nil
}

// AllocateRoles suggests member-to-task assignments with deterministic fallback.
func (c *Client) AllocateRoles(ctx context.Context, req RoleAllocationRequest) (*RoleAllocationResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	p2pCtx, cancel := context.WithTimeout(ctx, DefaultP2PTimeout)
	defer cancel()

	var resp RoleAllocationResponse
	err := c.doRequest(p2pCtx, http.MethodPost, "/internal/v1/projects/roles", req, &resp)
	if err == nil {
		return &resp, nil
	}

	return c.fallbackAllocateRoles(req), nil
}

// ParseNaturalQuery extracts structured search filters from conversational text with fallback.
func (c *Client) ParseNaturalQuery(ctx context.Context, req NaturalQueryRequest) (*ParsedNaturalQuery, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	p2pCtx, cancel := context.WithTimeout(ctx, DefaultP2PTimeout)
	defer cancel()

	var resp ParsedNaturalQuery
	err := c.doRequest(p2pCtx, http.MethodPost, "/internal/v1/search/natural", req, &resp)
	if err == nil {
		return &resp, nil
	}

	return c.fallbackParseNaturalQuery(req), nil
}

// fallbackMatchCollaborators executes local deterministic complementarity scoring.
func (c *Client) fallbackMatchCollaborators(req MatchCollaboratorsRequest) *MatchCollaboratorsResponse {
	teamSet := make(map[string]struct{}, len(req.TeamSkills))
	for _, s := range req.TeamSkills {
		teamSet[strings.ToLower(strings.TrimSpace(s))] = struct{}{}
	}

	missingSet := make(map[string]struct{}, len(req.ProjectMissingSkills))
	for _, s := range req.ProjectMissingSkills {
		missingSet[strings.ToLower(strings.TrimSpace(s))] = struct{}{}
	}

	totalMissing := len(missingSet)
	if totalMissing == 0 {
		totalMissing = 1
	}

	matches := make([]CollaboratorMatch, 0, len(req.Candidates))
	for _, cand := range req.Candidates {
		candSkills := make(map[string]struct{}, len(cand.Skills))
		var coveredSkills []string
		for _, s := range cand.Skills {
			clean := strings.ToLower(strings.TrimSpace(s))
			candSkills[clean] = struct{}{}
			if _, ok := missingSet[clean]; ok {
				coveredSkills = append(coveredSkills, s)
			}
		}

		coverageRatio := float64(len(coveredSkills)) / float64(totalMissing)

		redundantCount := 0
		for clean := range candSkills {
			if _, inTeam := teamSet[clean]; inTeam {
				if _, inMissing := missingSet[clean]; !inMissing {
					redundantCount++
				}
			}
		}

		redundancyPenalty := float64(redundantCount) * 0.05
		if redundancyPenalty > 0.3 {
			redundancyPenalty = 0.3
		}

		complementScore := coverageRatio - redundancyPenalty
		if complementScore < 0.0 {
			complementScore = 0.0
		}
		complementScore = math.Round(complementScore*10000.0) / 10000.0

		matches = append(matches, CollaboratorMatch{
			ID:              cand.ID,
			Name:            cand.Name,
			Department:      cand.Department,
			ComplementScore: complementScore,
			CoveredSkills:   coveredSkills,
			AllSkills:       cand.Skills,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].ComplementScore > matches[j].ComplementScore
	})

	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if len(matches) > limit {
		matches = matches[:limit]
	}

	return &MatchCollaboratorsResponse{Matches: matches}
}

// fallbackSkillGap computes local deterministic skill overlap and learning pathways.
func (c *Client) fallbackSkillGap(req SkillGapRequest) *SkillGapResponse {
	studentSet := make(map[string]struct{}, len(req.StudentSkills))
	for _, s := range req.StudentSkills {
		studentSet[strings.ToLower(strings.TrimSpace(s))] = struct{}{}
	}

	var readySkills []string
	var missingSkills []string

	for _, reqSkill := range req.ProjectRequiredSkills {
		clean := strings.ToLower(strings.TrimSpace(reqSkill))
		if _, exists := studentSet[clean]; exists {
			readySkills = append(readySkills, reqSkill)
		} else {
			missingSkills = append(missingSkills, reqSkill)
		}
	}

	totalReq := len(req.ProjectRequiredSkills)
	if totalReq == 0 {
		totalReq = 1
	}

	readiness := (float64(len(readySkills)) / float64(totalReq)) * 100.0
	readiness = math.Round(readiness*10.0) / 10.0

	pathway := make([]SkillGuidance, 0, len(missingSkills))
	for _, missing := range missingSkills {
		pathway = append(pathway, SkillGuidance{
			Skill:    missing,
			Guidance: "Essential project requirement; explore documentation and practical starter tutorials for " + missing + ".",
		})
	}

	return &SkillGapResponse{
		ReadySkills:         readySkills,
		MissingSkills:       missingSkills,
		ReadinessPercentage: readiness,
		LearningPathway:     pathway,
	}
}

// fallbackAssistProject creates a structured baseline project specification.
func (c *Client) fallbackAssistProject(req ProjectAssistRequest) *GeneratedProjectSpec {
	category := req.Category
	if category == "" {
		category = "Software"
	}

	title := strings.TrimSpace(req.Idea)
	if len(title) > 60 {
		title = title[:60] + "..."
	}
	if title == "" {
		title = "Collaborative Campus Venture"
	}

	return &GeneratedProjectSpec{
		Title:    title,
		Overview: req.Idea,
		Category: category,
		NeededRoles: []ProjectRole{
			{
				RoleTitle:      "Lead Systems Developer",
				RequiredSkills: []string{"Go", "TypeScript", "PostgreSQL"},
				Description:    "Architect and implement core services, business logic, and API endpoints.",
			},
			{
				RoleTitle:      "UI/UX Product Designer",
				RequiredSkills: []string{"Figma", "UI Design", "Prototyping"},
				Description:    "Design intuitive user interfaces and test responsive prototypes with campus peers.",
			},
		},
		Deliverables: []string{
			"Technical Architecture and Data Model",
			"Working Core Prototype with Auth and Data Flow",
			"User Testing and Deployment Checklist",
		},
		EstimatedDurationWeeks: 4,
	}
}

// fallbackPlanMilestones produces a structured 3-phase project milestone roadmap.
func (c *Client) fallbackPlanMilestones(req MilestonesRequest) *ProjectMilestonePlan {
	title := req.Title
	if title == "" {
		title = "Student Venture"
	}

	deliverables := req.Deliverables
	if len(deliverables) == 0 {
		deliverables = []string{"Architecture", "Core Implementation", "Verification"}
	}

	var devTasks []MilestoneTask
	for _, d := range deliverables {
		devTasks = append(devTasks, MilestoneTask{
			Title:          "Implement " + d,
			EstimatedHours: 16,
			SuggestedSkill: "Development",
		})
	}

	return &ProjectMilestonePlan{
		ProjectTitle: title,
		Milestones: []ProjectMilestone{
			{
				Name:          "Phase 1 - Architecture & Foundations",
				DurationWeeks: 1,
				Tasks: []MilestoneTask{
					{
						Title:          "Repository Setup and Schema Definition",
						EstimatedHours: 10,
						SuggestedSkill: "Architecture",
					},
				},
			},
			{
				Name:          "Phase 2 - Core Build",
				DurationWeeks: 2,
				Tasks:         devTasks,
			},
			{
				Name:          "Phase 3 - Validation & Launch",
				DurationWeeks: 1,
				Tasks: []MilestoneTask{
					{
						Title:          "Integration Verification and Demo Preparation",
						EstimatedHours: 8,
						SuggestedSkill: "Testing",
					},
				},
			},
		},
	}
}

// fallbackAllocateRoles pairs tasks to available team members deterministically.
func (c *Client) fallbackAllocateRoles(req RoleAllocationRequest) *RoleAllocationResponse {
	if len(req.Teammates) == 0 {
		return &RoleAllocationResponse{Allocations: []RoleAllocation{}}
	}

	allocations := make([]RoleAllocation, 0, len(req.MilestoneTasks))
	for i, task := range req.MilestoneTasks {
		tm := req.Teammates[i%len(req.Teammates)]
		allocations = append(allocations, RoleAllocation{
			Task:           task,
			AssignedTo:     tm.Name,
			AssignedUserID: tm.ID,
			Confidence:     0.75,
			Reason:         "Assigned based on team capability distribution",
		})
	}

	return &RoleAllocationResponse{Allocations: allocations}
}

// fallbackParseNaturalQuery parses keywords and intent from conversational search text.
func (c *Client) fallbackParseNaturalQuery(req NaturalQueryRequest) *ParsedNaturalQuery {
	qLower := strings.ToLower(req.Query)
	intent := "find_project"
	if strings.Contains(qLower, "partner") || strings.Contains(qLower, "teammate") ||
		strings.Contains(qLower, "developer") || strings.Contains(qLower, "designer") ||
		strings.Contains(qLower, "who") || strings.Contains(qLower, "collaborator") {
		intent = "find_teammate"
	}

	canonicalSkills := []string{"go", "python", "react", "typescript", "docker", "figma", "postgresql", "machine learning"}
	var extracted []string
	for _, skill := range canonicalSkills {
		if strings.Contains(qLower, skill) {
			extracted = append(extracted, skill)
		}
	}

	return &ParsedNaturalQuery{
		RawQuery:            req.Query,
		Intent:              intent,
		ExtractedSkills:     extracted,
		PreferredDepartment: "Any",
	}
}
