package team

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service coordinates business logic and access control for project teams and invitations.
type Service struct {
	repo Repository
}

// NewService constructs a new team service instance.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ListMembers returns all active members for a project.
func (s *Service) ListMembers(ctx context.Context, projectID uuid.UUID) ([]ProjectMember, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("invalid project id")
	}
	return s.repo.ListMembers(ctx, projectID)
}

// GetMember retrieves a single active member from a project.
func (s *Service) GetMember(ctx context.Context, projectID uuid.UUID, memberID string) (*ProjectMember, error) {
	if projectID == uuid.Nil || strings.TrimSpace(memberID) == "" {
		return nil, ErrMemberNotFound
	}
	return s.repo.GetMember(ctx, projectID, memberID)
}

// AddMember directly adds an active member (useful for project creator initialization).
func (s *Service) AddMember(ctx context.Context, pm *ProjectMember) error {
	if pm == nil || pm.ProjectID == uuid.Nil || strings.TrimSpace(pm.MemberID) == "" {
		return errors.New("invalid project member payload")
	}
	if !pm.Role.IsValid() {
		return ErrInvalidRole
	}
	return s.repo.AddMember(ctx, pm)
}

// UpdateMemberRole modifies the role of an active project member.
// Invariants:
// - Caller must be a Lead.
// - Members cannot change their own role or promote themselves.
// - Leads cannot demote another lead without leadership transfer.
// - New role cannot be Lead via this method; leadership transfer must be used instead.
func (s *Service) UpdateMemberRole(ctx context.Context, projectID uuid.UUID, callerID, targetMemberID string, newRole Role) error {
	callerID = strings.TrimSpace(callerID)
	targetMemberID = strings.TrimSpace(targetMemberID)
	if callerID == "" || targetMemberID == "" {
		return ErrUnauthorizedAction
	}
	if !newRole.IsValid() {
		return ErrInvalidRole
	}
	if newRole == RoleLead {
		return ErrCannotDemoteLead
	}
	if callerID == targetMemberID {
		return ErrCannotChangeOwnRole
	}

	caller, err := s.repo.GetMember(ctx, projectID, callerID)
	if err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			return ErrUnauthorizedAction
		}
		return fmt.Errorf("failed to get caller member: %w", err)
	}
	if caller.Role != RoleLead {
		return ErrUnauthorizedAction
	}

	target, err := s.repo.GetMember(ctx, projectID, targetMemberID)
	if err != nil {
		return err
	}
	if target.Role == RoleLead {
		return ErrCannotDemoteLead
	}

	return s.repo.UpdateRole(ctx, projectID, targetMemberID, newRole)
}

// RemoveMember removes a member from the project.
// Invariants:
// - A member can remove themselves (leave the project), unless they are the last lead.
// - Only a Lead can remove other members.
// - The last lead cannot be removed.
func (s *Service) RemoveMember(ctx context.Context, projectID uuid.UUID, callerID, targetMemberID string) error {
	callerID = strings.TrimSpace(callerID)
	targetMemberID = strings.TrimSpace(targetMemberID)
	if callerID == "" || targetMemberID == "" {
		return ErrUnauthorizedAction
	}

	target, err := s.repo.GetMember(ctx, projectID, targetMemberID)
	if err != nil {
		return err
	}

	if callerID == targetMemberID {
		// Self-departure
		if target.Role == RoleLead {
			leads, err := s.repo.CountLeads(ctx, projectID)
			if err != nil {
				return fmt.Errorf("failed to count leads: %w", err)
			}
			if leads <= 1 {
				return ErrCannotRemoveLastLead
			}
		}
		return s.repo.RemoveMember(ctx, projectID, targetMemberID)
	}

	// Removing another member: caller must be a Lead
	caller, err := s.repo.GetMember(ctx, projectID, callerID)
	if err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			return ErrUnauthorizedAction
		}
		return fmt.Errorf("failed to get caller member: %w", err)
	}
	if caller.Role != RoleLead {
		return ErrUnauthorizedAction
	}

	if target.Role == RoleLead {
		leads, err := s.repo.CountLeads(ctx, projectID)
		if err != nil {
			return fmt.Errorf("failed to count leads: %w", err)
		}
		if leads <= 1 {
			return ErrCannotRemoveLastLead
		}
	}

	return s.repo.RemoveMember(ctx, projectID, targetMemberID)
}

