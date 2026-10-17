package team_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httpx"
	"github.com/lynk/backend/internal/team"
)

type mockTeamRepo struct {
	mu           sync.RWMutex
	members      map[string]*team.ProjectMember
	invitations  map[uuid.UUID]*team.ProjectInvitation
	projectOwner map[uuid.UUID]string
}

func newMockTeamRepo() *mockTeamRepo {
	return &mockTeamRepo{
		members:      make(map[string]*team.ProjectMember),
		invitations:  make(map[uuid.UUID]*team.ProjectInvitation),
		projectOwner: make(map[uuid.UUID]string),
	}
}

func (m *mockTeamRepo) memberKey(projectID uuid.UUID, memberID string) string {
	return fmt.Sprintf("%s:%s", projectID.String(), memberID)
}

func (m *mockTeamRepo) AddMember(ctx context.Context, pm *team.ProjectMember) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pm.ID == uuid.Nil {
		pm.ID = uuid.New()
	}
	if pm.JoinedAt.IsZero() {
		pm.JoinedAt = time.Now().UTC()
	}
	if pm.Status == "" {
		pm.Status = "active"
	}
	copy := *pm
	m.members[m.memberKey(pm.ProjectID, pm.MemberID)] = &copy
	return nil
}

func (m *mockTeamRepo) GetMember(ctx context.Context, projectID uuid.UUID, memberID string) (*team.ProjectMember, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pm, ok := m.members[m.memberKey(projectID, memberID)]
	if !ok || pm.Status != "active" {
		return nil, team.ErrMemberNotFound
	}
	copy := *pm
	return &copy, nil
}

func (m *mockTeamRepo) ListMembers(ctx context.Context, projectID uuid.UUID) ([]team.ProjectMember, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make([]team.ProjectMember, 0)
	for _, pm := range m.members {
		if pm.ProjectID == projectID && pm.Status == "active" {
			res = append(res, *pm)
		}
	}
	return res, nil
}

func (m *mockTeamRepo) UpdateRole(ctx context.Context, projectID uuid.UUID, memberID string, role team.Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pm, ok := m.members[m.memberKey(projectID, memberID)]
	if !ok || pm.Status != "active" {
		return team.ErrMemberNotFound
	}
	pm.Role = role
	return nil
}

func (m *mockTeamRepo) RemoveMember(ctx context.Context, projectID uuid.UUID, memberID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pm, ok := m.members[m.memberKey(projectID, memberID)]
	if !ok || pm.Status != "active" {
		return team.ErrMemberNotFound
	}
	pm.Status = "departed"
	now := time.Now().UTC()
	pm.DepartedAt = &now
	return nil
}

func (m *mockTeamRepo) CreateInvitation(ctx context.Context, inv *team.ProjectInvitation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.invitations {
		if existing.ProjectID == inv.ProjectID && existing.InviteeID == inv.InviteeID && existing.Status == "pending" {
			return team.ErrDuplicateInvitation
		}
	}

	if inv.ID == uuid.Nil {
		inv.ID = uuid.New()
	}
	if inv.CreatedAt.IsZero() {
		inv.CreatedAt = time.Now().UTC()
	}
	if inv.ExpiresAt.IsZero() {
		inv.ExpiresAt = time.Now().UTC().Add(14 * 24 * time.Hour)
	}
	if inv.Status == "" {
		inv.Status = "pending"
	}
	copy := *inv
	m.invitations[inv.ID] = &copy
	return nil
}

func (m *mockTeamRepo) GetInvitation(ctx context.Context, id uuid.UUID) (*team.ProjectInvitation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	inv, ok := m.invitations[id]
	if !ok {
		return nil, team.ErrInvitationNotFound
	}
	copy := *inv
	return &copy, nil
}

func (m *mockTeamRepo) UpdateInvitationStatus(ctx context.Context, id uuid.UUID, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inv, ok := m.invitations[id]
	if !ok {
		return team.ErrInvitationNotFound
	}
	inv.Status = status
	return nil
}

func (m *mockTeamRepo) CountLeads(ctx context.Context, projectID uuid.UUID) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, pm := range m.members {
		if pm.ProjectID == projectID && pm.Role == team.RoleLead && pm.Status == "active" {
			count++
		}
	}
	return count, nil
}

func (m *mockTeamRepo) TransferLeadershipTx(ctx context.Context, projectID uuid.UUID, currentLeadID, newLeadID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.members[m.memberKey(projectID, currentLeadID)]
	if !ok || current.Role != team.RoleLead || current.Status != "active" {
		return team.ErrUnauthorizedAction
	}

	target, ok := m.members[m.memberKey(projectID, newLeadID)]
	if !ok || target.Status != "active" {
		return team.ErrMemberNotFound
	}

	current.Role = team.RoleMaintainer
	target.Role = team.RoleLead
	m.projectOwner[projectID] = newLeadID
	return nil
}

