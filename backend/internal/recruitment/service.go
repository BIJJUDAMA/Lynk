package recruitment

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/project"
	"github.com/lynk/backend/internal/sanitizer"
	"github.com/lynk/backend/internal/team"
)

// ProjectReader provides read-only project access to verify project existence and recruitment status.
type ProjectReader interface {
	GetProjectByID(ctx context.Context, id uuid.UUID) (*project.Project, error)
}

// MembershipReader provides read-only membership verification for reviewer authorization.
type MembershipReader interface {
	GetMember(ctx context.Context, projectID uuid.UUID, memberID string) (*team.ProjectMember, error)
}

// Service manages recruitment domain rules, application lifecycles, and atomic team admissions.
type Service struct {
	repo             Repository
	projectReader    ProjectReader
	membershipReader MembershipReader
	leaser           CapacityLeaser
}

// NewService constructs a recruitment domain service.
func NewService(repo Repository, projectReader ProjectReader, membershipReader MembershipReader) *Service {
	return &Service{
		repo:             repo,
		projectReader:    projectReader,
		membershipReader: membershipReader,
	}
}

// WithCapacityLeaser attaches a front-line capacity leaser to the service.
func (s *Service) WithCapacityLeaser(leaser CapacityLeaser) *Service {
	s.leaser = leaser
	return s
}

func maxMembersFromSettings(settings map[string]any) int {
	if settings == nil {
		return 0
	}

	value, ok := settings["max_members"]
	if !ok || value == nil {
		return 0
	}

	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

// authorizeReviewer checks whether the user is an active Lead, Maintainer, or Project Owner.
func (s *Service) authorizeReviewer(ctx context.Context, projectID uuid.UUID, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ErrUnauthorized
	}

	if s.projectReader != nil {
		proj, err := s.projectReader.GetProjectByID(ctx, projectID)
		if err == nil && proj != nil && proj.OwnerMemberID == userID {
			return nil
		}
	}

	if s.membershipReader != nil {
		member, err := s.membershipReader.GetMember(ctx, projectID, userID)
		if err != nil {
			if errors.Is(err, team.ErrMemberNotFound) {
				return ErrForbidden
			}
			return fmt.Errorf("failed to verify reviewer membership: %w", err)
		}
		if member.Status != "active" || (member.Role != team.RoleLead && member.Role != team.RoleMaintainer) {
			return ErrForbidden
		}
		return nil
	}

	return nil
}

// SubmitApplication validates requirements and registers a member's application to join a project.
func (s *Service) SubmitApplication(ctx context.Context, claims *auth.UserClaims, projectID uuid.UUID, input SubmitApplicationInput) (*Application, error) {
	if claims == nil || strings.TrimSpace(claims.UserID) == "" {
		return nil, ErrUnauthorized
	}

	// 1. Enforce verified institutional email gate
	if err := auth.CheckEmailVerified(claims); err != nil {
		return nil, ErrEmailNotVerified
	}

	if projectID == uuid.Nil {
		return nil, errors.New("invalid project id")
	}

	// 2. Validate statement length
	stmt := sanitizer.SanitizeStrict(input.Statement)
	if stmt == "" || len([]rune(stmt)) > 5000 {
		return nil, ErrInvalidStatement
	}

	// 3. Verify project exists and recruitment is not closed
	if s.projectReader != nil {
		proj, err := s.projectReader.GetProjectByID(ctx, projectID)
		if err != nil {
			if errors.Is(err, project.ErrProjectNotFound) {
				return nil, project.ErrProjectNotFound
			}
			return nil, fmt.Errorf("failed to lookup project: %w", err)
		}
		if proj == nil {
			return nil, project.ErrProjectNotFound
		}
		if proj.RecruitmentStatus == project.RecruitmentClosed {
			return nil, ErrRecruitmentClosed
		}
	}

	// 4. Verify applicant is not already an active team member
	if s.membershipReader != nil {
		member, err := s.membershipReader.GetMember(ctx, projectID, claims.UserID)
		if err == nil && member != nil && member.Status == "active" {
			return nil, ErrAlreadyApplied
		}
	}

	// 5. Check required questions
	questions, err := s.repo.ListQuestions(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list project questions: %w", err)
	}

	answersByQuestion := make(map[uuid.UUID]string)
	for _, ans := range input.Answers {
		if ans.QuestionID != uuid.Nil && strings.TrimSpace(ans.AnswerText) != "" {
			answersByQuestion[ans.QuestionID] = sanitizer.SanitizeStrict(ans.AnswerText)
		}
	}

	for _, q := range questions {
		ansText, ok := answersByQuestion[q.ID]
		if q.Required && (!ok || strings.TrimSpace(ansText) == "") {
			return nil, ErrMissingRequiredAnswer
		}
	}

	appID := uuid.New()
	now := time.Now().UTC()

	var answersToSave []ApplicationAnswer
	for qID, text := range answersByQuestion {
		answersToSave = append(answersToSave, ApplicationAnswer{
			ID:            uuid.New(),
			ApplicationID: appID,
			QuestionID:    qID,
			AnswerText:    text,
			CreatedAt:     now,
		})
	}

	app := &Application{
		ID:             appID,
		ProjectID:      projectID,
		ApplicantID:    claims.UserID,
		Status:         StatusSubmitted,
		Statement:      stmt,
		ResumeAssetKey: input.ResumeAssetKey,
		ReviewerNotes:  "",
		CreatedAt:      now,
		UpdatedAt:      now,
		Answers:        answersToSave,
	}

	if err := s.repo.CreateApplication(ctx, app, answersToSave); err != nil {
		return nil, err
	}

	return app, nil
}