// InviteMember issues an invitation for a user to join a project workspace.
// Invariants:
// - Caller must be a Lead or Maintainer.
// - Cannot invite oneself.
// - Proposed role cannot be Lead (must join first, then receive leadership transfer).
// - Target user cannot already be an active member.
func (s *Service) InviteMember(ctx context.Context, projectID uuid.UUID, callerID, inviteeID string, role Role) (*ProjectInvitation, error) {
	callerID = strings.TrimSpace(callerID)
	inviteeID = strings.TrimSpace(inviteeID)
	if callerID == "" || inviteeID == "" {
		return nil, ErrUnauthorizedAction
	}
	if callerID == inviteeID {
		return nil, ErrSelfInvitation
	}
	if !role.IsValid() {
		return nil, ErrInvalidRole
	}
	if role == RoleLead {
		return nil, ErrUnauthorizedAction
	}

	caller, err := s.repo.GetMember(ctx, projectID, callerID)
	if err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			return nil, ErrUnauthorizedAction
		}
		return nil, fmt.Errorf("failed to get caller member: %w", err)
	}
	if caller.Role != RoleLead && caller.Role != RoleMaintainer {
		return nil, ErrUnauthorizedAction
	}

	// Verify target is not already an active member
	existing, err := s.repo.GetMember(ctx, projectID, inviteeID)
	if err == nil && existing != nil && existing.Status == "active" {
		return nil, ErrAlreadyMember
	}

	inv := &ProjectInvitation{
		ID:           uuid.New(),
		ProjectID:    projectID,
		InviterID:    callerID,
		InviteeID:    inviteeID,
		ProposedRole: role,
		Status:       "pending",
		ExpiresAt:    time.Now().UTC().Add(14 * 24 * time.Hour),
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.repo.CreateInvitation(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

// AcceptInvitation processes an invitation acceptance and enrolls the user as a project member.
// Invariants:
// - Caller must be the invitee.
// - Invitation must exist and be in pending status.
// - Invitation must not be expired.
func (s *Service) AcceptInvitation(ctx context.Context, invitationID uuid.UUID, callerID string) error {
	callerID = strings.TrimSpace(callerID)
	if callerID == "" {
		return ErrUnauthorizedAction
	}

	inv, err := s.repo.GetInvitation(ctx, invitationID)
	if err != nil {
		return err
	}

	if inv.InviteeID != callerID {
		return ErrUnauthorizedAction
	}
	if inv.Status != "pending" {
		return ErrInvitationNotPending
	}
	if time.Now().UTC().After(inv.ExpiresAt) {
		_ = s.repo.UpdateInvitationStatus(ctx, invitationID, "expired")
		return ErrInvitationExpired
	}

	// Check if already active member
	existing, err := s.repo.GetMember(ctx, inv.ProjectID, callerID)
	if err == nil && existing != nil && existing.Status == "active" {
		_ = s.repo.UpdateInvitationStatus(ctx, invitationID, "accepted")
		return ErrAlreadyMember
	}

	pm := &ProjectMember{
		ID:        uuid.New(),
		ProjectID: inv.ProjectID,
		MemberID:  callerID,
		Role:      inv.ProposedRole,
		Status:    "active",
		JoinedAt:  time.Now().UTC(),
	}
	if err := s.repo.AddMember(ctx, pm); err != nil {
		return fmt.Errorf("failed to add accepted member: %w", err)
	}

	if err := s.repo.UpdateInvitationStatus(ctx, invitationID, "accepted"); err != nil {
		return fmt.Errorf("failed to update invitation status: %w", err)
	}

	return nil
}

// TransferLeadership transfers the project leadership to an active project member.
// Invariants:
// - Caller must be the active Lead.
// - Target user must be an active project member.
// - Caller cannot transfer leadership to themselves.
func (s *Service) TransferLeadership(ctx context.Context, projectID uuid.UUID, callerID, newLeadID string) error {
	callerID = strings.TrimSpace(callerID)
	newLeadID = strings.TrimSpace(newLeadID)
	if callerID == "" || newLeadID == "" {
		return ErrUnauthorizedAction
	}
	if callerID == newLeadID {
		return ErrInvalidLeadership
	}

	caller, err := s.repo.GetMember(ctx, projectID, callerID)
	if err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			return ErrUnauthorizedAction
		}
		return fmt.Errorf("failed to get caller member: %w", err)
	}
	if caller.Role != RoleLead {
		return ErrUnauthorizedAction
	}

	target, err := s.repo.GetMember(ctx, projectID, newLeadID)
	if err != nil {
		return err
	}
	if target.Status != "active" {
		return ErrMemberNotFound
	}

	return s.repo.TransferLeadershipTx(ctx, projectID, callerID, newLeadID)
}