func TestTeamRoleValidation(t *testing.T) {
	roles := []struct {
		role  team.Role
		valid bool
	}{
		{team.RoleLead, true},
		{team.RoleMaintainer, true},
		{team.RoleMember, true},
		{team.RoleViewer, true},
		{team.Role("admin"), false},
		{team.Role("guest"), false},
		{team.Role(""), false},
	}

	for _, tt := range roles {
		if got := tt.role.IsValid(); got != tt.valid {
			t.Errorf("role %q valid = %v, expected %v", tt.role, got, tt.valid)
		}
	}
}

func TestTeamService_RoleTransitionsAndLeadership(t *testing.T) {
	ctx := context.Background()
	repo := newMockTeamRepo()
	svc := team.NewService(repo)

	projectID := uuid.New()
	leadID := "user-lead-1"
	maintainerID := "user-maint-2"
	memberID := "user-member-3"

	// Seed team members
	_ = repo.AddMember(ctx, &team.ProjectMember{
		ID:        uuid.New(),
		ProjectID: projectID,
		MemberID:  leadID,
		Role:      team.RoleLead,
		Status:    "active",
	})
	_ = repo.AddMember(ctx, &team.ProjectMember{
		ID:        uuid.New(),
		ProjectID: projectID,
		MemberID:  maintainerID,
		Role:      team.RoleMaintainer,
		Status:    "active",
	})
	_ = repo.AddMember(ctx, &team.ProjectMember{
		ID:        uuid.New(),
		ProjectID: projectID,
		MemberID:  memberID,
		Role:      team.RoleMember,
		Status:    "active",
	})

	t.Run("Non-lead cannot update roles", func(t *testing.T) {
		err := svc.UpdateMemberRole(ctx, projectID, maintainerID, memberID, team.RoleViewer)
		if err != team.ErrUnauthorizedAction {
			t.Fatalf("expected ErrUnauthorizedAction, got %v", err)
		}
	})

	t.Run("Member cannot change own role", func(t *testing.T) {
		err := svc.UpdateMemberRole(ctx, projectID, leadID, leadID, team.RoleMaintainer)
		if err != team.ErrCannotChangeOwnRole {
			t.Fatalf("expected ErrCannotChangeOwnRole, got %v", err)
		}
	})

	t.Run("Cannot update role to Lead directly", func(t *testing.T) {
		err := svc.UpdateMemberRole(ctx, projectID, leadID, memberID, team.RoleLead)
		if err != team.ErrCannotDemoteLead {
			t.Fatalf("expected ErrCannotDemoteLead, got %v", err)
		}
	})

	t.Run("Invalid role returns ErrInvalidRole", func(t *testing.T) {
		err := svc.UpdateMemberRole(ctx, projectID, leadID, memberID, team.Role("superman"))
		if err != team.ErrInvalidRole {
			t.Fatalf("expected ErrInvalidRole, got %v", err)
		}
	})

	t.Run("Lead updates member role successfully", func(t *testing.T) {
		err := svc.UpdateMemberRole(ctx, projectID, leadID, memberID, team.RoleMaintainer)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		updated, err := repo.GetMember(ctx, projectID, memberID)
		if err != nil {
			t.Fatalf("failed to get member: %v", err)
		}
		if updated.Role != team.RoleMaintainer {
			t.Fatalf("expected role %s, got %s", team.RoleMaintainer, updated.Role)
		}
	})

	t.Run("Leadership transfer demotes current lead and promotes target", func(t *testing.T) {
		err := svc.TransferLeadership(ctx, projectID, leadID, maintainerID)
		if err != nil {
			t.Fatalf("failed to transfer leadership: %v", err)
		}

		oldLead, err := repo.GetMember(ctx, projectID, leadID)
		if err != nil {
			t.Fatalf("failed to get old lead: %v", err)
		}
		if oldLead.Role != team.RoleMaintainer {
			t.Fatalf("expected old lead role to be maintainer, got %s", oldLead.Role)
		}

		newLead, err := repo.GetMember(ctx, projectID, maintainerID)
		if err != nil {
			t.Fatalf("failed to get new lead: %v", err)
		}
		if newLead.Role != team.RoleLead {
			t.Fatalf("expected new lead role to be lead, got %s", newLead.Role)
		}
	})

	t.Run("Cannot transfer leadership to yourself", func(t *testing.T) {
		err := svc.TransferLeadership(ctx, projectID, maintainerID, maintainerID)
		if err != team.ErrInvalidLeadership {
			t.Fatalf("expected ErrInvalidLeadership, got %v", err)
		}
	})

	t.Run("Previous lead cannot transfer leadership anymore", func(t *testing.T) {
		err := svc.TransferLeadership(ctx, projectID, leadID, memberID)
		if err != team.ErrUnauthorizedAction {
			t.Fatalf("expected ErrUnauthorizedAction, got %v", err)
		}
	})
}