// GetApplicationByID retrieves an application with access control:
// Allowed if caller is the applicant or an authorized project reviewer.
func (s *Service) GetApplicationByID(ctx context.Context, claims *auth.UserClaims, appID uuid.UUID) (*Application, error) {
	if claims == nil || strings.TrimSpace(claims.UserID) == "" {
		return nil, ErrUnauthorized
	}

	app, err := s.repo.GetApplicationByID(ctx, appID)
	if err != nil {
		return nil, err
	}

	if app.ApplicantID == claims.UserID {
		return app, nil
	}

	if err := s.authorizeReviewer(ctx, app.ProjectID, claims.UserID); err != nil {
		return nil, ErrForbidden
	}

	return app, nil
}

// ListProjectApplications retrieves all applications for a project. Requires reviewer authorization.
func (s *Service) ListProjectApplications(ctx context.Context, claims *auth.UserClaims, projectID uuid.UUID, statusFilter *ApplicationStatus) ([]Application, error) {
	if claims == nil || strings.TrimSpace(claims.UserID) == "" {
		return nil, ErrUnauthorized
	}

	if err := s.authorizeReviewer(ctx, projectID, claims.UserID); err != nil {
		return nil, err
	}

	return s.repo.ListApplications(ctx, projectID, statusFilter)
}

// ListMyApplications retrieves all applications submitted by the caller.
func (s *Service) ListMyApplications(ctx context.Context, claims *auth.UserClaims) ([]Application, error) {
	if claims == nil || strings.TrimSpace(claims.UserID) == "" {
		return nil, ErrUnauthorized
	}

	return s.repo.ListMyApplications(ctx, claims.UserID)
}

// ShortlistApplication transitions an application to shortlisted. Requires reviewer authorization.
func (s *Service) ShortlistApplication(ctx context.Context, claims *auth.UserClaims, appID uuid.UUID, reviewerNotes string) error {
	if claims == nil || strings.TrimSpace(claims.UserID) == "" {
		return ErrUnauthorized
	}

	app, err := s.repo.GetApplicationByID(ctx, appID)
	if err != nil {
		return err
	}

	if err := s.authorizeReviewer(ctx, app.ProjectID, claims.UserID); err != nil {
		return err
	}

	if app.Status != StatusSubmitted {
		return ErrAlreadyDecided
	}

	return s.repo.UpdateApplicationStatus(ctx, appID, StatusShortlisted, sanitizer.SanitizeStrict(reviewerNotes), claims.UserID)
}