func TestTeamService_MemberRemovalRules(t *testing.T) {
	ctx := context.Background()
	repo := newMockTeamRepo()
	svc := team.NewService(repo)

	projectID := uuid.New()
	leadID := "only-lead"
	memberID := "regular-member"

	_ = repo.AddMember(ctx, &team.ProjectMember{
		ID:        uuid.New(),
		ProjectID: projectID,
		MemberID:  leadID,
		Role:      team.RoleLead,
		Status:    "active",
	})
	_ = repo.AddMember(ctx, &team.ProjectMember{
		ID:        uuid.New(),
		ProjectID: projectID,
		MemberID:  memberID,
		Role:      team.RoleMember,
		Status:    "active",
	})

	t.Run("Sole lead cannot remove themselves", func(t *testing.T) {
		err := svc.RemoveMember(ctx, projectID, leadID, leadID)
		if err != team.ErrCannotRemoveLastLead {
			t.Fatalf("expected ErrCannotRemoveLastLead, got %v", err)
		}
	})

	t.Run("Regular member cannot remove lead", func(t *testing.T) {
		err := svc.RemoveMember(ctx, projectID, memberID, leadID)
		if err != team.ErrUnauthorizedAction {
			t.Fatalf("expected ErrUnauthorizedAction, got %v", err)
		}
	})

	t.Run("Regular member can remove themselves", func(t *testing.T) {
		err := svc.RemoveMember(ctx, projectID, memberID, memberID)
		if err != nil {
			t.Fatalf("unexpected error on self departure: %v", err)
		}

		_, err = repo.GetMember(ctx, projectID, memberID)
		if err != team.ErrMemberNotFound {
			t.Fatalf("expected member to be departed, got %v", err)
		}
	})
}

func TestTeamService_InvitationsLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := newMockTeamRepo()
	svc := team.NewService(repo)

	projectID := uuid.New()
	leadID := "project-lead"
	maintID := "project-maint"
	inviteeID := "student-candidate"

	_ = repo.AddMember(ctx, &team.ProjectMember{
		ID:        uuid.New(),
		ProjectID: projectID,
		MemberID:  leadID,
		Role:      team.RoleLead,
		Status:    "active",
	})
	_ = repo.AddMember(ctx, &team.ProjectMember{
		ID:        uuid.New(),
		ProjectID: projectID,
		MemberID:  maintID,
		Role:      team.RoleMaintainer,
		Status:    "active",
	})

	t.Run("Cannot invite yourself", func(t *testing.T) {
		_, err := svc.InviteMember(ctx, projectID, leadID, leadID, team.RoleMember)
		if err != team.ErrSelfInvitation {
			t.Fatalf("expected ErrSelfInvitation, got %v", err)
		}
	})

	t.Run("Cannot invite with role Lead", func(t *testing.T) {
		_, err := svc.InviteMember(ctx, projectID, leadID, inviteeID, team.RoleLead)
		if err != team.ErrUnauthorizedAction {
			t.Fatalf("expected ErrUnauthorizedAction, got %v", err)
		}
	})

	t.Run("Maintainer can invite new member", func(t *testing.T) {
		inv, err := svc.InviteMember(ctx, projectID, maintID, inviteeID, team.RoleMember)
		if err != nil {
			t.Fatalf("failed to invite member: %v", err)
		}
		if inv.ProposedRole != team.RoleMember {
			t.Fatalf("expected role %s, got %s", team.RoleMember, inv.ProposedRole)
		}
		if inv.Status != "pending" {
			t.Fatalf("expected status pending, got %s", inv.Status)
		}
	})

	t.Run("Cannot send duplicate pending invitation", func(t *testing.T) {
		_, err := svc.InviteMember(ctx, projectID, leadID, inviteeID, team.RoleMember)
		if err != team.ErrDuplicateInvitation {
			t.Fatalf("expected ErrDuplicateInvitation, got %v", err)
		}
	})

	// Retrieve pending invitation
	var pendingInv *team.ProjectInvitation
	for _, inv := range repo.invitations {
		if inv.InviteeID == inviteeID {
			pendingInv = inv
			break
		}
	}
	if pendingInv == nil {
		t.Fatal("expected to find pending invitation")
	}

	t.Run("Wrong user cannot accept invitation", func(t *testing.T) {
		err := svc.AcceptInvitation(ctx, pendingInv.ID, "impostor-user")
		if err != team.ErrUnauthorizedAction {
			t.Fatalf("expected ErrUnauthorizedAction, got %v", err)
		}
	})

	t.Run("Invitee successfully accepts invitation", func(t *testing.T) {
		err := svc.AcceptInvitation(ctx, pendingInv.ID, inviteeID)
		if err != nil {
			t.Fatalf("failed to accept invitation: %v", err)
		}

		member, err := repo.GetMember(ctx, projectID, inviteeID)
		if err != nil {
			t.Fatalf("failed to get accepted member: %v", err)
		}
		if member.Role != team.RoleMember {
			t.Fatalf("expected role member, got %s", member.Role)
		}

		acceptedInv, _ := repo.GetInvitation(ctx, pendingInv.ID)
		if acceptedInv.Status != "accepted" {
			t.Fatalf("expected invitation status accepted, got %s", acceptedInv.Status)
		}
	})

	t.Run("Expired invitation cannot be accepted", func(t *testing.T) {
		secondInvitee := "another-student"
		inv, err := svc.InviteMember(ctx, projectID, leadID, secondInvitee, team.RoleViewer)
		if err != nil {
			t.Fatalf("failed to create invite: %v", err)
		}

		// Backdate expiration
		inv.ExpiresAt = time.Now().UTC().Add(-1 * time.Hour)
		repo.invitations[inv.ID] = inv

		err = svc.AcceptInvitation(ctx, inv.ID, secondInvitee)
		if err != team.ErrInvitationExpired {
			t.Fatalf("expected ErrInvitationExpired, got %v", err)
		}
	})
}

func TestTeamHandler_HTTPEndpoints(t *testing.T) {
	ctx := context.Background()
	repo := newMockTeamRepo()
	svc := team.NewService(repo)
	handler := team.NewHandler(svc, repo)

	projectID := uuid.New()
	leadID := "lead-http-user"
	memberID := "member-http-user"

	_ = repo.AddMember(ctx, &team.ProjectMember{
		ID:        uuid.New(),
		ProjectID: projectID,
		MemberID:  leadID,
		Role:      team.RoleLead,
		Status:    "active",
		Email:     "lead@university.edu",
		FirstName: "Alice",
		LastName:  "Lead",
	})
	_ = repo.AddMember(ctx, &team.ProjectMember{
		ID:        uuid.New(),
		ProjectID: projectID,
		MemberID:  memberID,
		Role:      team.RoleMember,
		Status:    "active",
		Email:     "member@university.edu",
		FirstName: "Bob",
		LastName:  "Member",
	})

	router := handler.Routes(httpx.RequireAuthFromContext())

	t.Run("GET /projects/{id}/members - Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/projects/%s/members", projectID), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("PATCH /projects/{id}/members/{memberId} - Role Update", func(t *testing.T) {
		body, _ := json.Marshal(team.UpdateRoleInput{Role: team.RoleMaintainer})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/projects/%s/members/%s", projectID, memberID), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		// Auth context as lead
		claims := &auth.UserClaims{UserID: leadID, Email: "lead@university.edu", EmailVerified: true}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /projects/{id}/invitations - Send Invitation", func(t *testing.T) {
		inviteeID := "new-candidate-http"
		body, _ := json.Marshal(team.InviteMemberInput{
			InviteeID:    inviteeID,
			ProposedRole: team.RoleMember,
		})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/projects/%s/invitations", projectID), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		claims := &auth.UserClaims{UserID: leadID, Email: "lead@university.edu", EmailVerified: true}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /projects/{id}/transfer-leadership - Transfer Leadership", func(t *testing.T) {
		body, _ := json.Marshal(team.TransferLeadershipInput{
			NewLeadID: memberID,
		})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/projects/%s/transfer-leadership", projectID), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		claims := &auth.UserClaims{UserID: leadID, Email: "lead@university.edu", EmailVerified: true}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("DELETE /projects/{id}/members/{memberId} - Remove Member", func(t *testing.T) {
		// Currently memberID is lead now due to previous transfer!
		// memberID removes leadID (which is now maintainer)
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/projects/%s/members/%s", projectID, leadID), nil)

		claims := &auth.UserClaims{UserID: memberID, Email: "member@university.edu", EmailVerified: true}
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}