// RejectApplication transitions an application to rejected. Requires reviewer authorization.
func (s *Service) RejectApplication(ctx context.Context, claims *auth.UserClaims, appID uuid.UUID, reviewerNotes string) error {
	if claims == nil || strings.TrimSpace(claims.UserID) == "" {
		return ErrUnauthorized
	}

	app, err := s.repo.GetApplicationByID(ctx, appID)
	if err != nil {
		return err
	}

	if err := s.authorizeReviewer(ctx, app.ProjectID, claims.UserID); err != nil {
		return err
	}

	if app.Status == StatusAccepted || app.Status == StatusRejected || app.Status == StatusWithdrawn {
		return ErrAlreadyDecided
	}

	return s.repo.UpdateApplicationStatus(ctx, appID, StatusRejected, sanitizer.SanitizeStrict(reviewerNotes), claims.UserID)
}

// AcceptApplication accepts an application and atomically inserts the member into the project team.
// Requires reviewer authorization. Zero contracts created.
func (s *Service) AcceptApplication(ctx context.Context, claims *auth.UserClaims, appID uuid.UUID) error {
	if claims == nil || strings.TrimSpace(claims.UserID) == "" {
		return ErrUnauthorized
	}

	app, err := s.repo.GetApplicationByID(ctx, appID)
	if err != nil {
		return err
	}

	if err := s.authorizeReviewer(ctx, app.ProjectID, claims.UserID); err != nil {
		return err
	}

	if app.Status == StatusAccepted || app.Status == StatusRejected || app.Status == StatusWithdrawn {
		return ErrAlreadyDecided
	}

	if s.leaser != nil && s.projectReader != nil {
		proj, err := s.projectReader.GetProjectByID(ctx, app.ProjectID)
		if err == nil && proj != nil {
			if maxMembers := maxMembersFromSettings(proj.Settings); maxMembers > 0 {
				acquired, err := s.leaser.AcquireLease(ctx, app.ProjectID, maxMembers)
				if err != nil {
					return fmt.Errorf("failed to acquire capacity lease: %w", err)
				}
				if !acquired {
					return ErrTeamCapacityExceeded
				}

				if err := s.repo.AcceptApplicationTx(ctx, appID, claims.UserID); err != nil {
					releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
					defer cancel()
					_ = s.leaser.ReleaseLease(releaseCtx, app.ProjectID)
					return err
				}
				return nil
			}
		}
	}

	return s.repo.AcceptApplicationTx(ctx, appID, claims.UserID)
}

// WithdrawApplication allows an applicant to withdraw their application while it is submitted or shortlisted.
func (s *Service) WithdrawApplication(ctx context.Context, claims *auth.UserClaims, appID uuid.UUID) error {
	if claims == nil || strings.TrimSpace(claims.UserID) == "" {
		return ErrUnauthorized
	}

	app, err := s.repo.GetApplicationByID(ctx, appID)
	if err != nil {
		return err
	}

	if app.ApplicantID != claims.UserID {
		return ErrForbidden
	}

	if app.Status != StatusSubmitted && app.Status != StatusShortlisted {
		return ErrCannotWithdraw
	}

	return s.repo.WithdrawApplication(ctx, appID, claims.UserID)
}

// ListQuestions retrieves questions configured for a project.
func (s *Service) ListQuestions(ctx context.Context, projectID uuid.UUID) ([]ApplicationQuestion, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("invalid project id")
	}
	return s.repo.ListQuestions(ctx, projectID)
}

// CreateQuestion adds a new recruitment question to a project. Requires reviewer authorization.
func (s *Service) CreateQuestion(ctx context.Context, claims *auth.UserClaims, projectID uuid.UUID, input CreateQuestionInput) (*ApplicationQuestion, error) {
	if claims == nil || strings.TrimSpace(claims.UserID) == "" {
		return nil, ErrUnauthorized
	}

	if err := s.authorizeReviewer(ctx, projectID, claims.UserID); err != nil {
		return nil, err
	}

	text := sanitizer.SanitizeStrict(input.QuestionText)
	if text == "" || len([]rune(text)) > 1000 {
		return nil, ErrInvalidQuestionText
	}

	q := &ApplicationQuestion{
		ID:           uuid.New(),
		ProjectID:    projectID,
		QuestionText: text,
		HelpText:     sanitizer.SanitizeStrict(input.HelpText),
		Required:     input.Required,
		Position:     input.Position,
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.repo.CreateQuestion(ctx, q); err != nil {
		return nil, err
	}

	return q, nil
}
